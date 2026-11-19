package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/openjiuwen/agent-festival/go/internal/adopt"
	"github.com/openjiuwen/agent-festival/go/internal/replay"
)

// reconcileAdoptions runs before serving requests. It uses the existing replay
// engine on scratch storage; never edits ledger evidence or guesses a lost
// model result. A mismatch withdraws this run's lease, not other runs' leases.
func (s *Service) reconcileAdoptions() error {
	changed := false
	for _, r := range s.st.Runs {
		if err := s.checkAdoptionSnapshot(r); err != nil {
			reason := "RECONCILIATION_REQUIRED: " + err.Error() + "；保留原账本与快照，需人工核对，不能重新生成决策。"
			if r.Status != "blocked" || r.Final != reason || r.Token != "" {
				s.blockRun(r, reason)
				changed = true
			}
		}
	}
	if changed {
		// Keep the pre-reconciliation snapshot for operator comparison. The
		// content-derived name makes repeated restarts retain the same evidence.
		backup := filepath.Join(s.cfg.DataDir, "reconciliation-"+strings.TrimPrefix(sha(string(s.lastSaved)), "sha256:")+".json")
		if err := s.writeState(backup, json.RawMessage(s.lastSaved)); err != nil {
			return fmt.Errorf("save reconciliation evidence: %w", err)
		}
		return s.save()
	}
	return nil
}

func (s *Service) checkAdoptionSnapshot(r *Run) error {
	m, err := adopt.Open(filepath.Join(s.cfg.DataDir, "runs", r.ID, "adopt.jsonl"))
	if err != nil {
		return fmt.Errorf("采纳账本不可读取或版本/格式不兼容")
	}
	if m.TornTail() {
		return fmt.Errorf("采纳账本存在不完整尾记录")
	}
	events, err := m.Events()
	if err != nil {
		return fmt.Errorf("采纳账本读取失败")
	}
	if len(events) == 0 && len(r.ReplaySteps) == 0 && r.Decision == nil && r.DecisionOp == "" && r.BusinessReq == "" {
		return nil
	}

	// Adopt.Open validates raw documents before typed projection. Replay then
	// checks snapshot commands against the whole validated event sequence.
	raw, err := json.Marshal(map[string]any{"case_version": "0.2.0", "case_id": strings.ToLower(r.ID),
		"steps": r.ReplaySteps, "recorded_events": events,
		"expected": map[string]any{"fidelity": "match", "conformance": "pass"}})
	if err != nil {
		return fmt.Errorf("采纳快照无法编码")
	}
	var doc map[string]any
	if err := numberDecode(raw, &doc); err != nil {
		return err
	}
	c, err := replay.LoadCase(doc)
	if err != nil {
		return fmt.Errorf("采纳快照不满足当前回放协议")
	}
	result, err := replay.Replay(c)
	if err != nil || result.Fidelity != replay.Match || result.Conformance != replay.Pass {
		return fmt.Errorf("采纳账本与快照操作序列不一致")
	}
	for _, event := range events {
		if event.Outcome.Status != "adopted" {
			continue
		}
		digest, err := adopt.Digest(r.Decision)
		if err != nil || r.Decision == nil || event.Idempotency == nil || digest != event.Idempotency.PayloadDigest ||
			r.DecisionOp != event.Identity.OperationID || r.BusinessReq != event.Identity.BusinessRequestID ||
			event.Versions.Policy != r.PlanHash || event.Versions.Model != s.runModel(r) {
			return fmt.Errorf("决策内容、身份或绑定与采纳记录不一致")
		}
		// A pre-fix snapshot can contain a decision without its node result.
		// Do not ask a model to reproduce it or silently manufacture a result.
		for id, st := range r.Steps {
			if e := expertByID(st.Expert); e != nil && e.Kind == "judge" && st.Status == "done" &&
				st.Output == "[step:"+id+"]\n"+judgmentOutput(r.Decision) {
				return nil
			}
		}
		return fmt.Errorf("已采纳决策缺少匹配的持久节点结果")
	}
	return fmt.Errorf("快照缺少可核对的原始采纳事件")
}

func judgmentOutput(decision map[string]any) string {
	return fmt.Sprintf("DECISION=%s\n依据：%v\n证据步骤：%v", strings.ToUpper(fmt.Sprint(decision["level"])), decision["rationale"], decision["evidence_steps"])
}
