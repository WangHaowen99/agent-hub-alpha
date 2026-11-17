package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// An existing remote call must qualify its actual observed output before C03.
// Pure local nodes and explicit local control fixtures have no remote call.
// Observations remain evidence; none independently becomes a formal result.
func (s *Store) qualifyRemoteResult(ctx context.Context, tx pgx.Tx, id, node string, attempt int, h, outputDigest string) error {
	var raw []byte
	err := tx.QueryRow(ctx, "SELECT document FROM remote_calls WHERE scope=$1 AND task_id=$2 AND node_id=$3 AND attempt=$4 FOR SHARE", s.Scope, id, node, attempt).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var call Call
	if err = json.Unmarshal(raw, &call); err != nil {
		return err
	}
	if call.Conflicted {
		return fault("INPUT_CONFLICT", "conflicting remote observations cannot qualify a formal result", 409)
	}
	if call.State != "COMPLETED" || call.RemoteTaskID == "" {
		return fault("REMOTE_UNRESOLVED", "remote acceptance/unknown/conflict is not a qualified completed result", 409)
	}
	if call.InputDigest != h || call.Attempt != attempt {
		return fault("INPUT_CONFLICT", "remote call binding differs", 409)
	}
	digests, err := s.completedOutputDigests(ctx, tx, call)
	if err != nil {
		return err
	}
	if len(digests) != 1 || !digests[outputDigest] {
		return fault("INPUT_CONFLICT", "candidate differs from the uniquely observed bound remote output", 409)
	}
	return nil
}

func (s *Store) completedOutputDigests(ctx context.Context, tx pgx.Tx, call Call) (map[string]bool, error) {
	var model string
	if err := tx.QueryRow(ctx, "SELECT COALESCE(document->>'model_binding_digest','') FROM tasks WHERE scope=$1 AND id=$2", s.Scope, call.TaskID).Scan(&model); err != nil {
		return nil, err
	}
	var raw []byte
	rows, err := tx.Query(ctx, "SELECT document FROM observations WHERE scope=$1 AND call_id=$2 AND document->>'state'='COMPLETED'", s.Scope, call.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	digests := map[string]bool{}
	for rows.Next() {
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var observation Observation
		if err = json.Unmarshal(raw, &observation); err != nil {
			return nil, err
		}
		if observation.RemoteTaskID != call.RemoteTaskID || observation.ContextID != call.ContextID {
			continue
		}
		var task struct {
			ID        string `json:"id"`
			Context   string `json:"contextId"`
			Artifacts []struct {
				Parts []struct {
					Data json.RawMessage `json:"data"`
					Text string          `json:"text"`
				} `json:"parts"`
			} `json:"artifacts"`
		}
		if err = json.Unmarshal(observation.Payload, &task); err != nil || task.ID != call.RemoteTaskID || task.Context != call.ContextID {
			continue
		}
		for _, artifact := range task.Artifacts {
			for _, part := range artifact.Parts {
				body := part.Data
				if len(body) == 0 {
					body = []byte(part.Text)
				}
				var envelope map[string]any
				if DecodeStrict(body, &envelope) != nil || envelope["call_id"] != call.ID || envelope["input_digest"] != call.InputDigest {
					continue
				}
				if model != "" {
					evidence, ok := envelope["runtime_evidence"].(map[string]any)
					if !ok || evidence["agent_id"] != call.AgentID || evidence["model_binding_digest"] != model {
						continue
					}
				}
				a, ok := envelope["attempt"].(json.Number)
				if !ok {
					continue
				}
				n, err := a.Int64()
				if err != nil || n != int64(call.Attempt) {
					continue
				}
				output, exists := envelope["output"]
				if !exists {
					continue
				}
				digest, err := Digest(output)
				if err != nil {
					return nil, err
				}
				digests[digest] = true
			}
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return digests, nil
}
