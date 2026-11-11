package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Graph is the one view model of a plan version and its run. The MCP App in
// WorkSwarm and the confirm and run pages all draw from it, so what the user
// reviews, approves and watches is the same picture.
type Graph struct {
	Plan       string      `json:"plan"`
	Version    int         `json:"version"`
	Case       string      `json:"case"`
	Goal       string      `json:"goal"`
	Layers     [][]string  `json:"layers"`
	Nodes      []GraphNode `json:"nodes"`
	OK         bool        `json:"ok"` // precheck passed
	Problems   []string    `json:"problems,omitempty"`
	Hash       string      `json:"hash"`                 // what an approval binds to
	Approve    string      `json:"approve,omitempty"`    // approval endpoint, while awaiting approval
	Confirm    string      `json:"confirm,omitempty"`    // confirm page, while awaiting approval
	Superseded int         `json:"superseded,omitempty"` // newer version that replaced this one
	Run        *GraphRun   `json:"run,omitempty"`
	Verify     *Report     `json:"verify,omitempty"`
	Live       string      `json:"live,omitempty"` // poll this while something can still change
}

type GraphNode struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Kind     string        `json:"kind"` // investigate blackbox judge write effect
	Role     string        `json:"role,omitempty"`
	Task     string        `json:"task"`
	Deps     []string      `json:"deps,omitempty"`
	When     *RunIf        `json:"when,omitempty"`
	Effect   string        `json:"effect,omitempty"`
	Reads    []string      `json:"reads,omitempty"`
	Contract string        `json:"contract,omitempty"`
	Denied   string        `json:"denied,omitempty"`
	Status   string        `json:"status,omitempty"` // pending running done reused skipped failed
	Activity string        `json:"activity,omitempty"`
	Output   string        `json:"output,omitempty"`
	Branches []GraphBranch `json:"branches,omitempty"`
}

// GraphBranch is one sub-goal a Loop Forest step forked, as the graph draws it.
type GraphBranch struct {
	Goal     string `json:"goal"`
	Status   string `json:"status"` // started finished interrupted
	Title    string `json:"title"`
	Relation string `json:"relation,omitempty"`
	Activity string `json:"activity,omitempty"`
	Finding  string `json:"finding,omitempty"` // what the branch read, once it finished
	Tone     string `json:"tone,omitempty"`    // ok warn
	Retried  bool   `json:"retried,omitempty"` // an earlier attempt died with its executor
}

type GraphRun struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Decision string `json:"decision,omitempty"` // adopted level, e.g. grant-50000
	Receipt  string `json:"receipt,omitempty"`  // approved declined cancelled unknown
	Leases   int    `json:"leases"`
	Stalled  bool   `json:"stalled,omitempty"` // heartbeats stopped; the lease is running out
	URL      string `json:"url"`
}

