package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func publicationTestRef() publicationReference {
	r := publicationReference{SchemaVersion: "publication-reference/v2", CapabilityVersion: "1.0.0", DeploymentVersion: "experiment-a", InstanceID: "owned-a", LogicalID: "intent-gate", NativeAgentID: "candidate-a", ArtifactSHA256: strings.Repeat("a", 64), ContractVersion: "1.0.0", ProtocolVersion: "1.0", ContractDigest: "f43c1b171d43cfb975684ed244bbb09ddd11308671f5d79fb268d2cf0c703c61", Tenant: "h02b", SourceRevision: strings.Repeat("f", 40), ManifestSHA256: strings.Repeat("b", 64), WheelSHA256: strings.Repeat("d", 64), AcceptanceSHA256: strings.Repeat("e", 64), ClosureSHA256: strings.Repeat("f", 64), OriginalHandoff: json.RawMessage(`{"status":"bound_to_original_projections","candidate_ref":{"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`), Address: "127.0.0.1", Port: 9999, EndpointPath: "/a2a/jsonrpc/", Governance: "unverified", AgentCard: map[string]any{"name": "candidate-a", "description": "查余额<&>", "supportedInterfaces": []any{map[string]any{"protocolBinding": "JSONRPC", "protocolVersion": "1.0", "url": "http://127.0.0.1:9999/a2a/jsonrpc/"}}}}
	r.CardDigest, _ = publicationCardDigest(r.AgentCard)
	return r
}
func publicationTestConfig(u string, r publicationReference) *builderPublicationConfig {
	return &builderPublicationConfig{RegistryURL: u, Tenant: "h02b", Token: strings.Repeat("s", 32), AllowedUsers: []string{"alice"}, Publications: []publicationReference{r}}
}
func publicationTestUpstream(t *testing.T, r publicationReference, mutate func(string, map[string]any), hook func()) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != "GET" || req.Header.Get("Authorization") != "Bearer "+strings.Repeat("s", 32) || req.Header.Get("X-Tenant-ID") != "h02b" {
			t.Error("wrong request authority")
		}
		if !strings.Contains(req.URL.Path, "/"+r.NativeAgentID+"/") {
			t.Error("alias replaced original native ID")
		}
		var out map[string]any
		if strings.HasSuffix(req.URL.Path, "/card") {
			raw, _ := json.Marshal(r.AgentCard)
			_ = json.Unmarshal(raw, &out)
		} else {
			out = map[string]any{"agentId": r.NativeAgentID, "instances": []any{map[string]any{"instanceId": r.InstanceID, "serviceId": r.InstanceID, "health": "ONLINE", "contractVersion": "1.0.0", "capabilityVersion": r.CapabilityVersion}}}
		}
		if mutate != nil {
			mutate(req.URL.Path, out)
		}
		if hook != nil {
			hook()
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
}

