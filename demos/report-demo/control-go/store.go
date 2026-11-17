package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	Pool                     *pgxpool.Pool
	Scope, Session           string
	LeaseSeconds             int
	MaterialProjectionDigest string
	ExecutionProfile         string
	ModelBindingDigest       string
	BusBinding               *BusBinding
	CurrentCatalogDigest     string
	DeliveryPolicyVersion    string
	ReuseFiles               *Files
}

func OpenStore(ctx context.Context, dsn, scope string, leaseSeconds int) (*Store, error) {
	c, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	c.MaxConns = 4
	c.MinConns = 0
	c.ConnConfig.ConnectTimeout = 5 * time.Second
	c.ConnConfig.RuntimeParams["statement_timeout"] = "5000"
	c.ConnConfig.RuntimeParams["lock_timeout"] = "2000"
	c.ConnConfig.RuntimeParams["application_name"] = "agent-hub-report-demo-control"
	p, err := pgxpool.NewWithConfig(ctx, c)
	if err != nil {
		return nil, err
	}
	if err = p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	return &Store{Pool: p, Scope: scope, Session: newID("control-"), LeaseSeconds: leaseSeconds}, nil
}
func (s *Store) Migrate(ctx context.Context) error {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(7194401)"); err != nil {
		return err
	}
	for _, entry := range entries {
		b, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(b)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func emit(ctx context.Context, tx pgx.Tx, scope, task, kind string, doc any) error {
	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO audit_outbox(scope,aggregate,event_id,kind,document) VALUES($1,$2,$3,$4,$5)", scope, task, newID("event-"), kind, b)
	return err
}
func (s *Store) GetTask(ctx context.Context, id string) (*Task, error) {
	var b, deliveries, approvals []byte
	if err := s.Pool.QueryRow(ctx, `SELECT document,
	 COALESCE((SELECT jsonb_agg(d.receipt ORDER BY d.id) FROM delivery_intents d WHERE d.scope=$1 AND d.task_id=$2 AND d.state='DELIVERED'),'[]'::jsonb),
	 COALESCE((SELECT jsonb_agg(jsonb_build_object('approval_id',a.id,'artifact_digest',a.object_digest,'principal',a.principal,'expires_at',a.expires_at) ORDER BY a.id) FROM approvals a WHERE a.scope=$1 AND a.task_id=$2 AND a.action='deliver'),'[]'::jsonb)
	 FROM tasks WHERE scope=$1 AND id=$2`, s.Scope, id).Scan(&b, &deliveries, &approvals); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fault("NOT_FOUND", "task not found", 404)
		}
		return nil, err
	}
	var t Task
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(deliveries, &t.DeliveryReceipts); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(approvals, &t.ReportApprovals); err != nil {
		return nil, err
	}
	return &t, nil
}

// Demo authority decisions also share a short scope gate so a late withdrawal
// cannot race a cross-task reuse/delivery. No network/model/file I/O in fn.
func (s *Store) lockAuthority(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", s.Scope+"/authority")
	return err
}

