package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func busFixture(t *testing.T) (*Service, CallCommand, *atomic.Int64) {
	t.Helper()
	version := &atomic.Int64{}
	version.Store(1)
	rdc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-registry" || r.Header.Get("X-Tenant-ID") != "fixture" {
			w.WriteHeader(401)
			return
		}
		id := strings.Split(r.URL.Path, "/")[3]
		v := "1.0.0"
		if version.Load() != 1 {
			v = "2.0.0"
		}
		if strings.HasSuffix(r.URL.Path, "/instances") {
			_ = json.NewEncoder(w).Encode(map[string]any{"instances": []map[string]any{{"instanceId": "fixture-1", "routeHandle": "private-route", "endpoint": "http://private-runtime", "health": "ONLINE", "contractVersion": "1.0", "capabilityVersion": v}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"name": id, "version": v, "skills": []string{"controlled"}})
	}))

	t.Cleanup(rdc.Close)
	s := &Service{cfg: Config{DataDir: t.TempDir(), BusRegistryURL: rdc.URL, BusRegistryToken: "fixture-registry", BusTenant: "fixture", Model: "fixture-model", LeaseSeconds: 30},
		cat: &Catalog{Hashes: map[string]string{"report-writer": "writer-contract", "financial-analyst": "analysis-contract"}}, writeState: writeJSON}
	p := &Plan{ID: "P1", Version: 1, Agents: []PlanAgent{{ID: "report", Expert: "report-writer"}}, Layers: [][]string{{"report"}}}
	if err := s.bindPlan(p); err != nil {
		t.Fatal(err)
	}
	r := &Run{ID: "R1", PlanID: "P1", PlanVersion: 1, Status: "running", Token: "lease-old", Epoch: 1, Steps: map[string]*StepRec{"report": {ID: "report", Expert: "report-writer", Status: "pending"}}}
	s.st = &State{Plans: map[string][]*Plan{"P1": {p}}, Runs: map[string]*Run{"R1": r}}
	if err := s.save(); err != nil {
		t.Fatal(err)
	}
	return s, CallCommand{RunID: "R1", Token: "lease-old", Step: "report", Input: "case C006", Goal: "write a grounded report"}, version
}

func beginFixture(t *testing.T, s *Service, in CallCommand) *RemoteCall {
	t.Helper()
	out, err := s.BeginCall(in)
	if err != nil {
		t.Fatal(err)
	}
	if out["action"] != "send" {
		t.Fatalf("first dispatch: %v", out["action"])
	}
	return out["call"].(*RemoteCall)
}

func callIdentity(in CallCommand, c *RemoteCall) CallCommand {
	in.NodeExecutionID, in.InputDigest, in.Attempt = c.NodeExecutionID, c.InputDigest, c.Attempt
	in.ContextID, in.RemoteTaskID = c.ContextID, "remote-task-1"
	return in
}

func TestBusDuplicateBeginNeverResendsAndRejectsChangedInput(t *testing.T) {
	s, in, _ := busFixture(t)
	c := beginFixture(t, s, in)
	got, err := s.BeginCall(in)
	if err != nil || got["action"] != "unknown" {
		t.Fatalf("duplicate dispatch must remain unresolved: %v %v", got, err)
	}
	if got["call"].(*RemoteCall).RequestID != c.RequestID {
		t.Fatal("request identity changed")
	}
	in.Input = "different material"
	if _, err = s.BeginCall(in); !errors.Is(err, errCallConflict) {
		t.Fatalf("input conflict accepted: %v", err)
	}
}

func TestBusCapabilityVersionDriftRejectsBeforeIntent(t *testing.T) {
	s, in, version := busFixture(t)
	version.Store(2)
	if _, err := s.BeginCall(in); !errors.Is(err, errCallConflict) {
		t.Fatalf("card version drift accepted: %v", err)
	}
	if s.st.Runs[in.RunID].Steps[in.Step].Call != nil {
		t.Fatal("created a dispatch on changed capability")
	}
}

