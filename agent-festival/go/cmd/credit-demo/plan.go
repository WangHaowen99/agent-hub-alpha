package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/openjiuwen/agent-festival/go/internal/compose"
	"github.com/openjiuwen/agent-festival/go/internal/rfc8785"
)

// PlanAgent is one team member as proposed by the planner: which registered
// expert does it, in what role, on which sub-task, after whom.
type PlanAgent struct {
	ID        string   `json:"id" jsonschema:"step id, short ascii such as financial"`
	Expert    string   `json:"expert" jsonschema:"catalog id from list_capabilities"`
	Role      string   `json:"role" jsonschema:"role of this member in the team"`
	Task      string   `json:"task" jsonschema:"sub-task for this member"`
	DependsOn []string `json:"depends_on,omitempty" jsonschema:"ids of steps whose output this step needs"`
	RunIf     *RunIf   `json:"run_if,omitempty" jsonschema:"run only when an upstream step's output contains (or not) a marker"`
}

type RunIf struct {
	Step     string `json:"step"`
	Contains string `json:"contains"`
	Negate   bool   `json:"negate,omitempty"`
}

// Step is the compiled blueprint step as returned by the orchestrator.
type Step struct {
	ID        string   `json:"id"`
	Member    string   `json:"member"`
	DependsOn []string `json:"depends_on"`
	When      *RunIf   `json:"when,omitempty"`
}

type Precheck struct {
	OK        bool           `json:"ok"`
	Errors    []string       `json:"errors,omitempty"`
	NotHeld   []string       `json:"not_held,omitempty"`
	Rules     []string       `json:"rules,omitempty"` // due-diligence rule violations
	Composed  map[string]any `json:"composed,omitempty"`
	CheckedAt string         `json:"checked_at"`
}

// Plan is one immutable version of the f proposed for a case.
type Plan struct {
	ID         string           `json:"id"`
	Version    int              `json:"version"`
	CaseID     string           `json:"case_id"`
	Goal       string           `json:"goal"`
	Agents     []PlanAgent      `json:"agents"`
	Blueprint  string           `json:"blueprint_yaml"`
	SpecDigest string           `json:"spec_digest"`
	Team       []map[string]any `json:"team"`
	Type       string           `json:"type"`
	Steps      []Step           `json:"steps"`
	Layers     [][]string       `json:"layers"`
	Precheck   Precheck         `json:"precheck"`
	Hash       string           `json:"hash"`
	ProposedAt string           `json:"proposed_at"`
	ApprovedBy string           `json:"approved_by,omitempty"`
	RunID      string           `json:"run_id,omitempty"`
}

func (p *Plan) Key() string { return fmt.Sprintf("%s-v%d", p.ID, p.Version) }

func (p *Plan) agent(id string) *PlanAgent {
	for i := range p.Agents {
		if p.Agents[i].ID == id {
			return &p.Agents[i]
		}
	}
	return nil
}

// The customer manager's grants and budget: what compose checks against.
var callerHolds = []grant{
	read("data:credit/applications/*", ""),
	read("data:credit/financials/*", ""),
	read("data:industry/*", ""),
	read("data:credit/bureau/summary/*", ""),
	read("data:credit/evidence/*", ""),
	read("data:credit/related/*", ""),
	read("data:credit/related-bureau/*", ""),
	{kind: "tool", selector: "credit.apply", actions: []string{"execute"}, effect: "irreversible"},
}

var parentBounds = map[string]any{
	"max_cost":             map[string]any{"amount": 500, "unit": "USD-cents"},
	"max_downstream_calls": 36, // parallel siblings split it evenly; 36 keeps a 7-wide layer above the black box's 5
	"max_seconds":          1800,
}

func checkAgents(agents []PlanAgent) error {
	if len(agents) == 0 {
		return fmt.Errorf("agents is empty")
	}
	seen := map[string]bool{}
	for _, a := range agents {
		if a.ID == "" || strings.ContainsAny(a.ID, " /\\\"'") {
			return fmt.Errorf("agent id %q must be a short ascii word", a.ID)
		}
		if seen[a.ID] {
			return fmt.Errorf("duplicate agent id %q", a.ID)
		}
		seen[a.ID] = true
		if expertByID(a.Expert) == nil {
			return fmt.Errorf("agent %s: %q is not a registered capability (use list_capabilities)", a.ID, a.Expert)
		}
	}
	for _, a := range agents {
		for _, d := range a.DependsOn {
			if !seen[d] {
				return fmt.Errorf("agent %s depends on unknown step %q", a.ID, d)
			}
		}
		if a.RunIf != nil && !seen[a.RunIf.Step] {
			return fmt.Errorf("agent %s: run_if refers to unknown step %q", a.ID, a.RunIf.Step)
		}
	}
	return nil
}