func (s *Store) mutate(ctx context.Context, id, kind string, fn func(pgx.Tx, *Task) (any, error)) (any, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = s.lockAuthority(ctx, tx); err != nil {
		return nil, err
	}
	var b []byte
	err = tx.QueryRow(ctx, "SELECT document FROM tasks WHERE scope=$1 AND id=$2 FOR UPDATE", s.Scope, id).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fault("NOT_FOUND", "task not found", 404)
	}
	if err != nil {
		return nil, err
	}
	var t Task
	if err = json.Unmarshal(b, &t); err != nil {
		return nil, err
	}
	out, err := fn(tx, &t)
	if err != nil {
		return nil, err
	}
	g, err := strconv.ParseUint(t.Generation, 10, 64)
	if err != nil {
		return nil, err
	}
	t.Generation = strconv.FormatUint(g+1, 10)
	b, err = json.Marshal(t)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "UPDATE tasks SET document=$3,updated_at=clock_timestamp() WHERE scope=$1 AND id=$2", s.Scope, id, b); err != nil {
		return nil, err
	}
	if err = emit(ctx, tx, s.Scope, id, kind, out); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) Propose(ctx context.Context, principal, key string, g Graph, runtimeDigest string) (*Task, error) {
	if key == "" || len(key) > 200 {
		return nil, fault("INVALID_COMMAND", "command_id required (max 200)", 400)
	}
	h, err := Digest(g)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = s.lockAuthority(ctx, tx); err != nil {
		return nil, err
	}
	// Advisory lock is scoped to the command key; concurrent same-key requests reuse one receipt.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", s.Scope+"/"+principal+"/propose/"+key); err != nil {
		return nil, err
	}
	var oldDigest string
	var receipt []byte
	err = tx.QueryRow(ctx, "SELECT payload_digest,receipt FROM commands WHERE scope=$1 AND principal=$2 AND kind='propose' AND key=$3", s.Scope, principal, key).Scan(&oldDigest, &receipt)
	if err == nil {
		if oldDigest != h {
			return nil, fault("INPUT_CONFLICT", "same command with changed content", 409)
		}
		var old Task
		err = json.Unmarshal(receipt, &old)
		return &old, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if g.Selection != nil {
		if err = s.validateSelection(ctx, tx, g); err != nil {
			return nil, err
		}
	}
	t := Task{IdentityContract: pr25Contract, SchemaVersion: Protocol, ID: newID("T-"), Revision: "g1", Status: "AWAITING_APPROVAL", Generation: "1", Graph: g, GraphDigest: h, RuntimeDigest: runtimeDigest, Nodes: map[string]*NodeState{}, CreatedAt: time.Now().UTC()}
	t.ExecutionProfile = s.ExecutionProfile
	t.ModelBindingDigest = s.ModelBindingDigest
	if s.BusBinding != nil {
		frozen := *s.BusBinding
		t.BusBinding = &frozen
	}
	for _, n := range g.Nodes {
		t.Nodes[n.ID] = &NodeState{Status: "READY"}
	}
	b, err := json.Marshal(t)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO tasks(scope,id,document) VALUES($1,$2,$3)", s.Scope, t.ID, b); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO commands(scope,principal,kind,key,payload_digest,receipt) VALUES($1,$2,'propose',$3,$4,$5)", s.Scope, principal, key, h, b); err != nil {
		return nil, err
	}
	if err = emit(ctx, tx, s.Scope, t.ID, "PROPOSED", map[string]string{"task_id": t.ID, "graph_digest": h}); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &t, nil
}

type Lease struct {
	TaskID    string    `json:"task_id"`
	Domain    string    `json:"domain"`
	Epoch     string    `json:"epoch"`
	Owner     string    `json:"owner"`
	ExpiresAt time.Time `json:"expires_at"`
	Task      *Task     `json:"task,omitempty"`
}

