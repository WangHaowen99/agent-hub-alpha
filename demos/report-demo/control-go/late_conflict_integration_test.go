package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func completedFixtureObservation(t *testing.T, call Call, event string, output []byte) Observation {
	t.Helper()
	var value any
	if err := DecodeStrict(output, &value); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"id": "late-fixture-remote-task", "contextId": call.ContextID, "artifacts": []any{map[string]any{"parts": []any{map[string]any{"data": map[string]any{"call_id": call.ID, "input_digest": call.InputDigest, "attempt": call.Attempt, "output": value}}}}}})
	return Observation{EventID: event, State: "COMPLETED", RemoteTaskID: "late-fixture-remote-task", ContextID: call.ContextID, Payload: payload}
}

func TestPGLateRemoteConflictWithdrawsFormalResultsAndCloneDelivery(t *testing.T) {
	for _, kind := range []string{"different-output", "contradictory-terminal"} {
		t.Run(kind, func(t *testing.T) {
			ctx, s, cfg, cat, material, source, request, raw := commitFixture(t)
			v, err := s.BeginCall(ctx, source.ID, "extract", request.Owner, request.Epoch, json.RawMessage(`{"fixture":"late conflict; no model/provider call"}`), cat)
			if err != nil {
				t.Fatal(err)
			}
			call := v.(Call)
			observation := completedFixtureObservation(t, call, "first-completed", raw)
			if _, err = s.Observe(ctx, call.ID, "fixture-worker", observation); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Commit(ctx, source.ID, "extract", request, raw, cat, material.Digest); err != nil {
				t.Fatal(err)
			}
			files := Files{cfg.ArtifactRoot}
			finish := func(task *Task, lease Lease, graph Graph) string {
				report := ""
				for _, node := range graph.Nodes {
					if node.ID == "extract" {
						continue
					}
					v, err := s.ResolveNode(ctx, task.ID, node.ID, lease.Owner, lease.Epoch, "", cat, material.Digest)
					if err != nil {
						t.Fatal(err)
					}
					permit := v.(map[string]any)
					var body []byte
					if node.ID == "check" {
						body, _ = json.Marshal(map[string]string{"outcome": "PASS", "object_digest": "sha256:" + report, "checker_version": "control-fixture/1"})
					} else {
						body, _ = json.Marshal(map[string]string{"fixture": "control only, not model/business acceptance", "task": task.ID, "node": node.ID})
					}
					artifact, err := files.Put(body)
					if err != nil {
						t.Fatal(err)
					}
					if _, err = s.Commit(ctx, task.ID, node.ID, CommitRequest{lease.Owner, lease.Epoch, permit["input_digest"].(string), permit["attempt"].(int), artifact}, body, cat, material.Digest); err != nil {
						t.Fatal(err)
					}
					if node.ID == "draft" {
						report = artifact
					}
				}
				if _, err = s.Complete(ctx, task.ID, lease.Owner, lease.Epoch); err != nil {
					t.Fatal(err)
				}
				return report
			}
			finish(source, Lease{Owner: request.Owner, Epoch: request.Epoch}, source.Graph)
			graph := source.Graph
			graph.Goal = "another report using the same extraction"
			graph.Nodes = append(append([]Node(nil), graph.Nodes...), Node{ID: "check", Expert: "checker@1", Task: "control fixture check", DependsOn: []string{"draft"}})
			clone, err := s.Propose(ctx, "fixture-planner", "clone", graph, "compiler-fixture")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.Admit(ctx, clone.ID, clone.GraphDigest, "reviewer"); err != nil {
				t.Fatal(err)
			}
			v, err = s.ClaimRun(ctx, clone.ID, "clone-worker", "graph")
			if err != nil {
				t.Fatal(err)
			}
			lease := leaseFrom(t, v)
			v, err = s.ResolveNode(ctx, clone.ID, "extract", lease.Owner, lease.Epoch, "", cat, material.Digest)
			if err != nil {
				t.Fatal(err)
			}
			if v.(map[string]any)["status"] != "REUSE" {
				t.Fatal("fixture did not exercise actual cross-task extraction reuse")
			}
			report := finish(clone, lease, graph)
			if _, err = s.ApproveReport(ctx, clone.ID, report, "reviewer"); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Deliver(ctx, clone.ID, report, "already-delivered", "reviewer"); err != nil {
				t.Fatal(err)
			}
			before, err := s.GetTask(ctx, source.ID)
			if err != nil {
				t.Fatal(err)
			}
			original := before.Nodes["extract"].Result
			observation = completedFixtureObservation(t, call, "late-contradiction", []byte(`{"fixture":"different remote business output"}`))
			if kind == "contradictory-terminal" {
				observation.State = "FAILED"
				observation.Payload = json.RawMessage(`{"fixture":"trusted late terminal failure"}`)
			}
			// An actual required-audit failure must roll back the observation,
			// call conflict and every source/clone validity change together.
			server := NewServer(s, cfg, cat, material)
			removeFault := effectAuditFault(t, server, "INVALIDATED")
			if _, err = s.Observe(ctx, call.ID, "fixture-worker", observation); err == nil {
				t.Fatal("conflict withdrawal ignored required audit failure")
			}
			for _, task := range []*Task{source, clone} {
				unchanged, err := s.GetTask(ctx, task.ID)
				if err != nil {
					t.Fatal(err)
				}
				for _, node := range unchanged.Nodes {
					if node.Result != nil && !node.Result.Valid {
						t.Fatal("failed observation partially withdrew lineage")
					}
				}
			}
			stored, err := s.Calls(ctx, source.ID, "extract")
			if err != nil || stored[0].Conflicted {
				t.Fatal("failed observation persisted half a conflict", err)
			}
			removeFault()
			if _, err = s.Observe(ctx, call.ID, "fixture-worker", observation); err != nil {
				t.Fatal(err)
			}
			for _, task := range []*Task{source, clone} {
				after, err := s.GetTask(ctx, task.ID)
				if err != nil {
					t.Fatal(err)
				}
				if after.Status != "NEEDS_REVIEW" {
					t.Fatal("late conflict left formal task eligible for success", after.Status)
				}
				for _, node := range after.Nodes {
					if node.Result != nil && node.Result.Valid {
						t.Fatal("late conflict did not withdraw formal result lineage")
					}
				}
			}
			after, err := s.GetTask(ctx, source.ID)
			if err != nil {
				t.Fatal(err)
			}
			retained := after.Nodes["extract"].Result
			if retained.ArtifactID != original.ArtifactID || retained.Receipt != original.Receipt || retained.InputDigest != original.InputDigest {
				t.Fatal("withdrawal overwrote immutable formal identity")
			}
			body, err := files.Read(original.ArtifactID)
			if err != nil || string(body) != string(raw) {
				t.Fatal("withdrawal rewrote original artifact", err)
			}
			if _, err = s.ApproveReport(ctx, clone.ID, report, "reviewer"); err == nil {
				t.Fatal("conflicting lineage could be approved")
			}
			for _, b := range []string{"already-delivered", "new-delivery"} {
				if _, err = s.Deliver(ctx, clone.ID, report, b, "reviewer"); err == nil {
					t.Fatal("old/new delivery identity bypassed late conflict")
				}
			}
			session := newID("fixture-read-")
			if _, err = s.Pool.Exec(ctx, "INSERT INTO sessions(id_digest,principal,csrf_digest,expires_at) VALUES($1,'reviewer',$2,clock_timestamp()+interval '1 hour')", tokenDigest(session), tokenDigest("fixture-csrf")); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest("GET", "/v1/tasks/"+clone.ID+"/artifacts/"+report+"/download", nil)
			req.AddCookie(&http.Cookie{Name: "report_session", Value: session})
			response := httptest.NewRecorder()
			server.Routes().ServeHTTP(response, req)
			if response.Code != 403 {
				t.Fatal("withdrawn cloned report remained downloadable", response.Code)
			}
			if _, err = s.Observe(ctx, call.ID, "fixture-worker", observation); err != nil {
				t.Fatal(err)
			}
			var cases int
			if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM cases_and_validity WHERE scope=$1 AND artifact_id=$2", s.Scope, original.ArtifactID).Scan(&cases); err != nil || cases != 1 {
				t.Fatal("duplicate callback repeated invalidation", err, cases)
			}
			saveCommitEvidence(t, "late-remote-conflict-"+kind, map[string]any{"source_task": source.ID, "clone_task": clone.ID, "original_artifact": original.ArtifactID, "immutable_identity_retained": true, "source_and_clone_withdrawn": true, "required_audit_rollback_verified": true, "download_http": response.Code, "cases": cases, "scope": "real PG/control/HTTP; no model or external provider"})
		})
	}
}

func TestPGCompletedRemoteCallIgnoresLateNonterminalOrder(t *testing.T) {
	ctx, s, _, cat, _, task, request, raw := commitFixture(t)
	v, err := s.BeginCall(ctx, task.ID, "extract", request.Owner, request.Epoch, json.RawMessage(`{"fixture":true}`), cat)
	if err != nil {
		t.Fatal(err)
	}
	call := v.(Call)
	o := completedFixtureObservation(t, call, "completed", raw)
	if _, err = s.Observe(ctx, call.ID, "fixture-worker", o); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"ACCEPTED", "WORKING", "UNKNOWN", "TRANSPORT"} {
		o.State = state
		o.EventID = "late-" + state
		if _, err = s.Observe(ctx, call.ID, "fixture-worker", o); err != nil {
			t.Fatal(err)
		}
	}
	calls, err := s.Calls(ctx, task.ID, "extract")
	if err != nil || calls[0].State != "COMPLETED" || calls[0].Conflicted {
		t.Fatal("out-of-order nonterminal observation falsely conflicted with completed fact", err)
	}
}
