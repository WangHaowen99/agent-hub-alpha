package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func receiptCommand(in CallCommand, c *RemoteCall) CallCommand {
	in = callIdentity(in, c)
	in.Sequence, in.Status = 1, "waiting"
	in.Gateway = &GatewayExchange{RPCID: c.RequestID, Method: "SendMessage", StartedAt: "2026-11-11T01:00:00Z", FinishedAt: "2026-11-11T01:00:01Z", HTTPStatus: 200, Outcome: "task_received"}
	return in
}

func TestGatewayObservationNamesInvalidFieldWithoutEchoingContent(t *testing.T) {
	for _, tc := range []struct {
		field  string
		change func(*GatewayExchange)
	}{
		{"rpc_id", func(e *GatewayExchange) { e.RPCID = "secret invalid id" }},
		{"method", func(e *GatewayExchange) { e.Method = "secret method" }},
		{"started_at", func(e *GatewayExchange) { e.StartedAt = "secret time" }},
		{"finished_at", func(e *GatewayExchange) { e.FinishedAt = "secret time" }},
		{"time_order", func(e *GatewayExchange) { e.FinishedAt = "2026-11-10T00:00:00Z" }},
		{"http_status", func(e *GatewayExchange) { e.HTTPStatus = 600 }},
		{"error_code", func(e *GatewayExchange) { e.ErrorCode = "secret error" }},
		{"outcome", func(e *GatewayExchange) { e.Outcome = "secret outcome" }},
	} {
		t.Run(tc.field, func(t *testing.T) {
			c := &RemoteCall{RequestID: "request", ContextID: "context"}
			in := CallCommand{RemoteTaskID: "task", ContextID: "context", Gateway: &GatewayExchange{
				RPCID: "request", Method: "SendMessage", StartedAt: "2026-11-11T00:00:00Z", FinishedAt: "2026-11-11T00:00:01Z", HTTPStatus: 200, Outcome: "task_received"}}
			tc.change(in.Gateway)
			err := recordGateway(c, in)
			if err == nil || err.Error() != "invalid Gateway observation: "+tc.field || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unexpected diagnostic: %v", err)
			}
			if len(c.GatewayExchanges) != 0 {
				t.Fatal("rejected observation was committed")
			}
		})
	}
}

func TestDiscoverySnapshotPersistsWithoutChangingApprovedBinding(t *testing.T) {
	s, in, version := busFixture(t)
	binding := s.st.Plans["P1"][0].BusBindings["report"]
	c := beginFixture(t, s, in)
	if c.Binding != binding || c.Discovery == nil || len(c.Discovery.Candidates) != 1 || !c.Discovery.Candidates[0].Eligible || c.Discovery.Candidates[0].InstanceID != "fixture-1" {
		t.Fatal("missing bound discovery snapshot")
	}
	raw, err := os.ReadFile(filepath.Join(s.cfg.DataDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), c.Discovery.CheckedAt) || strings.Contains(string(raw), "private-route") || strings.Contains(string(raw), "private-runtime") {
		t.Fatal("snapshot was not safely persisted")
	}
	before, _ := json.Marshal(c.Discovery)
	commit := callIdentity(in, c)
	commit.Output = "saved result"
	if _, err = s.CommitCall(commit); err != nil {
		t.Fatal(err)
	}
	version.Store(2) // Reuse must preserve the original discovery, not consult current RDC.
	out, err := s.BeginCall(in)
	if err != nil || out["action"] != "reuse" {
		t.Fatal("failed reuse", err)
	}
	after, _ := json.Marshal(out["call"].(*RemoteCall).Discovery)
	if string(before) != string(after) {
		t.Fatal("historical discovery was overwritten")
	}
}

func TestGatewayObservationsDeduplicateAndKeepProtocolErrors(t *testing.T) {
	s, in, _ := busFixture(t)
	c := beginFixture(t, s, in)
	update := receiptCommand(in, c)
	if err := s.UpdateCall(update); err != nil {
		t.Fatal(err)
	}
	update.Sequence = 2
	if err := s.UpdateCall(update); err != nil {
		t.Fatal(err)
	}
	update.Sequence = 3
	update.Status = "unknown"
	update.Gateway = &GatewayExchange{RPCID: "query-1", Method: "GetTask", StartedAt: "2026-11-11T01:00:02Z", FinishedAt: "2026-11-11T01:00:03Z", HTTPStatus: 200, Outcome: "protocol_error", ErrorCode: "TASK_NOT_FOUND"}
	if err := s.UpdateCall(update); err != nil {
		t.Fatal(err)
	}
	v := gatewayView(s.st.Runs["R1"].Steps["report"].Call)
	if v.SendCount != 1 || v.QueryCount != 1 || v.FirstReceipt.RPCID != c.RequestID || v.LastExchange.Outcome != "protocol_error" || v.LastError.HTTPStatus != 200 {
		t.Fatalf("misleading Gateway summary: %+v", v)
	}
	update.Sequence = 4
	update.Gateway.Outcome = "task_received"
	update.Gateway.ErrorCode = ""
	if err := s.UpdateCall(update); !errors.Is(err, errCallConflict) {
		t.Fatal("conflicting RPC observation accepted", err)
	}
	update.Sequence = 1
	update.Gateway.RPCID = "late-query"
	if err := s.UpdateCall(update); err != nil {
		t.Fatal(err)
	}
	if len(s.st.Runs["R1"].Steps["report"].Call.GatewayExchanges) != 2 {
		t.Fatal("old event replaced newer evidence")
	}
}

