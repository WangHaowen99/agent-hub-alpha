package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/jackc/pgx/v5"
)

type Files struct{ Root string }

var blobID = regexp.MustCompile(`^[0-9a-f]{64}$`)

func (f Files) Read(id string) ([]byte, error) {
	if !blobID.MatchString(id) {
		return nil, fault("INVALID_ARTIFACT", "invalid content address", 400)
	}
	b, err := os.ReadFile(filepath.Join(f.Root, id+".json"))
	if err != nil {
		return nil, fault("EVIDENCE_MISSING", "artifact unavailable", 409)
	}
	h := sha256.Sum256(b)
	if hex.EncodeToString(h[:]) != id {
		return nil, fault("EVIDENCE_CHANGED", "artifact digest mismatch", 409)
	}
	return b, nil
}
func (f Files) Put(raw []byte) (string, error) {
	var v any
	if err := DecodeStrict(raw, &v); err != nil {
		return "", fault("INVALID_ARTIFACT", "artifact must be strict JSON", 400)
	}
	if err := os.MkdirAll(f.Root, 0700); err != nil {
		return "", err
	}
	h := sha256.Sum256(raw)
	id := hex.EncodeToString(h[:])
	dest := filepath.Join(f.Root, id+".json")
	if old, err := os.ReadFile(dest); err == nil {
		if !bytes.Equal(old, raw) {
			return "", errors.New("content address collision")
		}
		return id, nil
	}
	fp, err := os.CreateTemp(f.Root, ".candidate-")
	if err != nil {
		return "", err
	}
	name := fp.Name()
	defer os.Remove(name)
	if _, err = fp.Write(raw); err != nil {
		fp.Close()
		return "", err
	}
	if err = fp.Sync(); err != nil {
		fp.Close()
		return "", err
	}
	if err = fp.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(name, dest); err != nil {
		return "", err
	}
	// Windows does not implement directory fsync. File sync and atomic rename are
	// still used; platform crash-durability is recorded separately in verification.
	if runtime.GOOS != "windows" {
		dir, err := os.Open(f.Root)
		if err != nil {
			return "", err
		}
		err = dir.Sync()
		dir.Close()
		if err != nil {
			return "", err
		}
	}
	return id, nil
}

type CommitRequest struct {
	Owner       string `json:"owner"`
	Epoch       string `json:"epoch"`
	InputDigest string `json:"input_digest"`
	Attempt     int    `json:"attempt"`
	ArtifactID  string `json:"artifact_id"`
}

