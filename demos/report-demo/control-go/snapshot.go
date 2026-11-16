package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/openjiuwen/agent-festival/go/internal/ledger"
)

var snapshotTables = []string{"tasks", "commands", "run_leases", "artifacts", "committed_results", "artifact_dependencies", "node_attempts", "remote_calls", "observations", "checks", "approvals", "delivery_intents", "cases_and_validity", "extraction_reuse", "audit_outbox"}

type SnapshotPayload struct {
	Version          string                       `json:"schema_version"`
	Scope            string                       `json:"scope"`
	CreatedAt        time.Time                    `json:"created_at"`
	ExecutionProfile string                       `json:"execution_profile"`
	Catalog          Catalog                      `json:"catalog"`
	Material         Material                     `json:"material"`
	Tables           map[string][]json.RawMessage `json:"tables"`
	Contents         map[string][]byte            `json:"contents"`
}
type Snapshot struct {
	Payload SnapshotPayload `json:"payload"`
	Digest  string          `json:"digest"`
}

// Backup freezes a PG snapshot first, releases the transaction, then reads
// immutable content. No filesystem/network/model work runs in that transaction.
func (s *Store) Backup(ctx context.Context, files Files, cat Catalog, material Material) (Snapshot, error) {
	p := SnapshotPayload{Version: "report-demo-backup/1", Scope: s.Scope, CreatedAt: time.Now().UTC(), ExecutionProfile: s.ExecutionProfile, Catalog: cat, Material: material, Tables: map[string][]json.RawMessage{}, Contents: map[string][]byte{}}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Snapshot{}, err
	}
	defer tx.Rollback(ctx)
	for _, table := range snapshotTables {
		query := "SELECT to_jsonb(r) FROM " + pgx.Identifier{table}.Sanitize() + " r WHERE scope=$1 ORDER BY to_jsonb(r)::text"
		rows, err := tx.Query(ctx, query, s.Scope)
		if err != nil {
			return Snapshot{}, err
		}
		data := []json.RawMessage{}
		for rows.Next() {
			var raw []byte
			if err = rows.Scan(&raw); err != nil {
				rows.Close()
				return Snapshot{}, err
			}
			data = append(data, json.RawMessage(raw))
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return Snapshot{}, err
		}
		p.Tables[table] = data
	}
	if err = tx.Commit(ctx); err != nil {
		return Snapshot{}, err
	}
	for _, raw := range p.Tables["artifacts"] {
		var row struct {
			ID string `json:"id"`
		}
		if err = json.Unmarshal(raw, &row); err != nil {
			return Snapshot{}, err
		}
		body, err := files.Read(row.ID)
		if err != nil {
			return Snapshot{}, err
		}
		p.Contents[row.ID] = body
	}
	digest, err := Digest(p)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{p, digest}
	return snapshot, snapshot.Validate()
}

func (snapshot Snapshot) Validate() error {
	p := snapshot.Payload
	if p.Version != "report-demo-backup/1" || p.Scope == "" {
		return errors.New("unsupported snapshot")
	}
	digest, err := Digest(p)
	if err != nil {
		return err
	}
	if digest != snapshot.Digest {
		return errors.New("snapshot manifest digest differs")
	}
	if len(p.Tables) != len(snapshotTables) {
		return errors.New("snapshot table manifest incomplete")
	}
	for _, table := range snapshotTables {
		rows, ok := p.Tables[table]
		if !ok {
			return errors.New("snapshot table missing")
		}
		for _, raw := range rows {
			var row map[string]any
			if err = DecodeStrict(raw, &row); err != nil {
				return err
			}
			if row["scope"] != p.Scope {
				return errors.New("mixed snapshot scope")
			}
		}
	}
	artifacts := map[string]bool{}
	for _, raw := range p.Tables["artifacts"] {
		var row struct {
			ID     string `json:"id"`
			Digest string `json:"digest"`
			Size   int64  `json:"size"`
		}
		if err = json.Unmarshal(raw, &row); err != nil {
			return err
		}
		body, ok := p.Contents[row.ID]
		if !ok {
			return errors.New("snapshot referenced content missing")
		}
		hash := sha256.Sum256(body)
		if !blobID.MatchString(row.ID) || hex.EncodeToString(hash[:]) != row.ID || row.Digest != "sha256:"+row.ID || row.Size != int64(len(body)) {
			return errors.New("snapshot content differs")
		}
		var value any
		if err = DecodeStrict(body, &value); err != nil {
			return err
		}
		artifacts[row.ID] = true
	}
	if len(artifacts) != len(p.Contents) {
		return errors.New("snapshot contains unmanifested content")
	}
	for _, raw := range p.Tables["tasks"] {
		var row struct {
			Document json.RawMessage `json:"document"`
		}
		if err = json.Unmarshal(raw, &row); err != nil {
			return err
		}
		var task map[string]any
		if err = DecodeStrict(row.Document, &task); err != nil {
			return err
		}
		graph, ok := task["graph"]
		if !ok {
			return errors.New("task graph missing")
		}
		graphDigest, err := Digest(graph)
		if err != nil {
			return err
		}
		if graphDigest != task["graph_digest"] {
			return errors.New("admitted graph digest differs")
		}
	}
	for _, raw := range p.Tables["committed_results"] {
		var row struct {
			Artifact string          `json:"artifact_id"`
			Digest   string          `json:"artifact_digest"`
			Output   json.RawMessage `json:"output"`
		}
		if err = json.Unmarshal(raw, &row); err != nil {
			return err
		}
		if !artifacts[row.Artifact] || row.Digest != "sha256:"+row.Artifact {
			return errors.New("formal result content reference differs")
		}
		var a, b any
		if err = DecodeStrict(row.Output, &a); err != nil {
			return err
		}
		if err = DecodeStrict(p.Contents[row.Artifact], &b); err != nil {
			return err
		}
		ah, _ := Digest(a)
		bh, _ := Digest(b)
		if ah != bh {
			return errors.New("formal result and content disagree")
		}
	}
	_, err = snapshot.projections()
	return err
}

