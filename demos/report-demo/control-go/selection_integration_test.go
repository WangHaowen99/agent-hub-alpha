package main

import (
	"github.com/openjiuwen/agent-festival/go/cmd/report-demo/selection"
	"testing"
)

func selectionQuery(c Catalog, material string, quality bool) selection.SelectionQuery {
	v := c.Selection
	focus := []string{"经营"}
	if quality {
		focus = append(focus, "资产质量")
	}
	return selection.SelectionQuery{Package: v.Package, Entity: v.Entity, Basis: v.Basis, Period: v.Period, ComparePeriod: v.ComparePeriod, Deliverable: v.Deliverable, Focus: focus, Materials: []string{material}}
}
func TestSelectionRulesExcludeUnknownRankAndCompose(t *testing.T) {
	c, g := sample(t)
	m := Material{ID: g.MaterialID, Digest: "fixture-material"}
	for _, quality := range []bool{false, true} {
		pkg, err := selectionPackage(c, m)
		if err != nil {
			t.Fatal(err)
		}
		rec, err := selection.Select(pkg, selectionQuery(c, m.ID, quality))
		if err != nil {
			t.Fatal(err)
		}
		count := 3
		if quality {
			count = 4
		}
		if rec.Outcome != "graph" || len(rec.Selection) != count {
			t.Fatalf("selection differs: %+v", rec)
		}
		for _, e := range rec.Eligibility {
			if e.Capability == "parent-only@1" && (e.Result != "excluded" || e.Score != 4) {
				t.Fatal("hard scope must exclude even at top configured score", e)
			}
			if e.Capability == "unknown@1" && e.Result != "unknown" {
				t.Fatal("unknown became eligible")
			}
		}
		for _, id := range rec.Selection {
			if id == "parent-only@1" || id == "unknown@1" {
				t.Fatal("ineligible expert selected")
			}
		}
		rec.Seal("fixture")
		if !rec.ValidDigest() {
			t.Fatal("identity not bound")
		}
		rec.ID = "changed"
		if rec.ValidDigest() {
			t.Fatal("changed record identity accepted")
		}
		for i := range pkg.Capabilities {
			if pkg.Capabilities[i].ID == "writer@1" {
				copy := *pkg.Capabilities[i].Input
				copy.Requires = []string{"unavailable-indicator"}
				pkg.Capabilities[i].Input = &copy
			}
		}
		bad, err := selection.Select(pkg, selectionQuery(c, m.ID, quality))
		if err != nil || bad.Outcome != "no_graph" || bad.Composition.Result != "incompatible" {
			t.Fatal("incompatible data composition became a graph", err, bad)
		}
	}
}

func TestPGSelectionReferenceCannotBeReplacedAtAdmission(t *testing.T) {
	ctx, store, cfg, c := integration(t)
	cfg.ModelBindingDigest = "sha256:selection-control-fixture"
	_, g := reportSample(t)
	server := NewServer(store, cfg, c, Material{ID: g.MaterialID, Digest: "fixture-material"})
	rec, err := server.selectCapabilities(ctx, selectionQuery(c, g.MaterialID, false))
	if err != nil {
		t.Fatal(err)
	}
	g.Selection = &selection.SelectionRef{ID: rec.ID, Digest: rec.Digest}
	if err = store.validateSelection(ctx, store.Pool, g); err != nil {
		t.Fatal(err)
	}
	task, err := store.Propose(ctx, "planner", "selected", g, "fixture-runtime")
	if err != nil {
		t.Fatal(err)
	}
	changed := g
	ref := *g.Selection
	ref.Digest = "sha256:substituted"
	changed.Selection = &ref
	if err = store.validateSelection(ctx, store.Pool, changed); err == nil {
		t.Fatal("substituted selection accepted")
	}
	changed = g
	changed.Nodes = append([]Node{}, g.Nodes...)
	changed.Nodes[1].Expert = "parent-only@1"
	if err = store.validateSelection(ctx, store.Pool, changed); err == nil {
		t.Fatal("unselected member accepted")
	}
	store.CurrentCatalogDigest = "sha256:new-release"
	_, err = store.Admit(ctx, task.ID, task.GraphDigest, "fixture-user")
	requireCode(t, err, "BINDING_CHANGED")
	after, err := store.GetTask(ctx, task.ID)
	if err != nil || after.Status != "AWAITING_APPROVAL" {
		t.Fatal("changed binding partly admitted", err)
	}
	store.CurrentCatalogDigest = g.CatalogDigest
	if _, err = store.Admit(ctx, task.ID, task.GraphDigest, "fixture-user"); err != nil {
		t.Fatal(err)
	}
	backup, err := store.Backup(ctx, Files{Root: t.TempDir()}, c, server.Material)
	if err != nil {
		t.Fatal(err)
	}
	if backup.Payload.Version != "report-demo-backup/2" || len(backup.Payload.Tables["selection_records"]) != 1 {
		t.Fatal("selection omitted from authoritative snapshot")
	}
}

func TestPGBlockedTaskCannotUseOrdinaryResume(t *testing.T) {
	ctx, s, _, _, _, task, permit, _ := commitFixture(t)
	if _, err := s.MarkWaiting(ctx, task.ID, "extract", permit.Owner, permit.Epoch, "BLOCKED", "MODEL_BINDING_CHANGED"); err != nil {
		t.Fatal(err)
	}
	after, err := s.GetTask(ctx, task.ID)
	if err != nil || after.Status != "BLOCKED" {
		t.Fatal("blocked projection lost", err)
	}
	_, err = s.SetPause(ctx, task.ID, false)
	requireCode(t, err, "BINDING_CHANGED")
	pending, err := s.Pending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pending {
		if p == task.ID {
			t.Fatal("blocked task remained dispatchable")
		}
	}
	var active bool
	if err = s.Pool.QueryRow(ctx, "SELECT status='ACTIVE' FROM run_leases WHERE scope=$1 AND task_id=$2 AND domain='graph'", s.Scope, task.ID).Scan(&active); err != nil || active {
		t.Fatal("blocked graph still owns live permit", err)
	}
}
