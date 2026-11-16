package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"
	"unicode/utf8"

	jsoncanonicalizer "github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/openjiuwen/agent-festival/go/internal/ledger"
	"github.com/openjiuwen/agent-festival/go/internal/rfc8785"
)

const Protocol = "report-demo/1"

type Fault struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	HTTP    int    `json:"-"`
}

func (e *Fault) Error() string                   { return e.Code + ": " + e.Message }
func fault(code, message string, http int) error { return &Fault{code, message, http} }

// DecodeStrict preserves integer tokens and rejects duplicate keys, including nested keys.
func DecodeStrict(raw []byte, into any) error {
	if !utf8.Valid(raw) {
		return errors.New("invalid UTF-8 JSON")
	}
	// encoding/json replaces lone Unicode surrogates with U+FFFD. Reject them
	// before decoding so two different signed/hash inputs cannot collapse to
	// one replacement string. Reuse the existing RFC author's parser, wrapped
	// as an array because its entry point expects an object/array root.
	wrapped := make([]byte, 0, len(raw)+2)
	wrapped = append(wrapped, '[')
	wrapped = append(wrapped, raw...)
	wrapped = append(wrapped, ']')
	if _, err := jsoncanonicalizer.Transform(wrapped); err != nil {
		return errors.New("invalid I-JSON syntax, Unicode or number")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func() (any, error)
	value = func() (any, error) {
		t, err := d.Token()
		if err != nil {
			return nil, err
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				m := map[string]any{}
				for d.More() {
					k, err := d.Token()
					if err != nil {
						return nil, err
					}
					key, ok := k.(string)
					if !ok {
						return nil, errors.New("object key is not a string")
					}
					if _, exists := m[key]; exists {
						return nil, fmt.Errorf("duplicate JSON key %q", key)
					}
					v, err := value()
					if err != nil {
						return nil, err
					}
					m[key] = v
				}
				_, err = d.Token()
				return m, err
			case '[':
				a := []any{}
				for d.More() {
					v, err := value()
					if err != nil {
						return nil, err
					}
					a = append(a, v)
				}
				_, err = d.Token()
				return a, err
			default:
				return nil, errors.New("unexpected JSON delimiter")
			}
		}
		return t, nil
	}
	v, err := value()
	if err != nil {
		return err
	}
	if _, err = d.Token(); err != io.EOF {
		return errors.New("trailing JSON content")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	dec.DisallowUnknownFields()
	return dec.Decode(into)
}

func Digest(value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var v any
	if err = DecodeStrict(b, &v); err != nil {
		return "", err
	}
	b, err = rfc8785.Canonicalize(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
func newID(prefix string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + hex.EncodeToString(b)
}

type Condition struct {
	Step    string `json:"step"`
	Outcome string `json:"outcome"`
}
type Node struct {
	ID        string     `json:"id"`
	Expert    string     `json:"expert"`
	Role      string     `json:"role"`
	Task      string     `json:"task"`
	DependsOn []string   `json:"depends_on,omitempty" jsonschema:"Predecessor node IDs. Omit or use an empty array only for the material extractor root; all other roles require their typed data dependencies."`
	Condition *Condition `json:"condition,omitempty"`
}
type Graph struct {
	Goal          string `json:"goal"`
	MaterialID    string `json:"material_id"`
	Focus         string `json:"focus" jsonschema:"Supported focus is exactly business or business+quality."`
	Nodes         []Node `json:"nodes"`
	CatalogDigest string `json:"catalog_digest"`
	PolicyVersion string `json:"policy_version"`
}
type Capability struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Version       string         `json:"version"`
	Kind          string         `json:"kind"`
	AgentID       string         `json:"agent_id,omitempty"`
	InputSchema   string         `json:"input_schema"`
	OutputSchema  string         `json:"output_schema"`
	Scope         string         `json:"scope"`
	Enabled       bool           `json:"enabled"`
	RemoteBinding *RemoteBinding `json:"remote_binding,omitempty"`
}
type Catalog struct {
	SchemaVersion string       `json:"schema_version"`
	PolicyVersion string       `json:"policy_version"`
	Capabilities  []Capability `json:"capabilities"`
}

func (c Catalog) Get(id string) (Capability, bool) {
	for _, cap := range c.Capabilities {
		if cap.ID == id {
			return cap, true
		}
	}
	return Capability{}, false
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

// ValidateGraph checks admission, not scheduling. Java's compiler owns runtime mapping.
func ValidateGraph(g Graph, c Catalog) error {
	if g.Goal == "" || g.MaterialID == "" || len(g.Nodes) < 3 || len(g.Nodes) > 12 {
		return fault("POLICY_REJECTED", "goal/material and 3..12 nodes required", 422)
	}
	cd, err := Digest(c)
	if err != nil {
		return err
	}
	if g.CatalogDigest != cd || g.PolicyVersion != c.PolicyVersion {
		return fault("BINDING_CHANGED", "catalog or policy version differs", 409)
	}
	nodes := map[string]Node{}
	kinds := map[string]int{}
	for _, n := range g.Nodes {
		if !identifier.MatchString(n.ID) || n.Task == "" || len(n.Task) > 8000 {
			return fault("POLICY_REJECTED", "invalid node identity/instruction", 422)
		}
		if _, duplicate := nodes[n.ID]; duplicate {
			return fault("POLICY_REJECTED", "duplicate node", 422)
		}
		cap, ok := c.Get(n.Expert)
		if !ok || !cap.Enabled || cap.Scope != "group-annual-report" {
			return fault("POLICY_REJECTED", "unapproved or incompatible capability", 422)
		}
		if cap.RemoteBinding != nil {
			if cap.AgentID == "" {
				return fault("POLICY_REJECTED", "local capability cannot bind RDC", 422)
			}
			if err := cap.RemoteBinding.validate(); err != nil {
				return err
			}
		}
		if cap.RemoteBinding != nil {
			if cap.AgentID == "" {
				return fault("POLICY_REJECTED", "local capability cannot bind RDC", 422)
			}
			if err := cap.RemoteBinding.validate(); err != nil {
				return err
			}
		}
		nodes[n.ID] = n
		kinds[cap.Kind]++
	}
	for _, kind := range []string{"extract", "business", "write"} {
		if kinds[kind] == 0 {
			return fault("POLICY_REJECTED", "three distinct business expert capabilities required", 422)
		}
	}
	if kinds["extract"] != 1 || kinds["business"] != 1 || kinds["quality"] > 1 || kinds["write"] > 2 || kinds["check"] > 2 || kinds["select"] > 1 {
		return fault("POLICY_REJECTED", "bounded scene allows one extractor/business/quality, one draft plus at most one revision, two checks and one selector", 422)
	}
	if g.Focus != "business" && g.Focus != "business+quality" {
		return fault("POLICY_REJECTED", "supported focus is business or business+quality", 422)
	}
	if g.Focus == "business+quality" && kinds["quality"] != 1 {
		return fault("POLICY_REJECTED", "asset-quality goal requires its real expert branch", 422)
	}
	seen, active := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if active[id] {
			return fault("POLICY_REJECTED", "cyclic graph", 422)
		}
		if seen[id] {
			return nil
		}
		active[id] = true
		n := nodes[id]
		deps := map[string]bool{}
		for _, dep := range n.DependsOn {
			if _, ok := nodes[dep]; !ok || deps[dep] {
				return fault("POLICY_REJECTED", "missing or duplicate dependency", 422)
			}
			deps[dep] = true
			if err := visit(dep); err != nil {
				return err
			}
		}
		if n.Condition != nil {
			if !deps[n.Condition.Step] || (n.Condition.Outcome != "PASS" && n.Condition.Outcome != "REVISE") {
				return fault("POLICY_REJECTED", "condition must reference a direct checker dependency", 422)
			}
			cap, _ := c.Get(nodes[n.Condition.Step].Expert)
			if cap.Kind != "check" {
				return fault("POLICY_REJECTED", "condition source is not a checker", 422)
			}
		}
		active[id] = false
		seen[id] = true
		return nil
	}
	for id := range nodes {
		if err := visit(id); err != nil {
			return err
		}
	}
	initialWriters, revisions := 0, 0
	// Typed business edges: every downstream role must consume the declared upstream role.
	for _, n := range g.Nodes {
		cap, _ := c.Get(n.Expert)
		depKinds := map[string]bool{}
		for _, dep := range n.DependsOn {
			dc, _ := c.Get(nodes[dep].Expert)
			depKinds[dc.Kind] = true
		}
		switch cap.Kind {
		case "extract":
			if len(n.DependsOn) != 0 || n.Condition != nil {
				return fault("POLICY_REJECTED", "extractor must consume fixed material", 422)
			}
		case "business", "quality":
			if !depKinds["extract"] || n.Condition != nil {
				return fault("POLICY_REJECTED", "analysis must consume extractor output", 422)
			}
		case "write":
			if n.Condition == nil {
				initialWriters++
			} else {
				revisions++
			}
			if n.Condition != nil && (n.Condition.Outcome != "REVISE" || !depKinds["write"] || !depKinds["check"]) {
				return fault("POLICY_REJECTED", "revision requires original draft and checker REVISE condition", 422)
			}
			if n.Condition == nil && depKinds["check"] {
				return fault("POLICY_REJECTED", "revision must be explicitly conditional", 422)
			}
			if !depKinds["business"] && !depKinds["check"] {
				return fault("POLICY_REJECTED", "writer must consume analysis or revision feedback", 422)
			}
			if kinds["quality"] > 0 && !depKinds["quality"] && !depKinds["check"] {
				return fault("POLICY_REJECTED", "full report must join both analysis branches", 422)
			}
		case "check":
			if !depKinds["write"] {
				return fault("POLICY_REJECTED", "checker must consume report", 422)
			}
		case "select":
			if !depKinds["check"] {
				return fault("POLICY_REJECTED", "selection must consume checks", 422)
			}
		default:
			return fault("POLICY_REJECTED", "unsupported capability kind", 422)
		}
	}
	if initialWriters != 1 || revisions > 1 {
		return fault("POLICY_REJECTED", "exactly one initial draft and at most one conditional revision required", 422)
	}
	return nil
}

// ValidateReportGraph is the public report-delivery admission policy.
// ValidateGraph remains usable for internal computation/control fixtures.
func ValidateReportGraph(g Graph, c Catalog) error {
	if err := ValidateGraph(g, c); err != nil {
		return err
	}
	checks := map[string]Node{}
	var selector *Node
	for i := range g.Nodes {
		n := &g.Nodes[i]
		cap, _ := c.Get(n.Expert)
		if cap.Kind == "check" {
			checks[n.ID] = *n
		}
		if cap.Kind == "select" {
			selector = n
		}
	}
	if len(checks) == 0 || selector == nil || selector.Condition != nil {
		return fault("POLICY_REJECTED", "report delivery requires report checks and one unconditional selector", 422)
	}
	selected := map[string]bool{}
	for _, dep := range selector.DependsOn {
		selected[dep] = true
	}
	for id := range checks {
		if !selected[id] {
			return fault("POLICY_REJECTED", "report selector must consume every report check", 422)
		}
	}
	for _, n := range g.Nodes {
		cap, _ := c.Get(n.Expert)
		if cap.Kind != "write" {
			continue
		}
		checked := false
		for _, check := range checks {
			for _, dep := range check.DependsOn {
				if dep != n.ID {
					continue
				}
				// A revision and its check take the same admitted branch; the
				// initial report must always have an unconditional check.
				if (n.Condition == nil && check.Condition == nil) ||
					(n.Condition != nil && check.Condition != nil && *n.Condition == *check.Condition) {
					checked = true
				}
			}
		}
		if !checked {
			return fault("POLICY_REJECTED", "every report draft/revision requires a check on the same branch", 422)
		}
	}
	return nil
}

type Result struct {
	Identity       *ledger.Identity `json:"identity,omitempty"`
	NodeID         string           `json:"node_id"`
	InputDigest    string           `json:"input_digest"`
	ArtifactID     string           `json:"artifact_id"`
	ArtifactDigest string           `json:"artifact_digest"`
	Receipt        string           `json:"receipt"`
	Valid          bool             `json:"valid"`
	Output         json.RawMessage  `json:"output"`
	ReusedFrom     *ResultSource    `json:"reused_from,omitempty"`
}

type ResultSource struct {
	TaskID  string `json:"task_id"`
	NodeID  string `json:"node_id"`
	Receipt string `json:"receipt"`
}
type NodeState struct {
	Review      *CheckReview `json:"review,omitempty"`
	Status      string       `json:"status"`
	InputDigest string       `json:"input_digest,omitempty"`
	Attempt     int          `json:"attempt"`
	OwnerEpoch  string       `json:"owner_epoch,omitempty"`
	Reason      string       `json:"reason,omitempty"`
	SkipCheck   string       `json:"skip_check,omitempty"`
	Result      *Result      `json:"result,omitempty"`
}
type Task struct {
	IdentityContract string                `json:"identity_contract,omitempty"`
	DeliveryReceipts []json.RawMessage     `json:"delivery_receipts,omitempty"`
	ReportApprovals  []json.RawMessage     `json:"report_approvals,omitempty"`
	ExecutionProfile string                `json:"execution_profile"`
	SchemaVersion    string                `json:"schema_version"`
	ID               string                `json:"task_id"`
	Revision         string                `json:"graph_revision"`
	Status           string                `json:"status"`
	Generation       string                `json:"generation"`
	Graph            Graph                 `json:"graph"`
	GraphDigest      string                `json:"graph_digest"`
	RuntimeDigest    string                `json:"runtime_spec_digest"`
	Pause            bool                  `json:"pause_requested"`
	ApprovedBy       string                `json:"approved_by,omitempty"`
	Nodes            map[string]*NodeState `json:"nodes"`
	CreatedAt        time.Time             `json:"created_at"`
}
type Binding struct {
	IdentityContract         string                   `json:"identity_contract,omitempty"`
	LogicalExecution         *ledger.LogicalExecution `json:"logical_execution,omitempty"`
	Reviews                  []CheckReview            `json:"reviews,omitempty"`
	ExecutionProfile         string                   `json:"execution_profile"`
	SchemaVersion            string                   `json:"schema_version"`
	TaskID                   string                   `json:"task_id"`
	Revision                 string                   `json:"graph_revision"`
	NodeID                   string                   `json:"node_id"`
	Instruction              string                   `json:"instruction"`
	Goal                     string                   `json:"goal"`
	Focus                    string                   `json:"focus"`
	MaterialDigest           string                   `json:"material_digest"`
	MaterialProjectionDigest string                   `json:"material_projection_digest"`
	Capability               Capability               `json:"capability"`
	PolicyVersion            string                   `json:"policy_version"`
	Upstream                 []Result                 `json:"upstream"`
	AuthorizedSkips          []SkipProof              `json:"authorized_skips"`
}

type SkipProof struct {
	ReviewReceipt string `json:"review_receipt,omitempty"`
	NodeID        string `json:"node_id"`
	CheckArtifact string `json:"check_artifact"`
	Outcome       string `json:"outcome"`
}

func skipProof(t *Task, id string) (SkipProof, bool) {
	ns := t.Nodes[id]
	if ns == nil || ns.Status != "AUTHORIZED_SKIP" || ns.SkipCheck == "" {
		return SkipProof{}, false
	}
	for _, n := range t.Graph.Nodes {
		if n.ID == id && n.Condition != nil {
			check := t.Nodes[n.Condition.Step]
			if check == nil || check.Result == nil || !check.Result.Valid || check.Result.ArtifactID != ns.SkipCheck {
				return SkipProof{}, false
			}
			var out struct {
				Outcome string `json:"outcome"`
			}
			if json.Unmarshal(check.Result.Output, &out) != nil {
				return SkipProof{}, false
			}
			out.Outcome = effectiveCheck(check)
			if (out.Outcome == "PASS" || out.Outcome == "REVISE") && out.Outcome != n.Condition.Outcome {
				proof := SkipProof{NodeID: id, CheckArtifact: ns.SkipCheck, Outcome: out.Outcome}
				if check.Review != nil {
					proof.ReviewReceipt = check.Review.Receipt
				}
				return proof, true
			}
		}
	}
	return SkipProof{}, false
}
