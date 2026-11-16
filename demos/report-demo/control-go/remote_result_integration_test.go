package main

import (
	"encoding/json"
	"testing"
)

func TestPGRemoteCommitRequiresUniqueObservedBoundOutput(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		name := "candidate-tamper"
		if conflict {
			name = "contradictory-completions"
		}
		t.Run(name, func(t *testing.T) {
			ctx, s, cfg, cat, material, task, request, raw := commitFixture(t)
			v, err := s.BeginCall(ctx, task.ID, "extract", request.Owner, request.Epoch, json.RawMessage(`{"fixture":true}`), cat)
			if err != nil {
				t.Fatal(err)
			}
			call := v.(Call)
			_, err = s.Commit(ctx, task.ID, "extract", request, raw, cat, material.Digest)
			requireCode(t, err, "REMOTE_UNRESOLVED")
			observe := func(event string, body []byte) {
				var output any
				if err := DecodeStrict(body, &output); err != nil {
					t.Fatal(err)
				}
				payload, _ := json.Marshal(map[string]any{"id": "remote-task", "contextId": call.ContextID, "artifacts": []any{map[string]any{"parts": []any{map[string]any{"data": map[string]any{"call_id": call.ID, "input_digest": call.InputDigest, "attempt": call.Attempt, "output": output}}}}}})
				_, err := s.Observe(ctx, call.ID, "fixture-worker", Observation{EventID: event, State: "COMPLETED", RemoteTaskID: "remote-task", ContextID: call.ContextID, Payload: payload})
				if err != nil {
					t.Fatal(err)
				}
			}
			observe("complete-original", raw)
			changed := []byte(`{"fixture":"different/unobserved business output"}`)
			if conflict {
				observe("complete-contradictory", changed)
			} else {
				artifact, err := (Files{cfg.ArtifactRoot}).Put(changed)
				if err != nil {
					t.Fatal(err)
				}
				tampered := request
				tampered.ArtifactID = artifact
				_, err = s.Commit(ctx, task.ID, "extract", tampered, changed, cat, material.Digest)
				requireCode(t, err, "INPUT_CONFLICT")
			}
			v, err = s.Commit(ctx, task.ID, "extract", request, raw, cat, material.Digest)
			if conflict {
				requireCode(t, err, "INPUT_CONFLICT")
				after, err := s.GetTask(ctx, task.ID)
				if err != nil {
					t.Fatal(err)
				}
				if after.Nodes["extract"].Result != nil {
					t.Fatal("ambiguous remote output became a formal result")
				}
			} else if err != nil || v.(*Result).ArtifactID != request.ArtifactID {
				t.Fatal("valid observed result rejected", err)
			}
		})
	}
}
