package main

import (
	"os"
	"testing"
)

func TestPGReleaseDriftRejectsOldApprovalAndAllowsNewTask(t *testing.T) {
	ctx, s, cfg, cat := integration(t)
	_, g := sample(t)
	cfg.ExecutionProfile = "release-one"
	_, _, material, err := LoadConfig(os.Getenv("REPORT_DEMO_CONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	NewServer(s, cfg, cat, material)
	old, err := s.Propose(ctx, "planner", "before-release", g, "fixture-spec")
	if err != nil {
		t.Fatal(err)
	}
	s.ExecutionProfile = "release-two"
	_, err = s.Admit(ctx, old.ID, old.GraphDigest, "user")
	requireCode(t, err, "BINDING_CHANGED")
	unchanged, err := s.GetTask(ctx, old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Generation != old.Generation || unchanged.Status != "AWAITING_APPROVAL" || unchanged.ExecutionProfile != "release-one" {
		t.Fatal("old proposal mutated by failed approval")
	}
	fresh, err := s.Propose(ctx, "planner", "after-release", g, "fixture-spec")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Admit(ctx, fresh.ID, fresh.GraphDigest, "user"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Binding(ctx, fresh, "extract", cat, material.Digest); err != nil {
		t.Fatal(err)
	}
	_, err = s.Binding(ctx, old, "extract", cat, material.Digest)
	requireCode(t, err, "BINDING_CHANGED")
}
