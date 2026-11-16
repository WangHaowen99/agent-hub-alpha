package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

func integration(t *testing.T) (context.Context, *Store, Config, Catalog) {
	t.Helper()
	path := os.Getenv("REPORT_DEMO_CONFIG")
	if path == "" {
		t.Skip("REPORT_DEMO_CONFIG needed for real PostgreSQL integration")
	}
	cfg, c, _, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	scope := newID("test-")
	migrator, err := OpenStore(ctx, cfg.MigratorDSN, scope, 30)
	if err != nil {
		t.Fatal(err)
	}
	if err = migrator.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	migrator.Pool.Close()
	s, err := OpenStore(ctx, cfg.DSN, scope, 30)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Pool.Close() })
	return ctx, s, cfg, c
}
func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var f *Fault
	if !errors.As(err, &f) || f.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}
func leaseFrom(t *testing.T, v any) Lease {
	t.Helper()
	l, ok := v.(Lease)
	if !ok {
		t.Fatalf("unexpected lease %T", v)
	}
	return l
}

func TestPGConcurrentCommandAndFormalResult(t *testing.T) {
	ctx, s, _, c := integration(t)
	_, g := sample(t)
	ids := make(chan string, 12)
	errs := make(chan error, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := s.Propose(ctx, "planner", "one-command", g, "runtime-digest")
			if err != nil {
				errs <- err
				return
			}
			ids <- task.ID
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	id := ""
	for got := range ids {
		if id == "" {
			id = got
		}
		if id != got {
			t.Fatal("concurrent command created multiple tasks")
		}
	}
	g.Goal = "changed"
	_, err := s.Propose(ctx, "planner", "one-command", g, "runtime-digest")
	requireCode(t, err, "INPUT_CONFLICT")
	task, err := s.GetTask(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Admit(ctx, id, task.GraphDigest, "user"); err != nil {
		t.Fatal(err)
	}
	v, err := s.ClaimRun(ctx, id, "worker-1", "graph")
	if err != nil {
		t.Fatal(err)
	}
	l := leaseFrom(t, v)
	_, err = s.ResolveNode(ctx, id, "business", l.Owner, l.Epoch, "", c, "sha256:source")
	requireCode(t, err, "WAIT")
	v, err = s.ResolveNode(ctx, id, "extract", l.Owner, l.Epoch, "", c, "sha256:source")
	if err != nil {
		t.Fatal(err)
	}
	granted := v.(map[string]any)
	files := Files{t.TempDir()}
	raw := []byte(`{"indicators":[],"source":"test-provider"}`)
	artifact, err := files.Put(raw)
	if err != nil {
		t.Fatal(err)
	}
	r := CommitRequest{l.Owner, l.Epoch, granted["input_digest"].(string), granted["attempt"].(int), artifact}
	results := make(chan *Result, 8)
	errs = make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := s.Commit(ctx, id, "extract", r, raw, c, "sha256:source")
			if err != nil {
				errs <- err
				return
			}
			results <- v.(*Result)
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	receipt := ""
	for result := range results {
		if receipt == "" {
			receipt = result.Receipt
		}
		if receipt != result.Receipt {
			t.Fatal("multiple formal receipts")
		}
	}
	var count int
	if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM committed_results WHERE scope=$1 AND task_id=$2", s.Scope, id).Scan(&count); err != nil || count != 1 {
		t.Fatalf("formal count %d err %v", count, err)
	}
	_, err = s.ResolveNode(ctx, id, "extract", l.Owner, "old-epoch", "sha256:different", c, "sha256:source")
	requireCode(t, err, "INPUT_CONFLICT")
	v, err = s.ResolveNode(ctx, id, "extract", l.Owner, "old-epoch", "", c, "sha256:source")
	if err != nil || v.(map[string]any)["status"] != "REUSE" {
		t.Fatalf("authorized result reread needs no write permit: %v", err)
	}
	v, err = s.Complete(ctx, id, l.Owner, l.Epoch)
	if err != nil || v.(map[string]any)["status"] != "WAIT" {
		t.Fatal("premature completion accepted")
	}
}

func TestPGExpiredPermitLateObservationAndAuditRollback(t *testing.T) {
	ctx, s, _, c := integration(t)
	_, g := sample(t)
	task, err := s.Propose(ctx, "planner", "lease", g, "runtime-digest")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Admit(ctx, task.ID, task.GraphDigest, "user"); err != nil {
		t.Fatal(err)
	}
	v, err := s.ClaimRun(ctx, task.ID, "worker-1", "graph")
	if err != nil {
		t.Fatal(err)
	}
	l := leaseFrom(t, v)
	if _, err = s.ResolveNode(ctx, task.ID, "extract", l.Owner, l.Epoch, "", c, "source"); err != nil {
		t.Fatal(err)
	}
	v, err = s.BeginCall(ctx, task.ID, "extract", l.Owner, l.Epoch, routeFixture(t, c, "extractor@1"), c)
	if err != nil {
		t.Fatal(err)
	}
	call := v.(CallDecision).Call
	// Test-only SQL uses the migrator to create the exact expiration boundary.
	var cfg Config
	b, _ := os.ReadFile(os.Getenv("REPORT_DEMO_CONFIG"))
	if err = json.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	admin, err := pgx.Connect(ctx, cfg.MigratorDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err = admin.Exec(ctx, "UPDATE run_leases SET expires_at=clock_timestamp()-interval '1 second' WHERE scope=$1 AND task_id=$2", s.Scope, task.ID); err != nil {
		t.Fatal(err)
	}
	_, err = s.Heartbeat(ctx, task.ID, l.Owner, l.Epoch, "graph")
	requireCode(t, err, "LEASE_EXPIRED")
	obs := Observation{"late-accepted", "ACCEPTED", "remote-task", call.ContextID, json.RawMessage(`{"provider":"isolated-test"}`)}
	if _, err = s.Observe(ctx, call.ID, "worker-adapter", obs); err != nil {
		t.Fatal(err)
	}
	obs.State = "COMPLETED"
	obs.EventID = "late-completed"
	if _, err = s.Observe(ctx, call.ID, "worker-adapter", obs); err != nil {
		t.Fatal(err)
	}
	obs.State = "TRANSPORT"
	obs.EventID = "receipt-after-completion"
	if _, err = s.Observe(ctx, call.ID, "worker-adapter", obs); err != nil {
		t.Fatal(err)
	}
	receipts, err := s.Calls(ctx, task.ID, "extract")
	if err != nil || receipts[0].State != "COMPLETED" || receipts[0].Conflicted {
		t.Fatal("transport receipt regressed terminal state")
	}
	obs.State = "ACCEPTED"
	obs.RemoteTaskID = "conflicting-task"
	obs.EventID = "late-conflict"
	if _, err = s.Observe(ctx, call.ID, "worker-adapter", obs); err != nil {
		t.Fatal(err)
	}
	calls, err := s.Calls(ctx, task.ID, "extract")
	if err != nil || calls[0].State != "UNKNOWN" {
		t.Fatalf("conflicting observations lost: %v", err)
	}
	v, err = s.ClaimRun(ctx, task.ID, "worker-2", "graph")
	if err != nil {
		t.Fatal(err)
	}
	newLease := leaseFrom(t, v)
	if newLease.Epoch == l.Epoch {
		t.Fatal("epoch did not advance")
	}
	// Force a real transaction error after mutating the in-memory document.
	_, err = s.mutate(ctx, task.ID, "FAILURE_TEST", func(tx pgx.Tx, task *Task) (any, error) {
		task.Status = "DELIVERED"
		_, err := tx.Exec(ctx, "INSERT INTO audit_outbox(scope,aggregate,event_id,kind,document) VALUES($1,$2,'deliberately-invalid','test',NULL)", s.Scope, task.ID)
		return nil, err
	})
	if err == nil {
		t.Fatal("audit failure ignored")
	}
	after, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status == "DELIVERED" {
		t.Fatal("transaction did not roll back")
	}
}
