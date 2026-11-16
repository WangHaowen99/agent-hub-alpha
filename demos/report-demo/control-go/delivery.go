package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// Approval binds the exact report bytes and a qualified PASS check. Reading
// that immutable object later is not another publish action.
func (s *Store) ApproveReport(ctx context.Context, id, artifact, principal string) (any, error) {
	return s.mutate(ctx, id, "REPORT_APPROVED", func(tx pgx.Tx, t *Task) (any, error) {
		if err := s.currentDeliveryPolicy(t); err != nil {
			return nil, err
		}
		if t.Status != "AWAITING_ACCEPTANCE" && t.Status != "DELIVERED" {
			return nil, fault("POLICY_REJECTED", "computation is not complete", 409)
		}
		var digest string
		var valid bool
		err := tx.QueryRow(ctx, "SELECT r.artifact_digest,r.valid FROM committed_results r JOIN artifacts a ON a.scope=r.scope AND a.id=r.artifact_id WHERE r.scope=$1 AND r.task_id=$2 AND r.artifact_id=$3 AND a.schema_version='report/1'", s.Scope, id, artifact).Scan(&digest, &valid)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fault("NOT_FOUND", "report is not a formal result of this task", 404)
		}
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, fault("INVALIDATED", "report withdrawn", 409)
		}
		passed, err := s.qualifiedReportCheck(ctx, tx, t, digest)
		if err != nil {
			return nil, err
		}
		if !passed {
			return nil, fault("CHECK_REQUIRED", "matching PASS check required", 409)
		}
		receipt := newID("approval-")
		_, err = tx.Exec(ctx, "INSERT INTO approvals(scope,id,task_id,action,object_digest,principal,expires_at) VALUES($1,$2,$3,'deliver',$4,$5,clock_timestamp()+interval '1 hour')", s.Scope, receipt, id, digest, principal)
		return map[string]string{"approval_id": receipt, "artifact_id": artifact, "artifact_digest": digest, "task_id": id}, err
	})
}

