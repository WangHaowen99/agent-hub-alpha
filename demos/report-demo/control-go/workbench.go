package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// W02's view model is a projection of W04. It owns no plans, ready queue or results.
func workbenchStatus(t *Task) string {
	if t.Pause && t.Status != "BLOCKED" && t.Status != "NEEDS_REVIEW" {
		return "waiting"
	}
	switch t.Status {
	case "AWAITING_APPROVAL":
		return ""
	case "QUEUED":
		return "queued"
	case "BLOCKED", "NEEDS_REVIEW":
		return "blocked"
	case "WAITING_INPUT", "WAITING_AUTH", "WAITING_REMOTE", "WAITING_RECONCILIATION", "UNKNOWN":
		return "waiting"
	case "COMPUTE_COMPLETED", "AWAITING_ACCEPTANCE", "DELIVERED":
		return "done"
	case "FAILED":
		return "failed"
	default:
		if t.Pause {
			return "waiting"
		}
		return "running"
	}
}
func (s *Server) workbenchRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /v1/workbench/plans", s.wrap([]string{"operator", "user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		rows, err := s.Store.Pool.Query(r.Context(), "SELECT id FROM tasks WHERE scope=$1 ORDER BY updated_at DESC,id LIMIT 30", s.Store.Scope)
		if err != nil {
			return nil, err
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return nil, err
		}
		plans := []any{}
		for _, id := range ids {
			t, err := s.Store.GetTask(r.Context(), id)
			if err != nil {
				return nil, err
			}
			row := map[string]any{"plan": id, "version": 1, "case": t.Graph.MaterialID, "goal": t.Graph.Goal, "ok": true}
			if t.Status != "AWAITING_APPROVAL" {
				row["run"] = id
				row["run_status"] = workbenchStatus(t)
			}
			plans = append(plans, row)
		}
		return map[string]any{"plans": plans}, nil
	}))
	m.HandleFunc("GET /v1/workbench/tasks/{id}", s.wrap([]string{"operator", "user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		t, err := s.Store.GetTask(r.Context(), r.PathValue("id"))
		if err != nil {
			return nil, err
		}
		nodes := []any{}
		byNode := map[string]map[string]any{}
		layers := [][]string{}
		depth := map[string]int{}
		// Graph edges were already validated; recursive depth only lays them out.
		var visit func(string) int
		visit = func(id string) int {
			if d, ok := depth[id]; ok {
				return d
			}
			d := 0
			for _, n := range t.Graph.Nodes {
				if n.ID == id {
					for _, dep := range n.DependsOn {
						v := visit(dep) + 1
						if v > d {
							d = v
						}
					}
					break
				}
			}
			depth[id] = d
			return d
		}
		for _, n := range t.Graph.Nodes {
			cap, _ := s.Catalog.Get(n.Expert)
			ns := t.Nodes[n.ID]
			state := "pending"
			reason := ""
			if ns != nil {
				reason = ns.Reason
				switch ns.Status {
				case "COMMITTED":
					state = "done"
				case "AUTHORIZED_SKIP":
					state = "skipped"
				case "RUNNING":
					state = "running"
				case "UNKNOWN":
					state = "unknown"
				case "WAITING_REMOTE", "WAITING_INPUT", "WAITING_RECONCILIATION":
					state = "waiting"
				case "BLOCKED", "FAILED", "NEEDS_REVIEW":
					state = "failed"
				}
			}
			node := map[string]any{"id": n.ID, "name": cap.Name, "kind": cap.Kind, "role": n.Role, "task": n.Task, "deps": n.DependsOn, "status": state, "activity": reason, "contract": n.Expert}
			if n.Condition != nil {
				node["when"] = map[string]any{"step": n.Condition.Step, "contains": n.Condition.Outcome}
			}
			if ns != nil && ns.Result != nil {
				node["output"] = string(ns.Result.Output)
			}
			nodes = append(nodes, node)
			byNode[n.ID] = node
			d := visit(n.ID)
			for len(layers) <= d {
				layers = append(layers, []string{})
			}
			layers[d] = append(layers[d], n.ID)
		}
		out := map[string]any{"plan": t.ID, "version": 1, "case": t.Graph.MaterialID, "goal": t.Graph.Goal, "layers": layers, "nodes": nodes, "ok": true, "hash": t.GraphDigest, "rev": t.Generation, "task_status": t.Status, "delivery_receipts": t.DeliveryReceipts}
		if err := s.workbenchCalls(r, t, byNode, out); err != nil {
			return nil, err
		}
		if t.Status == "AWAITING_APPROVAL" {
			out["approve"] = "plugin"
		} else {
			out["run"] = map[string]any{"id": t.ID, "status": workbenchStatus(t), "leases": 1, "url": "", "decision": t.Status}
		}
		if ref := t.Graph.Selection; ref != nil {
			rec, err := s.Store.selectionRecord(r.Context(), s.Store.Pool, ref)
			if err != nil {
				return nil, err
			}
			out["selection"] = rec
		}
		reports := []any{}
		for _, graphNode := range t.Graph.Nodes {
			id := graphNode.ID
			ns := t.Nodes[id]
			if ns.Result == nil || !ns.Result.Valid {
				continue
			}
			var body map[string]any
			if json.Unmarshal(ns.Result.Output, &body) != nil {
				continue
			}
			isReport := false
			for _, n := range t.Graph.Nodes {
				if n.ID == id {
					cap, _ := s.Catalog.Get(n.Expert)
					isReport = cap.Kind == "write"
					break
				}
			}
			if isReport {
				outcome := "UNKNOWN"
				for _, check := range t.Nodes {
					if check.Result == nil || !check.Result.Valid {
						continue
					}
					var value struct {
						Object string `json:"object_digest"`
					}
					if json.Unmarshal(check.Result.Output, &value) == nil && value.Object == ns.Result.ArtifactDigest {
						outcome = effectiveCheck(check)
						break
					}
				}
				reports = append(reports, map[string]any{"node": id, "artifact_id": ns.Result.ArtifactID, "digest": ns.Result.ArtifactDigest, "output": body, "check_outcome": outcome, "can_approve": outcome == "PASS" && (t.Status == "AWAITING_ACCEPTANCE" || t.Status == "DELIVERED")})
			}
		}
		out["reports"] = reports
		if review := s.workbenchHumanReview(t); review != nil {
			out["human_review"] = review
		}
		if r.URL.Query().Get("verify") == "1" {
			v, err := s.VerifyRun(r.Context(), t.ID)
			if err != nil {
				return nil, err
			}
			out["verification"] = v
			check := v.(map[string]any)
			intact, _ := check["evidence_intact"].(bool)
			out["verify"] = map[string]any{"ok": intact, "summary": "正式事实与产物完整性；不代表独立业务验收", "checks": []any{map[string]any{"name": "evidence_intact", "ok": intact, "detail": "保留程序检查、用户批准与交付各自语义"}}}
		}
		current, err := s.Store.GetTask(r.Context(), t.ID)
		if err != nil {
			return nil, err
		}
		if current.Generation != t.Generation {
			return nil, fault("SNAPSHOT_CHANGED", "projection changed during read; retain the last view and poll again", 409)
		}
		return out, nil
	}))
}

