package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAdoptionFailuresAreNotContentRetries(t *testing.T) {
	for _, mode := range []string{"ledger-open", "ledger-append", "snapshot-after-adoption", "conflict", "content"} {
		t.Run(mode, func(t *testing.T) {
			e := newAR04(t, true)
			s := e.open()
			_, r := e.proposeAndApprove(s)
			job, err := s.LeaseAs("worker-a", "model-a")
			if err != nil || job == nil {
				t.Fatalf("lease: %v", err)
			}
			if err := s.StepPut(r.ID, job.Token, "related", "verified evidence"); err != nil {
				t.Fatal(err)
			}
			raw := `{"level":"grant-50000","evidence_steps":["related"],"rationale":"test"}`
			status, code := 503, "STORAGE_UNAVAILABLE"
			switch mode {
			case "ledger-open", "ledger-append":
				if mode == "ledger-append" {
					if _, err := s.machine(r.ID); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.MkdirAll(filepath.Join(s.cfg.DataDir, "runs", r.ID, "adopt.jsonl"), 0755); err != nil {
					t.Fatal(err)
				}
			case "snapshot-after-adoption":
				s.writeState = func(string, any) error { return errors.New("disk full") }
			case "conflict":
				if _, err := s.Judgment(r.ID, job.Token, "judge", raw); err != nil {
					t.Fatal(err)
				}
				raw = `{"level":"grant-50000","evidence_steps":["related"],"rationale":"changed"}`
				status, code = 409, "INPUT_CONFLICT"
			case "content":
				raw = "invalid json"
				status, code = 422, "POLICY_REJECTED"
			}
			mux := http.NewServeMux()
			s.routes(mux)
			srv := httptest.NewServer(mux)
			defer srv.Close()
			got := ar07Post(t, srv, "/exec/judgment", map[string]any{"run_id": r.ID, "token": job.Token, "step": "judge", "raw": raw})
			expectCode(t, mode, got, status, code)
			if status == 503 {
				expectCode(t, "freeze after storage failure", ar07Post(t, srv, "/exec/heartbeat", map[string]any{"run_id": r.ID, "token": job.Token}), 503, "STORAGE_UNAVAILABLE")
			}
			if mode == "snapshot-after-adoption" {
				events, err := s.machines[r.ID].Events()
				if err != nil || len(events) != 1 || events[0].Outcome.Status != "adopted" {
					t.Fatalf("ledger must retain adopted result despite snapshot failure: %v %v", events, err)
				}
			}
		})
	}
}