func TestBusCommitIsInputBoundAndFirstResultCannotBeOverwritten(t *testing.T) {
	s, in, _ := busFixture(t)
	c := beginFixture(t, s, in)
	in = callIdentity(in, c)
	in.Output = "accepted artifact"
	bad := in
	bad.InputDigest = "other-input"
	if _, err := s.CommitCall(bad); !errors.Is(err, errCallConflict) {
		t.Fatal("accepted wrong input")
	}
	bad = in
	bad.ContextID = "other-context"
	if _, err := s.CommitCall(bad); !errors.Is(err, errCallConflict) {
		t.Fatal("accepted wrong context")
	}
	first, err := s.CommitCall(in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.CommitCall(in)
	if err != nil || again != first {
		t.Fatal("duplicate did not reuse formal result")
	}
	in.Output = "late replacement"
	if _, err := s.CommitCall(in); !errors.Is(err, errCallConflict) {
		t.Fatal("late result overwrote formal result")
	}
}

func TestBusIntentWriteFailurePreventsDispatchAndFreezesDirtyMemory(t *testing.T) {
	s, in, _ := busFixture(t)
	s.writeState = func(string, any) error { return os.ErrPermission }
	if _, err := s.BeginCall(in); !errors.Is(err, errStorage) {
		t.Fatalf("write failure hidden: %v", err)
	}
	if s.st.Runs[in.RunID].Steps[in.Step].Call != nil {
		t.Fatal("dirty intent survived failed commit")
	}
	if _, err := s.Lease("replacement"); !errors.Is(err, errStorage) {
		t.Fatal("continued leasing on uncertain storage")
	}
}

func TestBusResultWriteFailureDoesNotExposeSuccess(t *testing.T) {
	s, in, _ := busFixture(t)
	c := beginFixture(t, s, in)
	in = callIdentity(in, c)
	in.Output = "not committed"
	s.writeState = func(string, any) error { return os.ErrPermission }
	if _, err := s.CommitCall(in); !errors.Is(err, errStorage) {
		t.Fatalf("write failure hidden: %v", err)
	}
	st := s.st.Runs[in.RunID].Steps[in.Step]
	if st.Status == "done" || st.Output != "" || st.Call.ArtifactID != "" {
		t.Fatal("failed write published a formal result")
	}
	raw, err := os.ReadFile(filepath.Join(s.cfg.DataDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "not committed") {
		t.Fatal("failed result was persisted")
	}
}

func TestBusPersistenceReloadReusesArtifactWithoutModelIncrement(t *testing.T) {
	s, in, _ := busFixture(t)
	c := beginFixture(t, s, in)
	// the runtime reports the model it is about to call (AR-04 M1); the fixture runs on its bound model
	auth := CallCommand{RunID: in.RunID, Step: in.Step, Token: c.CapabilityToken, ServiceID: c.Binding.AgentID, ServiceVersion: c.Binding.Version, ContextID: c.ContextID, EventID: "model-1", Model: "fixture-model"}
	if _, err := s.AgentContext(auth); err != nil {
		t.Fatal(err)
	}
	if err := s.ModelCall(auth); err != nil {
		t.Fatal(err)
	}
	if err := s.ModelCall(auth); err != nil {
		t.Fatal(err)
	}
	commit := callIdentity(in, c)
	commit.Output = "durable artifact"
	if _, err := s.CommitCall(commit); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(s.cfg.DataDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	for _, r := range restored.Runs {
		r.Token = r.TokenSaved
	}
	restarted := &Service{cfg: s.cfg, cat: s.cat, st: &restored, lastSaved: raw, writeState: writeJSON}
	out, err := restarted.BeginCall(in)
	if err != nil || out["action"] != "reuse" {
		t.Fatalf("restart failed to reuse: %v %v", out, err)
	}
	got := out["call"].(*RemoteCall)
	if len(got.ModelEvents) != 1 || got.ArtifactID == "" || got.ArtifactDigest != sha(commit.Output) {
		t.Fatal("formal artifact or model count changed")
	}
}

func TestBusEventsDoNotRegressOrReplaceTaskOwner(t *testing.T) {
	s, in, _ := busFixture(t)
	c := beginFixture(t, s, in)
	in = callIdentity(in, c)
	in.Sequence = 2
	in.Status = "waiting"
	in.Reason = "accepted"
	if err := s.UpdateCall(in); err != nil {
		t.Fatal(err)
	}
	in.Sequence = 1
	in.Status = "unknown"
	in.Reason = "old event"
	if err := s.UpdateCall(in); err != nil {
		t.Fatal(err)
	}
	if s.st.Runs[in.RunID].Steps[in.Step].Call.Reason != "accepted" {
		t.Fatal("out-of-order event regressed state")
	}
	in.Sequence = 3
	in.RemoteTaskID = "other-owner"
	if err := s.UpdateCall(in); !errors.Is(err, errCallConflict) {
		t.Fatal("remote task owner changed")
	}
	in.RemoteTaskID = "remote-task-1"
	in.Status = "failed"
	if err := s.UpdateCall(in); err != nil {
		t.Fatal(err)
	}
	in.Sequence = 4
	in.Status = "waiting"
	if err := s.UpdateCall(in); err != nil {
		t.Fatal(err)
	}
	if s.st.Runs[in.RunID].Steps[in.Step].Status != "failed" {
		t.Fatal("late event revived a failed execution")
	}
}

func TestBusScopedCredentialCannotCrossNodeOrAcquireControl(t *testing.T) {
	s, in, _ := busFixture(t)
	c := beginFixture(t, s, in)
	auth := CallCommand{RunID: in.RunID, Step: "other", Token: c.CapabilityToken, ServiceID: c.Binding.AgentID}
	if _, err := s.AgentContext(auth); !errors.Is(err, errFenced) {
		t.Fatal("credential crossed node boundary")
	}
	auth.Step = in.Step
	auth.ServiceID = "other-service"
	if _, err := s.AgentContext(auth); !errors.Is(err, errFenced) {
		t.Fatal("credential crossed service boundary")
	}
	auth.ServiceID = c.Binding.AgentID
	auth.ContextID = "unapproved-context"
	if _, err := s.AgentContext(auth); !errors.Is(err, errFenced) {
		t.Fatal("credential crossed context boundary")
	}
	if s.st.Runs[in.RunID].Steps[in.Step].Call.AgentStartedAt != "" {
		t.Fatal("rejected context consumed the execution claim")
	}
	auth.ContextID = c.ContextID
	auth.ServiceVersion = "wrong-version"
	if _, err := s.AgentContext(auth); !errors.Is(err, errFenced) {
		t.Fatal("runtime version drift accepted")
	}
	auth.ServiceVersion = c.Binding.Version
	if _, err := s.AgentContext(auth); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AgentContext(auth); err == nil {
		t.Fatal("second remote execution accepted")
	}
	control := callIdentity(in, c)
	control.Token = c.CapabilityToken
	control.Output = "forged"
	if _, err := s.CommitCall(control); !errors.Is(err, errFenced) {
		t.Fatal("agent credential gained control write")
	}
}

func TestBusWaitingResumeQueriesOriginalTaskAndFencesOldHost(t *testing.T) {
	s, in, _ := busFixture(t)
	c := beginFixture(t, s, in)
	update := callIdentity(in, c)
	update.Status = "unknown"
	update.Sequence = 1
	if err := s.UpdateCall(update); err != nil {
		t.Fatal(err)
	}
	if err := s.Done(in.RunID, in.Token, nil, "", "connection lost"); err != nil {
		t.Fatal(err)
	}
	if s.st.Runs[in.RunID].Status != "waiting" {
		t.Fatal("unknown collapsed into failed/done")
	}
	if err := s.ResumeRun(in.RunID); err != nil {
		t.Fatal(err)
	}
	job, err := s.Lease("new-host")
	if err != nil || job == nil {
		t.Fatal("resume could not lease", err)
	}
	if _, err := s.BeginCall(in); !errors.Is(err, errFenced) {
		t.Fatal("old host not fenced")
	}
	in.Token = job.Token
	out, err := s.BeginCall(in)
	if err != nil || out["action"] != "query" {
		t.Fatal("resume created another dispatch", out, err)
	}
	if out["call"].(*RemoteCall).RequestID != c.RequestID {
		t.Fatal("resume changed identity")
	}
}

func TestBusLegacyEndpointsCannotBypassRemoteCommit(t *testing.T) {
	s, in, _ := busFixture(t)
	if _, _, err := s.StepGet(in.RunID, in.Token, in.Step); err == nil {
		t.Fatal("legacy get bypassed input digest")
	}
	if err := s.StepPut(in.RunID, in.Token, in.Step, "fake"); err == nil {
		t.Fatal("legacy put bypassed remote identity")
	}
}

func TestBusModeRejectsLegacyPlanWithoutTrustedBindings(t *testing.T) {
	s, _, _ := busFixture(t)
	p := s.st.Plans["P1"][0]
	p.BusBindings = nil
	if err := s.checkBindings(p); err == nil {
		t.Fatal("bus mode silently accepted an old local-only plan")
	}
}
