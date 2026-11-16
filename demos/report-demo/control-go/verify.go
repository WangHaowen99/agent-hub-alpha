package main

import (
	"context"
	"net/http"
)

// VerifyRun is a read-only evidence inspection. It never runs a graph, renews a
// lease, republishes an artifact, or treats a saved trace as a new approval.
func (s *Server) VerifyRun(ctx context.Context, id string) (any, error) {
	t, err := s.Store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	issues := []map[string]string{}
	formal, skipped := 0, 0
	for node, ns := range t.Nodes {
		if ns.Result != nil {
			formal++
			if !ns.Result.Valid {
				issues = append(issues, map[string]string{"node_id": node, "code": "INVALIDATED"})
			}
			if _, err := s.Files.Read(ns.Result.ArtifactID); err != nil {
				issues = append(issues, map[string]string{"node_id": node, "code": "EVIDENCE_MISSING_OR_CHANGED"})
			}
		} else if ns.Status == "AUTHORIZED_SKIP" && ns.SkipCheck != "" {
			skipped++
		} else {
			issues = append(issues, map[string]string{"node_id": node, "code": "NOT_FORMAL"})
		}
	}
	var calls, observations, audit int
	if err = s.Store.Pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM remote_calls WHERE scope=$1 AND task_id=$2),(SELECT count(*) FROM observations WHERE scope=$1 AND call_id IN (SELECT id FROM remote_calls WHERE scope=$1 AND task_id=$2)),(SELECT count(*) FROM audit_outbox WHERE scope=$1 AND aggregate=$2)", s.Store.Scope, id).Scan(&calls, &observations, &audit); err != nil {
		return nil, err
	}
	current, err := s.Store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if current.Generation != t.Generation {
		issues = append(issues, map[string]string{"code": "SNAPSHOT_CHANGED_RETRY_READ"})
	}
	return map[string]any{"task_id": id, "generation": t.Generation, "status": t.Status, "formal_nodes": formal, "authorized_skips": skipped, "remote_calls": calls, "observations": observations, "audit_events": audit, "issues": issues, "evidence_intact": len(issues) == 0, "read_only": true, "claim": "artifact integrity and recorded formal facts; not independent business acceptance"}, nil
}

func (s *Server) verifyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/tasks/{id}/verify", s.wrap([]string{"planner", "worker", "operator", "user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.VerifyRun(r.Context(), r.PathValue("id"))
	}))
}
