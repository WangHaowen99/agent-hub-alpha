package main

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPGSendDecisionIsNotReissued(t *testing.T) {
	ctx, s, _, cat, _, task, req, _ := commitFixture(t)
	for i, want := range []string{"SEND", "UNKNOWN"} {
		v, err := s.BeginCall(ctx, task.ID, "extract", req.Owner, req.Epoch, routeFixture(t, cat, "extractor@1"), cat)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(v)
		var response map[string]any
		if err = json.Unmarshal(raw, &response); err != nil {
			t.Fatal(err)
		}
		if response["action"] != want {
			t.Fatalf("request %d: want action %s, got %v", i, want, response["action"])
		}
	}
}

func TestPGConcurrentSendDecisionsAndReconciliation(t *testing.T) {
	ctx, s, _, cat, _, task, req, _ := commitFixture(t)
	var wg sync.WaitGroup
	results := make(chan CallDecision, 8)
	failures := make(chan error, 8)
	route := routeFixture(t, cat, "extractor@1")
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := s.BeginCall(ctx, task.ID, "extract", req.Owner, req.Epoch, route, cat)
			if err != nil {
				failures <- err
				return
			}
			results <- v.(CallDecision)
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	sends := 0
	var first Call
	for d := range results {
		if d.Action == "SEND" {
			sends++
		}
		if first.ID != "" && first.ID != d.ID {
			t.Fatal("different call identities")
		}
		first = d.Call
	}
	if sends != 1 {
		t.Fatalf("send permits=%d", sends)
	}
	if _, err := s.Observe(ctx, first.ID, "fixture", Observation{"accept", "ACCEPTED", "remote-original", first.ContextID, json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	v, err := s.BeginCall(ctx, task.ID, "extract", req.Owner, req.Epoch, route, cat)
	if err != nil || v.(CallDecision).Action != "QUERY" || v.(CallDecision).RemoteTaskID != "remote-original" {
		t.Fatal("accepted call not reconciled", err)
	}
	changed := cat
	changed.PolicyVersion = "changed-before-dispatch"
	_, err = s.BeginCall(ctx, task.ID, "extract", req.Owner, req.Epoch, route, changed)
	requireCode(t, err, "BINDING_CHANGED")
	saveCommitEvidence(t, "concurrent-send-decisions", map[string]any{"send_permits": sends, "call_id": first.ID, "repeat_action": "QUERY", "model_calls": 0})
}

func TestPGSendIntentCommitReplyLostNeverReissuesSend(t *testing.T) {
	ctx, s, cfg, cat, _, task, req, _ := commitFixture(t)
	pc, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		t.Fatal(err)
	}
	if pc.ConnConfig.TLSConfig != nil || len(pc.ConnConfig.Fallbacks) > 0 {
		t.Skip("requires existing plaintext test DSN; no TLS downgrade")
	}
	var armed atomic.Bool
	var cuts atomic.Int64
	pc.MaxConns = 1
	pc.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return &commitAckCut{Conn: conn, reader: bufio.NewReader(conn), armed: &armed, cuts: &cuts}, nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	faulty := *s
	faulty.Pool = pool
	armed.Store(true)
	route := routeFixture(t, cat, "extractor@1")
	_, err = faulty.BeginCall(ctx, task.ID, "extract", req.Owner, req.Epoch, route, cat)
	if err == nil || cuts.Load() != 1 {
		t.Fatal("real PG commit response was not cut")
	}
	v, err := s.BeginCall(ctx, task.ID, "extract", req.Owner, req.Epoch, route, cat)
	if err != nil || v.(CallDecision).Action != "UNKNOWN" {
		t.Fatal("lost send permission reissued", err)
	}
	calls, err := s.Calls(ctx, task.ID, "extract")
	if err != nil || len(calls) != 1 {
		t.Fatal("intent did not persist once", err)
	}
	saveCommitEvidence(t, "send-intent-commit-reply-lost", map[string]any{"actual_pg_commit_ack_cut": cuts.Load(), "intents": len(calls), "retry_action": "UNKNOWN", "model_calls": 0})
}