func TestBuilderPublicationDiscoveryAndOldBinding(t *testing.T) {
	a := publicationTestRef()
	up := publicationTestUpstream(t, a, nil, nil)
	defer up.Close()
	c := publicationTestConfig(up.URL, a)
	if c.validate() != nil {
		t.Fatal("valid config rejected")
	}
	v, e := observePublication(context.Background(), c, a)
	if e != nil || !v.Available || v.ExecutionAuthorized || v.Governance != "unverified" {
		t.Fatalf("observation failed: %+v %v", v, e)
	}
	b := publicationTestRef()
	b.InstanceID = "owned-b"
	b.CapabilityVersion = "1.0.1"
	b.DeploymentVersion = "experiment-b"
	b.NativeAgentID = "candidate-b"
	b.ArtifactSHA256 = strings.Repeat("b", 64)
	b.AgentCard["name"] = b.NativeAgentID
	b.CardDigest, _ = publicationCardDigest(b.AgentCard)
	b.OriginalHandoff = json.RawMessage(strings.ReplaceAll(string(b.OriginalHandoff), strings.Repeat("a", 64), b.ArtifactSHA256))
	c.Publications = []publicationReference{b}
	c.RetainedPublications = []publicationReference{a}
	// An already adopted A reference is read directly, never through the mutable alias.
	v, e = observePublication(context.Background(), c, a)
	if e != nil || v.NativeAgentID != "candidate-a" {
		t.Fatal("old binding changed")
	}
}
func TestBuilderPublicationMismatch(t *testing.T) {
	for _, kind := range []string{"card", "version", "contract", "health", "instance", "service", "agent"} {
		t.Run(kind, func(t *testing.T) {
			r := publicationTestRef()
			up := publicationTestUpstream(t, r, func(path string, m map[string]any) {
				if strings.HasSuffix(path, "/card") {
					if kind == "card" {
						m["name"] = "other"
					}
					return
				}
				instance := m["instances"].([]any)[0].(map[string]any)
				switch kind {
				case "version":
					instance["capabilityVersion"] = strings.Repeat("0", 64)
				case "contract":
					instance["contractVersion"] = "1.0"
				case "health":
					instance["health"] = "OFFLINE"
				case "instance":
					instance["instanceId"] = "another-nonempty-instance"
				case "service":
					instance["serviceId"] = r.NativeAgentID
				case "agent":
					m["agentId"] = "other"
				}
			}, nil)
			defer up.Close()
			v, e := observePublication(context.Background(), publicationTestConfig(up.URL, r), r)
			if e == nil || v.Available {
				t.Fatal("mismatch accepted")
			}
		})
	}
}
func TestBuilderPublicationConfigBoundary(t *testing.T) {
	r := publicationTestRef()
	c := publicationTestConfig("http://127.0.0.1:12345", r)
	for _, u := range []string{"http://example.com:1234", "http://127.0.0.1:1234/path", "http://user:password@127.0.0.1:1234", "http://127.0.0.1:1234?x=1", "https://127.0.0.1:1234"} {
		c.RegistryURL = u
		if c.validate() == nil {
			t.Fatalf("unsafe URL %s", u)
		}
	}
	c.RegistryURL = "http://127.0.0.1:1234"
	c.Publications = append(c.Publications, r)
	if c.validate() == nil {
		t.Fatal("duplicate alias")
	}
	t.Setenv("BUILDER_PUBLICATION_ENABLED", "")
	if got, e := builderPublicationFromEnv(); e != nil || got != nil {
		t.Fatal("default enabled")
	}
	t.Setenv("BUILDER_PUBLICATION_ENABLED", "1")
	t.Setenv("CREDIT_DEMO_PASSWORD", "demo")
	if _, e := builderPublicationFromEnv(); e == nil {
		t.Fatal("implicit demo password")
	}
	t.Setenv("CREDIT_DEMO_PASSWORD", "explicit-password")
	t.Setenv("BUILDER_PUBLICATION_RDC_TOKEN", strings.Repeat("s", 32))
	c.Publications = c.Publications[:1]
	path := filepath.Join(t.TempDir(), "config.json")
	raw, _ := json.Marshal(c)
	_ = os.WriteFile(path, raw, 0600)
	t.Setenv("BUILDER_PUBLICATION_CONFIG", path)
	if _, e := builderPublicationFromEnv(); e != nil {
		t.Fatal(e)
	}
	_ = os.WriteFile(path, []byte(`{"tenant":"one","tenant":"two"}`), 0600)
	if _, e := builderPublicationFromEnv(); e == nil {
		t.Fatal("duplicate fields")
	}
}

func TestBuilderPublicationCardProjection(t *testing.T) {
	r := publicationTestRef()
	r.AgentCard["version"] = "untrusted"
	first, _ := publicationCardDigest(r.AgentCard)
	r.AgentCard["supportedInterfaces"].([]any)[0].(map[string]any)["url"] = "http://127.0.0.1:9998/a2a"
	second, _ := publicationCardDigest(r.AgentCard)
	if first != second || first != r.CardDigest {
		t.Fatal("deployment address affected semantic digest")
	}
}

func TestBuilderPublicationCrossLanguageFixture(t *testing.T) {
	raw, e := os.ReadFile("testdata/publication-reference-v2.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Descriptor     publicationReference `json:"descriptor"`
		CanonicalCard  string               `json:"canonical_card"`
		Digest         string               `json:"expected_card_digest"`
		Contract       map[string]any       `json:"contract"`
		ContractDigest string               `json:"expected_contract_digest"`
	}
	if json.Unmarshal(raw, &f) != nil || !f.Descriptor.valid(f.Descriptor.Tenant) {
		t.Fatal("invalid shared fixture")
	}
	got, e := publicationCardDigest(f.Descriptor.AgentCard)
	if e != nil || got != f.Digest {
		t.Fatal("cross-language card digest mismatch")
	}
	var semantic map[string]any
	if json.Unmarshal([]byte(f.CanonicalCard), &semantic) != nil {
		t.Fatal("invalid canonical card")
	}
	b, e := publicationCanonical(semantic)
	if e != nil || string(b) != f.CanonicalCard {
		t.Fatal("canonical representation mismatch")
	}
	if f.ContractDigest != f.Descriptor.ContractDigest {
		t.Fatal("contract mismatch")
	}
}

