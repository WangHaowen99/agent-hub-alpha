package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AR-04 (#27): an adopted task must keep the business semantics it was
// approved under. These are target assertions: they describe the required
// behaviour and replace the review's counterexample
// TestArchitectureReviewJudgmentSpecNotInPlanHash, whose PASS meant the
// defect existed.
//
// Decisions the tests encode:
//   - The approval digest covers the judgment spec and the execution config
//     (model), so a plan shown under one rule version cannot be approved
//     after the rules changed; the planner proposes again and the user
//     reviews the new version.
//   - A run that is already adopted keeps resolving its bound spec and model
//     across restarts; a new run uses the currently published ones.
//   - If the bound spec can no longer be resolved, the run stops in an
//     explicit "blocked" state instead of silently using the current spec.
//
// Every restart here is OpenService on the same data directory, i.e. all
// state is rebuilt from disk exactly as after a process restart.

func ar04Spec(t *testing.T, dir, version, marker string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "schema", "examples", "judgment", "valid", "judgment-spec-credit.json"))
	if err != nil {
		t.Fatal(err)
	}
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	spec["question"].(map[string]any)["spec_version"] = version
	rule := spec["rules"].([]any)[0].(map[string]any)
	rule["coverage"] = marker + " " + rule["coverage"].(string)
	path := filepath.Join(dir, "spec-"+version+".json")
	b, _ := json.MarshalIndent(spec, "", "  ")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// ar04RDC mirrors busFixture's registry: one card and one online instance per agent.
func ar04RDC(t *testing.T) string {
	t.Helper()
	rdc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-registry" || r.Header.Get("X-Tenant-ID") != "fixture" {
			w.WriteHeader(401)
			return
		}
		id := strings.Split(r.URL.Path, "/")[3]
		if strings.HasSuffix(r.URL.Path, "/instances") {
			_ = json.NewEncoder(w).Encode(map[string]any{"instances": []map[string]any{{"instanceId": "fixture-1", "routeHandle": "private-route", "endpoint": "http://private-runtime", "health": "ONLINE", "contractVersion": "1.0", "capabilityVersion": "1.0.0"}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"name": id, "version": "1.0.0", "skills": []string{"controlled"}})
	}))
	t.Cleanup(rdc.Close)
	return rdc.URL
}

