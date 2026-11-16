package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/openjiuwen/agent-festival/go/internal/ledger"
	"github.com/openjiuwen/agent-festival/go/internal/schemalib"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

var identityValidator = sync.OnceValues(func() (*jsonschema.Schema, error) {
	var doc map[string]any
	if err := json.Unmarshal([]byte(identitySchemaJSON), &doc); err != nil {
		return nil, err
	}
	return schemalib.Compile(doc["$id"].(string), doc)
})

// identitySchema returns the shared schema resource for transport exporters.
// It is generated from event-ledger.schema.json, never inferred from Go tags.
func identitySchema() json.RawMessage { return json.RawMessage(identitySchemaJSON) }

// validateIdentity checks the PR25 graph-aware identity against the shared
// schema and enforces the report Demo mapping of task/revision aliases. No file lookup depends on CWD.
func validateIdentity(identity ledger.Identity) error {
	schema, err := identityValidator()
	if err != nil {
		return err
	}
	o := identity.LogicalExecution
	if o == nil || identity.ExecutionGeneration == nil {
		return fmt.Errorf("graph identity requires logical execution and generation")
	}
	if identity.BusinessRequestID != o.TaskID || identity.EvaluationRevision != o.GraphVersion {
		return fmt.Errorf("conflicting identity bindings")
	}
	raw, err := json.Marshal(identity)
	if err != nil {
		return err
	}
	var doc any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err = d.Decode(&doc); err != nil {
		return err
	}
	return schema.Validate(doc)
}
