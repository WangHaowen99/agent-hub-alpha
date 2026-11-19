package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// AR-07 (#27): every control endpoint answers an error with a typed code, so
// an executor can tell a lost lease from an input conflict, a model binding
// refusal, a rejected judgment and a storage failure — several share 409.

type ar07Reply struct {
	status int
	Error  string `json:"error"`
	Code   string `json:"code"`
}

func ar07Post(t *testing.T, srv *httptest.Server, path string, body map[string]any) ar07Reply {
	t.Helper()
	raw, _ := json.Marshal(body)
	resp, err := http.Post(srv.URL+path, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var r ar07Reply
	_ = json.NewDecoder(resp.Body).Decode(&r)
	r.status = resp.StatusCode
	return r
}

func expectCode(t *testing.T, what string, got ar07Reply, status int, code string) {
	t.Helper()
	if got.status != status || got.Code != code {
		t.Fatalf("%s: HTTP %d %q (%s), want %d %q", what, got.status, got.Code, got.Error, status, code)
	}
}

func TestAR07_ErrorMatrixOverHTTP(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	mux := http.NewServeMux()
	s.routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	_, r := e.proposeAndApprove(s)
	job, err := s.LeaseAs("worker-a", "model-a")
	if err != nil || job == nil {
		t.Fatalf("lease: %v", err)
	}

	expectCode(t, "heartbeat with a foreign token",
		ar07Post(t, srv, "/exec/heartbeat", map[string]any{"run_id": r.ID, "token": "not-the-lease"}), 409, "FENCED")
	expectCode(t, "unknown step",
		ar07Post(t, srv, "/exec/step/get", map[string]any{"run_id": r.ID, "token": job.Token, "step": "no-such-step"}), 400, "INVALID_REQUEST")

	begin := map[string]any{"run_id": r.ID, "token": job.Token, "step": "intake", "input": "case C006", "goal": "intake"}
	if got := ar07Post(t, srv, "/exec/call/begin", begin); got.status != 200 {
		t.Fatalf("begin: %d %s", got.status, got.Error)
	}
	begin["input"] = "different material"
	expectCode(t, "same call, different input", ar07Post(t, srv, "/exec/call/begin", begin), 409, "INPUT_CONFLICT")

	expectCode(t, "judgment that is not JSON",
		ar07Post(t, srv, "/exec/judgment", map[string]any{"run_id": r.ID, "token": job.Token, "step": "judge", "raw": "no json here"}), 422, "POLICY_REJECTED")

	s.mu.Lock()
	c := s.st.Runs[r.ID].Steps["intake"].Call
	s.mu.Unlock()
	expectCode(t, "remote runtime on another model",
		ar07Post(t, srv, "/agent/model", map[string]any{"run_id": r.ID, "step": "intake", "token": c.CapabilityToken, "service_id": c.Binding.AgentID,
			"context_id": c.ContextID, "service_version": c.Binding.Version, "event_id": "ev-1", "model": "model-b"}), 409, "MODEL_BINDING_MISMATCH")
	// the run is now blocked: its former executor is fenced, not told about a conflict
	expectCode(t, "executor after the run was blocked",
		ar07Post(t, srv, "/exec/heartbeat", map[string]any{"run_id": r.ID, "token": job.Token}), 409, "FENCED")

	_, r2 := e.proposeAndApprove(s)
	job2, err := s.LeaseAs("worker-a", "model-a")
	if err != nil || job2 == nil || job2.RunID != r2.ID {
		t.Fatalf("second lease: %+v %v", job2, err)
	}
	s.writeState = func(string, any) error { return errors.New("disk full") }
	expectCode(t, "write that cannot be made durable",
		ar07Post(t, srv, "/exec/step/get", map[string]any{"run_id": r2.ID, "token": job2.Token, "step": "related"}), 503, "STORAGE_UNAVAILABLE")
	expectCode(t, "any later write after a storage failure",
		ar07Post(t, srv, "/exec/heartbeat", map[string]any{"run_id": r2.ID, "token": job2.Token}), 503, "STORAGE_UNAVAILABLE")
}