// ar04Orchestrator stands in for the Java orchestrator's /compile: it turns
// the team into graph steps the same way, with a fixed blueprint digest.
func ar04Orchestrator(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Agents []struct {
				ID        string   `json:"id"`
				DependsOn []string `json:"dependsOn"`
				Condition *struct {
					Type, Node, Text string
				} `json:"condition"`
			} `json:"agents"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			w.WriteHeader(400)
			return
		}
		out := compileResult{Blueprint: "fixture-blueprint", SpecDigest: "sha256:fixture-blueprint", Type: "graph"}
		for _, a := range in.Agents {
			st := Step{ID: a.ID, Member: a.ID, DependsOn: a.DependsOn}
			if a.Condition != nil {
				st.When = &RunIf{Step: a.Condition.Node, Contains: a.Condition.Text, Negate: a.Condition.Type == "output_not_contains"}
			}
			out.Steps = append(out.Steps, st)
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

type ar04Env struct {
	t    *testing.T
	dir  string
	rdc  string
	orch string
	spec string
	mdl  string
}

func newAR04(t *testing.T, withBus bool) *ar04Env {
	e := &ar04Env{t: t, dir: t.TempDir(), mdl: "model-a", orch: ar04Orchestrator(t)}
	if withBus {
		e.rdc = ar04RDC(t)
	}
	e.spec = ar04Spec(t, t.TempDir(), "1.1.0", "RULE-V1")
	return e
}

// open starts (or restarts) the service on the same data directory.
func (e *ar04Env) open() *Service {
	e.t.Helper()
	cfg := Config{DataDir: e.dir, SpecPath: e.spec, Model: e.mdl, LeaseSeconds: 30, OrchestratorURL: e.orch}
	if e.rdc != "" {
		cfg.BusRegistryURL, cfg.BusRegistryToken, cfg.BusTenant = e.rdc, "fixture-registry", "fixture"
	}
	s, err := OpenService(cfg)
	if err != nil {
		e.t.Fatal(err)
	}
	return s
}

func ar04Agents() []PlanAgent {
	var out []PlanAgent
	for _, st := range goodSteps() {
		out = append(out, PlanAgent{ID: st.ID, Expert: st.Member, Role: st.ID, Task: "process case C006", DependsOn: st.DependsOn, RunIf: st.When})
	}
	return out
}

func (e *ar04Env) proposeAndApprove(s *Service) (*Plan, *Run) {
	e.t.Helper()
	p, err := s.Propose(ProposeInput{CaseID: "C006", Goal: "credit due diligence", Agents: ar04Agents()})
	if err != nil {
		e.t.Fatal(err)
	}
	r, err := s.Approve(p.ID, p.Version, p.Hash, "approver", nil)
	if err != nil {
		e.t.Fatal(err)
	}
	return p, r
}

func judgePromptOf(t *testing.T, j *Job) string {
	t.Helper()
	if j == nil {
		t.Fatal("no job leased")
	}
	for _, m := range j.Members {
		if p, ok := m["judge_prompt"].(string); ok {
			return p
		}
	}
	t.Fatal("job has no judge member")
	return ""
}

// Original counterexample, turned into the target: changing the judgment
// spec must change the digest an approver confirms.
func TestAR04_ApprovalDigestCoversJudgmentSpec(t *testing.T) {
	e := newAR04(t, false)
	s := e.open()
	p1, err := s.Propose(ProposeInput{CaseID: "C006", Goal: "credit due diligence", Agents: ar04Agents()})
	if err != nil {
		t.Fatal(err)
	}
	e.spec = ar04Spec(t, t.TempDir(), "2.0.0", "RULE-V2")
	s = e.open()
	p2, err := s.Propose(ProposeInput{CaseID: "C006", Goal: "credit due diligence", Agents: ar04Agents()})
	if err != nil {
		t.Fatal(err)
	}
	if p1.Hash == p2.Hash {
		t.Fatalf("same approval digest %s under judgment spec 1.1.0 and 2.0.0", p1.Hash)
	}
	// the plan shown under 1.1.0 cannot be approved once 2.0.0 is in force
	if _, err := s.Approve(p1.ID, p1.Version, p1.Hash, "approver", nil); err == nil {
		t.Fatal("approval shown under the old rules was accepted after the rules changed")
	}
	// the plan proposed under 2.0.0 can
	if _, err := s.Approve(p2.ID, p2.Version, p2.Hash, "approver", nil); err != nil {
		t.Fatalf("current plan refused: %v", err)
	}
}

// Full restart experiment: adopt under 1.1.0, publish 2.0.0, restart.
func TestAR04_RestartKeepsAdoptedRunOnBoundSpec(t *testing.T) {
	e := newAR04(t, false)
	s := e.open()
	_, r1 := e.proposeAndApprove(s)

	e.spec = ar04Spec(t, t.TempDir(), "2.0.0", "RULE-V2")
	s = e.open()
	job := mustLease(t, s)
	if job.RunID != r1.ID {
		t.Fatalf("leased %s, want %s", job.RunID, r1.ID)
	}
	prompt := judgePromptOf(t, job)
	if !strings.Contains(prompt, "RULE-V1") || strings.Contains(prompt, "RULE-V2") || !strings.Contains(prompt, "@1.1.0") {
		t.Fatalf("run adopted under 1.1.0 was resumed with another spec:\n%s", prompt)
	}
	// a run approved after the restart uses 2.0.0
	_, r2 := e.proposeAndApprove(s)
	s.st.Runs[r1.ID].Status = "done" // let the next lease pick r2
	job2 := mustLease(t, s)
	if job2.RunID != r2.ID {
		t.Fatalf("leased %s, want %s", job2.RunID, r2.ID)
	}
	if p := judgePromptOf(t, job2); !strings.Contains(p, "RULE-V2") || !strings.Contains(p, "@2.0.0") {
		t.Fatalf("new run does not use the current spec:\n%s", p)
	}
}

// If the bound spec cannot be resolved after a restart, the run stops in an
// explicit state; it is never resumed under the current spec.
func TestAR04_UnresolvableBoundSpecBlocksRun(t *testing.T) {
	e := newAR04(t, false)
	s := e.open()
	_, r1 := e.proposeAndApprove(s)

	e.spec = ar04Spec(t, t.TempDir(), "2.0.0", "RULE-V2")
	// the archived copies of earlier specs are lost
	if err := os.RemoveAll(filepath.Join(e.dir, "specs")); err != nil {
		t.Fatal(err)
	}
	s = e.open()
	job, err := s.Lease("worker-after-restart")
	if err != nil {
		t.Fatal(err)
	}
	if job != nil && job.RunID == r1.ID {
		t.Fatalf("run %s was handed out although its spec is gone; prompt:\n%s", r1.ID, judgePromptOf(t, job))
	}
	r := s.st.Runs[r1.ID]
	if r.Status != "blocked" || !strings.Contains(r.Final, "规约") {
		t.Fatalf("run state = %q %q, want blocked with a reason naming the spec", r.Status, r.Final)
	}
}

// Legal positive: with nothing changed, a restart resumes normally.
func TestAR04_UnchangedSpecResumesNormally(t *testing.T) {
	e := newAR04(t, false)
	s := e.open()
	_, r1 := e.proposeAndApprove(s)
	s = e.open()
	job := mustLease(t, s)
	if job.RunID != r1.ID || !strings.Contains(judgePromptOf(t, job), "RULE-V1") {
		t.Fatalf("normal resume broken: %+v", job)
	}
}

// The execution config is part of the binding too: a remote call begun under
// model-a is recognised after a restart with model-b, not refused as an
// input conflict, and a new run uses model-b.
func TestAR04_RestartKeepsInFlightCallOnBoundModel(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	_, r1 := e.proposeAndApprove(s)
	job := mustLease(t, s)
	in := CallCommand{RunID: r1.ID, Token: job.Token, Step: "intake", Input: "case C006", Goal: "intake"}
	first, err := s.BeginCall(in)
	if err != nil {
		t.Fatal(err)
	}
	call := first["call"].(*RemoteCall)

	e.mdl = "model-b"
	s = e.open()
	again, err := s.BeginCall(in)
	if err != nil {
		t.Fatalf("in-flight call refused after the model config changed: %v", err)
	}
	if got := again["call"].(*RemoteCall); got.InputDigest != call.InputDigest || got.RequestID != call.RequestID {
		t.Fatalf("call identity changed: %s/%s -> %s/%s", call.InputDigest, call.RequestID, got.InputDigest, got.RequestID)
	}
}

func mustLease(t *testing.T, s *Service) *Job {
	t.Helper()
	j, err := s.Lease("worker")
	if err != nil {
		t.Fatal(err)
	}
	if j == nil {
		t.Fatal("nothing to lease")
	}
	return j
}
