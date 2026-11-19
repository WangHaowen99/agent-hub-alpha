package main

import (
	"errors"
	"strings"
	"testing"
)

// AR-04 M1/M2 (#31 review): the model a run was adopted under must be the
// model that actually executes it. Executors report the model they will call;
// the control service compares it with the run's binding before anything
// runs, and stops the run as blocked on a mismatch instead of letting the
// ledger say A while B is called.

func runStatus(s *Service, id string) (string, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.st.Runs[id]
	return r.Status, r.Final
}

func TestAR04M1_LeaseOnlyToExecutorOnBoundModel(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	_, r := e.proposeAndApprove(s)

	job, err := s.LeaseAs("worker-b", "model-b")
	if err != nil || job != nil {
		t.Fatalf("executor on model-b got a job for a model-a run: %+v %v", job, err)
	}
	s.mu.Lock()
	run := s.st.Runs[r.ID]
	status, leases, refusals := run.Status, len(run.Leases), append([]LeaseRefusal(nil), run.LeaseRefusals...)
	s.mu.Unlock()
	if status != "queued" || leases != 0 {
		t.Fatalf("a refused executor changed the run: status %s, leases %d", status, leases)
	}
	if len(refusals) != 1 || refusals[0].Worker != "worker-b" || refusals[0].Model != "model-b" || refusals[0].Bound != "model-a" {
		t.Fatalf("refusal not recorded: %+v", refusals)
	}
	for i := 0; i < 3; i++ { // repeated polls are counted, not appended
		_, _ = s.LeaseAs("worker-b", "model-b")
	}
	s.mu.Lock()
	n, count := len(s.st.Runs[r.ID].LeaseRefusals), s.st.Runs[r.ID].LeaseRefusals[0].Count
	s.mu.Unlock()
	if n != 1 || count != 4 {
		t.Fatalf("repeated refusals: %d entries, count %d", n, count)
	}

	// the same run goes to an executor on its bound model, with the model it must use
	job, err = s.LeaseAs("worker-a", "model-a")
	if err != nil || job == nil || job.RunID != r.ID || job.Model != "model-a" {
		t.Fatalf("executor on the bound model: %+v %v", job, err)
	}
}

func TestAR04M1_ExecutorThatDoesNotReportItsModelIsRefused(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	_, r := e.proposeAndApprove(s)
	if job, err := s.LeaseAs("legacy", ""); err != nil || job != nil {
		t.Fatalf("legacy executor without a model got a job: %+v %v", job, err)
	}
	s.mu.Lock()
	run := s.st.Runs[r.ID]
	status, refusals := run.Status, run.LeaseRefusals
	s.mu.Unlock()
	if status != "queued" || len(refusals) != 1 || refusals[0].Model != "" {
		t.Fatalf("legacy executor: status %s refusals %+v", status, refusals)
	}
	if g, err := s.graph(mustPlan(t, s, r), false); err != nil || g.Run == nil || g.Run.Waiting == "" {
		t.Fatalf("projection does not say why the run waits: %+v %v", g, err)
	}
}

func mustPlan(t *testing.T, s *Service, r *Run) *Plan {
	t.Helper()
	p, ok := s.Plan(r.PlanID, r.PlanVersion)
	if !ok {
		t.Fatal("no plan")
	}
	return p
}

// m1RemoteCall leases on the bound model and opens a remote call for intake.
func m1RemoteCall(t *testing.T, s *Service, e *ar04Env) (*Run, *Job, *RemoteCall) {
	t.Helper()
	_, r := e.proposeAndApprove(s)
	job, err := s.LeaseAs("worker-a", "model-a")
	if err != nil || job == nil {
		t.Fatalf("lease: %v", err)
	}
	out, err := s.BeginCall(CallCommand{RunID: r.ID, Token: job.Token, Step: "intake", Input: "case C006", Goal: "intake"})
	if err != nil || out["action"] != "send" {
		t.Fatalf("begin: %v %v", out, err)
	}
	return r, job, out["call"].(*RemoteCall)
}