func (s *Store) Commit(ctx context.Context, id, node string, r CommitRequest, raw []byte, c Catalog, materialDigest string) (any, error) {
	// Files are prepared and verified before entering the authoritative transaction.
	h := sha256.Sum256(raw)
	if hex.EncodeToString(h[:]) != r.ArtifactID {
		return nil, fault("INPUT_CONFLICT", "content address mismatch", 409)
	}
	var output any
	if err := DecodeStrict(raw, &output); err != nil {
		return nil, fault("INVALID_ARTIFACT", "malformed output", 422)
	}
	outputDigest, err := Digest(output)
	if err != nil {
		return nil, err
	}
	return s.mutate(ctx, id, "RESULT_COMMITTED", func(tx pgx.Tx, t *Task) (any, error) {
		b, err := s.Binding(ctx, t, node, c, materialDigest)
		if err != nil {
			return nil, err
		}
		expected, err := Digest(b)
		if err != nil {
			return nil, err
		}
		if expected != r.InputDigest {
			return nil, fault("INPUT_CONFLICT", "input binding mismatch", 409)
		}
		ns := t.Nodes[node]
		if ns.Result != nil {
			if CanReuseCommit(resultRecord(ns.Result), expected, r.ArtifactID) {
				return ns.Result, nil
			}
			return nil, fault("INPUT_CONFLICT", "formal result cannot be overwritten", 409)
		}
		if err = s.leaseCheck(ctx, tx, id, "graph", r.Owner, r.Epoch); err != nil {
			return nil, err
		}
		if ns.OwnerEpoch != r.Epoch || ns.Attempt != r.Attempt || ns.InputDigest != expected {
			return nil, fault("FENCED", "attempt does not own node", 403)
		}
		if err = s.qualifyRemoteResult(ctx, tx, id, node, r.Attempt, expected, outputDigest); err != nil {
			return nil, err
		}
		digest := "sha256:" + r.ArtifactID
		_, err = tx.Exec(ctx, "INSERT INTO artifacts(scope,id,digest,size,schema_version) VALUES($1,$2,$3,$4,$5) ON CONFLICT(scope,id) DO NOTHING", s.Scope, r.ArtifactID, digest, len(raw), b.Capability.OutputSchema)
		if err != nil {
			return nil, err
		}
		result := &Result{NodeID: node, InputDigest: expected, ArtifactID: r.ArtifactID, ArtifactDigest: digest, Receipt: newID("receipt-"), Valid: true, Output: json.RawMessage(raw)}
		result.Identity, err = executionIdentity(t, node, r.Attempt, r.Epoch)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, "INSERT INTO committed_results(scope,task_id,node_id,input_digest,artifact_id,artifact_digest,receipt,output,identity) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)", s.Scope, id, node, expected, r.ArtifactID, digest, result.Receipt, raw, result.Identity)
		if err != nil {
			return nil, err
		}
		if b.Capability.Kind == "extract" {
			key, err := extractionKey(b)
			if err != nil {
				return nil, err
			}
			if _, err = tx.Exec(ctx, "INSERT INTO extraction_reuse(scope,reuse_key,task_id,node_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING", s.Scope, key, id, node); err != nil {
				return nil, err
			}
		}
		for _, up := range b.Upstream {
			if _, err = tx.Exec(ctx, "INSERT INTO artifact_dependencies(scope,task_id,node_id,upstream_artifact) VALUES($1,$2,$3,$4)", s.Scope, id, node, up.ArtifactID); err != nil {
				return nil, err
			}
		}
		_, err = tx.Exec(ctx, "UPDATE node_attempts SET status='COMMITTED' WHERE scope=$1 AND task_id=$2 AND node_id=$3 AND attempt=$4", s.Scope, id, node, r.Attempt)
		if err != nil {
			return nil, err
		}
		ns.Status = "COMMITTED"
		ns.Result = result
		ns.Reason = ""
		if b.Capability.Kind == "check" {
			m, ok := output.(map[string]any)
			if !ok {
				return nil, fault("INVALID_CHECK", "checker output must be an object", 422)
			}
			outcome, _ := m["outcome"].(string)
			object, _ := m["object_digest"].(string)
			version, _ := m["checker_version"].(string)
			if (outcome != "PASS" && outcome != "REVISE" && outcome != "HUMAN" && outcome != "FAIL" && outcome != "UNKNOWN") || version == "" {
				return nil, fault("INVALID_CHECK", "typed outcome/version required", 422)
			}
			bound := false
			for _, up := range b.Upstream {
				if up.ArtifactDigest == object {
					bound = true
				}
			}
			if !bound {
				return nil, fault("INVALID_CHECK", "check is not bound to consumed report", 422)
			}
			_, err = tx.Exec(ctx, "INSERT INTO checks(scope,id,task_id,object_digest,checker_version,outcome,document) VALUES($1,$2,$3,$4,$5,$6,$7)", s.Scope, r.ArtifactID, id, object, version, outcome, raw)
			if err != nil {
				return nil, err
			}
			if outcome == "HUMAN" {
				t.Status = "WAITING_INPUT"
			}
		}
		return result, nil
	})
}

func (s *Store) MarkWaiting(ctx context.Context, id, node, owner, epoch, status, reason string) (any, error) {
	allowed := map[string]bool{"WAITING_REMOTE": true, "WAITING_INPUT": true, "WAITING_AUTH": true, "UNKNOWN": true, "FAILED": true}
	if !allowed[status] {
		return nil, fault("INVALID_STATE", "typed waiting/failure state required", 400)
	}
	return s.mutate(ctx, id, "NODE_WAITING", func(tx pgx.Tx, t *Task) (any, error) {
		if err := s.leaseCheck(ctx, tx, id, "graph", owner, epoch); err != nil {
			return nil, err
		}
		ns, ok := t.Nodes[node]
		if !ok {
			return nil, fault("NOT_FOUND", "node missing", 404)
		}
		if ns.Result != nil {
			return nil, fault("INPUT_CONFLICT", "cannot replace formal result state", 409)
		}
		ns.Status = status
		ns.Reason = reason
		if t.Status == "NEEDS_REVIEW" {
			// A later technical status must not erase a trusted rejection or
			// artifact invalidation. The actual node observation is still kept.
		} else if status == "FAILED" {
			t.Status = "FAILED"
		} else if t.Status == "FAILED" {
			// Another parallel member's waiting update cannot regress failure.
		} else if status == "WAITING_INPUT" || status == "WAITING_AUTH" {
			t.Status = status
		} else if status == "UNKNOWN" || status == "WAITING_REMOTE" {
			t.Status = "WAITING_REMOTE"
		}
		return map[string]string{"node_id": node, "status": status, "reason": reason}, nil
	})
}