func TestBuilderPublicationV2BoundsAndHistoricalRejection(t *testing.T) {
	r := publicationTestRef()
	for _, mutate := range []func(*publicationReference){
		func(v *publicationReference) { v.SchemaVersion = "publication-reference/v1" },
		func(v *publicationReference) { v.CapabilityVersion = "" },
		func(v *publicationReference) { v.CapabilityVersion = strings.Repeat("a", 17) },
		func(v *publicationReference) { v.DeploymentVersion = strings.Repeat("a", 33) },
		func(v *publicationReference) { v.InstanceID = strings.Repeat("a", 65) },
		func(v *publicationReference) { v.InstanceID = "a/b" },
		func(v *publicationReference) { v.CapabilityVersion = "业务" },
		func(v *publicationReference) { v.DeploymentVersion = "a b" },
	} {
		bad := r
		mutate(&bad)
		if bad.valid(r.Tenant) {
			t.Fatal("invalid explicit v2 binding accepted")
		}
	}
	raw, err := os.ReadFile("testdata/publication-reference-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var historical struct {
		Descriptor publicationReference `json:"descriptor"`
	}
	if json.Unmarshal(raw, &historical) != nil || historical.Descriptor.valid(historical.Descriptor.Tenant) {
		t.Fatal("historical v1 accepted")
	}
}

func TestBuilderPublicationRetainedConflictsBeforeIO(t *testing.T) {
	r := publicationTestRef()
	calls := 0
	up := publicationTestUpstream(t, r, nil, func() { calls++ })
	defer up.Close()
	for _, kind := range []string{"capability", "instance"} {
		other := publicationTestRef()
		if kind == "capability" {
			other.ArtifactSHA256 = strings.Repeat("b", 64)
			other.OriginalHandoff = json.RawMessage(strings.ReplaceAll(string(other.OriginalHandoff), strings.Repeat("a", 64), other.ArtifactSHA256))
		} else {
			other.Port++
			other.AgentCard["supportedInterfaces"].([]any)[0].(map[string]any)["url"] = "http://127.0.0.1:10000/a2a/jsonrpc/"
		}
		c := publicationTestConfig(up.URL, r)
		c.RetainedPublications = []publicationReference{other}
		if c.validate() == nil {
			t.Fatal("retained conflict accepted")
		}
		if v, e := observePublication(context.Background(), c, r); e == nil || v.Available || calls != 0 {
			t.Fatal("conflict reached registry")
		}
	}
	c := publicationTestConfig(up.URL, r)
	c.RetainedPublications = []publicationReference{r, r}
	if c.validate() != nil {
		t.Fatal("identical retained bindings rejected")
	}
	c.RetainedPublications = make([]publicationReference, 33)
	if c.validate() == nil {
		t.Fatal("unbounded retained set accepted")
	}
}

func TestBuilderPublicationV2StrictConfigFields(t *testing.T) {
	t.Setenv("BUILDER_PUBLICATION_ENABLED", "1")
	t.Setenv("CREDIT_DEMO_PASSWORD", "explicit-password")
	t.Setenv("BUILDER_PUBLICATION_RDC_TOKEN", strings.Repeat("s", 32))
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("BUILDER_PUBLICATION_CONFIG", path)
	for _, mode := range []string{"v1", "missing", "extra", "retained-type"} {
		raw, _ := json.Marshal(publicationTestConfig("http://127.0.0.1:12345", publicationTestRef()))
		var value map[string]any
		_ = json.Unmarshal(raw, &value)
		ref := value["publications"].([]any)[0].(map[string]any)
		switch mode {
		case "v1":
			ref["schema_version"] = "publication-reference/v1"
		case "missing":
			delete(ref, "instance_id")
		case "extra":
			ref["unreviewed_default"] = "x"
		case "retained-type":
			value["retained_publications"] = "not-an-array"
		}
		raw, _ = json.Marshal(value)
		_ = os.WriteFile(path, raw, 0600)
		if _, e := builderPublicationFromEnv(); e == nil {
			t.Fatalf("bad %s config accepted", mode)
		}
	}
}
