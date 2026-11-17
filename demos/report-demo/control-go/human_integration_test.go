package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestPGHumanReviewDoesNotRewriteCheckerAndRevocationStillWins(t *testing.T) {
	ctx, s, cfg, cat := integration(t)
	_, g := sample(t)
	g.Nodes = append(g.Nodes, Node{ID: "check", Expert: "checker@1", Role: "program", Task: "isolated checker fixture", DependsOn: []string{"draft"}})
	task, err := s.Propose(ctx, "planner", "human-path", g, "compiler-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Admit(ctx, task.ID, task.GraphDigest, "test-reviewer"); err != nil {
		t.Fatal(err)
	}
	v, err := s.ClaimRun(ctx, task.ID, "test-worker", "graph")
	if err != nil {
		t.Fatal(err)
	}
	lease := leaseFrom(t, v)
	files := Files{Root: t.TempDir()}
	var report, check, digest string
	for _, node := range g.Nodes {
		v, err = s.ResolveNode(ctx, task.ID, node.ID, lease.Owner, lease.Epoch, "", cat, "fixed-source")
		if err != nil {
			t.Fatal(err)
		}
		permit := v.(map[string]any)
		var raw []byte
		if node.ID == "check" {
			raw, _ = json.Marshal(map[string]any{"checker_version": "control-fixture/1", "object_digest": digest, "outcome": "HUMAN", "failures": []string{}, "human_review": []string{"fixture; not business acceptance"}})
		} else {
			raw, _ = json.Marshal(map[string]string{"fixture": node.ID})
		}
		artifact, err := files.Put(raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.Commit(ctx, task.ID, node.ID, CommitRequest{lease.Owner, lease.Epoch, permit["input_digest"].(string), permit["attempt"].(int), artifact}, raw, cat, "fixed-source"); err != nil {
			t.Fatal(err)
		}
		if node.ID == "draft" {
			report = artifact
			digest = "sha256:" + artifact
		}
		if node.ID == "check" {
			check = artifact
		}
	}
	completion, err := s.Complete(ctx, task.ID, lease.Owner, lease.Epoch)
	if err != nil {
		t.Fatal(err)
	}
	if completion.(map[string]any)["status"] != "WAIT" {
		t.Fatal("unresolved HUMAN completed")
	}
	if _, err = s.ApproveReport(ctx, task.ID, report, "test-reviewer"); err == nil {
		t.Fatal("HUMAN bypassed")
	}
	in := CheckReview{CheckArtifact: check, ObjectDigest: digest, Decision: "ACCEPT", Reason: "CONTROL TEST ONLY: simulate a qualified reviewer decision; not a real business signature"}
	server := &Server{Store: s, Config: cfg, Catalog: cat, Files: files}
	readReview := func() map[string]any {
		r := httptest.NewRequest("GET", "/v1/workbench/tasks/"+task.ID, nil)
		r.Header.Set("Authorization", "Bearer "+cfg.Tokens["operator"])
		w := httptest.NewRecorder()
		server.Routes().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("workbench projection rejected: %d", w.Code)
		}
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if readReview()["human_review"] == nil {
		t.Fatal("HUMAN task has no trusted fallback in the HTTP projection")
	}
	if _, err = s.ReviewCheck(ctx, task.ID, "check", in, "test-reviewer"); err != nil {
		t.Fatal(err)
	}
	if readReview()["human_review"] != nil {
		t.Fatal("resolved check still offers pending manual review")
	}
	in.Reason = "changed explanation is a new fact and cannot replace the original review"
	_, err = s.ReviewCheck(ctx, task.ID, "check", in, "test-reviewer")
	requireCode(t, err, "INPUT_CONFLICT")
	state, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	var original map[string]any
	json.Unmarshal(state.Nodes["check"].Result.Output, &original)
	if original["outcome"] != "HUMAN" || effectiveCheck(state.Nodes["check"]) != "PASS" {
		t.Fatal("checker fact and human authority conflated")
	}
	if _, err = s.SetPause(ctx, task.ID, false); err != nil {
		t.Fatal(err)
	}
	v, err = s.ClaimRun(ctx, task.ID, "test-worker-after-review", "graph")
	if err != nil {
		t.Fatal(err)
	}
	lease = leaseFrom(t, v)
	if _, err = s.Complete(ctx, task.ID, lease.Owner, lease.Epoch); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApproveReport(ctx, task.ID, report, "test-reviewer"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Deliver(ctx, task.ID, report, "human-delivery-fixture", "test-reviewer"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Invalidate(ctx, check, "withdraw checker basis", "test-reviewer"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Deliver(ctx, task.ID, report, "new-delivery-after-withdrawal", "test-reviewer"); err == nil {
		t.Fatal("withdrawn checker allowed delivery")
	}
}
