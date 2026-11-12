package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Empty struct{}

// The plan graph, approval and the live run are shown in the 编排台
// workbench (a WorkSwarm Application Plugin), the one place to act on a plan;
// tool results only tell the planner where to send the user.
const workbench = "「编排台」（WorkSwarm 左侧导航）"

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
		v["next"] = fmt.Sprintf("编排图在%s里，用户在那里看成员、子任务和依赖，并点「批准」输入密码确认。你只用一两句话说明组队思路，请用户到%s确认 %s v%d；不要画图，不要逐个列出成员。批准前不会执行任何步骤，批准后编排台会实时显示执行到哪一步。", workbench, workbench, p.ID, p.Version)
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
		"where": fmt.Sprintf("执行图实时显示在%s的 %s v%d", workbench, r.PlanID, r.PlanVersion),
	}
}

// plannerServer is what the WorkSwarm orchestration skill sees: it can read
// the catalog, propose and revise plans, and read state. It cannot approve.
func (s *Service) plannerServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "credit-orchestration", Version: "0.1.0"}, nil)
	ro := &mcp.ToolAnnotations{ReadOnlyHint: true}
	planResult := func(p *Plan) *mcp.CallToolResult { return result(s.planView(p)) }
	mcp.AddTool(srv, &mcp.Tool{Name: "list_capabilities", Annotations: ro,
		Description: "List the registered experts (capabilities) with their signed contracts: what each reads, its effects and bounds. Only these experts can be put on a team. plan_rules is the due-diligence rule every plan must satisfy; precheck enforces it."},
		func(context.Context, *mcp.CallToolRequest, Empty) (*mcp.CallToolResult, any, error) {
			return result(map[string]any{"experts": s.cat.Cards(), "cases": []string{"C006", "C007"}, "plan_rules": ddRules()}), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "propose_plan",
		Description: "Propose (or revise, with plan_id) a team plan for a case: each agent binds a registered expert to a role, a sub-task and its dependencies. The service compiles it into a blueprint and runs the contract precheck. Nothing executes: execution needs the user's approval in the 编排台 workbench."},
		func(_ context.Context, _ *mcp.CallToolRequest, in ProposeInput) (*mcp.CallToolResult, any, error) {
			p, err := s.Propose(in)
			if err != nil {
				return fail(err), nil, nil
			}
			return planResult(p), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "get_plan", Annotations: ro, Description: "Read the latest version of a plan."},
		func(_ context.Context, _ *mcp.CallToolRequest, in PlanRef) (*mcp.CallToolResult, any, error) {
			p, ok := s.Plan(in.PlanID, 0)
			if !ok {
				return fail(fmt.Errorf("unknown plan %s", in.PlanID)), nil, nil
			}
			return planResult(p), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "run_status", Annotations: ro,
		Description: "Read the authoritative state of an approved run: each step's status, the adopted judgment, and the bank receipt. The 编排台 workbench shows it live."},
		func(_ context.Context, _ *mcp.CallToolRequest, in RunRef) (*mcp.CallToolResult, any, error) {
			id := in.RunID
			if id == "" {
				if r, ok := s.RunForPlan(in.PlanID); ok {
					id = r.ID
				}
			}
			r, p, ok := s.RunByID(id)
			if !ok {
				return fail(fmt.Errorf("no approved run yet — the user has to approve the plan in the 编排台 workbench")), nil, nil
			}
			return result(s.runView(r, p)), nil, nil
		})
	mcp.AddTool(srv, &mcp.Tool{Name: "verify_run", Annotations: ro,
		Description: "Verify a finished run with the festival checkers (signatures, approved plan, compose admission, evidence coverage, violations, risk judgment, replay, ledger rules)."},
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
			out := map[string]any{"summary": rep.Summary, "ok": rep.OK, "where": "检查结果显示在" + workbench + "的执行图下方"}
			checks := []map[string]any{}
			for _, c := range rep.Checks {
				ck := map[string]any{"name": c.Name, "ok": c.OK, "detail": c.Detail}
				if !c.OK {
					ck["items"] = c.Items // the evidence for a failure, so it is reported, not guessed
				}
				checks = append(checks, ck)
			}
			out["checks"] = checks
			return result(out), nil, nil
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
