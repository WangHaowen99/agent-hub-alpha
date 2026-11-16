package main

import (
	"encoding/json"
	"github.com/openjiuwen/agent-festival/go/internal/ledger"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGraphIdentityUsesSharedSchemaAndTrustedBindings(t *testing.T) {
	e := 2
	valid := ledger.Identity{BusinessRequestID: "T1", EvaluationRevision: "g1", OperationID: "attempt-1", LogicalExecution: &ledger.LogicalExecution{TaskID: "T1", GraphVersion: "g1", NodePath: "analysis", IterationPath: "0"}, ExecutionGeneration: &e}
	if err := validateIdentity(valid); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"missing-o", "missing-e", "empty-path", "task-conflict", "graph-conflict", "negative-e"} {
		t.Run(kind, func(t *testing.T) {
			v := valid
			o := *valid.LogicalExecution
			v.LogicalExecution = &o
			switch kind {
			case "missing-o":
				v.LogicalExecution = nil
			case "missing-e":
				v.ExecutionGeneration = nil
			case "empty-path":
				o.IterationPath = ""
			case "task-conflict":
				o.TaskID = "other"
			case "graph-conflict":
				o.GraphVersion = "other"
			case "negative-e":
				n := -1
				v.ExecutionGeneration = &n
			}
			if validateIdentity(v) == nil {
				t.Fatal("invalid graph identity accepted")
			}
		})
	}
}

func TestEmbeddedIdentityMatchesCanonicalSchema(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("../../../agent-festival/schema/event-ledger.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var canonical, embedded map[string]any
	if err = json.Unmarshal(raw, &canonical); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal([]byte(identitySchemaJSON), &embedded); err != nil {
		t.Fatal(err)
	}
	delete(embedded, "$id")
	if !reflect.DeepEqual(canonical["$defs"].(map[string]any)["identity"], embedded) {
		t.Fatal("regenerate schema resource with deploy/embed_identity_schema.py")
	}
}
