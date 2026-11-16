package main

// Explicitly opt-in isolated network-effect adapter. It is not registered in
// the normal report demo, not a bank integration, and never marks a task or a
// local report delivery successful. The durable b/intent and observations use
// the same scoped authority tables and are included in existing backups.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const testEffectSchema = "isolated-network-effect/1"

type TestEffectConfig struct {
	URL       string `json:"url"`
	Target    string `json:"target"`
	Token     string `json:"token"`
	TimeoutMS int    `json:"timeout_ms"`
}

func (c TestEffectConfig) validate() error {
	u, err := url.Parse(c.URL)
	if err != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") || u.Port() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.Trim(u.Path, "/") != "" || !identifier.MatchString(c.Target) || len(c.Token) < 24 || c.TimeoutMS < 50 || c.TimeoutMS > 10000 {
		return errors.New("isolated effect requires an explicit loopback HTTP origin, target, separate credential and 50..10000ms timeout")
	}
	return nil
}

type EffectIntent struct {
	Schema        string          `json:"schema_version"`
	ID            string          `json:"delivery_id"`
	TaskID        string          `json:"task_id"`
	ArtifactID    string          `json:"artifact_id"`
	ObjectDigest  string          `json:"object_digest"`
	PayloadDigest string          `json:"payload_digest"`
	Target        string          `json:"target"`
	Origin        string          `json:"origin"`
	Action        string          `json:"action"`
	Renderer      string          `json:"renderer"`
	Principal     string          `json:"principal"`
	Owner         string          `json:"owner"`
	Epoch         string          `json:"epoch"`
	Session       string          `json:"control_session"`
	State         string          `json:"state"`
	Receipt       json.RawMessage `json:"provider_receipt,omitempty"`
}

type EffectReceipt struct {
	ID            string `json:"delivery_id"`
	PayloadDigest string `json:"payload_digest"`
	Status        string `json:"status"`
	ReceiptID     string `json:"receipt_id,omitempty"`
}

