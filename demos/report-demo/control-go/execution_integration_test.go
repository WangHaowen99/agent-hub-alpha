package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/openjiuwen/agent-festival/go/internal/ledger"
)

func TestPGSharedIdentityAndReconciliationConsumer(t *testing.T) {
	ctx, s, cfg, cat, material, task, request, raw := commitFixture(t)
	if task.IdentityContract != pr25Contract {
		t.Fatal("shared identity not adopted")
	}
	_, err := s.BeginCall(ctx, task.ID, "extract", request.Owner, request.Epoch, json.RawMessage(`{"fixture":"accepted but uncommitted"}`), cat)
	if err != nil {
		t.Fatal(err)
	}
	query := func(h string) map[string]any {
		r := httptest.NewRequest("GET", "/v1/tasks/"+task.ID+"/nodes/extract/result?input_digest="+h, nil)
		r.Header.Set("Authorization", "Bearer "+cfg.Tokens["worker"])
		w := httptest.NewRecorder()
		NewServer(s, cfg, cat, material).Routes().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal("reconciliation HTTP", w.Code, w.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	before, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"", "expert", "planner"} {
		req := httptest.NewRequest("GET", "/v1/tasks/"+task.ID+"/nodes/extract/result?input_digest="+request.InputDigest, nil)
		if role != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.Tokens[role])
		}
		response := httptest.NewRecorder()
		NewServer(s, cfg, cat, material).Routes().ServeHTTP(response, req)
		if response.Code != 401 && response.Code != 403 {
			t.Fatal("untrusted reconciliation read allowed", role, response.Code)
		}
	}
	out := query(request.InputDigest)
	if out["committed"] != false || out["serve"] != false || out["execution_permission_granted"] != false {
		t.Fatal("uncommitted call granted re-execution")
	}
	after, err := s.GetTask(ctx, task.ID)
	if err != nil || before.Generation != after.Generation {
		t.Fatal("query mutated authority", err)
	}
	// Complete the same recorded call so C03 qualifies an observed output.
	calls, err := s.Calls(ctx, task.ID, "extract")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Observe(ctx, calls[0].ID, "fixture-worker", completedFixtureObservation(t, calls[0], "identity-completed", raw)); err != nil {
		t.Fatal(err)
	}
	v, err := s.Commit(ctx, task.ID, "extract", request, raw, cat, material.Digest)
	if err != nil {
		t.Fatal(err)
	}
	r := v.(*Result)
	if r.Identity == nil || ledger.ValidateIdentity(*r.Identity) != nil {
		t.Fatal("formal result lacks shared identity")
	}
	if r.Identity.LogicalExecution.TaskID != task.ID || r.Identity.LogicalExecution.NodePath != "extract" {
		t.Fatal("identity does not describe actual consumer")
	}
	out = query(request.InputDigest)
	if out["committed"] != true || out["serve"] != true {
		t.Fatal("same input not served")
	}
	out = query(r.Receipt)
	if out["committed"] != true || out["serve"] != false || out["state"] != "CONFLICT" {
		t.Fatal("receipt accepted as input")
	}
	var persisted []byte
	if err = s.Pool.QueryRow(ctx, "SELECT identity FROM committed_results WHERE scope=$1 AND task_id=$2 AND node_id='extract'", s.Scope, task.ID).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	var identity ledger.Identity
	if err = json.Unmarshal(persisted, &identity); err != nil || ledger.ValidateIdentity(identity) != nil {
		t.Fatal("identity not durably stored", err)
	}
	snapshot, err := s.Backup(ctx, Files{cfg.ArtifactRoot}, cat, material)
	if err != nil {
		t.Fatal(err)
	}
	if err = snapshot.Validate(); err != nil {
		t.Fatal("shared identity backup invalid", err)
	}
	for i, rawRow := range snapshot.Payload.Tables["committed_results"] {
		var row map[string]any
		if err = json.Unmarshal(rawRow, &row); err != nil {
			t.Fatal(err)
		}
		row["identity"].(map[string]any)["logical_execution"].(map[string]any)["node_path"] = "forged-node"
		forged, _ := json.Marshal(row)
		snapshot.Payload.Tables["committed_results"][i] = forged
		snapshot.Digest, _ = Digest(snapshot.Payload)
		if snapshot.Validate() == nil {
			t.Fatal("rehashed backup hid conflicting identity")
		}
		snapshot.Payload.Tables["committed_results"][i] = rawRow
		break
	}
	// A different row/attempt cannot create a second result for the same o.
	_, err = s.Pool.Exec(ctx, `INSERT INTO committed_results(scope,task_id,node_id,input_digest,artifact_id,artifact_digest,receipt,output,identity) SELECT scope,task_id,'another-node',input_digest,artifact_id,artifact_digest,'another-receipt',output,identity FROM committed_results WHERE scope=$1 AND task_id=$2 AND node_id='extract'`, s.Scope, task.ID)
	if pg, ok := err.(*pgconn.PgError); !ok || pg.Code != "23505" {
		t.Fatal("structured logical execution not unique", err)
	}
	if _, err = s.Invalidate(ctx, r.ArtifactID, "fixture withdrawal", "reviewer"); err != nil {
		t.Fatal(err)
	}
	out = query(request.InputDigest)
	if out["committed"] != true || out["serve"] != false || out["state"] != "INVALIDATED" {
		t.Fatal("withdrawn result was served")
	}
	saveCommitEvidence(t, "shared-contract-consumer", map[string]any{"task_id": task.ID, "identity": identity, "durable_identity": true, "same_logical_execution_unique": true, "receipt_not_input": true, "withdrawal_blocks_serve": true, "query_has_no_execution_permission": true, "scope": "actual Go/PG/HTTP; fixture provider, no model"})
}

func TestSharedIdentityKeepsLogicalExecutionAcrossAttempts(t *testing.T) {
	task := &Task{ID: "T1", Revision: "g1", IdentityContract: pr25Contract}
	one, err := executionIdentity(task, "quality", 1, "1")
	if err != nil {
		t.Fatal(err)
	}
	two, err := executionIdentity(task, "quality", 2, "2")
	if err != nil {
		t.Fatal(err)
	}
	if *one.LogicalExecution != *two.LogicalExecution || one.OperationID == two.OperationID || *one.ExecutionGeneration == *two.ExecutionGeneration {
		t.Fatal("o/a/e conflated")
	}
	legacy := *task
	legacy.IdentityContract = ""
	if id, err := executionIdentity(&legacy, "quality", 1, "1"); err != nil || id != nil {
		t.Fatal("legacy evidence rewritten", err)
	}
}