// compileResult is the orchestrator's answer: yaojun's TeamBlueprints
// compiled the team into a blueprint and BlueprintLoader validated it.
type compileResult struct {
	Blueprint  string `json:"blueprint_yaml"`
	SpecDigest string `json:"spec_digest"`
	Type       string `json:"type"`
	Steps      []Step `json:"steps"`
	Error      string `json:"error"`
}

func (s *Service) compile(agents []PlanAgent) (*compileResult, []map[string]any, error) {
	team := []map[string]any{}
	for _, a := range agents {
		e := expertByID(a.Expert)
		m := map[string]any{"id": a.ID, "role": a.Role, "task": a.Task, "systemPrompt": expertInstruction(e)}
		if len(a.DependsOn) > 0 {
			m["dependsOn"] = a.DependsOn
		}
		if a.RunIf != nil {
			typ := "output_contains"
			if a.RunIf.Negate {
				typ = "output_not_contains"
			}
			m["condition"] = map[string]any{"type": typ, "node": a.RunIf.Step, "text": a.RunIf.Contains}
		}
		team = append(team, m)
	}
	body, _ := json.Marshal(map[string]any{"mode": "graph", "agents": team})
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Post(s.cfg.OrchestratorURL+"/compile", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("orchestrator unreachable: %w", err)
	}
	defer resp.Body.Close()
	var out compileResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, nil, fmt.Errorf("orchestrator answer unreadable: %w", err)
	}
	if out.Error != "" {
		return &out, team, fmt.Errorf("blueprint rejected: %s", out.Error)
	}
	return &out, team, nil
}

func expertInstruction(e *Expert) string {
	if e.Kind == "judge" {
		return "你是风险判断员，按已发布的授信判断规约给出结构化判断。"
	}
	if e.Kind == "effect" {
		return "你是授信申请提交节点，只按已采纳的判断经网关提交申请。"
	}
	return e.Prompt
}

// layers orders steps topologically; steps in one layer may run in parallel.
func layers(steps []Step) ([][]string, error) {
	indeg := map[string]int{}
	down := map[string][]string{}
	order := map[string]int{}
	for i, st := range steps {
		order[st.ID] = i
		indeg[st.ID] += 0
		for _, d := range st.DependsOn {
			indeg[st.ID]++
			down[d] = append(down[d], st.ID)
		}
	}
	var out [][]string
	done := 0
	for done < len(steps) {
		var layer []string
		for _, st := range steps {
			if indeg[st.ID] == 0 {
				layer = append(layer, st.ID)
			}
		}
		if len(layer) == 0 {
			return nil, fmt.Errorf("dependency cycle")
		}
		sort.Slice(layer, func(i, j int) bool { return order[layer[i]] < order[layer[j]] })
		for _, id := range layer {
			indeg[id] = -1
			for _, n := range down[id] {
				indeg[n]--
			}
		}
		done += len(layer)
		out = append(out, layer)
	}
	return out, nil
}

// execGraph turns the blueprint into the festival execution graph: each
// step carries its expert's contract; the plan tree is the layer order.
func execGraph(p *Plan) map[string]any {
	nodes := map[string]any{}
	for _, st := range p.Steps {
		if a := p.agent(st.Member); a != nil {
			nodes[st.ID] = expertByID(a.Expert).contract()
		}
	}
	seq := []any{}
	for _, l := range p.Layers {
		if len(l) == 1 {
			seq = append(seq, l[0])
		} else {
			seq = append(seq, map[string]any{"par": toAny(l)})
		}
	}
	var plan any = map[string]any{"seq": seq}
	if len(seq) == 1 {
		plan = seq[0]
	}
	holds := []any{}
	for _, g := range callerHolds {
		holds = append(holds, g.doc())
	}
	return jsonDoc(map[string]any{"nodes": nodes, "plan": plan, "caller_holds": holds, "parent_bounds": parentBounds})
}

