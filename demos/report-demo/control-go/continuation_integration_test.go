package main

import (
	"encoding/json"
	"testing"
)

func TestPGRemoteInputAndAuthResumeReconcileSameCall(t *testing.T) {
	for _, waiting := range []string{"WAITING_INPUT", "WAITING_AUTH"} {
		t.Run(waiting, func(t *testing.T) {
			ctx, s, _, cat := integration(t)
			_, graph := sample(t)
			task, err := s.Propose(ctx, "planner", "resume", graph, "compiler-fixture")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.Admit(ctx, task.ID, task.GraphDigest, "test-user"); err != nil {
				t.Fatal(err)
			}
			v, err := s.ClaimRun(ctx, task.ID, "worker-1", "graph")
			if err != nil {
				t.Fatal(err)
			}
			old := leaseFrom(t, v)
			v, err = s.ResolveNode(ctx, task.ID, "extract", old.Owner, old.Epoch, "", cat, "fixed-source")
			if err != nil {
				t.Fatal(err)
			}
			permit := v.(map[string]any)
			v, err = s.BeginCall(ctx, task.ID, "extract", old.Owner, old.Epoch, json.RawMessage(`{"fixture":"native remote continuation"}`), cat)
			if err != nil {
				t.Fatal(err)
			}
			call := v.(Call)
			state := "INPUT_REQUIRED"
			if waiting == "WAITING_AUTH" {
				state = "AUTH_REQUIRED"
			}
			_, err = s.Observe(ctx, call.ID, "test-provider", Observation{EventID: "waiting", State: state, RemoteTaskID: "existing-remote-task", ContextID: call.ContextID, Payload: json.RawMessage(`{"fixture":true}`)})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.MarkWaiting(ctx, task.ID, "extract", old.Owner, old.Epoch, waiting, state); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Complete(ctx, task.ID, old.Owner, old.Epoch); err != nil {
				t.Fatal(err)
			}
			if _, err = s.SetPause(ctx, task.ID, false); err != nil {
				t.Fatal(err)
			}
			v, err = s.ClaimRun(ctx, task.ID, "worker-2", "graph")
			if err != nil {
				t.Fatal(err)
			}
			current := leaseFrom(t, v)
			v, err = s.ResolveNode(ctx, task.ID, "extract", current.Owner, current.Epoch, "", cat, "fixed-source")
			if err != nil {
				t.Fatal(err)
			}
			retry := v.(map[string]any)
			if retry["status"] != "WAIT" || retry["attempt"] != permit["attempt"] || retry["input_digest"] != permit["input_digest"] {
				t.Fatalf("resume granted a new send: %#v", retry)
			}
			calls, err := s.Calls(ctx, task.ID, "extract")
			if err != nil {
				t.Fatal(err)
			}
			if len(calls) != 1 || calls[0].ID != call.ID || calls[0].MessageID != call.MessageID || calls[0].RemoteTaskID != "existing-remote-task" {
				t.Fatal("remote identity changed")
			}
			after, err := s.GetTask(ctx, task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if after.GraphDigest != task.GraphDigest || after.Nodes["extract"].Result != nil {
				t.Fatal("resume changed graph or invented result")
			}
		})
	}
}
