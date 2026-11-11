package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Empty struct{}

// The plan graph is an MCP App: WorkSwarm renders it inline under the tool
// call, and it polls the read-only /api/graph projection while the plan waits
// for approval or runs (app-initiated tool calls would prompt the user each time).
const (
	graphURI = "ui://credit/graph"
	appMIME  = "text/html;profile=mcp-app"
)

//go:embed app/dist/graph-app.html
var graphAppHTML string

var graphMeta = mcp.Meta{"ui": map[string]any{"resourceUri": graphURI}}

func withGraph(res *mcp.CallToolResult, g *Graph) *mcp.CallToolResult {
	res.StructuredContent = g
	return res
}

type PlanRef struct {
	PlanID string `json:"plan_id" jsonschema:"plan id such as P1"`
}

type RunRef struct {
	RunID  string `json:"run_id,omitempty" jsonschema:"run id such as R1"`
	PlanID string `json:"plan_id,omitempty" jsonschema:"or the plan id whose approved run to read"`
}

func result(v any) *mcp.CallToolResult {
	raw, _ := json.Marshal(v)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}
}

func fail(err error) *mcp.CallToolResult {
	res := result(map[string]string{"error": err.Error()})
	res.IsError = true
	return res
}

func (s *Service) planView(p *Plan) map[string]any {
	v := map[string]any{
		"plan_id": p.ID, "version": p.Version, "case_id": p.CaseID, "hash": p.Hash,
		"precheck": map[string]any{"ok": p.Precheck.OK, "not_held": p.Precheck.NotHeld, "rules": p.Precheck.Rules, "errors": p.Precheck.Errors},
	}
	if p.Precheck.OK {
		v["next"] = "编排图已作为卡片显示在对话里，用户能看到每个成员、子任务和依赖：不要再贴图，也不要逐个列出成员，只用一两句话说明组队思路。请用户在卡片上点「批准」并输入密码确认；批准前不会执行任何步骤，批准后卡片会实时显示执行到哪一步。"
	} else {
		v["next"] = "规约预检未通过：把 not_held、rules 和 errors 如实告诉用户。权限不足就换用权限范围内的专家，违反尽调规约（rules）就按规约补齐成员或依赖；不要为了过预检删掉成员。改完用同一个 plan_id 重新 propose_plan。"
	}
	if p.RunID != "" {
		v["run_id"] = p.RunID
	}
	return v
}

func (s *Service) runView(r *Run, p *Plan) map[string]any {
	steps := []map[string]any{}
	for _, id := range r.Order {
		st := r.Steps[id]
		steps = append(steps, map[string]any{"step": id, "expert": expertByID(st.Expert).Name, "status": st.Status, "reused": st.Reused, "output": clip(st.Output, 200)})
	}
	return map[string]any{
		"run_id": r.ID, "plan_id": r.PlanID, "plan_version": r.PlanVersion, "status": r.Status, "approved_by": r.ApprovedBy,
		"steps": steps, "decision": r.Decision, "receipt": r.Receipt, "violations": r.Violations,
		"run_url":    fmt.Sprintf("%s/ui/run/%s", s.cfg.PublicURL, r.ID),
		"verify_url": fmt.Sprintf("%s/ui/run/%s/verify", s.cfg.PublicURL, r.ID),
	}
}