type EffectObservation struct {
	State      string          `json:"state"`
	HTTPStatus int             `json:"http_status"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

func (s *Store) getTestEffect(ctx context.Context, tx pgx.Tx, b string) (*EffectIntent, error) {
	var raw []byte
	var err error
	query := "SELECT receipt FROM delivery_intents WHERE scope=$1 AND id=$2"
	if tx == nil {
		err = s.Pool.QueryRow(ctx, query, s.Scope, b).Scan(&raw)
	} else {
		err = tx.QueryRow(ctx, query, s.Scope, b).Scan(&raw)
	}
	if err != nil {
		return nil, err
	}
	var intent EffectIntent
	if err = json.Unmarshal(raw, &intent); err != nil {
		return nil, err
	}
	if intent.Schema != testEffectSchema {
		return nil, fault("INPUT_CONFLICT", "delivery identity belongs to another action", 409)
	}
	return &intent, nil
}

func effectMatches(e *EffectIntent, task, artifact, principal string, c TestEffectConfig) bool {
	return e.TaskID == task && e.ArtifactID == artifact && e.Principal == principal && e.Target == c.Target && e.Origin == strings.TrimRight(c.URL, "/") && e.Action == "isolated-publish-report" && e.Renderer == "structured-json/1"
}

func (s *Store) saveTestEffect(ctx context.Context, tx pgx.Tx, e *EffectIntent) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "UPDATE delivery_intents SET state=$3,receipt=$4 WHERE scope=$1 AND id=$2", s.Scope, e.ID, "EFFECT_"+e.State, raw)
	return err
}

// Pure database checks. No content reads, discovery, HTTP or model in this tx.
func (s *Store) effectAdmission(ctx context.Context, tx pgx.Tx, t *Task, artifact, principal string) (string, error) {
	if err := s.currentDeliveryPolicy(t); err != nil {
		return "", err
	}
	if t.Pause {
		return "", fault("PAUSED", "new effect admission stopped", 409)
	}
	if t.Status != "AWAITING_ACCEPTANCE" && t.Status != "DELIVERED" {
		return "", fault("POLICY_REJECTED", "report computation incomplete", 409)
	}
	var digest string
	var valid bool
	err := tx.QueryRow(ctx, "SELECT r.artifact_digest,r.valid FROM committed_results r JOIN artifacts a ON a.scope=r.scope AND a.id=r.artifact_id WHERE r.scope=$1 AND r.task_id=$2 AND r.artifact_id=$3 AND a.schema_version='report/1'", s.Scope, t.ID, artifact).Scan(&digest, &valid)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fault("NOT_FOUND", "formal report missing", 404)
	}
	if err != nil {
		return "", err
	}
	if !valid {
		return "", fault("INVALIDATED", "report withdrawn", 409)
	}
	passed, err := s.qualifiedReportCheck(ctx, tx, t, digest)
	if err != nil {
		return "", err
	}
	var approved bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM approvals WHERE scope=$1 AND task_id=$2 AND action='deliver' AND object_digest=$3 AND principal=$4 AND expires_at>clock_timestamp())", s.Scope, t.ID, digest, principal).Scan(&approved)
	if err != nil {
		return "", err
	}
	if !passed || !approved {
		return "", fault("APPROVAL_REQUIRED", "matching current check and user approval required", 403)
	}
	return digest, nil
}

func (s *Store) prepareTestEffect(ctx context.Context, task, artifact, b, principal string, c TestEffectConfig) (*EffectIntent, bool, error) {
	if err := c.validate(); err != nil {
		return nil, false, err
	}
	if b == "" || len(b) > 200 {
		return nil, false, fault("INVALID_REQUEST", "stable delivery_id required", 400)
	}
	created := false
	v, err := s.mutate(ctx, task, "TEST_EFFECT_INTENT", func(tx pgx.Tx, t *Task) (any, error) {
		old, err := s.getTestEffect(ctx, tx, b)
		if err == nil {
			if !effectMatches(old, task, artifact, principal, c) {
				return nil, fault("INPUT_CONFLICT", "same b changed target/object/principal", 409)
			}
			return old, nil // Existing intent is only reconciled, never sent again.
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		digest, err := s.effectAdmission(ctx, tx, t, artifact, principal)
		if err != nil {
			return nil, err
		}
		payload, err := Digest(map[string]string{"task_id": task, "object_digest": digest, "target": c.Target, "origin": strings.TrimRight(c.URL, "/"), "action": "isolated-publish-report", "renderer": "structured-json/1", "principal": principal})
		if err != nil {
			return nil, err
		}
		var exists bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM delivery_intents WHERE scope=$1 AND payload_digest=$2)", s.Scope, payload).Scan(&exists); err != nil {
			return nil, err
		}
		if exists {
			return nil, fault("INPUT_CONFLICT", "existing effect must be reconciled under its original b", 409)
		}
		var epoch string
		owner := "go-isolated-effect-publisher"
		err = tx.QueryRow(ctx, `INSERT INTO run_leases(scope,task_id,domain,epoch,owner,control_session,expires_at,status) VALUES($1,$2,'delivery',1,$3,$4,clock_timestamp()+make_interval(secs=>$5),'ACTIVE')
		 ON CONFLICT(scope,task_id,domain) DO UPDATE SET epoch=run_leases.epoch+1,owner=EXCLUDED.owner,control_session=EXCLUDED.control_session,expires_at=EXCLUDED.expires_at,status='ACTIVE'
		 WHERE run_leases.expires_at<=clock_timestamp() OR run_leases.status<>'ACTIVE' OR run_leases.control_session<>EXCLUDED.control_session RETURNING epoch::text`, s.Scope, task, owner, s.Session, s.LeaseSeconds).Scan(&epoch)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fault("WAIT", "delivery permit occupied", 409)
		}
		if err != nil {
			return nil, err
		}
		e := &EffectIntent{Schema: testEffectSchema, ID: b, TaskID: task, ArtifactID: artifact, ObjectDigest: digest, PayloadDigest: payload, Target: c.Target, Origin: strings.TrimRight(c.URL, "/"), Action: "isolated-publish-report", Renderer: "structured-json/1", Principal: principal, Owner: owner, Epoch: epoch, Session: s.Session, State: "RESERVED"}
		raw, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, "INSERT INTO delivery_intents(scope,id,task_id,payload_digest,owner_epoch,state,receipt) VALUES($1,$2,$3,$4,$5::bigint,'EFFECT_RESERVED',$6)", s.Scope, b, task, payload, epoch, raw)
		created = true
		return e, err
	})
	if err != nil {
		return nil, false, err
	}
	return v.(*EffectIntent), created, nil
}

func (s *Store) dispatchTestEffect(ctx context.Context, e *EffectIntent) error {
	_, err := s.mutate(ctx, e.TaskID, "TEST_EFFECT_DISPATCH", func(tx pgx.Tx, t *Task) (any, error) {
		current, err := s.getTestEffect(ctx, tx, e.ID)
		if err != nil {
			return nil, err
		}
		if current.State != "RESERVED" || current.Epoch != e.Epoch || current.Session != e.Session || current.Owner != e.Owner || current.PayloadDigest != e.PayloadDigest || current.TaskID != e.TaskID || current.ArtifactID != e.ArtifactID || current.Principal != e.Principal {
			return nil, fault("WAIT", "effect already dispatched or ownership changed", 409)
		}
		if err = s.leaseCheck(ctx, tx, e.TaskID, "delivery", e.Owner, e.Epoch); err != nil {
			return nil, err
		}
		if _, err = s.effectAdmission(ctx, tx, t, e.ArtifactID, e.Principal); err != nil {
			return nil, err
		}
		current.State = "SENT"
		return current, s.saveTestEffect(ctx, tx, current)
	})
	return err
}

// A trusted client observed a response from the pinned provider. Recording a
// late fact intentionally does not require the expired delivery/run permit.
// It cannot publish a local report, renew a lease or acquire new execution.
func (s *Store) observeTestEffect(ctx context.Context, e *EffectIntent, o EffectObservation) (*EffectIntent, error) {
	v, err := s.mutate(ctx, e.TaskID, "TEST_EFFECT_OBSERVATION", func(tx pgx.Tx, t *Task) (any, error) {
		current, err := s.getTestEffect(ctx, tx, e.ID)
		if err != nil {
			return nil, err
		}
		if current.PayloadDigest != e.PayloadDigest {
			return nil, fault("INPUT_CONFLICT", "effect observation binding differs", 409)
		}
		raw, err := json.Marshal(o)
		if err != nil {
			return nil, err
		}
		digest, err := Digest(o)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, "INSERT INTO observations(scope,source,event_id,call_id,payload_digest,document) VALUES($1,$2,$3,$4,$5,$6)", s.Scope, "test-effect-provider:"+e.Target, newID("effect-observation-"), "effect:"+e.ID, digest, raw)
		if err != nil {
			return nil, err
		}
		terminal := current.State == "APPLIED" || current.State == "DENIED"
		if o.State != "UNKNOWN" {
			if terminal {
				var previous, next EffectReceipt
				if current.State != o.State || json.Unmarshal(current.Receipt, &previous) != nil || json.Unmarshal(o.Payload, &next) != nil || previous.ReceiptID != next.ReceiptID {
					current.State = "CONFLICT"
				}
			} else if current.State != "CONFLICT" {
				current.State = o.State
				if o.State == "APPLIED" || o.State == "DENIED" {
					current.Receipt = o.Payload
				}
			}
		} else if !terminal && current.State != "CONFLICT" {
			current.State = "UNKNOWN"
		}
		if err = s.saveTestEffect(ctx, tx, current); err != nil {
			return nil, err
		}
		if current.State == "APPLIED" || current.State == "DENIED" || current.State == "CONFLICT" {
			_, err = tx.Exec(ctx, "UPDATE run_leases SET status='RELEASED' WHERE scope=$1 AND task_id=$2 AND domain='delivery' AND epoch=$3::bigint AND owner=$4 AND control_session=$5", s.Scope, e.TaskID, e.Epoch, e.Owner, e.Session)
		}
		return current, err
	})
	if err != nil {
		return nil, err
	}
	return v.(*EffectIntent), nil
}

func (s *Server) publishTestEffect(ctx context.Context, task, artifact, b, principal string) (any, error) {
	c := s.Config.TestEffect
	if c == nil {
		return nil, fault("NOT_FOUND", "isolated effect disabled", 404)
	}
	// New admission requires intact content. Reconciliation of an existing b
	// records remote facts even if the local file has subsequently gone missing.
	// prepareTestEffect rechecks identity under the task lock before any HTTP.
	if _, err := s.Store.getTestEffect(ctx, nil, b); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if _, err = s.Files.Read(artifact); err != nil {
			return nil, err
		}
	}
	e, created, err := s.Store.prepareTestEffect(ctx, task, artifact, b, principal, *c)
	if err != nil {
		return nil, err
	}
	method := "GET"
	var body io.Reader
	if created {
		if err = s.Store.dispatchTestEffect(ctx, e); err != nil {
			return nil, err
		}
		method = "POST"
		raw, err := json.Marshal(map[string]string{"delivery_id": e.ID, "payload_digest": e.PayloadDigest, "object_digest": e.ObjectDigest, "target": e.Target, "action": e.Action, "renderer": e.Renderer, "task_id": e.TaskID, "principal": e.Principal})
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, e.Origin+"/effects/"+url.PathEscape(e.ID), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Duration(c.TimeoutMS) * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, requestErr := client.Do(req)
	o := EffectObservation{State: "UNKNOWN"}
	if requestErr == nil {
		defer response.Body.Close()
		o.HTTPStatus = response.StatusCode
		raw, readErr := io.ReadAll(io.LimitReader(response.Body, 64*1024+1))
		var receipt EffectReceipt
		if readErr == nil && len(raw) <= 64*1024 && DecodeStrict(raw, &receipt) == nil && receipt.ID == e.ID && receipt.PayloadDigest == e.PayloadDigest && response.StatusCode == 200 && (receipt.Status == "APPLIED" || receipt.Status == "DENIED") && receipt.ReceiptID != "" {
			o.State = receipt.Status
			o.Payload = raw
		}
	}
	// The caller may have disconnected after the actual effect. Persist the
	// trusted fact under a bounded independent context, rather than discarding it.
	observeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return s.Store.observeTestEffect(observeCtx, e, o)
}

func (s *Server) testEffectRoutes(mux *http.ServeMux) {
	if s.Config.TestEffect == nil {
		return
	}
	mux.HandleFunc("POST /test/effects/tasks/{id}/artifacts/{artifact}", s.wrap([]string{"user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		principal, err := s.userWrite(r)
		if err != nil {
			return nil, err
		}
		var in struct {
			ID string `json:"delivery_id"`
		}
		if err = readBody(r, &in); err != nil {
			return nil, err
		}
		return s.publishTestEffect(r.Context(), r.PathValue("id"), r.PathValue("artifact"), in.ID, principal)
	}))
	mux.HandleFunc("GET /test/effects/{effect}", s.wrap([]string{"user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		e, err := s.Store.getTestEffect(r.Context(), nil, r.PathValue("effect"))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fault("NOT_FOUND", "effect missing", 404)
		}
		if err != nil {
			return nil, err
		}
		principal, _ := s.authorize(r, "user")
		if principal != e.Principal {
			return nil, fault("FORBIDDEN", "effect read principal differs", 403)
		}
		observations, err := s.Store.Observations(r.Context(), "effect:"+e.ID)
		return map[string]any{"intent": e, "observations": observations, "scope": "isolated provider; not bank or local report delivery"}, err
	}))
}
