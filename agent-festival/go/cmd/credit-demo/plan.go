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
	ID          string                `json:"id"`
	Version     int                   `json:"version"`
	CaseID      string                `json:"case_id"`
	Goal        string                `json:"goal"`
	Agents      []PlanAgent           `json:"agents"`
	Blueprint   string                `json:"blueprint_yaml"`
	SpecDigest  string                `json:"spec_digest"`
	Team        []map[string]any      `json:"team"`
	Type        string                `json:"type"`
	Steps       []Step                `json:"steps"`
	Layers      [][]string            `json:"layers"`
	Precheck    Precheck              `json:"precheck"`
	Hash        string                `json:"hash"`
	ProposedAt  string                `json:"proposed_at"`
	ApprovedBy  string                `json:"approved_by,omitempty"`
	RunID       string                `json:"run_id,omitempty"`
	BusBindings map[string]BusBinding `json:"bus_bindings,omitempty"`
	Scenario    string                `json:"scenario,omitempty"`  // package id; empty for plans proposed before packages
	Selection   *SelectionRef         `json:"selection,omitempty"` // the capability selection the team came from
}

func (p *Plan) Key() string { return fmt.Sprintf("%s-v%d", p.ID, p.Version) }

// scenario is the package whose rules, grants and budget the plan is checked
// against; nil if that package is no longer loaded.
func (p *Plan) scenario() *ScenarioPackage { return scenarios.get(p.Scenario) }

func (p *Plan) agent(id string) *PlanAgent {
	for i := range p.Agents {
		if p.Agents[i].ID == id {
			return &p.Agents[i]
		}
	}
	return nil
}

// The approver's grants and the budget that compose checks against come from
// the plan's scenario package; see scenario.go.

func checkAgents(pkg *ScenarioPackage, agents []PlanAgent) error {
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
		e := expertByID(a.Expert)
		if e == nil {
			return fmt.Errorf("agent %s: %q is not a registered capability (use list_capabilities)", a.ID, a.Expert)
		}
		if e.Package != pkg.Package {
			return fmt.Errorf("agent %s: %s belongs to scenario %s, not %s", a.ID, a.Expert, e.Package, pkg.Package)
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
	return e.Prompt // the capability card's instruction
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
	pkg := p.scenario()
	holds := []any{}
	for _, g := range pkg.holds() {
		holds = append(holds, g.doc())
	}
	var bounds map[string]any
	if pkg != nil {
		bounds = pkg.ParentBounds
	}
	return jsonDoc(map[string]any{"nodes": nodes, "plan": plan, "caller_holds": holds, "parent_bounds": bounds})
}

// planRules are the package's published rules: the evidence classes some
// member must read, the judgment every investigation feeds, and the guard on
// irreversible effects. list_capabilities shows them and precheck enforces them.
func (pkg *ScenarioPackage) planRules() []string {
	out := []string{}
	for _, c := range pkg.Rules.Coverage {
		out = append(out, fmt.Sprintf("要有成员读取%s证据（%s）", c.Name, strings.Join(c.Selectors, " 或 ")))
	}
	if pkg.Rules.JudgeKind != "" {
		out = append(out, fmt.Sprintf("风险判断（kind=%s）直接或间接依赖全部调查步骤和黑盒步骤", pkg.Rules.JudgeKind))
	}
	if pkg.Rules.EffectMarker != "" {
		out = append(out, "有不可逆效果的步骤依赖风险判断，并设 run_if 含 "+pkg.Rules.EffectMarker)
	}
	if pkg.Selection != nil {
		out = append(out, "成员只能来自 select_capabilities 的选择记录；数据绑定在采纳前检查")
	}
	return out
}

func ruleViolations(p *Plan) []string {
	pkg := p.scenario()
	if pkg == nil {
		return []string{fmt.Sprintf("场景包 %s 未加载，无法按其规约检查", p.Scenario)}
	}
	judgeKind, marker := pkg.Rules.JudgeKind, pkg.Rules.EffectMarker
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
	for _, c := range pkg.Rules.Coverage {
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
		if judgeKind == "" || kind[st.ID] != judgeKind {
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
	if judgeKind != "" && judges == 0 {
		errs = append(errs, fmt.Sprintf("缺少风险判断步骤（kind=%s）", judgeKind))
	}
	for _, st := range p.Steps {
		if kind[st.ID] != "effect" {
			continue
		}
		w := st.When
		if judgeKind == "" || marker == "" || w == nil || w.Negate || kind[w.Step] != judgeKind || !strings.Contains(w.Contains, marker) || !ancestors(st.ID)[w.Step] {
			errs = append(errs, fmt.Sprintf("%s 有不可逆效果，要依赖风险判断并设 run_if 含 %s", st.ID, marker))
		}
	}
	return errs
}

func precheck(p *Plan) Precheck {
	pkg := p.scenario()
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
			if !pkg.held(g) {
				pc.NotHeld = append(pc.NotHeld, fmt.Sprintf("%s（%s）需要 %s %s，客户经理没有这项权限", st.ID, expertByID(a.Expert).Name, g.selector, strings.Join(g.actions, ",")))
			}
		}
	}
	return pc
}

// planHash binds what is approved: case, goal, the compiled blueprint (by
// its digest), and which signed contract each member runs under; for plans
// proposed under a scenario package also the package digest, and for plans
// from a capability selection the digest of that selection record.
func planHash(p *Plan, contractHashes map[string]string) string {
	members := map[string]any{}
	for _, a := range p.Agents {
		members[a.ID] = map[string]any{"expert": a.Expert, "contract_hash": contractHashes[a.Expert]}
		if b, ok := p.BusBindings[a.ID]; ok {
			members[a.ID].(map[string]any)["bus_binding"] = b
		}
	}
	doc := map[string]any{"case": p.CaseID, "goal": p.Goal, "spec_digest": p.SpecDigest, "members": members}
	if p.Scenario != "" {
		scenario := map[string]any{"package": p.Scenario}
		if pkg := p.scenario(); pkg != nil {
			scenario["ref"], scenario["digest"] = pkg.Ref(), pkg.Digest()
		}
		doc["scenario"] = scenario
	}
	if p.Selection != nil {
		doc["selection"] = map[string]any{"id": p.Selection.ID, "digest": p.Selection.Digest}
	}
	raw, _ := rfc8785.Canonicalize(jsonDoc(doc))
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
