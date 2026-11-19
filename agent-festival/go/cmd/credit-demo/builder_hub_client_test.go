package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const builderTestPreflight = `{"schema_version":"builder-hub/v1","scenario":{"id":"intent","version":"1.0","profile":"gate"},"operation":"build","status":"indeterminate","prerequisites":[{"code":"dependency-unavailable","status":"unknown"}],"execution_authorized":false}`

func builderTestCapabilities() string {
	steps := []string{}
	for _, s := range []string{"build", "observe", "evaluate", "analyze", "optimize", "reevaluate"} {
		steps = append(steps, fmt.Sprintf(`{"step":%q,"implementation":"supported","integration":"not_connected","qualifications":[],"constraints":[]}`, s))
	}
	return `{"schema_version":"builder-hub/v1","capabilities":[{"scenario":{"id":"intent","version":"1.0","profile":"gate"},"steps":[` + strings.Join(steps, ",") + `]}]}`
}
func builderTestConfig(target string) builderHubConfig {
	return builderHubConfig{Target: target, Key: []byte(strings.Repeat("k", 32)), ChannelToken: strings.Repeat("t", 32), Issuer: "hub-local", KeyID: "local-1", Audience: "intent-host", Scope: "intent-builder", LookupSubject: func(user string) (string, bool) { return user, user == "alice" }}
}

func TestBuilderHubClientVectors(t *testing.T) {
	raw, e := os.ReadFile("testdata/query-context-v1.json")
	if e != nil {
		t.Fatal(e)
	}
	var fixture struct {
		KeyHex  string `json:"key_hex"`
		Vectors []struct {
			Name   string
			Claims map[string]any
			Header string
			Valid  bool
		}
	}
	if e = json.Unmarshal(raw, &fixture); e != nil {
		t.Fatal(e)
	}
	key, _ := hex.DecodeString(fixture.KeyHex)
	count := 0
	for _, v := range fixture.Vectors {
		if !v.Valid {
			continue
		}
		t.Run(v.Name, func(t *testing.T) {
			got, e := builderProof(key, v.Claims)
			if e != nil || got != v.Header {
				t.Fatalf("golden mismatch %v", e)
			}
		})
		count++
	}
	if count != 3 {
		t.Fatalf("expected three positive vectors, got %d", count)
	}
}
func TestBuilderHubClientBinding(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("t", 32) {
			t.Error("missing separate channel")
		}
		parts := strings.Split(r.Header.Get(builderContextHeader), ".")
		payload, e := base64.RawURLEncoding.DecodeString(parts[0])
		if e != nil {
			t.Error(e)
		}
		var claims map[string]any
		_ = json.Unmarshal(payload, &claims)
		digest := sha256.Sum256([]byte{})
		if claims["body_sha256"] != hex.EncodeToString(digest[:]) || claims["subject"] != "alice" || claims["path"] != r.URL.Path {
			t.Error("wrong binding")
		}
		_, _ = w.Write([]byte(builderTestCapabilities()))
	}))
	defer upstream.Close()
	c, e := newBuilderHubClient(builderTestConfig(upstream.URL))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.query(context.Background(), "alice", "capabilities"); e != nil {
		t.Fatal(e)
	}
	if calls != 1 {
		t.Fatal("unexpected retry")
	}
}
func TestBuilderHubClientRefusals(t *testing.T) {
	for _, target := range []string{"http://example.com", "http://127.0.0.1:5/path", "http://user:pass@127.0.0.1", "http://127.0.0.1?target=x", "file:///tmp/a"} {
		if _, e := newBuilderHubClient(builderTestConfig(target)); e == nil {
			t.Fatalf("accepted %s", target)
		}
	}
	for name, raw := range map[string]string{"duplicate": strings.Replace(builderTestPreflight, `"operation":"build"`, `"operation":"build","operation":"build"`, 1), "authorize": strings.Replace(builderTestPreflight, `false`, `true`, 1), "unknown": strings.Replace(builderTestPreflight, `"status":"indeterminate"`, `"status":"indeterminate","secret":"private"`, 1), "wrong-profile": strings.Replace(builderTestPreflight, `"gate"`, `"dual"`, 1), "inconsistent": strings.Replace(builderTestPreflight, `"indeterminate"`, `"ready"`, 1), "trailing": builderTestPreflight + "{}"} {
		t.Run(name, func(t *testing.T) {
			if _, e := builderProjection([]byte(raw), "preflight"); e == nil {
				t.Fatal("unsafe response accepted")
			}
		})
	}
	for _, status := range []int{302, 500} {
		up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "http://127.0.0.1:1")
			w.WriteHeader(status)
			_, _ = w.Write([]byte("secret"))
		}))
		c, _ := newBuilderHubClient(builderTestConfig(up.URL))
		if _, e := c.query(context.Background(), "alice", "preflight"); e == nil || strings.Contains(e.Error(), "secret") {
			t.Error("unsafe upstream result")
		}
		up.Close()
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(strings.Repeat("x", 65537))) }))
	defer up.Close()
	c, _ := newBuilderHubClient(builderTestConfig(up.URL))
	if _, e := c.query(context.Background(), "alice", "capabilities"); e == nil {
		t.Error("accepted oversized result")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := c.query(ctx, "alice", "capabilities"); e == nil {
		t.Error("accepted canceled query")
	}
}
