package main

import (
	"encoding/json"
	"testing"
)

func TestPGBusEnvironmentChangePreservesOldTaskAndRejectsDispatch(t *testing.T) {
	ctx, s, _, cat := integration(t)
	_, g := sample(t)
	s.BusBinding = &BusBinding{"env-one", "tenant-a", "https://issuer.example/realms/demo", "demo", "worker-a"}
	task, err := s.Propose(ctx, "planner", "bus-original", g, "fixture-spec")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Admit(ctx, task.ID, task.GraphDigest, "reviewer"); err != nil {
		t.Fatal(err)
	}
	v, err := s.ClaimRun(ctx, task.ID, "worker", "graph")
	if err != nil {
		t.Fatal(err)
	}
	lease := leaseFrom(t, v)
	if _, err = s.ResolveNode(ctx, task.ID, "extract", lease.Owner, lease.Epoch, "", cat, "source"); err != nil {
		t.Fatal(err)
	}
	before, _ := s.GetTask(ctx, task.ID)
	for _, field := range []string{"tenant", "environment", "issuer", "audience", "caller", "missing"} {
		t.Run(field, func(t *testing.T) {
			changed := *task.BusBinding
			switch field {
			case "tenant":
				changed.Tenant = "tenant-b"
			case "environment":
				changed.EnvironmentID = "env-two"
			case "issuer":
				changed.Issuer = "https://other.example/realms/demo"
			case "audience":
				changed.Audience = "another-app"
			case "caller":
				changed.CallerSubject = "another-worker"
			}
			s.BusBinding = &changed
			if field == "missing" {
				s.BusBinding = nil
			}
			_, err := s.ResolveNode(ctx, task.ID, "extract", lease.Owner, lease.Epoch, "", cat, "source")
			requireCode(t, err, "BUS_BINDING_CHANGED")
			_, err = s.BeginCall(ctx, task.ID, "extract", lease.Owner, lease.Epoch, json.RawMessage(`{}`), cat)
			requireCode(t, err, "BUS_BINDING_CHANGED")
			after, err := s.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(before)
			b, _ := json.Marshal(after)
			if string(a) != string(b) {
				t.Fatal("rejected environment change rewrote task history")
			}
			var count int
			if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM remote_calls WHERE scope=$1 AND task_id=$2", s.Scope, task.ID).Scan(&count); err != nil || count != 0 {
				t.Fatal("rejection persisted a send intent", err)
			}
		})
	}
	// Missing historical bindings are not filled from the current deployment.
	s.BusBinding = nil
	legacy, err := s.Propose(ctx, "planner", "legacy-no-bus", g, "fixture-spec")
	if err != nil {
		t.Fatal(err)
	}
	s.BusBinding = task.BusBinding
	_, err = s.Admit(ctx, legacy.ID, legacy.GraphDigest, "reviewer")
	requireCode(t, err, "BUS_BINDING_CHANGED")
	_, err = s.Binding(ctx, legacy, "extract", cat, "source")
	requireCode(t, err, "BUS_BINDING_CHANGED")
	fresh, err := s.Propose(ctx, "planner", "fresh-bound", g, "fixture-spec")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Admit(ctx, fresh.ID, fresh.GraphDigest, "reviewer"); err != nil {
		t.Fatal(err)
	}
}
