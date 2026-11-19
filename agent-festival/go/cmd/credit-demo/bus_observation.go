package main

import (
	"fmt"
	"regexp"
	"time"
)

// Discovery is a historical observation, not part of the approved capability
// digest. Candidates must never be presented as the instance actually selected.
type DiscoveredInstance struct {
	InstanceID        string `json:"instance_id"`
	Health            string `json:"health"`
	ContractVersion   string `json:"contract_version"`
	CapabilityVersion string `json:"capability_version"`
	Eligible          bool   `json:"eligible"`
}

type DiscoverySnapshot struct {
	CheckedAt  string               `json:"checked_at"`
	Candidates []DiscoveredInstance `json:"candidates"`
}

// A narrow allowlist of metadata observed by the trusted Java caller. Never
// store request/response bodies, authorization headers, URLs or route handles.
type GatewayExchange struct {
	RPCID      string `json:"rpc_id"`
	Method     string `json:"method"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Outcome    string `json:"outcome"`
	ErrorCode  string `json:"error_code,omitempty"`
	Sequence   int64  `json:"sequence,omitempty"`
}

type GatewayView struct {
	Source       string           `json:"source"`
	Protocol     string           `json:"protocol"`
	PathMode     string           `json:"path_mode"`
	SendCount    int              `json:"send_count"`
	QueryCount   int              `json:"query_count"`
	FirstSend    *GatewayExchange `json:"first_send,omitempty"`
	FirstReceipt *GatewayExchange `json:"first_receipt,omitempty"`
	LastExchange *GatewayExchange `json:"last_exchange,omitempty"`
	LastError    *GatewayExchange `json:"last_error,omitempty"`
}

var rpcIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,160}$`)
var gatewayCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

func recordGateway(c *RemoteCall, in CallCommand) error {
	if in.Gateway == nil {
		return nil // Old hosts/records remain readable; no synthetic backfill.
	}
	e := *in.Gateway
	e.Sequence = 0
	start, errStart := time.Parse(time.RFC3339Nano, e.StartedAt)
	end, errEnd := time.Parse(time.RFC3339Nano, e.FinishedAt)
	// Name the rejected field, never its possibly sensitive value. This makes
	// a real deployment's failed observation diagnosable without weakening it.
	for _, check := range []struct {
		field string
		valid bool
	}{
		{"rpc_id", rpcIDPattern.MatchString(e.RPCID)},
		{"method", contains([]string{"SendMessage", "GetTask"}, e.Method)},
		{"started_at", errStart == nil},
		{"finished_at", errEnd == nil},
		{"time_order", !end.Before(start)},
		{"http_status", e.HTTPStatus == 0 || (e.HTTPStatus >= 100 && e.HTTPStatus <= 599)},
		{"error_code", e.ErrorCode == "" || gatewayCodePattern.MatchString(e.ErrorCode)},
		{"outcome", contains([]string{"task_received", "http_error", "protocol_error", "transport_error", "invalid_response"}, e.Outcome)},
	} {
		if !check.valid {
			return fmt.Errorf("invalid Gateway observation: %s", check.field)
		}
	}
	if e.Method == "SendMessage" && e.RPCID != c.RequestID {
		return errCallConflict
	}
	if e.Outcome == "task_received" && (e.HTTPStatus != 200 || e.ErrorCode != "" || in.RemoteTaskID == "" || in.ContextID != c.ContextID) {
		return fmt.Errorf("Gateway receipt requires a validated task and context")
	}
	if e.Outcome == "transport_error" && e.HTTPStatus != 0 {
		return fmt.Errorf("transport error cannot claim an HTTP response")
	}
	if previous, exists := c.GatewayExchanges[e.RPCID]; exists {
		previous.Sequence = 0
		if previous != e {
			return errCallConflict
		}
		return nil
	}
	if c.GatewayExchanges == nil {
		c.GatewayExchanges = map[string]GatewayExchange{}
	}
	e.Sequence = in.Sequence
	c.GatewayExchanges[e.RPCID] = e
	return nil
}

func gatewayView(c *RemoteCall) *GatewayView {
	if len(c.GatewayExchanges) == 0 {
		return nil
	}
	v := &GatewayView{Source: "java_host", Protocol: "A2A 1.0 JSON-RPC", PathMode: "DIRECT"}
	for _, event := range c.GatewayExchanges {
		e := event
		if e.Method == "SendMessage" {
			v.SendCount++
			if v.FirstSend == nil || e.Sequence < v.FirstSend.Sequence {
				v.FirstSend = &e
			}
		} else {
			v.QueryCount++
		}
		if e.Outcome == "task_received" && (v.FirstReceipt == nil || e.Sequence < v.FirstReceipt.Sequence) {
			v.FirstReceipt = &e
		}
		if e.Outcome != "task_received" && (v.LastError == nil || e.Sequence > v.LastError.Sequence) {
			v.LastError = &e
		}
		if v.LastExchange == nil || e.Sequence > v.LastExchange.Sequence {
			v.LastExchange = &e
		}
	}
	return v
}
