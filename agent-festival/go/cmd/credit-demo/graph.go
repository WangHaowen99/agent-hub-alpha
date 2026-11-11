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
	Confirm    string      `json:"confirm,omitempty"`    // confirm page, while awaiting approval
	Superseded int         `json:"superseded,omitempty"` // newer version that replaced this one
	Run        *GraphRun   `json:"run,omitempty"`
	Verify     *Report     `json:"verify,omitempty"`
	Live       string      `json:"live,omitempty"` // poll this while something can still change
}

type GraphNode struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Kind     string   `json:"kind"` // investigate blackbox judge write effect
	Role     string   `json:"role,omitempty"`
	Task     string   `json:"task"`
	Deps     []string `json:"deps,omitempty"`
	When     *RunIf   `json:"when,omitempty"`
	Effect   string   `json:"effect,omitempty"`
	Reads    []string `json:"reads,omitempty"`
	Contract string   `json:"contract,omitempty"`
	Denied   string   `json:"denied,omitempty"`
	Status   string   `json:"status,omitempty"` // pending running done reused skipped failed
	Output   string   `json:"output,omitempty"`
	Branches []Branch `json:"branches,omitempty"`
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
	g := &Graph{Plan: p.ID, Version: p.Version, Case: p.CaseID, Goal: p.Goal, Layers: p.Layers, OK: p.Precheck.OK}
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
				n.Branches = append([]Branch(nil), rec.Branches...)
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
