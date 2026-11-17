package selection

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/openjiuwen/agent-festival/go/internal/rfc8785"
)

// Adapted from xierongqiag2015 PR33 ee1bac5: pure rules only; no Service/state.json.
// Capability selection (ALK-05, AR-06 #27): for a task in a scenario package
// that publishes a selection policy, planning recalls the package's approved
// capabilities, excludes those whose declared scope does not cover the task
// (an undeclared scope is unknown, and unknown does not pass), ranks the rest
// by the package's configured weights, selects one per output, checks that
// the selected members' inputs and outputs bind, and maps them to graph
// nodes. The record of every step is stored, and its digest is bound into the
// approval of any plan proposed from it.

// SelectionQuery is the task a selection is made for.
type SelectionQuery struct {
	Package       string   `json:"package" jsonschema:"scenario package id from list_capabilities, e.g. annual-report-cmb-2025"`
	Entity        string   `json:"entity" jsonschema:"reporting entity"`
	Basis         string   `json:"basis" jsonschema:"reporting basis, e.g. 本集团 or 母公司"`
	Period        string   `json:"period" jsonschema:"reporting period, e.g. 2025"`
	ComparePeriod string   `json:"compare_period,omitempty" jsonschema:"comparison period, e.g. 2024"`
	Focus         []string `json:"focus" jsonschema:"what the task focuses on, e.g. 经营 or 资产质量"`
	Deliverable   string   `json:"deliverable" jsonschema:"deliverable template ref"`
	Materials     []string `json:"materials" jsonschema:"material ids from the scenario package"`
}

type PackageRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

type Eligibility struct {
	Capability string            `json:"capability"`
	Version    string            `json:"version"`
	Result     string            `json:"result"` // pass excluded unknown
	Checks     map[string]string `json:"checks,omitempty"`
	Reasons    []string          `json:"reasons,omitempty"`
	Score      float64           `json:"score"` // shown for every candidate; only pass candidates are ranked
}

type Ranked struct {
	Capability string  `json:"capability"`
	Score      float64 `json:"score"`
	Basis      string  `json:"basis"`
}

type NotSelected struct {
	Capability string `json:"capability"`
	Reason     string `json:"reason"`
}

type Binding struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Check string `json:"check"`
}

type Composition struct {
	Result   string    `json:"result"` // compatible incompatible
	Bindings []Binding `json:"bindings,omitempty"`
	Problems []string  `json:"problems,omitempty"`
}

type GraphMapping struct {
	Node       string   `json:"node"`
	Capability string   `json:"capability"` // id@version
	DependsOn  []string `json:"depends_on,omitempty"`
	Report     bool     `json:"report,omitempty"` // produces the deliverable
}

type SelectionRecord struct {
	ID          string         `json:"id"`
	Package     PackageRef     `json:"package"`
	Query       SelectionQuery `json:"query"`
	ScoreSource string         `json:"score_source"`
	Recall      struct {
		Method     string   `json:"method"`
		Candidates []string `json:"candidates"`
	} `json:"recall"`
	Eligibility []Eligibility  `json:"eligibility"`
	Ranking     []Ranked       `json:"ranking"`
	Selection   []string       `json:"selection"`
	NotSelected []NotSelected  `json:"not_selected,omitempty"`
	Composition Composition    `json:"composition"`
	Graph       []GraphMapping `json:"graph_mapping,omitempty"`
	Team        []PlanAgent    `json:"team,omitempty"` // what propose_plan adopts when given only the selection id
	Outcome     string         `json:"outcome"`        // graph no_graph
	Reason      string         `json:"reason,omitempty"`
	CreatedAt   string         `json:"created_at"`
	Digest      string         `json:"digest"`
}

// SelectionRef is what a plan keeps of the selection it came from.
type SelectionRef struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

