package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openjiuwen/agent-festival/go/internal/ledger"
	"github.com/openjiuwen/agent-festival/go/internal/rfc8785"
)

func ContractSchemas() (map[string]*jsonschema.Schema, error) {
	var identity, logical jsonschema.Schema
	raw := ledger.IdentitySchema()
	if err := json.Unmarshal(raw, &identity); err != nil {
		return nil, err
	}
	var shared struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &shared); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(shared.Properties["logical_execution"], &logical); err != nil {
		return nil, err
	}
	// RawMessage is already parsed JSON in these DTOs, never base64 bytes.
	opts := &jsonschema.ForOptions{TypeSchemas: map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[json.RawMessage](): {}, reflect.TypeFor[ledger.Identity](): &identity, reflect.TypeFor[ledger.LogicalExecution](): &logical,
	}}
	types := map[string]reflect.Type{
		"propose-input": reflect.TypeFor[ProposeInput](), "graph": reflect.TypeFor[Graph](),
		"binding": reflect.TypeFor[Binding](), "commit-request": reflect.TypeFor[CommitRequest](),
		"result": reflect.TypeFor[Result](), "remote-call": reflect.TypeFor[Call](),
		"observation": reflect.TypeFor[Observation](), "lease": reflect.TypeFor[Lease](),
		"task": reflect.TypeFor[Task](), "human-review": reflect.TypeFor[CheckReview](),
		"effect-intent": reflect.TypeFor[EffectIntent](), "effect-observation": reflect.TypeFor[EffectObservation](),
	}
	out := map[string]*jsonschema.Schema{}
	for name, t := range types {
		s, err := jsonschema.ForType(t, opts)
		if err != nil {
			return nil, err
		}
		s.Schema = "https://json-schema.org/draft/2020-12/schema"
		s.ID = "urn:agent-hub:report-demo:1:" + name
		out[name] = s
	}
	return out, nil
}

func ExportContracts(dir string) error {
	schemas, err := ContractSchemas()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for name, s := range schemas {
		raw, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(dir, name+".schema.json"), append(raw, '\n'), 0644); err != nil {
			return err
		}
	}
	return nil
}

func CanonicalJSONFile(path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value any
	if err = DecodeStrict(raw, &value); err != nil {
		return nil, err
	}
	canonical, err := rfc8785.Canonicalize(value)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(canonical)
	return map[string]any{"profile": "rfc8785/1", "canonical": string(canonical), "digest": "sha256:" + hex.EncodeToString(sum[:]), "database_calls": 0}, nil
}