func (s *Store) Deliver(ctx context.Context, id, artifact, b, principal string) (any, error) {
	if b == "" || len(b) > 200 {
		return nil, fault("INVALID_REQUEST", "stable delivery_id required", 400)
	}
	payload, err := Digest(map[string]string{"task_id": id, "artifact_id": artifact, "action": "publish-report", "renderer": "structured-json/1"})
	if err != nil {
		return nil, err
	}
	return s.mutate(ctx, id, "REPORT_DELIVERED", func(tx pgx.Tx, t *Task) (any, error) {
		if err := s.currentDeliveryPolicy(t); err != nil {
			return nil, err
		}
		if t.Pause {
			return nil, fault("PAUSED", "new delivery admission stopped", 409)
		}
		if t.Status != "AWAITING_ACCEPTANCE" && t.Status != "DELIVERED" {
			return nil, fault("POLICY_REJECTED", "report not ready for delivery", 409)
		}
		var digest string
		var valid bool
		err := tx.QueryRow(ctx, "SELECT r.artifact_digest,r.valid FROM committed_results r JOIN artifacts a ON a.scope=r.scope AND a.id=r.artifact_id WHERE r.scope=$1 AND r.task_id=$2 AND r.artifact_id=$3 AND a.schema_version='report/1'", s.Scope, id, artifact).Scan(&digest, &valid)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fault("NOT_FOUND", "formal report missing", 404)
		}
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, fault("INVALIDATED", "report withdrawn", 409)
		}
		passed, err := s.qualifiedReportCheck(ctx, tx, t, digest)
		if err != nil {
			return nil, err
		}
		var allowed bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM approvals WHERE scope=$1 AND task_id=$2 AND action='deliver' AND object_digest=$3 AND principal=$4 AND expires_at>clock_timestamp())`, s.Scope, id, digest, principal).Scan(&allowed)
		if err != nil {
			return nil, err
		}
		if !allowed || !passed {
			return nil, fault("APPROVAL_REQUIRED", "current same-object approval/check required", 403)
		}
		var previous string
		var receipt []byte
		err = tx.QueryRow(ctx, "SELECT payload_digest,receipt FROM delivery_intents WHERE scope=$1 AND id=$2", s.Scope, b).Scan(&previous, &receipt)
		if err == nil {
			if previous != payload {
				return nil, fault("INPUT_CONFLICT", "delivery identity changed payload", 409)
			}
			return json.RawMessage(receipt), nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		// The local publisher owns a separate domain. No graph worker lease is
		// revived when the user comes back after graph execution has finished.
		var epoch string
		err = tx.QueryRow(ctx, `INSERT INTO run_leases(scope,task_id,domain,epoch,owner,control_session,expires_at,status) VALUES($1,$2,'delivery',1,'go-publisher',$3,clock_timestamp()+make_interval(secs=>$4),'ACTIVE')
		 ON CONFLICT(scope,task_id,domain) DO UPDATE SET epoch=run_leases.epoch+1,owner='go-publisher',control_session=EXCLUDED.control_session,expires_at=EXCLUDED.expires_at,status='ACTIVE'
		 WHERE run_leases.expires_at<=clock_timestamp() OR run_leases.status<>'ACTIVE' OR run_leases.control_session<>EXCLUDED.control_session RETURNING epoch::text`, s.Scope, id, s.Session, s.LeaseSeconds).Scan(&epoch)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fault("WAIT", "another delivery publisher owns permit", 409)
		}
		if err != nil {
			return nil, err
		}
		out := map[string]any{"delivery_id": b, "task_id": id, "artifact_id": artifact, "artifact_digest": digest, "receipt_id": newID("delivery-receipt-"), "epoch": epoch, "status": "DELIVERED", "renderer": "structured-json/1", "recorded_at": time.Now().UTC()}
		receipt, err = json.Marshal(out)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, "INSERT INTO delivery_intents(scope,id,task_id,payload_digest,owner_epoch,state,receipt) VALUES($1,$2,$3,$4,$5::bigint,'DELIVERED',$6)", s.Scope, b, id, payload, epoch, receipt)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, "UPDATE run_leases SET status='RELEASED' WHERE scope=$1 AND task_id=$2 AND domain='delivery'", s.Scope, id)
		if err != nil {
			return nil, err
		}
		t.Status = "DELIVERED"
		return out, nil
	})
}

func (s *Store) currentDeliveryPolicy(t *Task) error {
	if s.DeliveryPolicyVersion != "" && t.Graph.PolicyVersion != s.DeliveryPolicyVersion {
		return fault("BINDING_CHANGED", "current delivery policy differs; old check/approval cannot admit a new action", 409)
	}
	return nil
}

func (s *Server) deliveryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/tasks/{id}/artifacts/{artifact}/approve", s.wrap([]string{"user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		p, err := s.userWrite(r)
		if err != nil {
			return nil, err
		}
		if _, err = s.Files.Read(r.PathValue("artifact")); err != nil {
			return nil, err
		}
		return s.Store.ApproveReport(r.Context(), r.PathValue("id"), r.PathValue("artifact"), p)
	}))
	mux.HandleFunc("POST /v1/tasks/{id}/artifacts/{artifact}/deliver", s.wrap([]string{"user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		p, err := s.userWrite(r)
		if err != nil {
			return nil, err
		}
		var body struct {
			ID string `json:"delivery_id"`
		}
		if err = readBody(r, &body); err != nil {
			return nil, err
		}
		if _, err = s.Files.Read(r.PathValue("artifact")); err != nil {
			return nil, err
		}
		return s.Store.Deliver(r.Context(), r.PathValue("id"), r.PathValue("artifact"), body.ID, p)
	}))
	mux.HandleFunc("GET /v1/tasks/{id}/artifacts/{artifact}/download", s.wrap([]string{"user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var allowed bool
		err := s.Store.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM delivery_intents d JOIN committed_results r ON r.scope=d.scope AND r.task_id=d.task_id AND r.artifact_id=$3 WHERE d.scope=$1 AND d.task_id=$2 AND d.receipt->>'artifact_id'=$3 AND d.state='DELIVERED' AND r.valid)`, s.Store.Scope, r.PathValue("id"), r.PathValue("artifact")).Scan(&allowed)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, fault("DELIVERY_REQUIRED", "no current valid delivered artifact", 403)
		}
		raw, err := s.Files.Read(r.PathValue("artifact"))
		if err != nil {
			return nil, err
		}
		w.Header().Set("Content-Disposition", "attachment; filename=annual-report.json")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		// The artifact digest identifies these exact bytes. Encoding RawMessage
		// again would compact/escape JSON and append a newline. A disconnected
		// reader does not change the immutable report or create another effect.
		_, _ = w.Write(raw)
		return nil, nil
	}))
}
