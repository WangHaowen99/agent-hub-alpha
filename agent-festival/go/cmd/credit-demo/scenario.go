package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openjiuwen/agent-festival/go/internal/rfc8785"
)

// A scenario package carries everything that is specific to one business
// scenario (AR-06, #27): the capability cards the planner may choose from,
// the evidence rules a plan must satisfy, the grants the approver holds and
// the budget. The control core and the Graph executor read packages; adding
// a scenario or an approved capability means adding or changing a package,
// not code. Only packages whose governance state is "approved" are loaded,
// and every package is pinned by the digest of its canonical form.

//go:embed scenarios/*.json
var builtinScenarios embed.FS

type GrantDoc struct {
	Kind     string   `json:"kind"`
	Selector string   `json:"selector"`
	Actions  []string `json:"actions"`
	Effect   string   `json:"effect"`
	Purpose  string   `json:"purpose,omitempty"`
}

func (g GrantDoc) grant() grant {
	return grant{kind: g.Kind, selector: g.Selector, actions: g.Actions, effect: g.Effect, purpose: g.Purpose}
}

type CapabilityCard struct {
	ID          string     `json:"id"`
	Version     string     `json:"version"`
	Name        string     `json:"name"`
	Kind        string     `json:"kind"` // investigate | blackbox | judge | write | effect
	Summary     string     `json:"summary"`
	Instruction string     `json:"instruction"`
	Tools       []string   `json:"tools,omitempty"`
	Requires    []GrantDoc `json:"requires"`
	Effects     struct {
		Level        string `json:"level"`
		Compensation string `json:"compensation,omitempty"`
	} `json:"effects"`
	Bounds struct {
		CostCents int `json:"cost_cents"`
		Calls     int `json:"calls"`
		Seconds   int `json:"seconds"`
	} `json:"bounds"`
	MaxIter  int    `json:"max_iter,omitempty"`
	BusAgent string `json:"bus_agent,omitempty"` // RDC agent id when the capability runs as a remote Agent

	// What selection (ALK-05) reads; packages planned by hand may omit them.
	Node          string         `json:"node,omitempty"`  // graph node id when a selection maps the capability
	Focus         []string       `json:"focus,omitempty"` // task focus it covers; none means it serves any focus
	Applicability *Applicability `json:"applicability,omitempty"`
	Input         *CapabilityIO  `json:"input,omitempty"`
	Output        *CapabilityIO  `json:"output,omitempty"`
	Limits        []string       `json:"limits,omitempty"`
}

// Applicability is the hard scope a capability declares. A declared dimension
// that does not cover the task excludes the capability; a dimension the
// package requires but the card leaves undeclared makes it unknown.
type Applicability struct {
	Entities  []string `json:"entities,omitempty"`
	Basis     []string `json:"basis,omitempty"`
	Periods   []string `json:"periods,omitempty"`
	Units     []string `json:"units,omitempty"`
	Materials []string `json:"materials,omitempty"`
	Templates []string `json:"templates,omitempty"`
}

// CapabilityIO describes what a capability consumes or produces, so a
// selection can bind one member's output to the next member's input.
type CapabilityIO struct {
	Schema              string   `json:"schema"`
	Requires            []string `json:"requires,omitempty"`              // input: items the upstream output must provide
	RequiresSectionsAny []string `json:"requires_sections_any,omitempty"` // input: at least one upstream section
	Provides            []string `json:"provides,omitempty"`              // output: items it produces
	Section             string   `json:"section,omitempty"`               // output: report section it fills
	Ref                 string   `json:"ref,omitempty"`                   // output: deliverable template it produces
}

type MaterialRef struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
	Source string `json:"source,omitempty"`
}

// SelectionPolicy is how planning picks capabilities for a task in this
// package (ALK-05). Scores order eligible candidates; they are not success
// probabilities.
type SelectionPolicy struct {
	Recall       string              `json:"recall"`
	SourceSchema string              `json:"source_schema"` // input schema the task's materials satisfy
	MustDeclare  map[string][]string `json:"must_declare"`  // input schema -> applicability dimensions a card must declare
	Weights      struct {
		Focus   float64 `json:"focus"`
		IO      float64 `json:"io"`
		Current float64 `json:"current"`
	} `json:"weights"`
	MinScore float64 `json:"min_score"`
}

