package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Reviewer counterexamples for #31 (chaosxingxc-orion, PR #31 comment
// 6967911632), kept verbatim: both failed on 6e42596.

// Independent counterexample: a persisted block must deny new model permits,
// even when a valid capability reports the correct model afterwards.
func TestReviewBlockedRunRejectsFurtherRemoteModelCalls(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	r, _, c := m1RemoteCall(t, s, e)
	if _, err := s.AgentContext(agentCmd(r, c)); err != nil {
		t.Fatalf("setup: runtime context rejected: %v", err)
	}
	mux := http.NewServeMux()
	s.busRoutes(mux)
	cmd := agentCmd(r, c)
	request := func(model, event string) *httptest.ResponseRecorder {
		cmd.Model, cmd.EventID = model, event
		raw, err := json.Marshal(cmd)
		if err != nil {
			t.Fatal(err)
		}
		resp := httptest.NewRecorder()
		mux.ServeHTTP(resp, httptest.NewRequest("POST", "/agent/model", bytes.NewReader(raw)))
		return resp
	}
	refused := request("model-b", "wrong-model")
	if refused.Code != http.StatusConflict {
		t.Fatalf("setup: expected 409, got %d", refused.Code)
	}
	if status, _ := runStatus(s, r.ID); status != "blocked" {
		t.Fatal("setup: task not blocked")
	}
	after := request("model-a", "new-call-after-block")
	s.mu.Lock()
	events := len(s.st.Runs[r.ID].Steps["intake"].Call.ModelEvents)
	s.mu.Unlock()
	if after.Code == http.StatusOK || events != 0 {
		t.Fatalf("blocked task issued new model permit: HTTP=%d model_events=%d body=%s", after.Code, events, after.Body.String())
	}
}

// A worker rejecting its own compatibility is not evidence that no compatible
// executor exists. This case frames the availability policy for discussion.
func TestReviewIncompatiblePollDoesNotDisableHealthyExecutor(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	_, r := e.proposeAndApprove(s)
	if job, err := s.LeaseAs("incompatible-worker", "model-b"); err != nil || job != nil {
		t.Fatalf("unexpected incompatible lease: %v %v", job, err)
	}
	job, err := s.LeaseAs("healthy-worker", "model-a")
	status, _ := runStatus(s, r.ID)
	if err != nil || job == nil || job.RunID != r.ID {
		t.Fatalf("healthy executor cannot obtain its matching task after an unrelated poll: job=%v err=%v status=%s", job, err, status)
	}
}

// --- P1 coverage requested in the review ---

func openRemote(t *testing.T, s *Service, r *Run, token, step string) *RemoteCall {
	t.Helper()
	out, err := s.BeginCall(CallCommand{RunID: r.ID, Token: token, Step: step, Input: "case C006", Goal: step})
	if err != nil || out["action"] != "send" {
		t.Fatalf("begin %s: %v %v", step, out, err)
	}
	return out["call"].(*RemoteCall)
}

func stepCmd(r *Run, step string, c *RemoteCall) CallCommand {
	return CallCommand{RunID: r.ID, Step: step, Token: c.CapabilityToken, ServiceID: c.Binding.AgentID,
		ContextID: c.ContextID, ServiceVersion: c.Binding.Version}
}

// Two remote nodes run in parallel; one reports another model. The run is
// blocked and the other node, holding a valid credential and the right model,
// gets no new permit either — also after a restart.
func TestAR04P1_ParallelNodesGetNoPermitOnceOneBlocksTheRun(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	_, r := e.proposeAndApprove(s)
	job, err := s.LeaseAs("worker-a", "model-a")
	if err != nil || job == nil {
		t.Fatalf("lease: %v", err)
	}
	fin, ind := openRemote(t, s, r, job.Token, "financial"), openRemote(t, s, r, job.Token, "industry")
	for step, c := range map[string]*RemoteCall{"financial": fin, "industry": ind} {
		if _, err := s.AgentContext(stepCmd(r, step, c)); err != nil {
			t.Fatalf("context %s: %v", step, err)
		}
	}
	bad := stepCmd(r, "financial", fin)
	bad.EventID, bad.Model = "f-1", "model-b"
	if err := s.ModelCall(bad); !errors.Is(err, errModelBinding) {
		t.Fatalf("mismatch on financial: %v", err)
	}
	good := stepCmd(r, "industry", ind)
	good.EventID, good.Model = "i-1", "model-a"
	if err := s.ModelCall(good); !errors.Is(err, errFenced) {
		t.Fatalf("parallel node got a model permit on a blocked run: %v", err)
	}
	s = e.open() // restart: still blocked, still no permit
	good.EventID = "i-2"
	if err := s.ModelCall(good); !errors.Is(err, errFenced) {
		t.Fatalf("model permit after restart of a blocked run: %v", err)
	}
	s.mu.Lock()
	events := len(s.st.Runs[r.ID].Steps["industry"].Call.ModelEvents)
	s.mu.Unlock()
	if status, _ := runStatus(s, r.ID); status != "blocked" || events != 0 {
		t.Fatalf("after restart: status %s, industry model events %d", status, events)
	}
}

// A waiting run is not blocked: its in-flight remote task keeps calling its
// bound model while the run waits for the original task to be queried.
func TestAR04P1_WaitingRunKeepsInFlightModelCalls(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	r, job, c := m1RemoteCall(t, s, e)
	if _, err := s.AgentContext(agentCmd(r, c)); err != nil {
		t.Fatal(err)
	}
	lost := CallCommand{RunID: r.ID, Token: job.Token, Step: "intake", NodeExecutionID: c.NodeExecutionID, InputDigest: c.InputDigest,
		Attempt: c.Attempt, ContextID: c.ContextID, RemoteTaskID: "remote-task-1", Status: "unknown", Sequence: 1}
	if err := s.UpdateCall(lost); err != nil {
		t.Fatal(err)
	}
	if err := s.Done(r.ID, job.Token, nil, "", "connection lost"); err != nil {
		t.Fatal(err)
	}
	if status, _ := runStatus(s, r.ID); status != "waiting" {
		t.Fatalf("run status %s, want waiting", status)
	}
	cmd := agentCmd(r, c)
	cmd.EventID, cmd.Model = "w-1", "model-a"
	if err := s.ModelCall(cmd); err != nil {
		t.Fatalf("in-flight model call refused on a waiting run: %v", err)
	}
}
