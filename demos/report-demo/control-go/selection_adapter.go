package main

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/openjiuwen/agent-festival/go/cmd/report-demo/selection"
)

// Declarations belong to the approved catalog, not a second scenario registry.
type SelectionDeclaration struct {
	Focus         []string                 `json:"focus,omitempty"`
	Applicability *selection.Applicability `json:"applicability,omitempty"`
	Input         *selection.CapabilityIO  `json:"input,omitempty"`
	Output        *selection.CapabilityIO  `json:"output,omitempty"`
}
type SelectionConfig struct {
	Package       string                     `json:"package"`
	Version       string                     `json:"version"`
	Entity        string                     `json:"entity"`
	Basis         string                     `json:"basis"`
	Period        string                     `json:"period"`
	ComparePeriod string                     `json:"compare_period"`
	Deliverable   string                     `json:"deliverable"`
	Policy        *selection.SelectionPolicy `json:"policy"`
}

func selectionPackage(c Catalog, m Material) (*selection.ScenarioPackage, error) {
	cfg := c.Selection
	if cfg == nil || cfg.Policy == nil {
		return nil, fault("SELECTION_UNAVAILABLE", "catalog has no approved selection policy", 409)
	}
	cd, err := Digest(c)
	if err != nil {
		return nil, err
	}
	pkg := &selection.ScenarioPackage{Package: cfg.Package, Version: cfg.Version, ContentDigest: cd, Selection: cfg.Policy, Materials: []selection.MaterialRef{{ID: m.ID, SHA256: m.Digest}}}
	for _, cap := range c.Capabilities {
		if !cap.Enabled || cap.AgentID == "" {
			continue
		} // local checks/selectors are mandatory control nodes, not model experts
		card := selection.CapabilityCard{ID: cap.ID, Version: cap.Version, Name: cap.Name, Summary: cap.Name, Node: cap.Kind}
		if d := cap.Selection; d != nil {
			card.Focus = d.Focus
			card.Applicability = d.Applicability
			card.Input = d.Input
			card.Output = d.Output
		}
		pkg.Capabilities = append(pkg.Capabilities, card)
	}
	return pkg, nil
}

func (s *Server) selectCapabilities(ctx context.Context, q selection.SelectionQuery) (*selection.SelectionRecord, error) {
	pkg, err := selectionPackage(s.Catalog, s.Material)
	if err != nil {
		return nil, err
	}
	cfg := s.Catalog.Selection
	// Fixed source snapshot cannot substantiate a different reporting scope.
	if q.Package != cfg.Package || q.Entity != cfg.Entity || q.Basis != cfg.Basis || q.Period != cfg.Period || q.ComparePeriod != cfg.ComparePeriod || q.Deliverable != cfg.Deliverable {
		return nil, fault("POLICY_REJECTED", "task scope differs from the published material; clarify rather than substitute", 422)
	}
	if len(q.Focus) == 0 {
		return nil, fault("POLICY_REJECTED", "clarify task focus", 422)
	}
	for _, v := range q.Focus {
		if v != "经营" && v != "资产质量" {
			return nil, fault("POLICY_REJECTED", "unsupported focus; clarify task", 422)
		}
	}
	hasBusiness := false
	for _, v := range q.Focus {
		hasBusiness = hasBusiness || v == "经营"
	}
	if !hasBusiness {
		return nil, fault("POLICY_REJECTED", "this slice requires business focus; supported choices are business or business plus asset quality", 422)
	}
	rec, err := selection.Select(pkg, q)
	if err != nil {
		return nil, fault("POLICY_REJECTED", err.Error(), 422)
	}
	// Scope metadata is not permission to dispatch: every selected expert must
	// resolve to a real published Bus capability with a pinned version.
	for _, id := range rec.Selection {
		cap, _ := s.Catalog.Get(id)
		if cap.Scope != "group-annual-report" || cap.AgentID == "" || cap.RemoteBinding == nil || cap.RemoteBinding.validate() != nil {
			return nil, fault("BINDING_CHANGED", "selected expert has no usable real Bus binding", 409)
		}
	}
	rec.Seal(newID("selection-"))
	raw, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	tx, err := s.Store.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, "INSERT INTO selection_records(scope,id,digest,document) VALUES($1,$2,$3,$4)", s.Store.Scope, rec.ID, rec.Digest, raw)
	if err != nil {
		return nil, err
	}
	if err = emit(ctx, tx, s.Store.Scope, rec.ID, "CAPABILITIES_SELECTED", map[string]any{"selection_id": rec.ID, "digest": rec.Digest, "outcome": rec.Outcome}); err != nil {
		return nil, err
	}
	return rec, tx.Commit(ctx)
}

type selectionReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *Store) selectionRecord(ctx context.Context, db selectionReader, ref *selection.SelectionRef) (*selection.SelectionRecord, error) {
	if ref == nil || ref.ID == "" || ref.Digest == "" {
		return nil, fault("POLICY_REJECTED", "candidate must reference its stored capability selection", 422)
	}
	var raw []byte
	err := db.QueryRow(ctx, "SELECT document FROM selection_records WHERE scope=$1 AND id=$2", s.Scope, ref.ID).Scan(&raw)
	if err == pgx.ErrNoRows {
		return nil, fault("POLICY_REJECTED", "selection not found", 422)
	}
	if err != nil {
		return nil, err
	}
	var rec selection.SelectionRecord
	if err = DecodeStrict(raw, &rec); err != nil {
		return nil, err
	}
	if rec.ID != ref.ID || rec.Digest != ref.Digest || !rec.ValidDigest() {
		return nil, fault("INPUT_CONFLICT", "selection digest differs", 409)
	}
	return &rec, nil
}
func (s *Store) validateSelection(ctx context.Context, db selectionReader, g Graph) error {
	rec, err := s.selectionRecord(ctx, db, g.Selection)
	if err != nil {
		return err
	}
	if rec.Outcome != "graph" || rec.Composition.Result != "compatible" {
		return fault("POLICY_REJECTED", "selection cannot form a compatible graph", 422)
	}
	if rec.Package.Digest != g.CatalogDigest || (s.CurrentCatalogDigest != "" && rec.Package.Digest != s.CurrentCatalogDigest) {
		return fault("BINDING_CHANGED", "selection catalog changed", 409)
	}
	if len(rec.Query.Materials) != 1 || rec.Query.Materials[0] != g.MaterialID {
		return fault("INPUT_CONFLICT", "selection material differs", 409)
	}
	focus := "business"
	quality := false
	for _, f := range rec.Query.Focus {
		if f == "资产质量" {
			quality = true
		}
	}
	if quality {
		focus = "business+quality"
	}
	if g.Focus != focus {
		return fault("INPUT_CONFLICT", "graph focus differs from selection", 409)
	}
	used := map[string]bool{}
	for _, n := range g.Nodes {
		used[n.Expert] = true
	}
	selected := map[string]bool{}
	for _, id := range rec.Selection {
		selected[id] = true
		if !used[id] {
			return fault("POLICY_REJECTED", "selected expert absent from graph", 422)
		}
	}
	for id := range used {
		if id != "checker@1" && id != "select@1" && !selected[id] {
			return fault("POLICY_REJECTED", "graph contains an unselected expert", 422)
		}
	}
	return nil
}
