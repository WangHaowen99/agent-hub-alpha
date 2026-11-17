package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishedModelBindingCannotFallBackToCurrentDefaults(t *testing.T) {
	cfg := Config{Listen: "127.0.0.1:0", PublicURL: "http://127.0.0.1", DSN: "unused-fixture", Scope: "fixture", CompilerURL: "http://127.0.0.1", User: "fixture", UserPassword: "fixture-password-only", LeaseSeconds: 30, ExecutionProfile: "reviewed-fixture-profile"}
	raw, _ := json.Marshal(cfg)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := LoadConfig(path); err == nil {
		t.Fatal("published profile accepted without its actual model binding")
	}
	c, g := reportSample(t)
	server := &Server{Store: &Store{}, Catalog: c, Material: Material{ID: g.MaterialID}, Config: cfg}
	// No Pool/HTTP client: the error must precede selection persistence/compiler,
	// rather than dispatching and merely writing a blocked label afterwards.
	_, err := server.checkPublicCandidate(context.Background(), g)
	requireCode(t, err, "BINDING_CHANGED")
}