type CoverageClass struct {
	Name      string   `json:"name"`
	Selectors []string `json:"selectors"` // reading any one covers the class
}

type PackageRules struct {
	RuleSet      string          `json:"rule_set"`
	Coverage     []CoverageClass `json:"coverage"`
	JudgeKind    string          `json:"judge_kind,omitempty"`
	EffectMarker string          `json:"effect_marker,omitempty"`
}

type ScenarioPackage struct {
	Package    string `json:"package"`
	Version    string `json:"version"`
	Title      string `json:"title"`
	Governance struct {
		State      string `json:"state"`
		ApprovedBy string `json:"approved_by"`
		ApprovedAt string `json:"approved_at"`
	} `json:"governance"`
	Rules        PackageRules     `json:"rules"`
	CallerHolds  []GrantDoc       `json:"caller_holds"`
	ParentBounds map[string]any   `json:"parent_bounds"`
	Capabilities []CapabilityCard `json:"capabilities"`
	Materials    []MaterialRef    `json:"materials,omitempty"`
	Selection    *SelectionPolicy `json:"selection,omitempty"` // set when plans come from select_capabilities

	digest string
}

func (p *ScenarioPackage) Ref() string    { return p.Package + "@" + p.Version }
func (p *ScenarioPackage) Digest() string { return p.digest }

// parseScenario validates a package and computes its digest.
func parseScenario(raw []byte) (*ScenarioPackage, error) {
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("scenario package is not JSON: %w", err)
	}
	canon, err := rfc8785.Canonicalize(doc)
	if err != nil {
		return nil, err
	}
	var p ScenarioPackage
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("scenario package: %w", err)
	}
	sum := sha256.Sum256(canon)
	p.digest = "sha256:" + hex.EncodeToString(sum[:])
	if p.Package == "" || p.Version == "" {
		return nil, fmt.Errorf("scenario package needs package and version")
	}
	if p.Governance.State != "approved" {
		return nil, fmt.Errorf("scenario package %s is %q, not approved: it stays invisible to planning", p.Ref(), p.Governance.State)
	}
	seen, nodes := map[string]bool{}, map[string]bool{}
	for i := range p.Capabilities {
		c := &p.Capabilities[i]
		if c.ID == "" || c.Kind == "" || c.Version == "" {
			return nil, fmt.Errorf("%s: capability without id, kind or version", p.Ref())
		}
		if seen[c.ID] {
			return nil, fmt.Errorf("%s: duplicate capability %s", p.Ref(), c.ID)
		}
		seen[c.ID] = true
		if p.Selection != nil {
			if c.Node == "" || strings.ContainsAny(c.Node, " /\\\"'") || nodes[c.Node] {
				return nil, fmt.Errorf("%s: capability %s needs a unique short ascii node id", p.Ref(), c.ID)
			}
			nodes[c.Node] = true
		}
	}
	return &p, nil
}

func (p *ScenarioPackage) card(id string) *CapabilityCard {
	for i := range p.Capabilities {
		if p.Capabilities[i].ID == id {
			return &p.Capabilities[i]
		}
	}
	return nil
}

// holds are the approver's grants that compose checks plans against.
func (p *ScenarioPackage) holds() []grant {
	if p == nil {
		return nil
	}
	out := make([]grant, 0, len(p.CallerHolds))
	for _, g := range p.CallerHolds {
		out = append(out, g.grant())
	}
	return out
}

func (p *ScenarioPackage) held(g grant) bool {
	for _, h := range p.holds() {
		if h.kind == g.kind && h.selector == g.selector {
			return true
		}
	}
	return false
}

// scenarioRegistry holds the approved packages and the expert catalog
// derived from them.
type scenarioRegistry struct {
	packages map[string]*ScenarioPackage // by package id
	owner    map[string]string           // expert id -> package id
}

var scenarios = &scenarioRegistry{packages: map[string]*ScenarioPackage{}, owner: map[string]string{}}

// defaultScenario is the package that plans use when they name none.
const defaultScenario = "credit-dd"

