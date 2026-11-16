package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The read boundary drops the real PostgreSQL CommandComplete(COMMIT), after
// the server committed but before pgx can receive that acknowledgement. It
// never records authentication frames, SQL payloads, DSNs or credentials.
type commitAckCut struct {
	net.Conn
	reader *bufio.Reader
	buffer []byte
	armed  *atomic.Bool
	cuts   *atomic.Int64
}

func (c *commitAckCut) Read(dst []byte) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}
	if len(c.buffer) == 0 {
		header := make([]byte, 5)
		if _, err := io.ReadFull(c.reader, header); err != nil {
			return 0, err
		}
		length := int(binary.BigEndian.Uint32(header[1:]))
		if length < 4 || length > 16*1024*1024 {
			return 0, errors.New("invalid PG test frame length")
		}
		payload := make([]byte, length-4)
		if _, err := io.ReadFull(c.reader, payload); err != nil {
			return 0, err
		}
		if header[0] == 'C' && string(payload) == "COMMIT\x00" && c.armed.CompareAndSwap(true, false) {
			c.cuts.Add(1)
			_ = c.Conn.Close()
			return 0, io.ErrUnexpectedEOF
		}
		c.buffer = append(header, payload...)
	}
	n := copy(dst, c.buffer)
	c.buffer = c.buffer[n:]
	return n, nil
}

func commitFixture(t *testing.T) (context.Context, *Store, Config, Catalog, Material, *Task, CommitRequest, []byte) {
	t.Helper()
	ctx, s, cfg, cat := integration(t)
	_, _, material, err := LoadConfig(os.Getenv("REPORT_DEMO_CONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ArtifactRoot = t.TempDir()
	NewServer(s, cfg, cat, material)
	_, graph := sample(t)
	graph.MaterialID = material.ID
	task, err := s.Propose(ctx, "fixture-planner", "commit-window", graph, "compiler-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Admit(ctx, task.ID, task.GraphDigest, "fixture-user"); err != nil {
		t.Fatal(err)
	}
	v, err := s.ClaimRun(ctx, task.ID, "fixture-worker", "graph")
	if err != nil {
		t.Fatal(err)
	}
	lease := leaseFrom(t, v)
	v, err = s.ResolveNode(ctx, task.ID, "extract", lease.Owner, lease.Epoch, "", cat, material.Digest)
	if err != nil {
		t.Fatal(err)
	}
	permit := v.(map[string]any)
	raw := []byte(`{"fixture":"commit-window; no model result or business acceptance","indicators":[]}`)
	artifact, err := (Files{cfg.ArtifactRoot}).Put(raw)
	if err != nil {
		t.Fatal(err)
	}
	request := CommitRequest{lease.Owner, lease.Epoch, permit["input_digest"].(string), permit["attempt"].(int), artifact}
	return ctx, s, cfg, cat, material, task, request, raw
}

func saveCommitEvidence(t *testing.T, name string, value any) {
	t.Helper()
	path := os.Getenv("REPORT_DEMO_CONFIG")
	if path == "" {
		return
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(filepath.Dir(path), "evidence", name+".json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPGCommitAckLostReconcilesAlreadyCommittedFormalResult(t *testing.T) {
	ctx, s, cfg, cat, material, task, request, raw := commitFixture(t)
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		t.Fatal(err)
	}
	if poolConfig.ConnConfig.TLSConfig != nil || len(poolConfig.ConnConfig.Fallbacks) > 0 {
		t.Skip("PG wire fault fixture requires an already plaintext DSN; never downgrades TLS")
	}
	var armed atomic.Bool
	var cuts atomic.Int64
	poolConfig.MaxConns = 1
	poolConfig.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return &commitAckCut{Conn: conn, reader: bufio.NewReader(conn), armed: &armed, cuts: &cuts}, nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
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
	_, err = faulty.Commit(ctx, task.ID, "extract", request, raw, cat, material.Digest)
	if err == nil || cuts.Load() != 1 {
		t.Fatal("real COMMIT acknowledgement was not lost", cuts.Load())
	}
	requireCode(t, storageError(err), "STORAGE_UNAVAILABLE")
	// A separate, unaffected connection reads the actual database outcome.
	after, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	formal := after.Nodes["extract"].Result
	if formal == nil || formal.ArtifactID != request.ArtifactID {
		t.Fatal("server did not commit before acknowledgement cut")
	}
	v, err := faulty.Commit(ctx, task.ID, "extract", request, raw, cat, material.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if v.(*Result).Receipt != formal.Receipt {
		t.Fatal("retry changed the already committed formal result")
	}
	var count, audit int
	if err = s.Pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM committed_results WHERE scope=$1 AND task_id=$2),(SELECT count(*) FROM audit_outbox WHERE scope=$1 AND aggregate=$2 AND kind='RESULT_COMMITTED')", s.Scope, task.ID).Scan(&count, &audit); err != nil {
		t.Fatal(err)
	}
	if count != 1 || audit < 1 {
		t.Fatal("formal result or mandatory audit missing/duplicated", count, audit)
	}
	saveCommitEvidence(t, "fault-pg-commit-ack-lost", map[string]any{"scope": s.Scope, "task_id": task.ID, "cuts": cuts.Load(), "formal_results": count, "receipt": formal.Receipt, "retry_reused_receipt": true, "actual_pg_commit_before_ack_cut": true, "model_calls": 0, "fixture": true})
}

func TestHTTPResultCommitResponseLostReusesSameReceipt(t *testing.T) {
	_, s, cfg, cat, material, task, request, _ := commitFixture(t)
	routes := NewServer(s, cfg, cat, material).Routes()
	var dropped atomic.Bool
	committed := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/commit") && dropped.CompareAndSwap(false, true) {
			recorder := httptest.NewRecorder()
			routes.ServeHTTP(recorder, r)
			if recorder.Code != http.StatusOK {
				w.WriteHeader(recorder.Code)
				_, _ = w.Write(recorder.Body.Bytes())
				return
			}
			committed <- append([]byte(nil), recorder.Body.Bytes()...)
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		routes.ServeHTTP(w, r)
	}))
	defer server.Close()
	body, _ := json.Marshal(request)
	post := func() (*http.Response, error) {
		r, err := http.NewRequest("POST", server.URL+"/v1/tasks/"+task.ID+"/nodes/extract/commit", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		r.Header.Set("Authorization", "Bearer "+cfg.Tokens["worker"])
		r.Header.Set("Content-Type", "application/json")
		return server.Client().Do(r)
	}
	response, err := post()
	if response != nil {
		response.Body.Close()
	}
	if err == nil {
		t.Fatal("response was not lost at HTTP boundary")
	}
	var first Result
	select {
	case raw := <-committed:
		if err = json.Unmarshal(raw, &first); err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("first HTTP handler did not commit successfully")
	}
	response, err = post()
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var retry Result
	if response.StatusCode != 200 {
		t.Fatal("same-identity retry was rejected", response.StatusCode)
	}
	if err = json.NewDecoder(response.Body).Decode(&retry); err != nil {
		t.Fatal(err)
	}
	if retry.Receipt != first.Receipt || retry.ArtifactID != first.ArtifactID {
		t.Fatal("HTTP retry changed receipt/content")
	}
	saveCommitEvidence(t, "fault-http-commit-response-lost", map[string]any{"scope": s.Scope, "task_id": task.ID, "receipt": first.Receipt, "response_dropped_after_handler_committed": true, "retry_reused_receipt": true, "model_calls": 0, "fixture": true})
}