func (r *SelectionRecord) digest() string {
	c := *r
	c.Digest = ""
	raw, _ := rfc8785.Canonicalize(jsonDoc(c))
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func (r *SelectionRecord) eligibility(id string) *Eligibility {
	for i := range r.Eligibility {
		if r.Eligibility[i].Capability == id {
			return &r.Eligibility[i]
		}
	}
	return nil
}

// whyNot explains why a capability is not part of the selection.
func (r *SelectionRecord) whyNot(id string) string {
	for _, n := range r.NotSelected {
		if n.Capability == id {
			return n.Reason
		}
	}
	if e := r.eligibility(id); e != nil && e.Result != "pass" {
		return e.Result + "：" + strings.Join(e.Reasons, "；")
	}
	return "不在本次召回范围内"
}

func (r *SelectionRecord) selected(id string) bool { return contains(r.Selection, id) }

func checkQuery(pkg *ScenarioPackage, q *SelectionQuery) error {
	var missing []string
	for name, v := range map[string]string{"entity": q.Entity, "basis": q.Basis, "period": q.Period, "deliverable": q.Deliverable} {
		if strings.TrimSpace(v) == "" {
			missing = append(missing, name)
		}
	}
	if len(q.Focus) == 0 {
		missing = append(missing, "focus")
	}
	if len(q.Materials) == 0 {
		missing = append(missing, "materials")
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("the task does not say %s: ask the user before selecting", strings.Join(missing, ", "))
	}
	known := map[string]bool{}
	for _, m := range pkg.Materials {
		known[m.ID] = true
	}
	for _, m := range q.Materials {
		if !known[m] {
			return fmt.Errorf("material %q is not in scenario %s", m, pkg.Ref())
		}
	}
	return nil
}

// eligible checks a card's declared scope against the task.
func eligible(pkg *ScenarioPackage, c *CapabilityCard, q *SelectionQuery) Eligibility {
	el := Eligibility{Capability: c.ID, Version: c.Version, Result: "pass", Checks: map[string]string{}}
	unknown := func(reason string) {
		el.Reasons = append(el.Reasons, reason)
		if el.Result == "pass" {
			el.Result = "unknown"
		}
	}
	exclude := func(dim, reason string) {
		el.Checks[dim] = "不满足"
		el.Reasons = append(el.Reasons, reason)
		el.Result = "excluded"
	}
	if c.Input == nil || c.Input.Schema == "" {
		unknown("未声明输入")
	}
	if c.Output == nil || c.Output.Schema == "" {
		unknown("未声明输出")
	}
	var required []string
	if c.Input != nil && c.Input.Schema != "" {
		var ok bool
		if required, ok = pkg.Selection.MustDeclare[c.Input.Schema]; !ok {
			unknown(fmt.Sprintf("场景包没有规定输入 %s 的能力要声明哪些适用范围", c.Input.Schema))
		}
	}
	a := c.Applicability
	if a == nil {
		unknown("未声明适用范围（主体、口径、期间等）")
		a = &Applicability{}
	}
	periods := []string{q.Period}
	if q.ComparePeriod != "" {
		periods = append(periods, q.ComparePeriod)
	}
	dims := []struct {
		name     string
		declared []string
		want     []string
		label    string
	}{
		{"entities", a.Entities, []string{q.Entity}, "主体"},
		{"basis", a.Basis, []string{q.Basis}, "口径"},
		{"periods", a.Periods, periods, "期间"},
		{"materials", a.Materials, q.Materials, "材料"},
		{"templates", a.Templates, []string{q.Deliverable}, "交付模板"},
	}
	for _, d := range dims {
		if len(d.declared) == 0 {
			if contains(required, d.name) {
				el.Checks[d.name] = "未声明"
				unknown(fmt.Sprintf("未声明%s，信息不足", d.label))
			}
			continue
		}
		var outside []string
		for _, w := range d.want {
			if !contains(d.declared, w) {
				outside = append(outside, w)
			}
		}
		if len(outside) > 0 {
			exclude(d.name, fmt.Sprintf("硬约束不满足：任务%s为 %s，能力只支持 %s", d.label, strings.Join(outside, "、"), strings.Join(d.declared, "、")))
			continue
		}
		el.Checks[d.name] = fmt.Sprintf("%s ⊆ [%s]", strings.Join(d.want, "/"), strings.Join(d.declared, ", "))
	}
	if c.Output != nil && c.Output.Ref != "" && c.Output.Ref != q.Deliverable {
		exclude("deliverable", fmt.Sprintf("产出模板 %s 与任务交付模板 %s 不一致", c.Output.Ref, q.Deliverable))
	}
	return el
}

// score orders eligible candidates by the package's configured weights.
func score(pkg *ScenarioPackage, c *CapabilityCard, q *SelectionQuery, inputs map[string]bool) (float64, string) {
	w := pkg.Selection.Weights
	var focus, io float64
	var basis []string
	switch hit := intersect(c.Focus, q.Focus); {
	case len(c.Focus) == 0:
		focus = 1
		basis = append(basis, "不限任务重点")
	case len(hit) > 0:
		focus = 1
		basis = append(basis, "覆盖重点「"+strings.Join(hit, "、")+"」")
	default:
		basis = append(basis, "不覆盖本任务重点")
	}
	if c.Input != nil && inputs[c.Input.Schema] {
		io = 1
	} else {
		basis = append(basis, "输入无合格来源")
	}
	s := w.Focus*focus + w.IO*io + w.Current*1 // every card of an approved package is its current approved version
	return math.Round(s*100) / 100, strings.Join(basis, "；")
}

func intersect(a, b []string) []string {
	var out []string
	for _, x := range a {
		if contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

// member is one node of a candidate team as composition sees it.
type member struct {
	node string
	card *CapabilityCard
	deps []string // nodes whose output it reads
}

// checkComposition checks that every member's input is bound: to the task's
// materials, or to the outputs of the members it depends on, which must
// provide every item it requires; and that one member produces the
// deliverable.
func checkComposition(pkg *ScenarioPackage, q *SelectionQuery, ms []member) (Composition, string) {
	comp := Composition{Result: "compatible"}
	byNode := map[string]*member{}
	for i := range ms {
		byNode[ms[i].node] = &ms[i]
	}
	sink := ""
	for _, m := range ms {
		in, out := m.card.Input, m.card.Output
		if in == nil || out == nil {
			comp.Problems = append(comp.Problems, fmt.Sprintf("%s 未声明输入输出，无法绑定", m.node))
			continue
		}
		if out.Ref != "" && out.Ref == q.Deliverable {
			if sink != "" {
				comp.Problems = append(comp.Problems, fmt.Sprintf("%s 和 %s 都产出交付模板，只能有一个", sink, m.node))
			}
			sink = m.node
		}
		if in.Schema == pkg.Selection.SourceSchema {
			comp.Bindings = append(comp.Bindings, Binding{From: "task.materials[" + strings.Join(q.Materials, ", ") + "]", To: m.node + ".input", Check: "材料在能力适用范围内"})
			continue
		}
		var from []*member
		for _, d := range m.deps {
			if u := byNode[d]; u != nil && u.card.Output != nil && u.card.Output.Schema == in.Schema {
				from = append(from, u)
			}
		}
		if len(from) == 0 {
			comp.Problems = append(comp.Problems, fmt.Sprintf("%s 的输入 %s 没有上游成员提供", m.node, in.Schema))
			continue
		}
		var names []string
		provided, sections := map[string]bool{}, []string{}
		for _, u := range from {
			names = append(names, u.node)
			for _, p := range u.card.Output.Provides {
				provided[p] = true
			}
			if u.card.Output.Section != "" {
				sections = append(sections, u.card.Output.Section)
			}
		}
		var missing []string
		for _, r := range in.Requires {
			if !provided[r] {
				missing = append(missing, r)
			}
		}
		if len(missing) > 0 {
			comp.Problems = append(comp.Problems, fmt.Sprintf("%s 需要 %s，上游 %s 不提供", m.node, strings.Join(missing, "、"), strings.Join(names, "、")))
			continue
		}
		if len(in.RequiresSectionsAny) > 0 && len(intersect(sections, in.RequiresSectionsAny)) == 0 {
			comp.Problems = append(comp.Problems, fmt.Sprintf("%s 至少需要 %s 中的一个段落，上游 %s 都不提供", m.node, strings.Join(in.RequiresSectionsAny, "、"), strings.Join(names, "、")))
			continue
		}
		b := Binding{From: strings.Join(names, ", ") + ".output", To: m.node + ".input", Check: "输入类型 " + in.Schema + " 一致"}
		if len(in.Requires) > 0 {
			b.From = strings.Join(names, ", ") + ".output[" + strings.Join(in.Requires, ", ") + "]"
			b.Check = "所需项齐全"
		}
		if len(in.RequiresSectionsAny) > 0 {
			b.Check = "至少有一个所需段落（" + strings.Join(intersect(in.RequiresSectionsAny, sections), "、") + "）"
		}
		comp.Bindings = append(comp.Bindings, b)
	}
	if sink == "" {
		comp.Problems = append(comp.Problems, "没有成员产出交付模板 "+q.Deliverable)
	}
	if len(comp.Problems) > 0 {
		comp.Result = "incompatible"
	}
	return comp, sink
}

// selectCapabilities runs recall, eligibility, ranking, selection,
// composition and graph mapping for one task. It does not store the record.
func Select(pkg *ScenarioPackage, q SelectionQuery) (*SelectionRecord, error) {
	if pkg.Selection == nil {
		return nil, fmt.Errorf("scenario %s publishes no selection policy: its plans are composed with propose_plan", pkg.Ref())
	}
	if err := checkQuery(pkg, &q); err != nil {
		return nil, err
	}
	pol := pkg.Selection
	rec := &SelectionRecord{Package: PackageRef{ID: pkg.Package, Version: pkg.Version, Digest: pkg.Digest()}, Query: q, Selection: []string{}, Ranking: []Ranked{}, CreatedAt: now()}
	rec.ScoreSource = fmt.Sprintf("场景包配置的权重：职责覆盖任务重点 %.2f + 输入有合格来源 %.2f + 获准包内当前版本 %.2f；选择阈值 %.2f。这是排序依据，不是成功概率。",
		pol.Weights.Focus, pol.Weights.IO, pol.Weights.Current, pol.MinScore)
	rec.Recall.Method = pol.Recall
	for _, c := range pkg.Capabilities {
		rec.Recall.Candidates = append(rec.Recall.Candidates, c.ID)
	}

	// eligibility, then the inputs the task and the eligible candidates can supply
	inputs := map[string]bool{pol.SourceSchema: true}
	for i := range pkg.Capabilities {
		c := &pkg.Capabilities[i]
		el := eligible(pkg, c, &q)
		rec.Eligibility = append(rec.Eligibility, el)
		if el.Result == "pass" {
			inputs[c.Output.Schema] = true
		}
	}
	basis := map[string]string{}
	for i := range pkg.Capabilities {
		c := &pkg.Capabilities[i]
		el := &rec.Eligibility[i]
		el.Score, basis[c.ID] = score(pkg, c, &q, inputs)
		if el.Result == "pass" {
			rec.Ranking = append(rec.Ranking, Ranked{Capability: c.ID, Score: el.Score, Basis: basis[c.ID]})
		}
	}
	sort.SliceStable(rec.Ranking, func(i, j int) bool { return rec.Ranking[i].Score > rec.Ranking[j].Score })
	if len(rec.Ranking) == 0 {
		var excluded, unknown int
		for _, el := range rec.Eligibility {
			if el.Result == "excluded" {
				excluded++
			} else {
				unknown++
			}
		}
		rec.Outcome = "no_graph"
		rec.Reason = fmt.Sprintf("没有通过适用性检查的候选（排除 %d 个，未知 %d 个）；未知不算通过，不生成候选图，先向用户澄清或待能力方补全声明", excluded, unknown)
		rec.Composition = Composition{Result: "incompatible", Problems: []string{"没有可组合的成员"}}
		rec.Digest = rec.digest()
		return rec, nil
	}

	// one member per output; the highest-ranked eligible candidate above the threshold takes it
	slot := func(c *CapabilityCard) string {
		return c.Output.Schema + "|" + c.Output.Section + "|" + c.Output.Ref
	}
	taken := map[string]string{}
	var chosen []*CapabilityCard
	for _, r := range rec.Ranking {
		c := pkg.card(r.Capability)
		switch {
		case r.Score < pol.MinScore:
			rec.NotSelected = append(rec.NotSelected, NotSelected{c.ID, fmt.Sprintf("合格，但分数 %.2f 低于选择阈值 %.2f：%s", r.Score, pol.MinScore, r.Basis)})
		case taken[slot(c)] != "":
			rec.NotSelected = append(rec.NotSelected, NotSelected{c.ID, "同一产出已由排序更高的 " + taken[slot(c)] + " 承担"})
		default:
			taken[slot(c)] = c.ID
			chosen = append(chosen, c)
		}
	}
	// a member whose output nobody consumes and that is not the deliverable is dropped
	for changed := true; changed; {
		changed = false
		for i, c := range chosen {
			if c.Output.Ref != "" && c.Output.Ref == q.Deliverable {
				continue
			}
			used := false
			for _, o := range chosen {
				used = used || (o != c && o.Input.Schema == c.Output.Schema)
			}
			if !used {
				rec.NotSelected = append(rec.NotSelected, NotSelected{c.ID, "合格，但产出 " + c.Output.Schema + " 没有被选中的成员使用"})
				chosen = append(chosen[:i], chosen[i+1:]...)
				changed = true
				break
			}
		}
	}
	ms := make([]member, 0, len(chosen))
	for _, c := range chosen {
		rec.Selection = append(rec.Selection, c.ID)
		m := member{node: c.Node, card: c}
		for _, u := range chosen {
			if u != c && c.Input.Schema != pol.SourceSchema && u.Output.Schema == c.Input.Schema {
				m.deps = append(m.deps, u.Node)
			}
		}
		ms = append(ms, m)
	}
	comp, sink := checkComposition(pkg, &q, ms)
	rec.Composition = comp
	if comp.Result != "compatible" {
		rec.Outcome = "no_graph"
		rec.Reason = "组合不兼容，在采纳前拒绝：" + strings.Join(comp.Problems, "；")
		rec.Digest = rec.digest()
		return rec, nil
	}
	for _, m := range ms {
		rec.Graph = append(rec.Graph, GraphMapping{Node: m.node, Capability: m.card.ID, DependsOn: m.deps, Report: m.node == sink})
		rec.Team = append(rec.Team, PlanAgent{ID: m.node, Expert: m.card.ID, Role: m.card.Name, Task: memberTask(m.card, &q), DependsOn: m.deps})
	}
	rec.Outcome = "graph"
	rec.Digest = rec.digest()
	return rec, nil
}

func memberTask(c *CapabilityCard, q *SelectionQuery) string {
	scope := fmt.Sprintf("%s，%s口径，%s 年", q.Entity, q.Basis, q.Period)
	if q.ComparePeriod != "" {
		scope += "（对比 " + q.ComparePeriod + " 年）"
	}
	return c.Summary + "。范围：" + scope + "；任务重点：" + strings.Join(q.Focus, "、")
}
