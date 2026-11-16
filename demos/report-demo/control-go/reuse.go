package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

// The extractor adapter removes orchestration identities/goal/focus from its
// model input. Everything that it actually consumes remains in this key.
// Changed instruction, source, schema, policy, prompt/model profile => no hit.
func extractionKey(binding Binding) (string, error) {
	binding.TaskID = ""
	binding.Revision = ""
	binding.NodeID = ""
	binding.LogicalExecution = nil
	binding.Goal = ""
	binding.Focus = ""
	return Digest(binding)
}

func (s *Store) prepareReuse(ctx context.Context, id, node string, c Catalog, materialDigest string) ([]string, error) {
	if s.ReuseFiles == nil {
		return nil, nil
	}
	t, err := s.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	b, err := s.Binding(ctx, t, node, c, materialDigest)
	if err != nil {
		return nil, err
	}
	if b.Capability.Kind != "extract" || t.Nodes[node].Attempt > 0 || t.Nodes[node].Result != nil {
		return nil, nil
	}
	key, err := extractionKey(b)
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, "SELECT DISTINCT r.artifact_id FROM extraction_reuse x JOIN committed_results r ON r.scope=x.scope AND r.task_id=x.task_id AND r.node_id=x.node_id WHERE x.scope=$1 AND x.reuse_key=$2 AND r.valid", s.Scope, key)
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
	for _, id := range ids {
		if _, err = s.ReuseFiles.Read(id); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

func (s *Store) reuseExtraction(ctx context.Context, tx pgx.Tx, t *Task, node, h, epoch string, b Binding, verified []string) (*Result, error) {
	key, err := extractionKey(b)
	if err != nil {
		return nil, err
	}
	var source ResultSource
	var artifact, digest string
	var output []byte
	err = tx.QueryRow(ctx, `SELECT r.task_id,r.node_id,r.receipt,r.artifact_id,r.artifact_digest,r.output FROM extraction_reuse x JOIN committed_results r ON r.scope=x.scope AND r.task_id=x.task_id AND r.node_id=x.node_id WHERE x.scope=$1 AND x.reuse_key=$2 AND x.task_id<>$3 AND r.valid AND r.artifact_id=ANY($4::text[]) ORDER BY r.task_id LIMIT 1`, s.Scope, key, t.ID, verified).Scan(&source.TaskID, &source.NodeID, &source.Receipt, &artifact, &digest, &output)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := &Result{NodeID: node, InputDigest: h, ArtifactID: artifact, ArtifactDigest: digest, Receipt: newID("receipt-"), Valid: true, Output: json.RawMessage(output), ReusedFrom: &source}
	result.Identity, err = executionIdentity(t, node, 0, epoch)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, "INSERT INTO committed_results(scope,task_id,node_id,input_digest,artifact_id,artifact_digest,receipt,output,identity) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)", s.Scope, t.ID, node, h, artifact, digest, result.Receipt, output, result.Identity)
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO artifact_dependencies(scope,task_id,node_id,upstream_artifact) VALUES($1,$2,$3,$4)", s.Scope, t.ID, node, artifact); err != nil {
		return nil, err
	}
	t.Nodes[node].InputDigest = h
	t.Nodes[node].Result = result
	t.Nodes[node].Status = "COMMITTED"
	if err = emit(ctx, tx, s.Scope, t.ID, "EXTRACTION_REUSED", result); err != nil {
		return nil, err
	}
	return result, nil
}
