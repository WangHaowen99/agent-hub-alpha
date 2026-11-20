package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestartReconcilesAdoptionBeforeExecution(t *testing.T) {
	for _, mode := range []string{"consistent", "before-adoption", "snapshot-failed", "after-rename", "missing-ledger", "extra-dispatch", "torn-tail", "old-ledger", "old-request", "missing-result", "changed-decision"} {
		t.Run(mode, func(t *testing.T) {
			e := newAR04(t, true)
			s := e.open()
			_, r := e.proposeAndApprove(s)
			job, err := s.LeaseAs("worker", "model-a")
			if err != nil || job == nil {
				t.Fatalf("lease: %v", err)
			}
			if err := s.StepPut(r.ID, job.Token, "related", "verified evidence"); err != nil {
				t.Fatal(err)
			}
			if mode == "snapshot-failed" || mode == "after-rename" {
				s.writeState = func(path string, value any) error {
					if mode == "after-rename" {
						if err := writeJSON(path, value); err != nil {
							return err
						}
					}
					return errors.New("injected persistence failure")
				}
			}
			if mode != "before-adoption" {
				_, err = s.Judgment(r.ID, job.Token, "judge", `{"level":"grant-50000","evidence_steps":["related"],"rationale":"original"}`)
				if (mode == "snapshot-failed" || mode == "after-rename") != errors.Is(err, errStorage) {
					t.Fatalf("judgment: %v", err)
				}
				if err != nil && !errors.Is(err, errStorage) {
					t.Fatal(err)
				}
			}
			path := filepath.Join(e.dir, "runs", r.ID, "adopt.jsonl")
			switch mode {
			case "missing-ledger":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "extra-dispatch":
				if out := s.machines[r.ID].Dispatch("op-"+r.ID, s.liveGrants(r)); out.Status != "dispatched" {
					t.Fatal(out)
				}
			case "torn-tail":
				f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				_, err = f.WriteString("{\"ledger_version\":")
				f.Close()
				if err != nil {
					t.Fatal(err)
				}
			case "old-ledger":
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var event map[string]any
				if err := json.Unmarshal(raw, &event); err != nil {
					t.Fatal(err)
				}
				if _, ok := event["grants_at_decision"]; !ok {
					t.Fatal("fixture lacks v0.2 grant pin")
				}
				delete(event, "grants_at_decision") // pre-v0.2 adoption omitted the pin
				old, err := json.Marshal(event)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, append(old, '\n'), 0600); err != nil {
					t.Fatal(err)
				}
			case "changed-decision":
				s.st.Runs[r.ID].Decision["rationale"] = "not the adopted result"
				if err := s.save(); err != nil {
					t.Fatal(err)
				}
			case "old-request":
				s.st.Runs[r.ID].ReplaySteps[0]["request"].(map[string]any)["adoption_version"] = "0.1.0"
				if err := s.save(); err != nil {
					t.Fatal(err)
				}
			case "missing-result":
				s.st.Runs[r.ID].Steps["judge"].Status = "running"
				s.st.Runs[r.ID].Steps["judge"].Output = ""
				if err := s.save(); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.ReadFile(path)
			restarted := e.open()
			blocked := mode != "consistent" && mode != "before-adoption" && mode != "after-rename"
			got := restarted.st.Runs[r.ID]
			if blocked {
				if got.Status != "blocked" || !strings.Contains(got.Final, "RECONCILIATION_REQUIRED") {
					t.Fatalf("unsafe restart: %+v", got)
				}
				if !errors.Is(restarted.Heartbeat(r.ID, job.Token), errFenced) {
					t.Fatal("old token still valid")
				}
				if err := restarted.ResumeRun(r.ID); err == nil {
					t.Fatal("ordinary resume bypassed reconciliation")
				}
				if next, err := restarted.LeaseAs("worker", "model-a"); err != nil || next != nil {
					t.Fatalf("blocked run leased: %v %v", next, err)
				}
				backups, err := filepath.Glob(filepath.Join(e.dir, "reconciliation-*.json"))
				if err != nil || len(backups) != 1 {
					t.Fatalf("missing original snapshot: %v %v", backups, err)
				}
				raw, err := os.ReadFile(backups[0])
				if err != nil {
					t.Fatal(err)
				}
				var original State
				if err := json.Unmarshal(raw, &original); err != nil {
					t.Fatal(err)
				}
				if original.Runs[r.ID].Status != "running" || original.Runs[r.ID].TokenSaved != job.Token {
					t.Fatal("original snapshot evidence lost")
				}
			} else if got.Status == "blocked" {
				t.Fatalf("consistent run blocked: %s", got.Final)
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("startup rewrote adoption evidence")
			}
			rev := restarted.st.Rev
			if again := e.open(); again.st.Rev != rev {
				t.Fatal("repeated restart changed an unchanged reconciliation")
			}
		})
	}
}