func agentCmd(r *Run, c *RemoteCall) CallCommand {
	return CallCommand{RunID: r.ID, Step: "intake", Token: c.CapabilityToken, ServiceID: c.Binding.AgentID,
		ContextID: c.ContextID, ServiceVersion: c.Binding.Version}
}

func TestAR04M1_RemoteRuntimeOnOtherModelIsStoppedBeforeTheCall(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	r, job, c := m1RemoteCall(t, s, e)

	ctx, err := s.AgentContext(agentCmd(r, c))
	if err != nil || ctx["model"] != "model-a" {
		t.Fatalf("context does not carry the bound model: %v %v", ctx["model"], err)
	}
	cmd := agentCmd(r, c)
	cmd.EventID, cmd.Model = "ev-1", "model-b"
	if err := s.ModelCall(cmd); !errors.Is(err, errModelBinding) {
		t.Fatalf("model call on model-b accepted: %v", err)
	}
	status, final := runStatus(s, r.ID)
	if status != "blocked" || !strings.Contains(final, "model-b") {
		t.Fatalf("run after a mismatched model call: %s %q", status, final)
	}
	s.mu.Lock()
	events, reason := len(s.st.Runs[r.ID].Steps["intake"].Call.ModelEvents), s.st.Runs[r.ID].Steps["intake"].Call.Reason
	s.mu.Unlock()
	if events != 0 || !strings.Contains(reason, "model-b") {
		t.Fatalf("mismatched call left %d model events, reason %q", events, reason)
	}
	if err := s.Heartbeat(r.ID, job.Token); err == nil {
		t.Fatal("the executor kept its lease after the run was blocked")
	}

	// blocked survives a restart and is not resumed like a waiting run
	s = e.open()
	if status, _ := runStatus(s, r.ID); status != "blocked" {
		t.Fatalf("after restart: %s", status)
	}
	if err := s.ResumeRun(r.ID); err == nil {
		t.Fatal("a blocked run was resumed like a waiting one")
	}
}

func TestAR04M1_RemoteModelCallOnBoundModelRecordsTheActualModel(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	r, _, c := m1RemoteCall(t, s, e)
	cmd := agentCmd(r, c)
	cmd.EventID, cmd.Model = "ev-1", "model-a"
	for i := 0; i < 2; i++ { // the second report of the same event is idempotent
		if err := s.ModelCall(cmd); err != nil {
			t.Fatalf("model call on the bound model: %v", err)
		}
	}
	s.mu.Lock()
	call := s.st.Runs[r.ID].Steps["intake"].Call
	events, actual := len(call.ModelEvents), call.ActualModels["ev-1"]
	s.mu.Unlock()
	if events != 1 || actual != "model-a" {
		t.Fatalf("model events %d, actual model %q", events, actual)
	}
	if status, _ := runStatus(s, r.ID); status != "running" {
		t.Fatalf("run status after a matching call: %s", status)
	}
}

// M2: after the configuration moves to model-b, new runs bind model-b and run
// on a model-b executor; a run adopted under model-a is not continued on it.
func TestAR04M2_NewRunsUseTheNewModelOldRunsAreNotMoved(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	_, old := e.proposeAndApprove(s)

	e.mdl = "model-b"
	s = e.open()
	_, fresh := e.proposeAndApprove(s)
	s.mu.Lock()
	bound := s.st.Runs[fresh.ID].Binding.Model
	s.mu.Unlock()
	if bound != "model-b" {
		t.Fatalf("new run bound to %q, want model-b", bound)
	}
	job, err := s.LeaseAs("worker-b", "model-b")
	if err != nil || job == nil || job.RunID != fresh.ID {
		t.Fatalf("model-b executor did not get the model-b run: %+v %v", job, err)
	}
	if status, _ := runStatus(s, old.ID); status != "queued" {
		t.Fatalf("model-a run on a model-b executor: %s, want it left queued for a model-a executor", status)
	}
}