// ddCoverage is the published due-diligence rule: every evidence class must be
// read by some member. list_capabilities shows it and precheck enforces it, so
// a plan cannot pass by leaving evidence out.
var ddCoverage = []struct {
	Name      string
	Selectors []string // reading any one covers the class
}{
	{"申请材料", []string{"data:credit/applications/*"}},
	{"财务", []string{"data:credit/financials/*"}},
	{"行业", []string{"data:industry/*"}},
	{"征信", []string{"data:credit/bureau/full/*", "data:credit/bureau/summary/*"}},
	{"关联方", []string{"data:credit/related/*"}},
}

func ddRules() []string {
	out := []string{}
	for _, c := range ddCoverage {
		out = append(out, fmt.Sprintf("要有成员读取%s证据（%s）", c.Name, strings.Join(c.Selectors, " 或 ")))
	}
	return append(out,
		"风险判断（kind=judge）直接或间接依赖全部调查步骤和黑盒步骤",
		"有不可逆效果的步骤依赖风险判断，并设 run_if 含 DECISION=GRANT")
}

func ruleViolations(p *Plan) []string {
	var errs []string
	kind := map[string]string{}
	deps := map[string][]string{}
	reads := map[string]bool{}
	for _, st := range p.Steps {
		if a := p.agent(st.Member); a != nil {
			e := expertByID(a.Expert)
			kind[st.ID] = e.Kind
			for _, g := range e.requires {
				reads[g.selector] = true
			}
		}
		deps[st.ID] = st.DependsOn
	}
	for _, c := range ddCoverage {
		covered := false
		for _, sel := range c.Selectors {
			covered = covered || reads[sel]
		}
		if !covered {
			errs = append(errs, fmt.Sprintf("缺少%s证据：没有成员读取 %s", c.Name, strings.Join(c.Selectors, " 或 ")))
		}
	}
	ancestors := func(id string) map[string]bool {
		seen := map[string]bool{}
		stack := append([]string{}, deps[id]...)
		for len(stack) > 0 {
			d := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if !seen[d] {
				seen[d] = true
				stack = append(stack, deps[d]...)
			}
		}
		return seen
	}
	judges := 0
	for _, st := range p.Steps {
		if kind[st.ID] != "judge" {
			continue
		}
		judges++
		up := ancestors(st.ID)
		for _, o := range p.Steps {
			if (kind[o.ID] == "investigate" || kind[o.ID] == "blackbox") && !up[o.ID] {
				errs = append(errs, fmt.Sprintf("风险判断 %s 没有依赖调查步骤 %s", st.ID, o.ID))
			}
		}
	}
	if judges == 0 {
		errs = append(errs, "缺少风险判断步骤（kind=judge）")
	}
	for _, st := range p.Steps {
		if kind[st.ID] != "effect" {
			continue
		}
		w := st.When
		if w == nil || w.Negate || kind[w.Step] != "judge" || !strings.Contains(w.Contains, "DECISION=GRANT") || !ancestors(st.ID)[w.Step] {
			errs = append(errs, fmt.Sprintf("%s 有不可逆效果，要依赖风险判断并设 run_if 含 DECISION=GRANT", st.ID))
		}
	}
	return errs
}

func precheck(p *Plan) Precheck {
	res := compose.ComposeErrors(execGraph(p))
	pc := Precheck{OK: len(res.Errors) == 0, Errors: res.Errors, Composed: res.Composed, CheckedAt: now()}
	if pc.Rules = ruleViolations(p); len(pc.Rules) > 0 {
		pc.OK = false
	}
	for _, st := range p.Steps {
		a := p.agent(st.Member)
		if a == nil {
			continue
		}
		for _, g := range expertByID(a.Expert).requires {
			if !held(g) {
				pc.NotHeld = append(pc.NotHeld, fmt.Sprintf("%s（%s）需要 %s %s，客户经理没有这项权限", st.ID, expertByID(a.Expert).Name, g.selector, strings.Join(g.actions, ",")))
			}
		}
	}
	return pc
}

func held(g grant) bool {
	for _, h := range callerHolds {
		if h.kind == g.kind && h.selector == g.selector {
			return true
		}
	}
	return false
}

// planHash binds what is approved: case, goal, the compiled blueprint (by
// its digest), and which signed contract each member runs under.
func planHash(p *Plan, contractHashes map[string]string) string {
	members := map[string]any{}
	for _, a := range p.Agents {
		members[a.ID] = map[string]any{"expert": a.Expert, "contract_hash": contractHashes[a.Expert]}
	}
	raw, _ := rfc8785.Canonicalize(jsonDoc(map[string]any{"case": p.CaseID, "goal": p.Goal, "spec_digest": p.SpecDigest, "members": members}))
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }
