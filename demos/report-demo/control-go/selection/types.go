package selection

import (
	"encoding/json"
	"time"
)

// Minimal data boundary for PR33's pure selection engine. W04 owns persistence.
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

type CapabilityCard struct {
	ID            string         `json:"id"`
	Version       string         `json:"version"`
	Name          string         `json:"name"`
	Summary       string         `json:"summary"`
	Node          string         `json:"node"`
	Focus         []string       `json:"focus,omitempty"`
	Applicability *Applicability `json:"applicability,omitempty"`
	Input         *CapabilityIO  `json:"input,omitempty"`
	Output        *CapabilityIO  `json:"output,omitempty"`
}
type ScenarioPackage struct {
	Package       string           `json:"package"`
	Version       string           `json:"version"`
	Capabilities  []CapabilityCard `json:"capabilities"`
	Materials     []MaterialRef    `json:"materials"`
	Selection     *SelectionPolicy `json:"selection"`
	ContentDigest string           `json:"-"`
}

func (p *ScenarioPackage) Ref() string    { return p.Package + "@" + p.Version }
func (p *ScenarioPackage) Digest() string { return p.ContentDigest }
func (p *ScenarioPackage) card(id string) *CapabilityCard {
	for i := range p.Capabilities {
		if p.Capabilities[i].ID == id {
			return &p.Capabilities[i]
		}
	}
	return nil
}

type PlanAgent struct {
	ID        string   `json:"id"`
	Expert    string   `json:"expert"`
	Role      string   `json:"role"`
	Task      string   `json:"task"`
	DependsOn []string `json:"depends_on,omitempty"`
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func jsonDoc(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var doc any
	if err = json.Unmarshal(b, &doc); err != nil {
		panic(err)
	}
	return doc
}

// Seal includes the persisted identity; the rules initially produce an unnamed record.
func (r *SelectionRecord) Seal(id string)    { r.ID = id; r.Digest = r.digest() }
func (r *SelectionRecord) ValidDigest() bool { return r.Digest == r.digest() }