func TestJudgmentResultSurvivesLostResponseBeforeStepPut(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	_, r := e.proposeAndApprove(s)
	job, _ := s.LeaseAs("worker", "model-a")
	if err := s.StepPut(r.ID, job.Token, "related", "verified evidence"); err != nil {
		t.Fatal(err)
	}
	out, err := s.Judgment(r.ID, job.Token, "judge", `{"level":"grant-50000","evidence_steps":["related"],"rationale":"original"}`)
	if err != nil {
		t.Fatal(err)
	}
	// No /step/put: simulate a lost judgment response or executor crash.
	restarted := e.open()
	recorded, saved, err := restarted.StepGet(r.ID, job.Token, "judge")
	if err != nil || !recorded || saved != "[step:judge]\n"+out {
		t.Fatalf("would call model again: %v %q %v", recorded, saved, err)
	}
	if err := restarted.StepPut(r.ID, job.Token, "judge", saved); err != nil {
		t.Fatal(err)
	}
	if err := restarted.StepPut(r.ID, job.Token, "judge", "different"); !errors.Is(err, errCallConflict) {
		t.Fatal("result was replaceable", err)
	}
}

func TestReconciliationStopsStartupIfEvidenceCannotBeSaved(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	_, r := e.proposeAndApprove(s)
	job, _ := s.LeaseAs("worker", "model-a")
	if err := s.StepPut(r.ID, job.Token, "related", "verified evidence"); err != nil {
		t.Fatal(err)
	}
	s.writeState = func(string, any) error { return errors.New("disk full") }
	if _, err := s.Judgment(r.ID, job.Token, "judge", `{"level":"grant-50000","evidence_steps":["related"],"rationale":"original"}`); !errors.Is(err, errStorage) {
		t.Fatal(err)
	}
	path := filepath.Join(e.dir, "state.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(e.dir, "reconciliation-"+strings.TrimPrefix(sha(string(s.lastSaved)), "sha256:")+".json")
	if err := os.Mkdir(backup, 0700); err != nil {
		t.Fatal(err)
	}
	opened, err := OpenService(s.cfg)
	if err == nil || opened != nil {
		t.Fatal("served without reconciliation evidence", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("original snapshot overwritten after failed evidence write")
	}
}

func TestReconciliationBlocksOnlyAffectedRun(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	_, bad := e.proposeAndApprove(s)
	job, _ := s.LeaseAs("worker", "model-a")
	if err := s.StepPut(bad.ID, job.Token, "related", "verified evidence"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Judgment(bad.ID, job.Token, "judge", `{"level":"grant-50000","evidence_steps":["related"],"rationale":"original"}`); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(e.dir, "runs", bad.ID, "adopt.jsonl")); err != nil {
		t.Fatal(err)
	}
	_, good := e.proposeAndApprove(s)
	restarted := e.open()
	next, err := restarted.LeaseAs("worker", "model-a")
	if err != nil || next == nil || next.RunID != good.ID {
		t.Fatalf("healthy task unavailable: %v %v", next, err)
	}
}
