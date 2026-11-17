package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Call struct {
	ID           string          `json:"call_id"`
	TaskID       string          `json:"task_id"`
	NodeID       string          `json:"node_id"`
	Attempt      int             `json:"attempt"`
	InputDigest  string          `json:"input_digest"`
	AgentID      string          `json:"agent_id"`
	MessageID    string          `json:"message_id"`
	ContextID    string          `json:"context_id"`
	RemoteTaskID string          `json:"remote_task_id,omitempty"`
	State        string          `json:"state"`
	Route        json.RawMessage `json:"route"`
	Conflicted   bool            `json:"conflicted"`
}

// Action is a response-only permission. It is never stored in remote_calls;
// rereading an intent cannot recover a lost permission to send it again.
type CallDecision struct {
	Call
	Action string `json:"action"`
}

func reconcileCall(call Call) CallDecision {
	action := "UNKNOWN"
	if call.RemoteTaskID != "" && !call.Conflicted {
		action = "QUERY"
	}
	return CallDecision{Call: call, Action: action}
}

func (s *Store) BeginCall(ctx context.Context, id, node, owner, epoch string, route json.RawMessage, c Catalog) (any, error) {
	return s.mutate(ctx, id, "SEND_INTENT", func(tx pgx.Tx, t *Task) (any, error) {
		if err := s.leaseCheck(ctx, tx, id, "graph", owner, epoch); err != nil {
			return nil, err
		}
		if err := s.checkBusBinding(t); err != nil {
			return nil, err
		}
		cd, err := Digest(c)
		if err != nil {
			return nil, err
		}
		if cd != t.Graph.CatalogDigest || t.ExecutionProfile != s.ExecutionProfile || t.ModelBindingDigest != s.ModelBindingDigest {
			return nil, fault("BINDING_CHANGED", "admitted catalog/execution profile changed before dispatch", 409)
		}
		if t.Pause {
			return nil, fault("PAUSED", "new call stopped", 409)
		}
		ns, ok := t.Nodes[node]
		if !ok || ns.OwnerEpoch != epoch {
			return nil, fault("FENCED", "node attempt not owned", 403)
		}
		var previous []byte
		err = tx.QueryRow(ctx, "SELECT document FROM remote_calls WHERE scope=$1 AND task_id=$2 AND node_id=$3 AND attempt=$4", s.Scope, id, node, ns.Attempt).Scan(&previous)
		if err == nil {
			var call Call
			err = json.Unmarshal(previous, &call)
			return reconcileCall(call), err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if ns.Status != "CLAIMED" {
			return nil, fault("FENCED", "node attempt not claimable for a new call", 403)
		}
		var cap Capability
		for _, n := range t.Graph.Nodes {
			if n.ID == node {
				cap, _ = c.Get(n.Expert)
			}
		}
		if cap.AgentID == "" || !cap.Enabled {
			return nil, fault("POLICY_REJECTED", "not a remote capability", 422)
		}
		if cap.RemoteBinding != nil {
			if err := validateRoute(cap, route); err != nil {
				return nil, err
			}
		}
		call := Call{ID: newID("c-"), TaskID: id, NodeID: node, Attempt: ns.Attempt, InputDigest: ns.InputDigest, AgentID: cap.AgentID, MessageID: newID("message-"), ContextID: newID("context-"), State: "SEND_INTENT", Route: route}
		b, err := json.Marshal(call)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, "INSERT INTO remote_calls(scope,id,task_id,node_id,attempt,input_digest,message_id,state,document) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)", s.Scope, call.ID, id, node, call.Attempt, call.InputDigest, call.MessageID, call.State, b)
		if err != nil {
			return nil, err
		}
		ns.Status = "WAITING_REMOTE"
		ns.Reason = "remote send intent persisted"
		return CallDecision{Call: call, Action: "SEND"}, nil
	})
}

type Observation struct {
	EventID      string          `json:"event_id"`
	State        string          `json:"state"`
	RemoteTaskID string          `json:"remote_task_id"`
	ContextID    string          `json:"context_id"`
	Payload      json.RawMessage `json:"payload"`
}