func TestGatewayRejectsUnverifiedReceipts(t *testing.T) {
	for _, kind := range []string{"context", "task", "method", "time", "transport", "request", "error_code"} {
		t.Run(kind, func(t *testing.T) {
			s, in, _ := busFixture(t)
			c := beginFixture(t, s, in)
			u := receiptCommand(in, c)
			switch kind {
			case "context":
				u.ContextID = "wrong"
			case "task":
				u.RemoteTaskID = ""
			case "method":
				u.Gateway.Method = "anything"
			case "time":
				u.Gateway.FinishedAt = "2026-11-10T00:00:00Z"
			case "transport":
				u.Gateway.Outcome = "transport_error"
			case "request":
				u.Gateway.RPCID = "wrong"
			case "error_code":
				u.Gateway.ErrorCode = "Bearer private"
			}
			if err := s.UpdateCall(u); err == nil {
				t.Fatal("invalid evidence accepted")
			}
			saved := s.st.Runs["R1"].Steps["report"].Call
			if len(saved.GatewayExchanges) != 0 || saved.RemoteTaskID != "" || saved.Sequence != 0 {
				t.Fatal("rejected event mutated state")
			}
		})
	}
}

func TestGatewayWriteFailureRollsBackEvidenceAndTask(t *testing.T) {
	s, in, _ := busFixture(t)
	c := beginFixture(t, s, in)
	u := receiptCommand(in, c)
	s.writeState = func(string, any) error { return os.ErrPermission }
	if err := s.UpdateCall(u); !errors.Is(err, errStorage) {
		t.Fatal("write failure hidden", err)
	}
	saved := s.st.Runs["R1"].Steps["report"].Call
	if saved.RemoteTaskID != "" || len(saved.GatewayExchanges) != 0 {
		t.Fatal("unsaved observation leaked")
	}
}

func TestGraphDistinguishesBindingFromInvocationAndKeepsLegacyGaps(t *testing.T) {
	s, in, _ := busFixture(t)
	p := s.st.Plans["P1"][0]
	p.Steps = []Step{{ID: "report", Member: "report"}}
	g, err := s.graph(p, false)
	if err != nil {
		t.Fatal(err)
	}
	if g.Bus == nil || g.Bus.PlannedNodes != 1 || g.Bus.LinkedTasks != 0 || g.Nodes[0].Remote != nil || g.Nodes[0].Execution.Binding == nil {
		t.Fatal("plan claims an actual invocation")
	}
	c := beginFixture(t, s, in)
	p.RunID = "R1"
	saved := s.st.Runs["R1"].Steps["report"].Call
	saved.Discovery = nil // legacy persisted call
	g, err = s.graph(p, false)
	if err != nil {
		t.Fatal(err)
	}
	r := g.Nodes[0].Remote
	if r["discovery"] != nil || r["gateway"] != nil {
		t.Fatal("legacy evidence fabricated")
	}
	raw, _ := json.Marshal(g)
	for _, secret := range []string{c.CapabilityToken, "fixture-registry", "private-route", "lease-old"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("private call data exposed")
		}
	}
	g.Nodes = append(g.Nodes, GraphNode{Execution: g.Nodes[0].Execution, Status: "reused", Remote: map[string]any{"task_id": "existing"}}, GraphNode{Execution: g.Nodes[0].Execution, Status: "unknown"}, GraphNode{Execution: GraphExecution{Kind: "host"}, Status: "done"})
	summary := busSummary(g.Nodes)
	if summary.PlannedNodes != 3 || summary.Services != 1 || summary.Completed != 1 || summary.Reused != 1 || summary.Unresolved != 1 || summary.LinkedTasks != 1 {
		t.Fatal("incorrect bus-only summary", summary)
	}
}