func (s *Store) AuthorizeSkip(ctx context.Context, id, node, owner, epoch string) (any, error) {
	return s.mutate(ctx, id, "SKIP_AUTHORIZED", func(tx pgx.Tx, t *Task) (any, error) {
		if err := s.leaseCheck(ctx, tx, id, "graph", owner, epoch); err != nil {
			return nil, err
		}
		if t.Pause {
			return nil, fault("PAUSED", "new admission stopped", 409)
		}
		var n *Node
		for i := range t.Graph.Nodes {
			if t.Graph.Nodes[i].ID == node {
				n = &t.Graph.Nodes[i]
				break
			}
		}
		if n == nil || n.Condition == nil {
			return nil, fault("POLICY_REJECTED", "no admitted skip condition", 422)
		}
		ns := t.Nodes[node]
		if ns.Result != nil {
			return nil, fault("INPUT_CONFLICT", "node already committed", 409)
		}
		check := t.Nodes[n.Condition.Step]
		if check == nil || check.Result == nil || !check.Result.Valid {
			return nil, fault("WAIT", "condition check not committed", 409)
		}
		var m struct {
			Outcome string `json:"outcome"`
		}
		if err := json.Unmarshal(check.Result.Output, &m); err != nil {
			return nil, err
		}
		m.Outcome = effectiveCheck(check)
		if m.Outcome == n.Condition.Outcome {
			return nil, fault("POLICY_REJECTED", "condition holds; node must execute", 422)
		}
		if m.Outcome != "PASS" && m.Outcome != "REVISE" {
			return nil, fault("WAIT", "check requires human resolution", 409)
		}
		ns.Status = "AUTHORIZED_SKIP"
		ns.SkipCheck = check.Result.ArtifactID
		return map[string]string{"node_id": node, "status": ns.Status, "check_ref": ns.SkipCheck}, nil
	})
}

func (s *Store) Invalidate(ctx context.Context, artifact, reason, principal string) (any, error) {
	if reason == "" {
		return nil, fault("INVALID_REQUEST", "reason required", 400)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err = s.lockAuthority(ctx, tx); err != nil {
		return nil, err
	}
	out, err := s.invalidateResults(ctx, tx, nil, artifact, reason, principal)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// Called under the same scope gate as commit/reuse/delivery. When current is
// supplied, mutate owns its final projection write/generation; other affected
// tasks are updated here. Evidence, validity, descendants and audits commit
// together, without acquiring a second transaction or rewriting original R.
func (s *Store) invalidateResults(ctx context.Context, tx pgx.Tx, current *Task, artifact, reason, principal string) (any, error) {
	// Lock task rows in stable order before changing any result projection.
	rows, err := tx.Query(ctx, "SELECT id,document FROM tasks WHERE scope=$1 ORDER BY id FOR UPDATE", s.Scope)
	if err != nil {
		return nil, err
	}
	tasks := []*Task{}
	for rows.Next() {
		var id string
		var b []byte
		if err = rows.Scan(&id, &b); err != nil {
			rows.Close()
			return nil, err
		}
		var t Task
		if err = json.Unmarshal(b, &t); err != nil {
			rows.Close()
			return nil, err
		}
		if current != nil && id == current.ID {
			tasks = append(tasks, current)
		} else {
			tasks = append(tasks, &t)
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	invalid := map[string]bool{artifact: true}
	changed := true
	for changed {
		changed = false
		for _, t := range tasks {
			for _, n := range t.Graph.Nodes {
				ns := t.Nodes[n.ID]
				if ns.Result == nil || invalid[ns.Result.ArtifactID] {
					continue
				}
				for _, dep := range n.DependsOn {
					up := t.Nodes[dep]
					if up.Result != nil && invalid[up.Result.ArtifactID] {
						invalid[ns.Result.ArtifactID] = true
						changed = true
					}
				}
			}
		}
	}
	for _, t := range tasks {
		dirty := false
		for _, ns := range t.Nodes {
			if ns.Result != nil && ns.Result.Valid && invalid[ns.Result.ArtifactID] {
				ns.Result.Valid = false
				dirty = true
				if _, err = tx.Exec(ctx, "UPDATE committed_results SET valid=false WHERE scope=$1 AND task_id=$2 AND node_id=$3", s.Scope, t.ID, ns.Result.NodeID); err != nil {
					return nil, err
				}
			}
		}
		if dirty {
			t.Status = "NEEDS_REVIEW"
			if current == nil || t.ID != current.ID {
				generation, err := strconvGeneration(t.Generation)
				if err != nil {
					return nil, err
				}
				t.Generation = generation
				b, err := json.Marshal(t)
				if err != nil {
					return nil, err
				}
				if _, err = tx.Exec(ctx, "UPDATE tasks SET document=$3 WHERE scope=$1 AND id=$2", s.Scope, t.ID, b); err != nil {
					return nil, err
				}
			}
			if err = emit(ctx, tx, s.Scope, t.ID, "INVALIDATED", map[string]string{"artifact_id": artifact, "reason": reason}); err != nil {
				return nil, err
			}
		}
	}
	_, err = tx.Exec(ctx, "INSERT INTO cases_and_validity(scope,id,artifact_id,reason,principal) VALUES($1,$2,$3,$4,$5)", s.Scope, newID("case-"), artifact, reason, principal)
	if err != nil {
		return nil, err
	}
	return map[string]any{"artifact_id": artifact, "invalidated_artifacts": len(invalid)}, nil
}
func strconvGeneration(s string) (string, error) {
	var n uint64
	if _, err := fmt.Sscan(s, &n); err != nil {
		return "", err
	}
	return fmt.Sprint(n + 1), nil
}
