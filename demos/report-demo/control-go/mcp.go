package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/json"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openjiuwen/agent-festival/go/cmd/report-demo/selection"
)

//go:embed web/*.html web/*.js
var webFiles embed.FS

const graphURI = "ui://report-demo/graph"
const appMIME = "text/html;profile=mcp-app"

type Empty struct{}
type ProposeInput struct {
	CommandID string `json:"command_id" jsonschema:"Stable command key. Reuse only for an identical retry."`
	Graph     Graph  `json:"graph" jsonschema:"Candidate from the approved capability catalog. Does not execute until user confirmation."`
}
type TaskRef struct {
	TaskID string `json:"task_id" jsonschema:"Authoritative task identity returned by propose_plan."`
}

func (s *Server) viewGrant(id string) string {
	b, _ := json.Marshal(map[string]any{"task_id": id, "expires": time.Now().Add(time.Hour).Unix()})
	payload := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, []byte(s.Config.Tokens["planner"]))
	mac.Write([]byte("report-view/1:" + payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (s *Server) allowView(id, token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(s.Config.Tokens["planner"]))
	mac.Write([]byte("report-view/1:" + parts[0]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return false
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	var v struct {
		TaskID  string `json:"task_id"`
		Expires int64  `json:"expires"`
	}
	if err = json.Unmarshal(b, &v); err != nil {
		return false
	}
	return v.TaskID == id && v.Expires > time.Now().Unix()
}

func toolResult(v any, err error) *mcp.CallToolResult {
	if err != nil {
		var f any = map[string]string{"code": "UNAVAILABLE", "message": "Operation unavailable; retain original identity"}
		if typed, ok := err.(*Fault); ok {
			f = typed
		}
		b, _ := json.Marshal(map[string]any{"error": f})
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}
	}
	b, _ := json.Marshal(v)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}, StructuredContent: v}
}
func (s *Server) graphToolResult(t *Task, err error) *mcp.CallToolResult {
	if err != nil {
		return toolResult(nil, err)
	}
	res := toolResult(map[string]any{"task": t, "user_entry": "WorkSwarm 编排台：选择原 task_id 核对并确认；confirm_url 仅为备用联调页", "confirm_url": s.Config.PublicURL + "/ui/tasks/" + t.ID, "projection_url": s.Config.PublicURL + "/v1/projections/" + t.ID}, nil)
	// A one-task, read-only, expiring UI capability is placed in UI metadata, not
	// ordinary model-visible fields. It cannot approve or execute anything.
	res.Meta = mcp.Meta{"report_view_authorization": s.viewGrant(t.ID)}
	return res
}
func (s *Server) MCP() http.Handler {
	srv := mcp.NewServer(&mcp.Implementation{Name: "report-orchestration", Version: "0.1.0"}, nil)
	doc, _ := embeddedPage("graph.html")
	appDoc := strings.Replace(string(doc), "<head>", "<head><meta name=\"report-base\" content=\""+html.EscapeString(s.Config.PublicURL)+"\">", 1)
	srv.AddResource(&mcp.Resource{URI: graphURI, Name: "annual-report-graph", MIMEType: appMIME}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: graphURI, MIMEType: appMIME, Text: appDoc, Meta: mcp.Meta{"ui": map[string]any{"csp": map[string]any{"connectDomains": []string{s.Config.PublicURL}}}}}}}, nil
	})
	meta := mcp.Meta{"ui": map[string]any{"resourceUri": graphURI}}
	mcp.AddTool(srv, &mcp.Tool{Name: "list_capabilities", Description: "Read approved annual-report capabilities, fixed material and policy. Exclude incompatible scope; choose at least three real business roles. No arbitrary URLs or code.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(ctx context.Context, _ *mcp.CallToolRequest, _ Empty) (*mcp.CallToolResult, any, error) {
		cd, err := Digest(s.Catalog)
		return toolResult(map[string]any{"catalog": s.Catalog, "catalog_digest": cd, "material_id": s.Material.ID, "material_digest": s.Material.Digest, "rules": []string{"Extract consumes fixed material; analysis consumes extraction; report joins business and quality when both selected.", "Every draft/revision requires a check on the same branch; one unconditional selector consumes all checks. Report candidates missing this delivery path are rejected before compilation.", "Only program checker may supply branch PASS/REVISE. At most one report revision; no unbounded topology.", "User approves in the WorkSwarm workbench plugin; Go page is a fallback debugger. Planner has no approval or execution tool."}}, err), nil, nil
	})
	mcp.AddTool(srv, &mcp.Tool{Name: "propose_plan", Description: "Submit a real candidate graph from the catalog, compile and precheck. User must confirm its digest before execution. Reuse command_id only for identical retry.", Meta: meta}, func(ctx context.Context, _ *mcp.CallToolRequest, in ProposeInput) (*mcp.CallToolResult, any, error) {
		d, err := s.checkPublicCandidate(ctx, in.Graph)
		if err != nil {
			return toolResult(nil, err), nil, nil
		}
		t, err := s.Store.Propose(ctx, "planner", in.CommandID, in.Graph, d)
		return s.graphToolResult(t, err), nil, nil
	})
	mcp.AddTool(srv, &mcp.Tool{Name: "select_capabilities", Description: "Select approved real experts for the clarified task. Persist eligibility, exclusions, configured ranking, composition and graph mapping. Unknown is not eligible; no_graph must not be proposed."}, func(ctx context.Context, _ *mcp.CallToolRequest, q selection.SelectionQuery) (*mcp.CallToolResult, any, error) {
		out, err := s.selectCapabilities(ctx, q)
		return toolResult(out, err), nil, nil
	})
	for _, name := range []string{"get_plan", "run_status", "verify_run"} {
		name := name
		toolMeta := meta
		if name == "verify_run" {
			toolMeta = nil
		}
		mcp.AddTool(srv, &mcp.Tool{Name: name, Description: "Read authoritative task, committed results and current validity. Completion is not user acceptance.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}, Meta: toolMeta}, func(ctx context.Context, _ *mcp.CallToolRequest, in TaskRef) (*mcp.CallToolResult, any, error) {
			if name == "verify_run" {
				out, err := s.VerifyRun(ctx, in.TaskID)
				return toolResult(out, err), nil, nil
			}
			t, err := s.Store.GetTask(ctx, in.TaskID)
			return s.graphToolResult(t, err), nil, nil
		})
	}
	inner := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := s.authorize(r, "planner"); err != nil {
			writeJSON(w, 401, map[string]string{"error": "scoped planner identity required"})
			return
		}
		inner.ServeHTTP(w, r)
	})
}