func (s *Service) graph(p *Plan, withVerify bool) (*Graph, error) {
	g := &Graph{Plan: p.ID, Version: p.Version, Case: p.CaseID, Goal: p.Goal, Layers: p.Layers, OK: p.Precheck.OK, Hash: p.Hash}
	g.Problems = append(append(g.Problems, p.Precheck.NotHeld...), p.Precheck.Rules...)
	for _, e := range p.Precheck.Errors {
		if !strings.HasPrefix(e, "not held by caller") { // already explained by NotHeld
			g.Problems = append(g.Problems, explainCompose(e))
		}
	}
	s.mu.Lock()
	if n := len(s.st.Plans[p.ID]); n > p.Version {
		g.Superseded = n
	}
	var run *Run
	if p.RunID != "" {
		run = s.st.Runs[p.RunID]
	}
	for _, st := range p.Steps {
		a := p.agent(st.Member)
		if a == nil {
			continue
		}
		e := expertByID(a.Expert)
		n := GraphNode{ID: st.ID, Name: e.Name, Kind: e.Kind, Role: a.Role, Task: a.Task, Deps: st.DependsOn, When: st.When,
			Contract: clip(strings.TrimPrefix(s.cat.Hashes[e.ID], "sha256:"), 12)}
		if e.effectLevel != "none" {
			n.Effect = e.effectLevel
		}
		for _, r := range e.requires {
			n.Reads = append(n.Reads, r.selector)
			if !held(r) {
				n.Denied = "缺少 " + r.selector + " 权限"
			}
		}
		if run != nil {
			n.Status = "pending"
			if rec := run.Steps[st.ID]; rec != nil {
				n.Status = rec.Status
				if rec.Status == "done" && rec.Reused > 0 {
					n.Status = "reused"
				}
				n.Output = clip(strings.TrimSpace(stripStepTag(rec.Output)), 240)
				if rec.Status == "running" {
					n.Activity = rec.Activity
				}
				n.Branches = graphBranches(p.CaseID, rec.Branches)
				if rec.Status == "running" && len(n.Branches) > 0 {
					n.Activity = "汇总分支结论"
					if k := countStarted(n.Branches); k > 0 {
						n.Activity = fmt.Sprintf("等待 %d 个分支", k)
					}
				}
			}
		}
		g.Nodes = append(g.Nodes, n)
	}
	if run != nil {
		g.Run = &GraphRun{ID: run.ID, Status: run.Status, Receipt: run.ReceiptOf, Leases: len(run.Leases),
			URL: fmt.Sprintf("%s/ui/run/%s", s.cfg.PublicURL, run.ID)}
		if run.Decision != nil {
			g.Run.Decision = fmt.Sprint(run.Decision["level"])
		}
		if run.Status == "running" && !run.LeaseUntil.IsZero() {
			// The executor beats every lease/3 seconds and each beat renews the
			// lease, so a lease shorter than lease-1.5 beats means beats stopped.
			lease := time.Duration(s.cfg.LeaseSeconds) * time.Second
			beat := max(3*time.Second, lease/3)
			g.Run.Stalled = time.Until(run.LeaseUntil) < lease-beat*3/2
		}
	}
	s.mu.Unlock()

	switch {
	case run != nil && run.Status != "done" && run.Status != "failed":
		g.Live = s.graphURL(p)
	case run == nil && g.OK && g.Superseded == 0:
		g.Confirm = fmt.Sprintf("%s/ui/plan/%s/v%d", s.cfg.PublicURL, p.ID, p.Version)
		g.Approve = s.cfg.PublicURL + "/api/approve"
		g.Live = s.graphURL(p) // turns into the run as soon as the user approves
	}
	if withVerify && run != nil && run.Status == "done" {
		rep, err := s.Verify(run.ID, false)
		if err != nil {
			return nil, err
		}
		g.Verify = rep
	}
	return g, nil
}

var budgetErr = regexp.MustCompile(`^node (\S+): (\w+) (\d+) exceeds parent budget (\d+)$`)

var boundNames = map[string]string{"max_downstream_calls": "下游调用次数", "max_cost": "费用", "max_seconds": "时长"}

// explainCompose turns compose's budget errors into the planner's language;
// anything else is passed through as compose wrote it.
func explainCompose(e string) string {
	m := budgetErr.FindStringSubmatch(e)
	if m == nil {
		return e
	}
	name := boundNames[m[2]]
	if name == "" {
		name = m[2]
	}
	return fmt.Sprintf("%s 的%s上限 %s 超过它在并行层里分到的预算 %s（同层越宽，每个节点分到越少）", m[1], name, m[3], m[4])
}

func (s *Service) graphURL(p *Plan) string {
	return fmt.Sprintf("%s/api/graph?plan=%s&v=%d", s.cfg.PublicURL, url.QueryEscape(p.ID), p.Version)
}

// stripStepTag drops the "[step:id]" line the executor prefixes to outputs.
func stripStepTag(out string) string {
	if strings.HasPrefix(out, "[step:") {
		if i := strings.Index(out, "\n"); i >= 0 {
			return out[i+1:]
		}
	}
	return out
}

