package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withScenarioDir(t *testing.T, files map[string]string) error {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	before := map[string]*ScenarioPackage{}
	for k, v := range scenarios.packages {
		before[k] = v
	}
	t.Cleanup(func() {
		scenarios.packages = before
		scenarios.owner = map[string]string{}
		for id, p := range before {
			for _, c := range p.Capabilities {
				scenarios.owner[c.ID] = id
			}
		}
		rebuildCatalogData()
	})
	return loadScenarioDir(dir)
}

const extraPackage = `{
  "package": "credit-extra", "version": "1.0.0", "title": "extra approved capability",
  "governance": {"state": "%s", "approved_by": "test", "approved_at": "2026-11-17"},
  "rules": {"rule_set": "evidence-coverage-v1", "coverage": []},
  "caller_holds": [], "parent_bounds": {},
  "capabilities": [{"id": "collateral-appraiser", "version": "1.0.0", "name": "抵押物评估", "kind": "investigate",
    "summary": "评估抵押物价值", "instruction": "只依据工具返回的内容评估抵押物。",
    "requires": [{"kind": "data", "selector": "data:credit/collateral/*", "actions": ["read"], "effect": "none"}],
    "effects": {"level": "none"}, "bounds": {"cost_cents": 20, "calls": 1, "seconds": 120}}]
}`

// AR-06: a newly approved capability becomes available by adding a package,
// without changing code; a package that is not approved stays invisible.
func TestApprovedPackageAddsCapabilityWithoutCodeChange(t *testing.T) {
	if expertByID("collateral-appraiser") != nil {
		t.Fatal("fixture capability already present")
	}
	if err := withScenarioDir(t, map[string]string{"extra.json": strings.Replace(extraPackage, "%s", "approved", 1)}); err != nil {
		t.Fatal(err)
	}
	e := expertByID("collateral-appraiser")
	if e == nil || e.Package != "credit-extra" {
		t.Fatalf("approved capability not in the catalog: %+v", e)
	}
	if err := checkAgents(scenarios.get("credit-extra"), []PlanAgent{{ID: "collateral", Expert: "collateral-appraiser"}}); err != nil {
		t.Fatalf("planner cannot use the new capability: %v", err)
	}
	if err := checkAgents(scenarios.get(defaultScenario), []PlanAgent{{ID: "collateral", Expert: "collateral-appraiser"}}); err == nil {
		t.Fatal("a credit-dd plan used a capability of another package")
	}
}

func TestUnapprovedPackageStaysInvisible(t *testing.T) {
	err := withScenarioDir(t, map[string]string{"draft.json": strings.Replace(extraPackage, "%s", "draft", 1)})
	if err == nil || !strings.Contains(err.Error(), "not approved") {
		t.Fatalf("draft package accepted: %v", err)
	}
	if expertByID("collateral-appraiser") != nil {
		t.Fatal("capability from a draft package is visible")
	}
}

func TestPackageCannotRedefineAnotherPackagesCapability(t *testing.T) {
	clash := strings.Replace(strings.Replace(extraPackage, "%s", "approved", 1), "collateral-appraiser", "intake", 1)
	if err := withScenarioDir(t, map[string]string{"clash.json": clash}); err == nil || !strings.Contains(err.Error(), "already provided") {
		t.Fatalf("package redefined intake: %v", err)
	}
	if e := expertByID("intake"); e == nil || e.Package != defaultScenario {
		t.Fatalf("intake changed owner: %+v", e)
	}
}
