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
	seen := map[string]bool{}
	for _, c := range p.Capabilities {
		if c.ID == "" || c.Kind == "" || c.Version == "" {
			return nil, fmt.Errorf("%s: capability without id, kind or version", p.Ref())
		}
		if seen[c.ID] {
			return nil, fmt.Errorf("%s: duplicate capability %s", p.Ref(), c.ID)
		}
		seen[c.ID] = true
	}
	return &p, nil
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

// The catalog data the rest of the service reads, derived from the packages.
var (
	experts      []*Expert
	busAgents    = map[string]string{}
	callerHolds  []grant
	parentBounds map[string]any
	ddCoverage   []CoverageClass
	effectMarker string
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
	p := scenarios.get(defaultScenario)
	callerHolds, parentBounds, ddCoverage, effectMarker = nil, nil, nil, ""
	if p == nil {
		return
	}
	for _, g := range p.CallerHolds {
		callerHolds = append(callerHolds, g.grant())
	}
	parentBounds = p.ParentBounds
	ddCoverage = p.Rules.Coverage
	effectMarker = p.Rules.EffectMarker
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
