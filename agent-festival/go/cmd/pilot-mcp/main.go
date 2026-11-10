// pilot-mcp is a spike for issue #4 (WorkSwarm as the user plane): a Go
// control stub exposed over MCP (streamable HTTP) with an MCP App console.
// It reuses internal/adopt for business-request adoption and dispatch, and
// calls an in-process test provider with injectable faults. Verification
// only — not a production component.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed app/dist/index.html
var consoleHTML string

const (
	consoleURI = "ui://pilot/console"
	appMIME    = "text/html;profile=mcp-app"
)

var uiMeta = mcp.Meta{"ui": map[string]any{"resourceUri": consoleURI}}

type SubmitInput struct {
	CommandID              string `json:"command_id,omitempty" jsonschema:"idempotency key of this application command; reuse the same value when retrying the same request"`
	CommandSource          string `json:"command_source,omitempty" jsonschema:"who assigned command_id: app or model"`
	CustomerID             string `json:"customer_id" jsonschema:"customer identifier"`
	AssetsWan              int    `json:"assets_wan" jsonschema:"total assets in units of 10,000 CNY"`
	AssetReportMissing     bool   `json:"asset_report_missing,omitempty" jsonschema:"true if the asset valuation report is not available"`
	LongestOverdueMonths   int    `json:"longest_overdue_months,omitempty" jsonschema:"longest overdue duration on record in months; 0 if none"`
	LatestOverdueMonthsAgo int    `json:"latest_overdue_months_ago,omitempty" jsonschema:"months since the most recent overdue; 0 if none"`
	Scenario               string `json:"scenario,omitempty" jsonschema:"test provider behaviour: normal, decline, timeout, lost_receipt, duplicate_receipt"`
}

type GetInput struct {
	TaskID    string `json:"task_id,omitempty" jsonschema:"task id such as T1"`
	CommandID string `json:"command_id,omitempty" jsonschema:"command id used when submitting"`
}

type CancelInput struct {
	RequestID string `json:"request_id" jsonschema:"business request id such as B1"`
}

type Empty struct{}

func result(v any) *mcp.CallToolResult {
	raw, _ := json.Marshal(v)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}, StructuredContent: v}
}

func headers(req *mcp.CallToolRequest) http.Header {
	if req.Extra != nil && req.Extra.Header != nil {
		return req.Extra.Header
	}
	return http.Header{}
}

func newServer(p *Pilot) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "credit-pilot", Version: "0.1.0"}, nil)
	submitMeta := uiMeta
	if os.Getenv("PILOT_SUBMIT_APP_ONLY") != "" {
		// MCP Apps visibility: the submit tool is meant for the console only,
		// so a business intent always comes from a user action in the app.
		submitMeta = mcp.Meta{"ui": map[string]any{"resourceUri": consoleURI, "visibility": []string{"app"}}}
	}

	s.AddResource(&mcp.Resource{URI: consoleURI, Name: "credit-pilot-console", MIMEType: appMIME},
		func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: consoleURI, MIMEType: appMIME, Text: consoleHTML}}}, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "pilot_submit_application", Meta: submitMeta,
		Description: "Submit a credit application to the pilot business service. Returns the command receipt; the business outcome arrives later and must be read with pilot_get_task, never assumed."},
		func(_ context.Context, req *mcp.CallToolRequest, in SubmitInput) (*mcp.CallToolResult, any, error) {
			id := identify(headers(req))
			p.recordCall("pilot_submit_application", id)
			if requireUser && !id.UserVerified {
				res := result(map[string]any{"status": "unauthenticated", "detail": "a verified end-user token is required; nothing was submitted", "problems": id.Problems})
				res.IsError = true
				return res, nil, nil
			}
			r := p.Submit(in.CommandID, in.CommandSource, id, Material{CustomerID: in.CustomerID, AssetsWan: in.AssetsWan,
				AssetReportOK: !in.AssetReportMissing, LongestOverdueMonths: in.LongestOverdueMonths,
				LatestOverdueMonthsAgo: in.LatestOverdueMonthsAgo, Scenario: in.Scenario})
			res := result(r)
			res.IsError = r.Status == "conflict"
			return res, nil, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "pilot_get_task", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Read the authoritative state of one task and its business request."},
		func(_ context.Context, req *mcp.CallToolRequest, in GetInput) (*mcp.CallToolResult, any, error) {
			p.recordCall("pilot_get_task", identify(headers(req)))
			v, ok := p.Get(in.TaskID, in.CommandID)
			if !ok {
				res := result(map[string]string{"error": "unknown task"})
				res.IsError = true
				return res, nil, nil
			}
			return result(v), nil, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "pilot_list_tasks", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "List the most recent tasks with their authoritative state."},
		func(_ context.Context, req *mcp.CallToolRequest, _ Empty) (*mcp.CallToolResult, any, error) {
			p.recordCall("pilot_list_tasks", identify(headers(req)))
			return result(map[string]any{"tasks": p.List(10)}), nil, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "pilot_open_console", Meta: uiMeta, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Open the credit pilot console showing recent tasks."},
		func(_ context.Context, req *mcp.CallToolRequest, _ Empty) (*mcp.CallToolResult, any, error) {
			p.recordCall("pilot_open_console", identify(headers(req)))
			return result(map[string]any{"tasks": p.List(10)}), nil, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "pilot_request_cancel",
		Description: "Ask the business service to cancel a pending business request. Only the provider's confirmation means it is cancelled."},
		func(_ context.Context, req *mcp.CallToolRequest, in CancelInput) (*mcp.CallToolResult, any, error) {
			p.recordCall("pilot_request_cancel", identify(headers(req)))
			ok, detail := p.RequestCancel(in.RequestID)
			return result(map[string]any{"request_id": in.RequestID, "cancel_confirmed": ok, "detail": detail}), nil, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "pilot_whoami", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		Description: "Show which caller identity the pilot service can see on this connection."},
		func(_ context.Context, req *mcp.CallToolRequest, _ Empty) (*mcp.CallToolResult, any, error) {
			id := identify(headers(req))
			p.recordCall("pilot_whoami", id)
			return result(map[string]any{"this_call": id, "recent_calls": p.RecentCalls(10)}), nil, nil
		})
	return s
}

// verifier is set when PILOT_OIDC_ISSUER names a trusted token issuer;
// requireUser then refuses submissions without a verified user token.
var (
	verifier    *oidcVerifier
	requireUser bool
)

func main() {
	addr := envOr("PILOT_ADDR", ":8090")
	if iss := os.Getenv("PILOT_OIDC_ISSUER"); iss != "" {
		verifier = newOIDCVerifier(iss, os.Getenv("PILOT_OIDC_JWKS"))
		requireUser = os.Getenv("PILOT_REQUIRE_USER") != ""
		log.Printf("verifying tokens from %s (require user: %v)", iss, requireUser)
	}
	p, err := OpenPilot(envOr("PILOT_DATA", "/data"), envOr("PILOT_SCOPE", "credit-pilot"))
	if err != nil {
		log.Fatal(err)
	}
	server := newServer(p)
	mux := http.NewServeMux()
	// The service is reached by container name inside the compose network,
	// so the localhost DNS-rebinding guard does not apply.
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{DisableLocalhostProtection: true}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/debug/state", func(w http.ResponseWriter, _ *http.Request) {
		p.reconcile()
		p.mu.Lock()
		defer p.mu.Unlock()
		events, _ := p.machine.Events()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"pilot": p, "ledger": events})
	})
	log.Printf("credit-pilot MCP on %s/mcp", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