// Only current, unresolved HUMAN checks offer a link. The trusted page still
// requires its own user session/CSRF and validates the exact checker/report.
func (s *Server) workbenchHumanReview(t *Task) map[string]any {
	checks := []any{}
	for _, node := range t.Graph.Nodes {
		ns := t.Nodes[node.ID]
		if ns == nil || ns.Result == nil || !ns.Result.Valid || effectiveCheck(ns) != "HUMAN" {
			continue
		}
		var check struct {
			ObjectDigest string `json:"object_digest"`
		}
		if json.Unmarshal(ns.Result.Output, &check) != nil || check.ObjectDigest == "" {
			continue
		}
		checks = append(checks, map[string]string{"node": node.ID, "check_artifact": ns.Result.ArtifactID, "object_digest": check.ObjectDigest})
	}
	if len(checks) == 0 {
		return nil
	}
	return map[string]any{"url": strings.TrimRight(s.Config.PublicURL, "/") + "/ui/tasks/" + url.PathEscape(t.ID), "checks": checks}
}

func (s *Server) workbenchCalls(r *http.Request, t *Task, nodes map[string]map[string]any, out map[string]any) error {
	rows, err := s.Store.Pool.Query(r.Context(), "SELECT document FROM remote_calls WHERE scope=$1 AND task_id=$2 ORDER BY attempt", s.Store.Scope, t.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	latest := map[string]Call{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return err
		}
		var call Call
		if err = json.Unmarshal(raw, &call); err != nil {
			return err
		}
		latest[call.NodeID] = call
	}
	if err = rows.Err(); err != nil {
		return err
	}
	services := map[string]bool{}
	planned, linked, completed, unresolved, failed := 0, 0, 0, 0, 0
	for _, n := range t.Graph.Nodes {
		cap, ok := s.Catalog.Get(n.Expert)
		if !ok || cap.AgentID == "" {
			continue
		}
		planned++
		services[cap.AgentID] = true
		execution := map[string]any{"kind": "agent_bus", "path_mode": "DIRECT"}
		// A changed live catalog is never substituted for a historical binding.
		if t.Graph.CatalogDigest == s.Store.CurrentCatalogDigest && cap.RemoteBinding != nil {
			execution["binding"] = map[string]any{"agent_id": cap.AgentID, "version": cap.RemoteBinding.CapabilityVersion}
		}
		nodes[n.ID]["execution"] = execution
		if call, ok := latest[n.ID]; ok {
			remote := map[string]any{"agent_id": call.AgentID, "task_id": call.RemoteTaskID, "status": call.State, "node_execution_id": call.ID, "attempt": call.Attempt, "request_id": call.MessageID, "context_id": call.ContextID, "input_digest": call.InputDigest}
			if call.RemoteTaskID != "" {
				linked++
			}
			if call.State == "FAILED" || call.Conflicted {
				failed++
			} else if call.State != "COMPLETED" {
				unresolved++
			}
			if ns := t.Nodes[n.ID]; ns != nil {
				remote["reason"] = ns.Reason
				if ns.Result != nil && ns.Result.Valid {
					remote["artifact_id"] = ns.Result.ArtifactID
					remote["artifact_digest"] = ns.Result.ArtifactDigest
					completed++
				}
			}
			var route struct {
				Card struct {
					Raw string `json:"rawJson"`
				} `json:"card"`
			}
			if json.Unmarshal(call.Route, &route) == nil && route.Card.Raw != "" {
				var card struct {
					Version string `json:"version"`
				}
				if json.Unmarshal([]byte(route.Card.Raw), &card) == nil {
					execution["binding"] = map[string]any{"agent_id": call.AgentID, "version": card.Version, "card_digest": "sha256:" + tokenDigest(route.Card.Raw)}
				}
			}
			nodes[n.ID]["remote"] = remote
		}
	}
	out["bus"] = map[string]any{"planned_nodes": planned, "services": len(services), "linked_tasks": linked, "completed": completed, "unresolved": unresolved, "failed": failed, "reused": 0}
	return nil
}