func (snapshot Snapshot) projections() (map[string]any, error) {
	results := map[string]Result{}
	for _, raw := range snapshot.Payload.Tables["committed_results"] {
		var row struct {
			Task string `json:"task_id"`
			Node string `json:"node_id"`
		}
		var result Result
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &result); err != nil {
			return nil, err
		}
		key := row.Task + "/" + row.Node
		results[key] = result
	}
	deliveries := map[string]int{}
	for _, raw := range snapshot.Payload.Tables["delivery_intents"] {
		var row struct {
			Task  string `json:"task_id"`
			State string `json:"state"`
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
		if row.State == "DELIVERED" {
			deliveries[row.Task]++
		}
	}
	views := map[string]any{}
	for _, raw := range snapshot.Payload.Tables["tasks"] {
		var row struct {
			ID       string          `json:"id"`
			Document json.RawMessage `json:"document"`
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
		var task Task
		if err := json.Unmarshal(row.Document, &task); err != nil {
			return nil, err
		}
		if task.ID != row.ID {
			return nil, errors.New("task document identity differs")
		}
		states := map[string]string{}
		formal := 0
		for _, node := range task.Graph.Nodes {
			ns := task.Nodes[node.ID]
			if ns == nil {
				return nil, errors.New("node projection missing")
			}
			states[node.ID] = ns.Status
			r, exists := results[task.ID+"/"+node.ID]
			if ns.Result == nil && exists {
				return nil, errors.New("formal result absent from task projection")
			}
			if ns.Result != nil {
				formal++
				saved := ns.Result
				if !exists || r.NodeID != saved.NodeID || r.Receipt != saved.Receipt || r.InputDigest != saved.InputDigest || r.ArtifactID != saved.ArtifactID || r.ArtifactDigest != saved.ArtifactDigest || r.Valid != saved.Valid {
					return nil, errors.New("task projection and normalized result disagree")
				}
				if task.IdentityContract != "" {
					if task.IdentityContract != pr25Contract || saved.Identity == nil || ledger.ValidateIdentity(*saved.Identity) != nil || *saved.Identity.LogicalExecution != *logicalExecution(&task, node.ID) {
						return nil, errors.New("formal result structured identity differs")
					}
				}
				identityDigest, err := Digest(r.Identity)
				if err != nil {
					return nil, err
				}
				projectionDigest, err := Digest(saved.Identity)
				if err != nil {
					return nil, err
				}
				if identityDigest != projectionDigest {
					return nil, errors.New("normalized identity and task projection disagree")
				}
				if saved.ReusedFrom != nil {
					parent, ok := results[saved.ReusedFrom.TaskID+"/"+saved.ReusedFrom.NodeID]
					if !ok || parent.Receipt != saved.ReusedFrom.Receipt || parent.ArtifactID != saved.ArtifactID || (saved.Valid && !parent.Valid) {
						return nil, errors.New("extraction reuse lineage differs")
					}
				}
			}
			if ns.Status == "AUTHORIZED_SKIP" {
				if _, ok := skipProof(&task, node.ID); !ok {
					return nil, errors.New("authorized skip proof no longer matches task projection")
				}
			}
		}
		if task.Status == "DELIVERED" && deliveries[task.ID] == 0 {
			return nil, errors.New("delivered projection has no publication receipt")
		}
		views[task.ID] = map[string]any{"status": task.Status, "graph_digest": task.GraphDigest, "node_states": states, "formal_results": formal, "publication_receipts": deliveries[task.ID], "read_only": true}
	}
	return views, nil
}

func WriteSnapshot(path string, snapshot Snapshot) error {
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".backup-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func ReadSnapshot(path string) (Snapshot, error) {
	var snapshot Snapshot
	raw, err := os.ReadFile(path)
	if err != nil {
		return snapshot, err
	}
	if err = DecodeStrict(raw, &snapshot); err != nil {
		return snapshot, err
	}
	return snapshot, snapshot.Validate()
}

// Replay has no Store, HTTP client, scheduler, model, or publisher dependency.
// It only reads the standalone package and reports verifiable recorded facts.
func ReplaySnapshot(path string) (any, error) {
	s, err := ReadSnapshot(path)
	if err != nil {
		return nil, err
	}
	views, err := s.projections()
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": "VERIFIED_OFFLINE", "scope": s.Payload.Scope, "digest": s.Digest, "tables": len(s.Payload.Tables), "artifacts": len(s.Payload.Contents), "task_projections": views, "read_only": true, "model_calls": 0, "runtime_calls": 0, "publication_calls": 0, "database_calls": 0, "business_acceptance_asserted": false}, nil
}

// Restore is permitted only into a different, explicitly named empty test DB.
// Existing rows are never deleted or overwritten. It does not launch services.
func RestoreSnapshot(ctx context.Context, snapshot Snapshot, targetDSN, sourceDSN, contentRoot, sourceContentRoot string) (any, error) {
	if contentRoot == "" || !filepath.IsAbs(contentRoot) {
		return nil, errors.New("explicit absolute isolated content directory required")
	}
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}
	target, err := pgx.ParseConfig(targetDSN)
	if err != nil {
		return nil, err
	}
	source, err := pgx.ParseConfig(sourceDSN)
	if err != nil {
		return nil, err
	}
	if target.Database == source.Database || !strings.HasSuffix(target.Database, "_restore_test") {
		return nil, errors.New("restore target must be a different *_restore_test database")
	}
	dest, err := filepath.Abs(contentRoot)
	if err != nil {
		return nil, err
	}
	original, err := filepath.Abs(sourceContentRoot)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Clean(dest), filepath.Clean(original)) {
		return nil, errors.New("isolated restore content directory required")
	}
	files := Files{Root: dest}
	for id, raw := range snapshot.Payload.Contents {
		saved, err := files.Put(raw)
		if err != nil {
			return nil, err
		}
		if saved != id {
			return nil, errors.New("restore content digest differs")
		}
	}
	store, err := OpenStore(ctx, targetDSN, snapshot.Payload.Scope, 30)
	if err != nil {
		return nil, err
	}
	defer store.Pool.Close()
	if err = store.Migrate(ctx); err != nil {
		return nil, err
	}
	tx, err := store.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	for _, table := range snapshotTables {
		var count int
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&count); err != nil {
			return nil, err
		}
		if count != 0 {
			return nil, fmt.Errorf("restore target table %s is not empty", table)
		}
	}
	for _, table := range snapshotTables {
		name := pgx.Identifier{table}.Sanitize()
		query := "INSERT INTO " + name + " OVERRIDING SYSTEM VALUE SELECT * FROM jsonb_populate_record(NULL::" + name + ",$1::jsonb)"
		for _, raw := range snapshot.Payload.Tables[table] {
			if _, err = tx.Exec(ctx, query, []byte(raw)); err != nil {
				return nil, err
			}
		}
	}
	if _, err = tx.Exec(ctx, "SELECT setval(pg_get_serial_sequence('audit_outbox','sequence'),GREATEST(COALESCE((SELECT max(sequence) FROM audit_outbox),1),1),true)"); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"status": "RESTORED_ISOLATED_TEST_DB", "database": target.Database, "scope": snapshot.Payload.Scope, "artifacts": len(snapshot.Payload.Contents), "digest": snapshot.Digest, "services_started": false, "login_sessions_restored": false}, nil
}