func (s *Store) leaseCheck(ctx context.Context, tx pgx.Tx, id, domain, owner, epoch string) error {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM run_leases WHERE scope=$1 AND task_id=$2 AND domain=$3 AND owner=$4 AND epoch::text=$5 AND control_session=$6 AND status='ACTIVE' AND expires_at>clock_timestamp())`, s.Scope, id, domain, owner, epoch, s.Session).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return fault("LEASE_EXPIRED", "current unexpired execution permit required", 403)
	}
	return nil
}
func (s *Store) ClaimRun(ctx context.Context, id, owner, domain string) (any, error) {
	if owner == "" || (domain != "graph" && domain != "delivery") {
		return nil, fault("INVALID_REQUEST", "owner/domain required", 400)
	}
	return s.mutate(ctx, id, "LEASE_CLAIMED", func(tx pgx.Tx, t *Task) (any, error) {
		if t.Pause {
			return nil, fault("PAUSED", "task paused", 409)
		}
		if domain == "graph" && t.Status != "QUEUED" && t.Status != "RUNNING" && t.Status != "WAITING_REMOTE" && t.Status != "WAITING_INPUT" && t.Status != "WAITING_AUTH" {
			return nil, fault("POLICY_REJECTED", "task not executable", 409)
		}
		if domain == "graph" {
			if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", s.Scope+"/single-active-graph"); err != nil {
				return nil, err
			}
			var other bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM run_leases WHERE scope=$1 AND task_id<>$2 AND domain='graph' AND status='ACTIVE' AND control_session=$3 AND expires_at>clock_timestamp())", s.Scope, id, s.Session).Scan(&other); err != nil {
				return nil, err
			}
			if other {
				return nil, fault("WAIT", "demo permits one active graph at a time", 409)
			}
		}
		if domain == "delivery" && t.Status != "AWAITING_ACCEPTANCE" && t.Status != "DELIVERED" {
			return nil, fault("POLICY_REJECTED", "report not ready", 409)
		}
		var l Lease
		l.TaskID = id
		l.Domain = domain
		l.Owner = owner
		err := tx.QueryRow(ctx, `INSERT INTO run_leases(scope,task_id,domain,epoch,owner,control_session,expires_at,status)
		 VALUES($1,$2,$3,1,$4,$5,clock_timestamp()+make_interval(secs=>$6),'ACTIVE')
		 ON CONFLICT(scope,task_id,domain) DO UPDATE SET epoch=run_leases.epoch+1,owner=EXCLUDED.owner,control_session=EXCLUDED.control_session,expires_at=EXCLUDED.expires_at,status='ACTIVE'
		 WHERE run_leases.expires_at<=clock_timestamp() OR run_leases.status<>'ACTIVE' OR run_leases.control_session<>EXCLUDED.control_session
		 RETURNING epoch::text,expires_at`, s.Scope, id, domain, owner, s.Session, s.LeaseSeconds).Scan(&l.Epoch, &l.ExpiresAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fault("WAIT", "active owner holds permit", 409)
		}
		if err != nil {
			return nil, err
		}
		if domain == "graph" {
			t.Status = "RUNNING"
		}
		l.Task = t
		return l, nil
	})
}
func (s *Store) Heartbeat(ctx context.Context, id, owner, epoch, domain string) (any, error) {
	return s.mutate(ctx, id, "HEARTBEAT", func(tx pgx.Tx, t *Task) (any, error) {
		if err := s.leaseCheck(ctx, tx, id, domain, owner, epoch); err != nil {
			return nil, err
		}
		var expiry time.Time
		err := tx.QueryRow(ctx, "UPDATE run_leases SET expires_at=clock_timestamp()+make_interval(secs=>$6) WHERE scope=$1 AND task_id=$2 AND domain=$3 AND owner=$4 AND epoch::text=$5 RETURNING expires_at", s.Scope, id, domain, owner, epoch, s.LeaseSeconds).Scan(&expiry)
		return map[string]any{"epoch": epoch, "expires_at": expiry, "pause_requested": t.Pause}, err
	})
}

func (s *Store) Admit(ctx context.Context, id, digest, principal string) (any, error) {
	return s.mutate(ctx, id, "ADMITTED", func(tx pgx.Tx, t *Task) (any, error) {
		if t.GraphDigest != digest {
			return nil, fault("INPUT_CONFLICT", "graph digest changed", 409)
		}
		if t.Status != "AWAITING_APPROVAL" {
			return map[string]string{"task_id": id, "graph_digest": digest, "status": t.Status}, nil
		}
		if err := s.checkBusBinding(t); err != nil {
			return nil, err
		}
		if t.ExecutionProfile != s.ExecutionProfile || t.ModelBindingDigest != s.ModelBindingDigest || (s.CurrentCatalogDigest != "" && t.Graph.CatalogDigest != s.CurrentCatalogDigest) {
			return nil, fault("BINDING_CHANGED", "release changed after proposal; review a new candidate", 409)
		}
		if t.Graph.Selection != nil && (t.ExecutionProfile == "" || t.ModelBindingDigest == "") {
			return nil, fault("BINDING_CHANGED", "selected task has no reviewed actual-model binding", 409)
		}
		t.Status = "QUEUED"
		if t.Graph.Selection != nil {
			if err := s.validateSelection(ctx, tx, t.Graph); err != nil {
				return nil, err
			}
		}
		t.ApprovedBy = principal
		_, err := tx.Exec(ctx, "INSERT INTO approvals(scope,id,task_id,action,object_digest,principal,expires_at) VALUES($1,$2,$3,'admit',$4,$5,clock_timestamp()+interval '24 hours')", s.Scope, newID("approve-"), id, digest, principal)
		return map[string]string{"task_id": id, "graph_digest": digest, "status": t.Status}, err
	})
}

func (s *Store) Binding(ctx context.Context, t *Task, node string, c Catalog, materialDigest string) (Binding, error) {
	if err := s.checkBusBinding(t); err != nil {
		return Binding{}, err
	}
	if t.ExecutionProfile != s.ExecutionProfile || t.ModelBindingDigest != s.ModelBindingDigest {
		return Binding{}, fault("BINDING_CHANGED", "execution source/model profile changed; new graph confirmation required", 409)
	}
	var n *Node
	for i := range t.Graph.Nodes {
		if t.Graph.Nodes[i].ID == node {
			n = &t.Graph.Nodes[i]
			break
		}
	}
	if n == nil {
		return Binding{}, fault("NOT_FOUND", "node not in admitted graph", 404)
	}
	cap, ok := c.Get(n.Expert)
	if !ok || !cap.Enabled {
		return Binding{}, fault("POLICY_REJECTED", "capability unavailable", 422)
	}
	cd, err := Digest(c)
	if err != nil {
		return Binding{}, err
	}
	if cd != t.Graph.CatalogDigest {
		return Binding{}, fault("BINDING_CHANGED", "catalog differs from admitted version", 409)
	}
	b := Binding{SchemaVersion: Protocol, TaskID: t.ID, Revision: t.Revision, NodeID: node, Instruction: n.Task, Goal: t.Graph.Goal, Focus: t.Graph.Focus, MaterialDigest: materialDigest, MaterialProjectionDigest: s.MaterialProjectionDigest, Capability: cap, PolicyVersion: t.Graph.PolicyVersion, Upstream: []Result{}, AuthorizedSkips: []SkipProof{}}
	b.ExecutionProfile = s.ExecutionProfile
	b.ModelBindingDigest = t.ModelBindingDigest
	b.BusBinding = t.BusBinding
	if t.IdentityContract != "" {
		if t.IdentityContract != pr25Contract {
			return Binding{}, fault("BINDING_CHANGED", "identity contract unavailable", 409)
		}
		b.IdentityContract, b.LogicalExecution = t.IdentityContract, logicalExecution(t, node)
	}
	for _, dep := range n.DependsOn {
		ns := t.Nodes[dep]
		if ns == nil || ns.Result == nil {
			if cap.Kind == "select" {
				if proof, ok := skipProof(t, dep); ok {
					b.AuthorizedSkips = append(b.AuthorizedSkips, proof)
					continue
				}
			}
			return Binding{}, fault("WAIT", "required predecessor not committed", 409)
		}
		if !ns.Result.Valid {
			return Binding{}, fault("INVALIDATED", "upstream result withdrawn", 409)
		}
		b.Upstream = append(b.Upstream, *ns.Result)
		if ns.Review != nil {
			b.Reviews = append(b.Reviews, *ns.Review)
		}
	}
	return b, nil
}

func (s *Store) ResolveNode(ctx context.Context, id, node, owner, epoch, expectedDigest string, c Catalog, materialDigest string) (any, error) {
	verified, prepareErr := s.prepareReuse(ctx, id, node, c, materialDigest)
	if prepareErr != nil {
		return nil, prepareErr
	}
	return s.mutate(ctx, id, "NODE_RESOLVED", func(tx pgx.Tx, t *Task) (any, error) {
		b, err := s.Binding(ctx, t, node, c, materialDigest)
		if err != nil {
			return nil, err
		}
		h, err := Digest(b)
		if err != nil {
			return nil, err
		}
		if expectedDigest != "" && expectedDigest != h {
			return nil, fault("INPUT_CONFLICT", "input digest differs from authoritative binding", 409)
		}
		ns := t.Nodes[node]
		if ns.InputDigest != "" && ns.InputDigest != h {
			return nil, fault("INPUT_CONFLICT", "same node has different binding", 409)
		}
		if committed, serve, _ := QueryForReconciliation(resultRecord(ns.Result), h); committed {
			if !serve {
				return nil, fault("INPUT_CONFLICT", "formal result input differs", 409)
			}
			if !ns.Result.Valid {
				return nil, fault("INVALIDATED", "result withdrawn", 409)
			}
			return map[string]any{"status": "REUSE", "binding": b, "input_digest": h, "result": ns.Result}, nil
		}
		if t.Pause {
			return nil, fault("PAUSED", "new admission stopped", 409)
		}
		if err = s.leaseCheck(ctx, tx, id, "graph", owner, epoch); err != nil {
			return nil, err
		}
		if b.Capability.Kind == "extract" && ns.Attempt == 0 && len(verified) > 0 {
			if result, err := s.reuseExtraction(ctx, tx, t, node, h, epoch, b, verified); err != nil {
				return nil, err
			} else if result != nil {
				return map[string]any{"status": "REUSE", "binding": b, "input_digest": h, "result": result}, nil
			}
		}
		// A persisted send intent dominates a projected node status, including
		// INPUT_REQUIRED/AUTH_REQUIRED. Resuming must reconcile that call, never
		// obtain a fresh attempt that could send the same work a second time.
		var hasCall bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM remote_calls WHERE scope=$1 AND task_id=$2 AND node_id=$3)", s.Scope, id, node).Scan(&hasCall); err != nil {
			return nil, err
		}
		if hasCall || ns.Status == "WAITING_REMOTE" || ns.Status == "UNKNOWN" {
			// Transfer only the right to reconcile this existing call. The attempt
			// and remote identity remain fixed; WAIT never grants a new send.
			ns.OwnerEpoch = epoch
			return map[string]any{"status": "WAIT", "binding": b, "input_digest": h, "attempt": ns.Attempt, "reason": ns.Reason}, nil
		}
		if ns.Status == "CLAIMED" && ns.OwnerEpoch == epoch {
			return map[string]any{"status": "WAIT", "input_digest": h, "reason": "current attempt in flight"}, nil
		}
		ns.InputDigest = h
		if ns.Attempt >= 3 {
			return nil, fault("ATTEMPT_BUDGET", "pure computation attempt budget exhausted", 409)
		}
		ns.Attempt++
		ns.OwnerEpoch = epoch
		ns.Status = "CLAIMED"
		_, err = tx.Exec(ctx, "INSERT INTO node_attempts(scope,task_id,node_id,attempt,epoch,input_digest,status) VALUES($1,$2,$3,$4,$5::bigint,$6,'CLAIMED')", s.Scope, id, node, ns.Attempt, epoch, h)
		return map[string]any{"status": "GRANTED", "binding": b, "input_digest": h, "attempt": ns.Attempt, "epoch": epoch}, err
	})
}

func (s *Store) Complete(ctx context.Context, id, owner, epoch string) (any, error) {
	return s.mutate(ctx, id, "COMPLETION_CHECKED", func(tx pgx.Tx, t *Task) (any, error) {
		if err := s.leaseCheck(ctx, tx, id, "graph", owner, epoch); err != nil {
			return nil, err
		}
		if t.Status == "NEEDS_REVIEW" || t.Status == "BLOCKED" {
			return nil, fault("POLICY_REJECTED", "rejected, blocked or invalidated task cannot complete", 409)
		}
		missing := []string{}
		failed := false
		for name, n := range t.Nodes {
			if n.Status == "FAILED" {
				failed = true
			}
			if n.Result != nil && effectiveCheck(n) == "HUMAN" {
				missing = append(missing, "human-review:"+name)
			}
			_, skipValid := skipProof(t, name)
			if (n.Result == nil || !n.Result.Valid) && !skipValid {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			if failed {
				t.Status = "FAILED"
			}
			if t.Status == "WAITING_INPUT" || t.Status == "WAITING_AUTH" || t.Status == "FAILED" {
				if _, err := tx.Exec(ctx, "UPDATE run_leases SET status='RELEASED' WHERE scope=$1 AND task_id=$2 AND domain='graph'", s.Scope, id); err != nil {
					return nil, err
				}
			}
			status := "WAIT"
			if failed {
				status = "FAILED"
			}
			return map[string]any{"status": status, "missing": missing}, nil
		}
		t.Status = "AWAITING_ACCEPTANCE"
		_, err := tx.Exec(ctx, "UPDATE run_leases SET status='RELEASED' WHERE scope=$1 AND task_id=$2 AND domain='graph'", s.Scope, id)
		return map[string]string{"task_id": id, "status": t.Status}, err
	})
}

func (s *Store) SetPause(ctx context.Context, id string, pause bool) (any, error) {
	return s.mutate(ctx, id, "PAUSE_CHANGED", func(tx pgx.Tx, t *Task) (any, error) {
		if !pause && t.Status == "BLOCKED" {
			return nil, fault("BINDING_CHANGED", "blocked release/identity requires controlled resolution or a newly reviewed task; ordinary resume is not allowed", 409)
		}
		t.Pause = pause
		if !pause && t.Status == "FAILED" {
			// An explicit trusted-user retry may reconcile a failed call or
			// retry unsubmitted pure computation. Existing call/h/attempt rules
			// still apply; this is never permission to resend an accepted call.
			t.Status = "QUEUED"
		}
		if !pause && t.Status == "WAITING_AUTH" {
			// The user's explicit retry requests reconciliation after repairing
			// authorization. It is not proof that authorization has been granted.
			t.Status = "QUEUED"
		}
		if !pause && t.Status == "WAITING_INPUT" {
			for _, ns := range t.Nodes {
				if ns.Result == nil && ns.Status == "WAITING_INPUT" {
					// Supplemental data belongs to the existing remote task. Poll
					// it after the trusted user resumes; do not mutate the graph/h.
					t.Status = "QUEUED"
					break
				}
				if ns.Review != nil && (ns.Review.Decision == "ACCEPT" || ns.Review.Decision == "REVISE") {
					t.Status = "QUEUED"
					break
				}
			}
		}
		return map[string]any{"task_id": id, "pause_requested": pause, "status": t.Status}, nil
	})
}
func (s *Store) Events(ctx context.Context, id string, after int64) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, "SELECT sequence::text,event_id,kind,document,created_at FROM audit_outbox WHERE scope=$1 AND aggregate=$2 AND sequence>$3 ORDER BY audit_outbox.sequence LIMIT 200", s.Scope, id, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var seq, event, kind string
		var doc json.RawMessage
		var at time.Time
		if err = rows.Scan(&seq, &event, &kind, &doc, &at); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"sequence": seq, "event_id": event, "kind": kind, "document": doc, "at": at})
	}
	return out, rows.Err()
}

func (s *Store) Pending(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx, "SELECT id FROM tasks WHERE scope=$1 AND document->>'status' IN ('QUEUED','RUNNING','WAITING_REMOTE') AND NOT (document->>'pause_requested')::boolean ORDER BY updated_at LIMIT 20", s.Scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func storageError(err error) error {
	var f *Fault
	if errors.As(err, &f) {
		return f
	}
	return fault("STORAGE_UNAVAILABLE", fmt.Sprintf("authoritative operation unavailable; reconcile original identity (%T)", err), 503)
}
