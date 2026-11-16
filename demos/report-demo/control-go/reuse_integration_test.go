package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPGCrossTaskReuseVerifiesBytesAndPropagatesRevocation(t *testing.T) {
	ctx, s, _, cat := integration(t)
	_, graph := sample(t)
	files := Files{Root: t.TempDir()}
	s.ReuseFiles = &files
	create := func(command string, goal string, changedInstruction bool) (*Task, Lease) {
		g := graph
		g.Nodes = append([]Node(nil), graph.Nodes...)
		g.Goal = goal
		if changedInstruction {
			g.Nodes[0].Task = "different extraction instruction"
		}
		task, err := s.Propose(ctx, "planner", command, g, "compiler-fixture")
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
		return task, leaseFrom(t, v)
	}
	release := func(id string) {
		if _, err := s.Pool.Exec(ctx, "UPDATE run_leases SET status='RELEASED' WHERE scope=$1 AND task_id=$2", s.Scope, id); err != nil {
			t.Fatal(err)
		}
	}
	first, lease := create("first", "first report goal", false)
	v, err := s.ResolveNode(ctx, first.ID, "extract", lease.Owner, lease.Epoch, "", cat, "fixed-source")
	if err != nil {
		t.Fatal(err)
	}
	permit := v.(map[string]any)
	raw := []byte(`{"source":"fixture; not a model result","indicators":[]}`)
	artifact, err := files.Put(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Commit(ctx, first.ID, "extract", CommitRequest{lease.Owner, lease.Epoch, permit["input_digest"].(string), permit["attempt"].(int), artifact}, raw, cat, "fixed-source"); err != nil {
		t.Fatal(err)
	}
	release(first.ID)
	second, lease := create("second", "different report goal; same extractor input", false)
	v, err = s.ResolveNode(ctx, second.ID, "extract", lease.Owner, lease.Epoch, "", cat, "fixed-source")
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["status"] != "REUSE" {
		t.Fatal("compatible extraction recomputed")
	}
	state, err := s.GetTask(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	result := state.Nodes["extract"].Result
	if result.ReusedFrom == nil || result.ReusedFrom.TaskID != first.ID || result.InputDigest == permit["input_digest"] || state.Nodes["extract"].Attempt != 0 {
		t.Fatal("reuse lineage or task-specific binding lost")
	}
	release(second.ID)
	third, lease := create("third", "same material", true)
	v, err = s.ResolveNode(ctx, third.ID, "extract", lease.Owner, lease.Epoch, "", cat, "fixed-source")
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["status"] != "GRANTED" {
		t.Fatal("changed instruction reused")
	}
	release(third.ID)
	fourth, lease := create("fourth", "same extractor input", false)
	if err = os.WriteFile(filepath.Join(files.Root, artifact+".json"), []byte(`{"corrupted":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = s.ResolveNode(ctx, fourth.ID, "extract", lease.Owner, lease.Epoch, "", cat, "fixed-source")
	requireCode(t, err, "EVIDENCE_CHANGED")
	if err = os.WriteFile(filepath.Join(files.Root, artifact+".json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Invalidate(ctx, artifact, "source correction fixture", "reviewer"); err != nil {
		t.Fatal(err)
	}
	for _, task := range []*Task{first, second} {
		state, err = s.GetTask(ctx, task.ID)
		if err != nil {
			t.Fatal(err)
		}
		if state.Nodes["extract"].Result.Valid || state.Status != "NEEDS_REVIEW" {
			b, _ := json.Marshal(state)
			t.Fatalf("reuse survived invalidation: %s", b)
		}
	}
}
