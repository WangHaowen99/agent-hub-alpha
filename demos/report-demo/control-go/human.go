package main

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"net/http"
)

// A human review is a distinct immutable fact. It never rewrites the checker
// artifact or says that the program proved a conclusion outside its coverage.
type CheckReview struct {
	CheckArtifact string `json:"check_artifact"`
	ObjectDigest  string `json:"object_digest"`
	Decision      string `json:"decision"`
	Reason        string `json:"reason"`
	Principal     string `json:"principal"`
	Receipt       string `json:"receipt"`
}

func effectiveCheck(ns *NodeState) string {
	if ns == nil || ns.Result == nil || !ns.Result.Valid {
		return "UNKNOWN"
	}
	var check struct {
		Outcome      string `json:"outcome"`
		ObjectDigest string `json:"object_digest"`
	}
	if json.Unmarshal(ns.Result.Output, &check) != nil {
		return "UNKNOWN"
	}
	if check.Outcome == "HUMAN" && ns.Review != nil && ns.Review.CheckArtifact == ns.Result.ArtifactID && ns.Review.ObjectDigest == check.ObjectDigest && ns.Review.Receipt != "" {
		switch ns.Review.Decision {
		case "ACCEPT":
			return "PASS"
		case "REVISE":
			return "REVISE"
		case "REJECT":
			return "FAIL"
		}
	}
	return check.Outcome
}

func (s *Store) ReviewCheck(ctx context.Context, id, node string, in CheckReview, principal string) (any, error) {
	if (in.Decision != "ACCEPT" && in.Decision != "REVISE" && in.Decision != "REJECT") || len(in.Reason) < 12 || len(in.Reason) > 4000 {
		return nil, fault("INVALID_REQUEST", "decision and substantive review reason required", 400)
	}
	return s.mutate(ctx, id, "HUMAN_REVIEW_RECORDED", func(tx pgx.Tx, t *Task) (any, error) {
		ns := t.Nodes[node]
		if ns == nil || ns.Result == nil || !ns.Result.Valid || ns.Result.ArtifactID != in.CheckArtifact {
			return nil, fault("INPUT_CONFLICT", "current valid checker artifact required", 409)
		}
		var kind, outcome, object string
		if err := tx.QueryRow(ctx, "SELECT checker_version,outcome,object_digest FROM checks WHERE scope=$1 AND task_id=$2 AND id=$3", s.Scope, id, in.CheckArtifact).Scan(&kind, &outcome, &object); err != nil {
			return nil, fault("NOT_FOUND", "qualified checker fact missing", 404)
		}
		if outcome != "HUMAN" || object != in.ObjectDigest {
			return nil, fault("INPUT_CONFLICT", "only exact HUMAN object can receive this review", 409)
		}
		var checked struct {
			Failures []string `json:"failures"`
		}
		if json.Unmarshal(ns.Result.Output, &checked) != nil || len(checked.Failures) > 0 {
			return nil, fault("CHECK_REQUIRED", "program failures cannot be waived by this review", 409)
		}
		validReport := false
		for _, n := range t.Nodes {
			if n.Result != nil && n.Result.Valid && n.Result.ArtifactDigest == object {
				validReport = true
			}
		}
		if !validReport {
			return nil, fault("INVALIDATED", "reviewed report no longer valid", 409)
		}
		if ns.Review != nil {
			if ns.Review.Decision == in.Decision && ns.Review.Reason == in.Reason && ns.Review.Principal == principal {
				return ns.Review, nil
			}
			return nil, fault("INPUT_CONFLICT", "review fact is immutable; new changed content requires a new check", 409)
		}
		in.Principal = principal
		in.Receipt = newID("human-review-")
		ns.Review = &in
		if in.Decision == "REJECT" {
			t.Status = "NEEDS_REVIEW"
		} else {
			t.Status = "WAITING_INPUT"
		}
		return in, nil
	})
}

func (s *Store) qualifiedReportCheck(ctx context.Context, tx pgx.Tx, t *Task, digest string) (bool, error) {
	rows, err := tx.Query(ctx, "SELECT c.id FROM checks c JOIN committed_results r ON r.scope=c.scope AND r.task_id=c.task_id AND r.artifact_id=c.id WHERE c.scope=$1 AND c.task_id=$2 AND c.object_digest=$3 AND r.valid", s.Scope, t.ID, digest)
	if err != nil {
		return false, err
	}
	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return false, err
		}
		ids[id] = true
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return false, err
	}
	for _, ns := range t.Nodes {
		if ns.Result != nil && ids[ns.Result.ArtifactID] && effectiveCheck(ns) == "PASS" {
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) humanRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/tasks/{id}/checks/{node}/review", s.wrap([]string{"user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		principal, err := s.userWrite(r)
		if err != nil {
			return nil, err
		}
		var in CheckReview
		if err = readBody(r, &in); err != nil {
			return nil, err
		}
		if in.Principal != "" || in.Receipt != "" {
			return nil, fault("INVALID_REQUEST", "review principal and receipt are assigned by server", 400)
		}
		if _, err = s.Files.Read(in.CheckArtifact); err != nil {
			return nil, err
		}
		return s.Store.ReviewCheck(r.Context(), r.PathValue("id"), r.PathValue("node"), in, principal)
	}))
}