// serveApprove lets the user approve from the plan card in the chat. The card
// runs in a sandboxed iframe without cookies, so it sends the password itself;
// the planner model never sees what is typed there and has no approval tool.
// The approval binds the hash the card showed, exactly like the confirm page.
func (s *Service) serveApprove(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	reply := func(code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	switch r.Method {
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
		return
	case http.MethodPost:
	default:
		reply(http.StatusMethodNotAllowed, map[string]string{"error": "只接受 POST"})
		return
	}
	var in struct {
		Plan     string `json:"plan"`
		Version  int    `json:"version"`
		Hash     string `json:"hash"`
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil {
		reply(http.StatusBadRequest, map[string]string{"error": "请求格式不对"})
		return
	}
	if !validLogin(in.User, in.Password) {
		reply(http.StatusUnauthorized, map[string]string{"error": "密码不对"})
		return
	}
	run, err := s.Approve(in.Plan, in.Version, in.Hash, in.User)
	if err != nil {
		msg := err.Error()
		switch {
		case strings.Contains(msg, "changed after it was shown"):
			msg = "编排单在展示后变了，请看最新版本再批准"
		case strings.Contains(msg, "precheck rejects"):
			msg = "规约预检不通过，不能批准"
		case strings.Contains(msg, "unknown plan"):
			msg = "找不到这一版编排单"
		}
		reply(http.StatusConflict, map[string]string{"error": msg})
		return
	}
	reply(http.StatusOK, map[string]string{"run": run.ID})
}

// serveGraph is the read-only projection the MCP App polls from its sandbox,
// so it answers any origin. It exposes nothing the confirm page does not show.
func (s *Service) serveGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	q := r.URL.Query()
	var p *Plan
	if id := q.Get("run"); id != "" {
		if _, rp, ok := s.RunByID(id); ok {
			p = rp
		}
	} else {
		v, _ := strconv.Atoi(q.Get("v"))
		if pp, ok := s.Plan(q.Get("plan"), v); ok {
			p = pp
		}
	}
	if p == nil {
		http.NotFound(w, r)
		return
	}
	g, err := s.graph(p, q.Get("verify") == "1")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(g)
}

// graphBranches keeps the latest attempt per sub-goal: a restarted executor
// forks the same companies again, and the graph shows each company once.
func graphBranches(caseID string, bs []Branch) []GraphBranch {
	var out []GraphBranch
	at := map[string]int{}
	for _, b := range bs {
		id, name, relation := relatedParty(caseID, b.Goal)
		key := id
		if key == "" {
			key = b.Goal
		}
		gb := GraphBranch{Goal: b.Goal, Status: b.Status, Title: shortCompany(name), Relation: strings.TrimPrefix(relation, "申请人")}
		if gb.Title == "" {
			gb.Title = branchGoal.ReplaceAllString(b.Goal, "")
		}
		if b.Status == "started" {
			gb.Activity = b.Activity
		}
		if b.Status == "finished" && b.Read != "" {
			gb.Finding, gb.Tone = bureauFinding(relatedBureau[b.Read])
		}
		if i, ok := at[key]; ok {
			gb.Retried = true
			out[i] = gb
			continue
		}
		at[key] = len(out)
		out = append(out, gb)
	}
	return out
}

func countStarted(bs []GraphBranch) int {
	n := 0
	for _, b := range bs {
		if b.Status == "started" {
			n++
		}
	}
	return n
}

// Loop Forest sub-goals often start with the field name, e.g. "company_id: R-602，查…".
var branchGoal = regexp.MustCompile(`^[A-Za-z_]+\s*[:：]\s*`)

func shortCompany(name string) string {
	for _, suf := range []string{"有限责任公司", "有限公司"} {
		name = strings.TrimSuffix(name, suf)
	}
	return name
}

// bureauFinding sums up the credit summary a branch read in a few words.
func bureauFinding(d map[string]any) (string, string) {
	if d == nil {
		return "", ""
	}
	num := func(k string) int {
		switch v := d[k].(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
		return 0
	}
	if n := num("overdue_events_last_12_months"); n > 0 {
		return fmt.Sprintf("近一年逾期 %d 次", n), "warn"
	}
	if n := num("guarantees_given"); n > 0 {
		return fmt.Sprintf("对外担保 %d 笔", n), "warn"
	}
	return "无逾期", "ok"
}