func (s *Store) Observe(ctx context.Context, callID, source string, o Observation) (any, error) {
	if o.EventID == "" || len(o.EventID) > 200 {
		return nil, fault("INVALID_REQUEST", "event_id required", 400)
	}
	allowed := map[string]bool{"TRANSPORT": true, "ACCEPTED": true, "WORKING": true, "COMPLETED": true, "INPUT_REQUIRED": true, "AUTH_REQUIRED": true, "FAILED": true, "REJECTED": true, "CANCELED": true, "UNKNOWN": true}
	if !allowed[o.State] {
		return nil, fault("INVALID_STATE", "unknown remote state", 400)
	}
	b, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	h, err := Digest(o)
	if err != nil {
		return nil, err
	}
	var taskID string
	if err = s.Pool.QueryRow(ctx, "SELECT task_id FROM remote_calls WHERE scope=$1 AND id=$2", s.Scope, callID).Scan(&taskID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fault("NOT_FOUND", "call not found", 404)
		}
		return nil, err
	}
	// This endpoint authenticates a service and the persisted call association. It
	// intentionally does not use an expired run lease to discard true late facts.
	return s.mutate(ctx, taskID, "REMOTE_OBSERVATION", func(tx pgx.Tx, t *Task) (any, error) {
		var doc []byte
		if err := tx.QueryRow(ctx, "SELECT document FROM remote_calls WHERE scope=$1 AND id=$2 FOR UPDATE", s.Scope, callID).Scan(&doc); err != nil {
			return nil, err
		}
		var call Call
		if err = json.Unmarshal(doc, &call); err != nil {
			return nil, err
		}
		if o.ContextID != "" && o.ContextID != call.ContextID {
			return nil, fault("INPUT_CONFLICT", "context association differs", 409)
		}
		var previous, previousCall string
		err := tx.QueryRow(ctx, "SELECT payload_digest,call_id FROM observations WHERE scope=$1 AND source=$2 AND event_id=$3", s.Scope, source, o.EventID).Scan(&previous, &previousCall)
		if err == nil {
			if previous != h || previousCall != callID {
				return nil, fault("INPUT_CONFLICT", "event identity changed content", 409)
			}
			return map[string]string{"status": "REUSE", "event_id": o.EventID}, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO observations(scope,source,event_id,call_id,payload_digest,document) VALUES($1,$2,$3,$4,$5,$6)", s.Scope, source, o.EventID, callID, h, b); err != nil {
			return nil, err
		}
		conflict := call.RemoteTaskID != "" && o.RemoteTaskID != "" && call.RemoteTaskID != o.RemoteTaskID
		terminal := map[string]bool{"COMPLETED": true, "FAILED": true, "REJECTED": true, "CANCELED": true}
		if terminal[o.State] && terminal[call.State] && call.State != o.State {
			conflict = true
		}
		ns := t.Nodes[call.NodeID]
		if o.State == "COMPLETED" && !conflict {
			bound := call
			if bound.RemoteTaskID == "" {
				bound.RemoteTaskID = o.RemoteTaskID
			}
			digests, err := s.completedOutputDigests(ctx, tx, bound)
			if err != nil {
				return nil, err
			}
			if len(digests) > 1 {
				conflict = true
			}
			if ns != nil && ns.Attempt == call.Attempt && ns.Result != nil && ns.Result.InputDigest == call.InputDigest && len(digests) > 0 {
				digest, err := Digest(ns.Result.Output)
				if err != nil {
					return nil, err
				}
				if !digests[digest] {
					conflict = true
				}
			}
		}
		wasConflicted := call.Conflicted
		if conflict || call.Conflicted {
			call.Conflicted = true
			call.State = "UNKNOWN"
		} else {
			if o.RemoteTaskID != "" {
				call.RemoteTaskID = o.RemoteTaskID
			}
			if o.State != "TRANSPORT" && !terminal[call.State] && !(call.State == "WORKING" && o.State == "ACCEPTED") {
				call.State = o.State
			}
		}
		doc, err = json.Marshal(call)
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, "UPDATE remote_calls SET state=$3,document=$4 WHERE scope=$1 AND id=$2", s.Scope, callID, call.State, doc)
		if err != nil {
			return nil, err
		}
		if conflict && !wasConflicted && ns != nil && ns.Attempt == call.Attempt && ns.Result != nil && ns.Result.InputDigest == call.InputDigest {
			_, err = s.invalidateResults(ctx, tx, t, ns.Result.ArtifactID, "conflicting remote observation for "+call.ID, source)
			if err != nil {
				return nil, err
			}
		}
		return map[string]any{"status": "OBSERVED", "call_id": callID, "state": call.State, "conflict": conflict}, err
	})
}
func (s *Store) Calls(ctx context.Context, id, node string) ([]Call, error) {
	rows, err := s.Pool.Query(ctx, "SELECT document FROM remote_calls WHERE scope=$1 AND task_id=$2 AND ($3='' OR node_id=$3) ORDER BY attempt", s.Scope, id, node)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Call{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var c Call
		if err = json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) Observations(ctx context.Context, call string) ([]map[string]any, error) {
	rows, err := s.Pool.Query(ctx, "SELECT source,event_id,payload_digest,document,received_at FROM observations WHERE scope=$1 AND call_id=$2 ORDER BY received_at,event_id", s.Scope, call)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var source, event, digest string
		var doc json.RawMessage
		var at any
		if err = rows.Scan(&source, &event, &digest, &doc, &at); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"source": source, "event_id": event, "payload_digest": digest, "document": doc, "received_at": at})
	}
	return out, rows.Err()
}