func (r *scenarioRegistry) add(p *ScenarioPackage) error {
	if old, ok := r.packages[p.Package]; ok && old.digest != p.digest {
		return fmt.Errorf("scenario package %s already loaded with different content (%s); publish a new version", p.Package, old.Ref())
	}
	for _, c := range p.Capabilities {
		if other, ok := r.owner[c.ID]; ok && other != p.Package {
			return fmt.Errorf("capability %s is already provided by package %s", c.ID, other)
		}
	}
	r.packages[p.Package] = p
	for _, c := range p.Capabilities {
		r.owner[c.ID] = p.Package
	}
	rebuildCatalogData()
	return nil
}

func (r *scenarioRegistry) get(id string) *ScenarioPackage {
	if id == "" {
		id = defaultScenario
	}
	return r.packages[id]
}

func (r *scenarioRegistry) ids() []string {
	out := make([]string, 0, len(r.packages))
	for id := range r.packages {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// loadScenarioDir imports approved packages placed by an operator in dir.
// A package that is not approved, malformed, or conflicts with a loaded one
// is reported and not loaded.
func loadScenarioDir(dir string) error {
	if dir == "" {
		return nil
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	var problems []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		p, err := parseScenario(raw)
		if err == nil {
			err = scenarios.add(p)
		}
		if err != nil {
			problems = append(problems, filepath.Base(f)+": "+err.Error())
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("scenario packages not loaded: %s", strings.Join(problems, "; "))
	}
	return nil
}

// The catalog the rest of the service reads, derived from the packages. A
// plan's rules, grants and budget come from its own package (Plan.scenario).
var (
	experts   []*Expert
	busAgents = map[string]string{}
)

func rebuildCatalogData() {
	experts = nil
	busAgents = map[string]string{}
	for _, id := range scenarioOrder() {
		p := scenarios.packages[id]
		for _, c := range p.Capabilities {
			e := &Expert{ID: c.ID, Name: c.Name, Kind: c.Kind, Summary: c.Summary, Tools: c.Tools, Prompt: c.Instruction, MaxIter: c.MaxIter,
				effectLevel: c.Effects.Level, compensate: c.Effects.Compensation, costCents: c.Bounds.CostCents, calls: c.Bounds.Calls, seconds: c.Bounds.Seconds,
				Package: p.Package}
			for _, g := range c.Requires {
				e.requires = append(e.requires, g.grant())
			}
			experts = append(experts, e)
			if c.BusAgent != "" {
				busAgents[c.ID] = c.BusAgent
			}
		}
	}
}

// scenarioViews is what the planner sees of the approved packages.
func scenarioViews() []map[string]any {
	out := []map[string]any{}
	for _, id := range scenarioOrder() {
		p := scenarios.packages[id]
		v := map[string]any{"package": p.Package, "version": p.Version, "title": p.Title, "digest": p.Digest(),
			"selection": p.Selection != nil, "rules": p.planRules()}
		var caps, mats []string
		for _, c := range p.Capabilities {
			caps = append(caps, c.ID)
		}
		for _, m := range p.Materials {
			mats = append(mats, m.ID)
		}
		v["capabilities"] = caps
		if len(mats) > 0 {
			v["materials"] = mats
		}
		out = append(out, v)
	}
	return out
}

// scenarioOrder keeps the default package first so the credit catalog keeps
// its original order; other packages follow by id.
func scenarioOrder() []string {
	out := []string{}
	if _, ok := scenarios.packages[defaultScenario]; ok {
		out = append(out, defaultScenario)
	}
	for _, id := range scenarios.ids() {
		if id != defaultScenario {
			out = append(out, id)
		}
	}
	return out
}

func init() {
	files, err := builtinScenarios.ReadDir("scenarios")
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		raw, err := builtinScenarios.ReadFile("scenarios/" + f.Name())
		if err != nil {
			panic(err)
		}
		p, err := parseScenario(raw)
		if err != nil {
			panic("built-in " + f.Name() + ": " + err.Error())
		}
		if err := scenarios.add(p); err != nil {
			panic("built-in " + f.Name() + ": " + err.Error())
		}
	}
}