// plannerServer is what the WorkSwarm orchestration skill sees: it can read
// the catalog, propose and revise plans, and read state. It cannot approve.
func (s *Service) plannerServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "credit-orchestration", Version: "0.1.0"}, nil)
	ro := &mcp.ToolAnnotations{ReadOnlyHint: true}
	// The app rebases links from old tool results onto this address.
	appDoc := strings.Replace(graphAppHTML, "<head>", `<head><meta name="credit-base" content="`+html.EscapeString(s.cfg.PublicURL)+`">`, 1)
	srv.AddResource(&mcp.Resource{URI: graphURI, Name: "credit-plan-graph", MIMEType: appMIME},
		func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: graphURI, MIMEType: appMIME, Text: appDoc,
				Meta: mcp.Meta{"ui": map[string]any{"csp": map[string]any{"connectDomains": s.cfg.Origins}}}}}}, nil
		})
	planResult := func(p *Plan) *mcp.CallToolResult {
		res := result(s.planView(p))
		if g, err := s.graph(p, false); err == nil {
			withGraph(res, g)
		}
		return res
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "list_capabilities", Annotations: ro,
		Description: "List the registered experts (capabilities) with their signed contracts: what each reads, its effects and bounds. Only these experts can be put on a team. plan_rules is the due-diligence rule every plan must satisfy; precheck enforces it."},
		func(context.Context, *mcp.CallToolRequest, Empty) (*mcp.CallToolResult, any, error) {
			return result(map[string]any{"experts": s.cat.Cards(), "cases": []string{"C006", "C007"}, "plan_rules": ddRules()}), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "propose_plan", Meta: graphMeta,
		Description: "Propose (or revise, with plan_id) a team plan for a case: each agent binds a registered expert to a role, a sub-task and its dependencies. The service compiles it into a blueprint and runs the contract precheck. Nothing executes: execution needs the user's approval on the confirm page."},
		func(_ context.Context, _ *mcp.CallToolRequest, in ProposeInput) (*mcp.CallToolResult, any, error) {
			p, err := s.Propose(in)
			if err != nil {
				return fail(err), nil, nil
			}
			return planResult(p), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "get_plan", Annotations: ro, Meta: graphMeta, Description: "Read the latest version of a plan."},
		func(_ context.Context, _ *mcp.CallToolRequest, in PlanRef) (*mcp.CallToolResult, any, error) {
			p, ok := s.Plan(in.PlanID, 0)
			if !ok {
				return fail(fmt.Errorf("unknown plan %s", in.PlanID)), nil, nil
			}
			return planResult(p), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "run_status", Annotations: ro, Meta: graphMeta,
		Description: "Read the authoritative state of an approved run: each step's status, the adopted judgment, and the bank receipt. The plan graph in the chat shows it live."},
		func(_ context.Context, _ *mcp.CallToolRequest, in RunRef) (*mcp.CallToolResult, any, error) {
			id := in.RunID
			if id == "" {
				if r, ok := s.RunForPlan(in.PlanID); ok {
					id = r.ID
				}
			}
			r, p, ok := s.RunByID(id)
			if !ok {
				return fail(fmt.Errorf("no approved run yet — the user has to approve the plan on the confirm page")), nil, nil
			}
			res := result(s.runView(r, p))
			if g, err := s.graph(p, false); err == nil {
				withGraph(res, g)
			}
			return res, nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "verify_run", Annotations: ro, Meta: graphMeta,
		Description: "Verify a finished run with the festival checkers (signatures, approved plan, compose admission, evidence coverage, violations, S1 judgment, replay, ledger rules)."},
		func(_ context.Context, _ *mcp.CallToolRequest, in RunRef) (*mcp.CallToolResult, any, error) {
			id := in.RunID
			if id == "" {
				if r, ok := s.RunForPlan(in.PlanID); ok {
					id = r.ID
				}
			}
			rep, err := s.Verify(id, false)
			if err != nil {
				return fail(err), nil, nil
			}
			out := map[string]any{"summary": rep.Summary, "ok": rep.OK, "verify_url": fmt.Sprintf("%s/ui/run/%s/verify", s.cfg.PublicURL, id)}
			checks := []map[string]any{}
			for _, c := range rep.Checks {
				ck := map[string]any{"name": c.Name, "ok": c.OK, "detail": c.Detail}
				if !c.OK {
					ck["items"] = c.Items // the evidence for a failure, so it is reported, not guessed
				}
				checks = append(checks, ck)
			}
			out["checks"] = checks
			res := result(out)
			if _, p, ok := s.RunByID(id); ok {
				if g, err := s.graph(p, false); err == nil {
					g.Verify = rep
					withGraph(res, g)
				}
			}
			return res, nil, nil
		})
	return srv
}

type ToolArg struct {
	CaseID    string `json:"case_id,omitempty" jsonschema:"case id such as C006"`
	CompanyID string `json:"company_id,omitempty" jsonschema:"related company id such as R-601"`
}

func hdr(req *mcp.CallToolRequest, k string) string {
	if req.Extra != nil && req.Extra.Header != nil {
		return req.Extra.Header.Get(k)
	}
	return ""
}

// memberServer serves one expert's read-only data tools to blueprint
// members (only the tools its contract names). The executor mounts it per
// step with X-Run / X-Step / X-Token headers; the call is still checked
// against the contract and the run's case.
func (s *Service) memberServer(e *Expert) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "credit-data-" + e.ID, Version: "0.1.0"}, nil)
	for _, t := range memberTools {
		t := t
		if !contains(e.Tools, t.name) {
			continue
		}
		mcp.AddTool(srv, &mcp.Tool{Name: t.name, Description: t.desc, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}},
			func(_ context.Context, req *mcp.CallToolRequest, in ToolArg) (*mcp.CallToolResult, any, error) {
				arg := in.CaseID
				if t.arg == "company_id" {
					arg = in.CompanyID
				}
				doc, err := s.ToolCall(hdr(req, "X-Run"), hdr(req, "X-Token"), hdr(req, "X-Step"), t.name, arg)
				if err != nil {
					return fail(err), nil, nil
				}
				return result(doc), nil, nil
			})
	}
	return srv
}

func mcpHandler(srv *mcp.Server) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{DisableLocalhostProtection: true})
}
