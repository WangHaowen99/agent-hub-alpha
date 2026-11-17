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
			v, err := s.BeginCall(ctx, task.ID, "extract", request.Owner, request.Epoch, routeFixture(t, cat, "extractor@1"), cat)
			if err != nil {
				t.Fatal(err)
			}
			call := v.(CallDecision).Call
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

func TestPGRemoteOutputRequiresActualModelBindingEvidence(t *testing.T) {
	for _, mode := range []string{"missing", "wrong-model", "matching"} {
		t.Run(mode, func(t *testing.T) {
			ctx, s, _, cat, _, task, request, raw := commitFixture(t)
			v, err := s.BeginCall(ctx, task.ID, "extract", request.Owner, request.Epoch, routeFixture(t, cat, "extractor@1"), cat)
			if err != nil {
				t.Fatal(err)
			}
			call := v.(CallDecision).Call
			_, err = s.Pool.Exec(ctx, `UPDATE tasks SET document=jsonb_set(document,'{model_binding_digest}','"sha256:model-a"') WHERE scope=$1 AND id=$2`, s.Scope, task.ID)
			if err != nil {
				t.Fatal(err)
			}
			var output any
			if err = DecodeStrict(raw, &output); err != nil {
				t.Fatal(err)
			}
			envelope := map[string]any{"call_id": call.ID, "input_digest": call.InputDigest, "attempt": call.Attempt, "output": output}
			if mode != "missing" {
				model := "sha256:model-a"
				if mode == "wrong-model" {
					model = "sha256:model-b"
				}
				envelope["runtime_evidence"] = map[string]any{"agent_id": call.AgentID, "model_binding_digest": model}
			}
			payload, _ := json.Marshal(map[string]any{"id": "model-evidence-task", "contextId": call.ContextID, "artifacts": []any{map[string]any{"parts": []any{map[string]any{"data": envelope}}}}})
			_, err = s.Observe(ctx, call.ID, "fixture-worker", Observation{EventID: "evidence", State: "COMPLETED", RemoteTaskID: "model-evidence-task", ContextID: call.ContextID, Payload: payload})
			if err != nil {
				t.Fatal(err)
			}
			call.RemoteTaskID = "model-evidence-task"
			tx, err := s.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			digests, err := s.completedOutputDigests(ctx, tx, call)
			if err != nil {
				t.Fatal(err)
			}
			if (mode == "matching") != (len(digests) == 1) {
				t.Fatal("model evidence qualification differs", mode, digests)
			}
		})
	}
}
