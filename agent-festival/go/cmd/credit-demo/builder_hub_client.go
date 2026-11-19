package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const builderContextHeader = "X-Builder-Hub-Context"
const builderPrefix = "/api/v1/builder-hub/"

var builderIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var builderToken = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,119}$`)
var builderDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)
var builderRevision = regexp.MustCompile(`^[0-9a-f]{40}$`)
var errBuilderHub = errors.New("builder query unavailable")

// This configuration is deliberately separate from persistent Service.State.
type builderHubConfig struct {
	Target, ChannelToken, Issuer, KeyID, Audience, Scope string
	Key                                                  []byte
	LookupSubject                                        func(string) (string, bool)
}
type builderHubClient struct {
	cfg  builderHubConfig
	http *http.Client
	now  func() time.Time
}

func newBuilderHubClient(cfg builderHubConfig) (*builderHubClient, error) {
	u, e := url.Parse(cfg.Target)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.RawPath != "" {
		return nil, errBuilderHub
	}
	ip := net.ParseIP(u.Hostname())
	if u.Scheme != "https" && !(u.Scheme == "http" && ip != nil && ip.IsLoopback()) {
		return nil, errBuilderHub
	}
	if len(cfg.Key) < 32 || len(cfg.Key) > 128 || len(cfg.ChannelToken) < 16 || len(cfg.ChannelToken) > 4096 || strings.ContainsAny(cfg.ChannelToken, "\r\n") || bytes.Equal(cfg.Key, []byte(cfg.ChannelToken)) || cfg.LookupSubject == nil {
		return nil, errBuilderHub
	}
	for _, s := range []string{cfg.Issuer, cfg.KeyID, cfg.Audience, cfg.Scope} {
		if !builderIdentifier.MatchString(s) {
			return nil, errBuilderHub
		}
	}
	cfg.Key = bytes.Clone(cfg.Key)
	cfg.Target = strings.TrimSuffix(cfg.Target, "/")
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &builderHubClient{cfg: cfg, http: &http.Client{Timeout: 10 * time.Second, Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, now: time.Now}, nil
}

func builderProof(key []byte, claims map[string]any) (string, error) {
	// All strings are restricted ASCII identifiers, fixed paths or fixed hashes.
	// Go's sorted map keys match the Python canonical byte contract exactly.
	raw, e := json.Marshal(claims)
	if e != nil {
		return "", errBuilderHub
	}
	m := hmac.New(sha256.New, key)
	m.Write([]byte("builder-hub-query-context/v1\n"))
	m.Write(raw)
	proof := base64.RawURLEncoding.EncodeToString(raw) + "." + hex.EncodeToString(m.Sum(nil))
	if len(proof) > 4096 {
		return "", errBuilderHub
	}
	return proof, nil
}

func (c *builderHubClient) query(ctx context.Context, subject, action string) ([]byte, error) {
	if !builderIdentifier.MatchString(subject) || (action != "capabilities" && action != "preflight") {
		return nil, errBuilderHub
	}
	method := http.MethodGet
	var body []byte
	if action == "preflight" {
		method = http.MethodPost
		body = []byte(`{"schema_version":"builder-hub/v1","scenario":{"id":"intent","version":"1.0","profile":"gate"},"operation":"build"}`)
	}
	path := builderPrefix + action
	digest := sha256.Sum256(body)
	now := c.now().Unix()
	id := make([]byte, 16)
	if _, e := rand.Read(id); e != nil {
		return nil, errBuilderHub
	}
	claims := map[string]any{"version": "builder-hub-context/v1", "issuer": c.cfg.Issuer, "key_id": c.cfg.KeyID, "audience": c.cfg.Audience, "subject": subject, "scope": c.cfg.Scope, "action": action, "method": method, "path": path, "body_sha256": hex.EncodeToString(digest[:]), "issued_at": now, "expires_at": now + 30, "request_id": hex.EncodeToString(id)}
	proof, e := builderProof(c.cfg.Key, claims)
	if e != nil {
		return nil, errBuilderHub
	}
	req, e := http.NewRequestWithContext(ctx, method, c.cfg.Target+path, bytes.NewReader(body))
	if e != nil {
		return nil, errBuilderHub
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.ChannelToken)
	req.Header.Set(builderContextHeader, proof)
	req.Header.Set("Content-Type", "application/json")
	resp, e := c.http.Do(req)
	if e != nil {
		return nil, errBuilderHub
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errBuilderHub
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if e != nil || len(raw) > 65536 || c.now().Unix() >= now+30 {
		return nil, errBuilderHub
	}
	return builderProjection(raw, action)
}

// Parse JSON without accepting duplicate keys, trailing documents or deep trees.
func builderJSON(raw []byte) (map[string]any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) (any, error)
	value = func(depth int) (any, error) {
		if depth > 12 {
			return nil, errBuilderHub
		}
		t, e := d.Token()
		if e != nil {
			return nil, e
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				m := map[string]any{}
				for d.More() {
					k, e := d.Token()
					if e != nil {
						return nil, e
					}
					s, ok := k.(string)
					if !ok {
						return nil, errBuilderHub
					}
					if _, exists := m[s]; exists {
						return nil, errBuilderHub
					}
					v, e := value(depth + 1)
					if e != nil {
						return nil, e
					}
					m[s] = v
				}
				_, e = d.Token()
				return m, e
			case '[':
				a := []any{}
				for d.More() {
					v, e := value(depth + 1)
					if e != nil {
						return nil, e
					}
					a = append(a, v)
					if len(a) > 64 {
						return nil, errBuilderHub
					}
				}
				_, e = d.Token()
				return a, e
			default:
				return nil, errBuilderHub
			}
		}
		return t, nil
	}
	v, e := value(0)
	if e != nil {
		return nil, errBuilderHub
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, errBuilderHub
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errBuilderHub
	}
	return m, nil
}
func builderKeys(m map[string]any, keys ...string) bool {
	if len(m) != len(keys) {
		return false
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}
func builderOne(v any, values ...string) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	for _, x := range values {
		if x == s {
			return true
		}
	}
	return false
}
func builderScenario(v any) bool {
	m, ok := v.(map[string]any)
	return ok && builderKeys(m, "id", "version", "profile") && m["id"] == "intent" && m["version"] == "1.0" && m["profile"] == "gate"
}
func builderReason(v any) bool {
	return builderOne(v, "missing-environment", "missing-asset", "missing-data", "missing-confirmation", "missing-parent", "source-unverified", "unauthorized", "dependency-unavailable", "unsupported", "stale-reference")
}
func builderQualification(v any) (bool, bool) {
	m, ok := v.(map[string]any)
	if !ok || !builderKeys(m, "scope", "result", "source_revision", "evidence_ref", "environment_digest") {
		return false, false
	}
	if !builderOne(m["scope"], "engineering", "rule-replay", "controlled-evaluation", "hub-integration", "business-quality") || !builderOne(m["result"], "not_run", "passed", "failed", "blocked", "indeterminate") {
		return false, false
	}
	revision, rok := m["source_revision"].(string)
	digest, dok := m["environment_digest"].(string)
	ref, refok := m["evidence_ref"].(map[string]any)
	validRef := refok && builderKeys(ref, "id", "sha256")
	if refok {
		id, iok := ref["id"].(string)
		h, hok := ref["sha256"].(string)
		validRef = validRef && iok && hok && builderToken.MatchString(id) && builderDigest.MatchString(h)
	}
	bound := rok && builderRevision.MatchString(revision) && dok && builderDigest.MatchString(digest) && validRef
	if m["source_revision"] != nil && (!rok || !builderRevision.MatchString(revision)) || m["environment_digest"] != nil && (!dok || !builderDigest.MatchString(digest)) || m["evidence_ref"] != nil && !validRef {
		return false, false
	}
	if m["result"] == "passed" && !bound {
		return false, false
	}
	return true, m["scope"] == "hub-integration" && m["result"] == "passed"
}
func builderProjection(raw []byte, action string) ([]byte, error) {
	m, e := builderJSON(raw)
	if e != nil || m["schema_version"] != "builder-hub/v1" {
		return nil, errBuilderHub
	}
	if action == "preflight" {
		if !builderKeys(m, "schema_version", "scenario", "operation", "status", "prerequisites", "execution_authorized") || !builderScenario(m["scenario"]) || m["operation"] != "build" || m["execution_authorized"] != false {
			return nil, errBuilderHub
		}
		arr, ok := m["prerequisites"].([]any)
		if !ok || len(arr) > 32 {
			return nil, errBuilderHub
		}
		expected := "ready"
		for _, v := range arr {
			p, ok := v.(map[string]any)
			if !ok || !builderKeys(p, "code", "status") || !builderReason(p["code"]) || !builderOne(p["status"], "missing", "unknown") {
				return nil, errBuilderHub
			}
			if p["status"] == "unknown" {
				expected = "indeterminate"
			} else if expected == "ready" {
				expected = "blocked"
			}
		}
		if m["status"] != expected {
			return nil, errBuilderHub
		}
	} else {
		if !builderKeys(m, "schema_version", "capabilities") {
			return nil, errBuilderHub
		}
		arr, ok := m["capabilities"].([]any)
		if !ok || len(arr) != 1 {
			return nil, errBuilderHub
		}
		c, ok := arr[0].(map[string]any)
		if !ok || !builderKeys(c, "scenario", "steps") || !builderScenario(c["scenario"]) {
			return nil, errBuilderHub
		}
		steps, ok := c["steps"].([]any)
		if !ok || len(steps) != 6 {
			return nil, errBuilderHub
		}
		seen := map[string]bool{}
		for _, v := range steps {
			s, ok := v.(map[string]any)
			if !ok || !builderKeys(s, "step", "implementation", "integration", "qualifications", "constraints") || !builderOne(s["step"], "build", "observe", "evaluate", "analyze", "optimize", "reevaluate") || !builderOne(s["implementation"], "supported", "unsupported", "unknown") || !builderOne(s["integration"], "not_connected", "contract_checked", "hub_verified") {
				return nil, errBuilderHub
			}
			name := s["step"].(string)
			if seen[name] {
				return nil, errBuilderHub
			}
			seen[name] = true
			qs, qok := s["qualifications"].([]any)
			cs, cok := s["constraints"].([]any)
			if !qok || !cok || len(qs) > 16 || len(cs) > 16 {
				return nil, errBuilderHub
			}
			hubQualified := false
			for _, q := range qs {
				valid, hq := builderQualification(q)
				if !valid {
					return nil, errBuilderHub
				}
				hubQualified = hubQualified || hq
			}
			for _, reason := range cs {
				if !builderReason(reason) {
					return nil, errBuilderHub
				}
			}
			if s["integration"] != "not_connected" && s["implementation"] != "supported" || s["integration"] == "hub_verified" && !hubQualified {
				return nil, errBuilderHub
			}
		}
	}
	out, e := json.Marshal(m)
	if e != nil {
		return nil, errBuilderHub
	}
	return out, nil
}
