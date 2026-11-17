package main

import (
	"encoding/json"
	"testing"
)

func TestWorkbenchHumanReviewUsesTrustedTaskPageAndCurrentCheck(t *testing.T) {
	s := &Server{Config: Config{PublicURL: "https://control.example"}}
	task := &Task{ID: "T-fixture", Graph: Graph{Nodes: []Node{{ID: "check"}}}, Nodes: map[string]*NodeState{
		"check": {Result: &Result{ArtifactID: "checker-artifact", Valid: true, Output: json.RawMessage(`{"outcome":"HUMAN","object_digest":"sha256:report"}`)}},
	}}
	review := s.workbenchHumanReview(task)
	if review == nil || review["url"] != "https://control.example/ui/tasks/T-fixture" {
		t.Fatal("missing trusted, credential-free review link")
	}
	checks := review["checks"].([]any)
	if len(checks) != 1 || checks[0].(map[string]string)["check_artifact"] != "checker-artifact" {
		t.Fatal("review link lost checker identity")
	}
	task.Nodes["check"].Result.Valid = false
	if s.workbenchHumanReview(task) != nil {
		t.Fatal("withdrawn check offered review")
	}
	task.Nodes["check"].Result.Valid = true
	task.Nodes["check"].Result.Output = json.RawMessage(`{"outcome":"PASS","object_digest":"sha256:report"}`)
	if s.workbenchHumanReview(task) != nil {
		t.Fatal("PASS offered unnecessary manual review")
	}
}
