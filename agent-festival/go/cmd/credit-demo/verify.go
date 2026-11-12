package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openjiuwen/agent-festival/go/internal/evidlog"
	"github.com/openjiuwen/agent-festival/go/internal/replay"
)

type Check struct {
	Name   string   `json:"name"`
	OK     bool     `json:"ok"`
	Detail string   `json:"detail"`
	Items  []string `json:"items,omitempty"`
}

type Report struct {
	RunID   string  `json:"run_id"`
	OK      bool    `json:"ok"`
	Checks  []Check `json:"checks"`
	Tamper  string  `json:"tamper,omitempty"`
	Summary string  `json:"summary"`
}

// Verify re-derives every claim of a finished run from the durable record
// with the festival checkers — nothing is taken from the executor's word.
func (s *Service) Verify(runID string, tamper bool) (*Report, error) {
	r, p, ok := s.RunByID(runID)
	if !ok || p == nil {
		return nil, fmt.Errorf("unknown run %s", runID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rep := &Report{RunID: runID, OK: true}
	add := func(c Check) {
		rep.Checks = append(rep.Checks, c)
		rep.OK = rep.OK && c.OK
	}

	// 1. every contract the plan uses is signed by the publisher and intact
	sig := Check{Name: "契约签名", OK: true}
	seen := map[string]bool{}
	for _, a := range p.Agents {
		if seen[a.Expert] {
			continue
		}
		seen[a.Expert] = true
		if errs := s.cat.Verify(a.Expert); len(errs) > 0 {
			sig.OK = false
			sig.Items = append(sig.Items, a.Expert+"："+strings.Join(errs, "; "))
		} else {
			sig.Items = append(sig.Items, fmt.Sprintf("%s ✓ %s", expertByID(a.Expert).Name, clip(s.cat.Hashes[a.Expert], 23)))
		}
	}
	sig.Detail = fmt.Sprintf("%d 份能力契约验签（ES256，发布者密钥）", len(seen))
	add(sig)

	// 2. what ran is what was approved
	h := planHash(p, s.cat.Hashes)
	add(Check{Name: "编排单一致", OK: h == r.PlanHash && h == p.Hash,
		Detail: fmt.Sprintf("批准时 %s；现在重算 %s；执行器按同一编排单摘要 %s 重新编译后才开跑", clip(r.PlanHash, 23), clip(h, 23), clip(p.SpecDigest, 23))})

	// 3. the composed contract is admissible for the approver
	pc := precheck(p)
	pcItems := []string{}
	if pc.Composed != nil {
		if eff, ok := pc.Composed["effects"]; ok {
			pcItems = append(pcItems, "合成效果："+compact(eff))
		}
		if b, ok := pc.Composed["bounds"]; ok {
			pcItems = append(pcItems, "合成上限："+compact(b))
		}
	}
	pcItems = append(pcItems, pc.Errors...)
	pcItems = append(pcItems, pc.Rules...)
	add(Check{Name: "规约准入（compose）", OK: pc.OK, Detail: "各节点权限 ⊆ 客户经理持有权限，预算按串并行汇总后在上限内，尽调规约逐条满足", Items: pcItems})

	// 4. each investigation read exactly what its contract declares
	cov := Check{Name: "证据覆盖（evidlog）", OK: true, Detail: "每个调查步骤实际查询的记录，与它的契约声明逐项对照"}
	for _, id := range r.Order {
		st := r.Steps[id]
		e := expertByID(st.Expert)
		if (e.Kind != "investigate" && e.Kind != "blackbox") || st.Status != "done" {
			continue
		}
		l, err := s.evidLog(runID, id)
		if err != nil {
			cov.OK = false
			cov.Items = append(cov.Items, id+"：证据日志打不开 "+err.Error())
			continue
		}
		if e.Kind == "blackbox" {
			// The inside of a black box is not declared ahead: check instead
			// that it read its case's list and that every read stayed inside
			// the contract (out-of-scope calls were refused and show up as
			// violations below).
			listed := declaredSelector("get_related_parties", r.CaseID)
			okList, inScope := false, true
			for _, en := range l.Entries() {
				if en.Selector == listed {
					okList = true
				}
				if !allowedBy(e, en.Selector) {
					inScope = false
				}
			}
			if okList && inScope {
				cov.Items = append(cov.Items, fmt.Sprintf("%s ✓ 黑盒：%d 次查询全部在契约范围内，分叉 %d 个分支", id, len(l.Entries()), len(st.Branches)))
			} else {
				cov.OK = false
				cov.Items = append(cov.Items, fmt.Sprintf("%s ✗ 黑盒：未读关联方名单或有越界查询", id))
			}
			continue
		}
		for _, tool := range e.Tools {
			decl := evidlog.Declaration{QueriedTool: tool, Selector: declaredSelector(tool, r.CaseID)}
			c := evidlog.CheckCoverage(l, decl)
			if c.OK {
				cov.Items = append(cov.Items, fmt.Sprintf("%s ✓ %s %s（%d 条记录）", id, tool, decl.Selector, len(l.Entries())))
			} else {
				cov.OK = false
				cov.Items = append(cov.Items, fmt.Sprintf("%s ✗ 缺少 %v，多出 %v", id, c.Missing, c.Unaccounted))
			}
		}
	}
	add(cov)

	// 5. no member reached outside its contract
	vio := Check{Name: "越权调用", OK: len(r.Violations) == 0, Detail: "成员调用工具时按契约放行；被拒绝的调用会记在这里"}
	vio.Items = append(vio.Items, r.Violations...)
	if vio.OK {
		vio.Detail += "：无"
	}
	add(vio)

	// 6. the risk judgment is one of the published levels and cites real steps
	jc := Check{Name: "判断合规约", OK: r.Decision != nil}
	if r.Decision != nil {
		level := fmt.Sprint(r.Decision["level"])
		jc.OK = contains(s.levels, level)
		q, _ := s.spec["question"].(map[string]any)
		jc.Detail = fmt.Sprintf("规约 %v@%v；结论 %s；依据步骤 %v", q["question_id"], q["spec_version"], level, r.Decision["evidence_steps"])
	} else {
		jc.Detail = "没有已采纳的判断"
	}
	add(jc)

	// 7. replay: drive a fresh adoption machine through the recorded calls
	// and compare with the durable ledger; conformance runs the ledger rules
	rc := Check{Name: "决策重算（replay）", Detail: "用全新的采纳状态机按记录的调用重算，与账本逐条比对"}
	lc := Check{Name: "账本规则", Detail: "账本按 festival 账本规则族逐条检查"}
	m, err := s.machine(runID)
	if err == nil {
		events, _ := m.Events()
		var evs []any
		raw, _ := json.Marshal(events)
		_ = numberDecode(raw, &evs)
		if tamper && len(evs) > 0 {
			if ev, ok := evs[0].(map[string]any); ok {
				if v, ok := ev["versions"].(map[string]any); ok {
					v["rule"] = "q-credit-eligibility@0.9.0"
					rep.Tamper = "已把账本副本里第 1 条记录的规则版本改成 q-credit-eligibility@0.9.0（真实账本未改动）"
				}
			}
		}
		steps := make([]any, 0, len(r.ReplaySteps))
		for _, st := range r.ReplaySteps {
			steps = append(steps, st)
		}
		var doc map[string]any
		rawDoc, _ := json.Marshal(map[string]any{"case_version": "0.1.0", "case_id": strings.ToLower(r.ID), "steps": steps,
			"recorded_events": evs, "expected": map[string]any{"fidelity": "match", "conformance": "pass"}})
		_ = numberDecode(rawDoc, &doc)
		c, lerr := replay.LoadCase(doc)
		if lerr != nil {
			rc.Detail, lc.Detail = "回放用例装载失败："+lerr.Error(), "未执行"
		} else if rr, rerr := replay.Replay(c); rerr != nil {
			rc.Detail, lc.Detail = "回放失败："+rerr.Error(), "未执行"
		} else {
			rc.OK = rr.Fidelity == replay.Match
			lc.OK = rr.Conformance == replay.Pass
			rc.Items = append(rc.Items, fmt.Sprintf("重算 %d 条事件 / 账本 %d 条", len(rr.Recomputed), len(rr.Recorded)))
			if rr.Detail != "" {
				rc.Items = append(rc.Items, rr.Detail)
			}
			lc.Items = append(lc.Items, fmt.Sprintf("conformance = %s", rr.Conformance))
		}
	} else {
		rc.Detail = "账本打不开：" + err.Error()
	}
	add(rc)
	add(lc)

	pass := 0
	for _, c := range rep.Checks {
		if c.OK {
			pass++
		}
	}
	rep.Summary = fmt.Sprintf("%d / %d 项通过", pass, len(rep.Checks))
	if rep.OK {
		rep.Summary = "验证通过：" + rep.Summary
	} else {
		rep.Summary = "验证未通过：" + rep.Summary
	}
	return rep, nil
}

// numberDecode keeps integers as json.Number: the replay conformance arm
// refuses documents whose integers were folded into floats.
func numberDecode(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(v)
}

func compact(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
