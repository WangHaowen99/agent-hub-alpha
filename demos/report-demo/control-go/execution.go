package main

import (
	"context"
	"strconv"

	"github.com/openjiuwen/agent-festival/go/internal/ledger"
)

const pr25Contract = "pr25/0.2.0@539d17a"

func logicalExecution(t *Task, node string) *ledger.LogicalExecution {
	// Every admitted node executes once; the bounded revision is a different
	// node. Node-internal Loop branches are evidence, not formal graph nodes.
	return &ledger.LogicalExecution{TaskID: t.ID, GraphVersion: t.Revision, NodePath: node, IterationPath: "0"}
}

func executionIdentity(t *Task, node string, attempt int, epoch string) (*ledger.Identity, error) {
	if t.IdentityContract == "" {
		return nil, nil
	} // retain legacy bindings
	if t.IdentityContract != pr25Contract {
		return nil, fault("BINDING_CHANGED", "identity contract unavailable", 409)
	}
	e, err := strconv.Atoi(epoch)
	if err != nil {
		return nil, fault("INVALID_REQUEST", "invalid execution generation", 422)
	}
	o := logicalExecution(t, node)
	key, err := Digest(struct {
		Execution *ledger.LogicalExecution
		Attempt   int
	}{o, attempt})
	if err != nil {
		return nil, err
	}
	identity := &ledger.Identity{BusinessRequestID: t.ID, EvaluationRevision: t.Revision, OperationID: key, LogicalExecution: o, ExecutionGeneration: &e}
	if err = validateIdentity(*identity); err != nil {
		return nil, fault("INVALID_REQUEST", "invalid PR25 structured identity", 422)
	}
	return identity, nil
}

func resultRecord(r *Result) *Record[*Result] {
	if r == nil {
		return nil
	}
	return &Record[*Result]{InputDigest: r.InputDigest, ContentDigest: r.ArtifactID, Value: r}
}

// ReconcileResult never mutates a task/lease or authorizes a new attempt.
func (s *Store) ReconcileResult(ctx context.Context, id, node, h string) (any, error) {
	if h == "" {
		return nil, fault("INVALID_REQUEST", "input_digest required", 400)
	}
	t, err := s.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	ns := t.Nodes[node]
	if ns == nil {
		return nil, fault("NOT_FOUND", "node not in admitted graph", 404)
	}
	committed, serve, stored := QueryForReconciliation(resultRecord(ns.Result), h)
	state := "UNCOMMITTED"
	if committed {
		state = "CONFLICT"
		if serve {
			state = "COMMITTED"
		}
		if !stored.Valid {
			serve = false
			state = "INVALIDATED"
		}
	}
	// An uncommitted projection is deliberately not a safe-to-restart signal.
	return map[string]any{"committed": committed, "serve": serve, "stored": stored, "state": state, "node_status": ns.Status, "logical_execution": logicalExecution(t, node), "identity_contract": t.IdentityContract, "read_only": true, "execution_permission_granted": false, "generation": t.Generation}, nil
}