func (s *Server) uiRoutes(m *http.ServeMux) {
	m.Handle("/mcp", s.MCP())
	m.HandleFunc("GET /ui/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		doc, err := embeddedPage("task.html")
		if err != nil {
			http.Error(w, "UI unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		_, _ = w.Write(doc)
	})
	m.HandleFunc("GET /v1/projections/{id}", s.wrap(nil, func(w http.ResponseWriter, r *http.Request) (any, error) {
		if !s.allowView(r.PathValue("id"), strings.TrimPrefix(r.Header.Get("Authorization"), "ReportView ")) {
			return nil, fault("UNAUTHORIZED", "task view grant expired or invalid; refresh via get_plan", 401)
		}
		w.Header().Set("Access-Control-Allow-Origin", "*") // No cookie credentials; only the one-task grant is accepted.
		return s.Store.GetTask(r.Context(), r.PathValue("id"))
	}))
	m.HandleFunc("OPTIONS /v1/projections/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET,OPTIONS")
		w.WriteHeader(204)
	})
}

func embeddedPage(name string) ([]byte, error) {
	doc, err := webFiles.ReadFile("web/" + name)
	if err != nil {
		return nil, err
	}
	guard, err := webFiles.ReadFile("web/projection.js")
	if err != nil {
		return nil, err
	}
	return []byte(strings.Replace(string(doc), "/* PROJECTION_GUARD */", string(guard), 1)), nil
}
