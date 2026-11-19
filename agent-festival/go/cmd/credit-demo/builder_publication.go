package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var errPublication = errors.New("publication unavailable")
var publicationDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var publicationIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// publicationReference is a private deployment reference, not an AgentCard extension.
type publicationReference struct {
	SchemaVersion       string          `json:"schema_version"`
	LogicalID           string          `json:"logical_id"`
	NativeAgentID       string          `json:"native_agent_id"`
	ArtifactSHA256      string          `json:"artifact_sha256"`
	CapabilityVersion   string          `json:"capability_version"`
	DeploymentVersion   string          `json:"deployment_version"`
	InstanceID          string          `json:"instance_id"`
	ContractVersion     string          `json:"contract_version"`
	ProtocolVersion     string          `json:"protocol_version"`
	CardDigest          string          `json:"card_digest"`
	ContractDigest      string          `json:"contract_digest"`
	Tenant              string          `json:"tenant"`
	SourceRevision      string          `json:"source_revision"`
	ManifestSHA256      string          `json:"manifest_sha256"`
	WheelSHA256         string          `json:"wheel_sha256"`
	AcceptanceSHA256    string          `json:"acceptance_sha256"`
	ClosureSHA256       string          `json:"closure_sha256"`
	OriginalHandoff     json.RawMessage `json:"original_handoff"`
	Address             string          `json:"address"`
	Port                int             `json:"port"`
	EndpointPath        string          `json:"endpoint_path"`
	AgentCard           map[string]any  `json:"agent_card"`
	Governance          string          `json:"governance"`
	ExecutionAuthorized bool            `json:"execution_authorized"`
}

type builderPublicationConfig struct {
	RegistryURL          string                 `json:"registry_url"`
	Tenant               string                 `json:"tenant"`
	AllowedUsers         []string               `json:"allowed_users"`
	Publications         []publicationReference `json:"publications"`
	RetainedPublications []publicationReference `json:"retained_publications,omitempty"`
	Token                string                 `json:"-"`
	// An optional current deployment-policy lookup permits revocation between reads.
	LookupUser func(string) bool `json:"-"`
}

type publicationView struct {
	LogicalID                     string `json:"logical_id"`
	NativeAgentID                 string `json:"native_agent_id"`
	ArtifactSHA256                string `json:"artifact_sha256"`
	CapabilityVersion             string `json:"capability_version"`
	DeploymentVersion             string `json:"deployment_version"`
	InstanceID                    string `json:"instance_id"`
	DeploymentVersionRegistration string `json:"deployment_version_registration"`
	ContractVersion               string `json:"contract_version"`
	ProtocolVersion               string `json:"protocol_version"`
	CardDigest                    string `json:"card_digest"`
	ContractDigest                string `json:"contract_digest"`
	SourceRevision                string `json:"source_revision"`
	Available                     bool   `json:"available"`
	Status                        string `json:"status"`
	Governance                    string `json:"governance"`
	ExecutionAuthorized           bool   `json:"execution_authorized"`
}

func publicationCanonical(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

// The card projection deliberately excludes addresses and does not invent version.
func publicationCardDigest(card map[string]any) (string, error) {
	v := map[string]any{}
	for _, k := range []string{"name", "description", "skills", "capabilities", "defaultInputModes", "defaultOutputModes"} {
		if x, ok := card[k]; ok {
			v[k] = x
		}
	}
	if x, ok := card["supportedInterfaces"]; ok {
		interfaces, ok := x.([]any)
		if !ok {
			return "", errPublication
		}
		out := []any{}
		for _, item := range interfaces {
			m, ok := item.(map[string]any)
			if !ok {
				return "", errPublication
			}
			n := map[string]any{}
			for _, k := range []string{"protocolBinding", "protocolVersion"} {
				if x, ok := m[k]; ok {
					n[k] = x
				}
			}
			out = append(out, n)
		}
		v["supportedInterfaces"] = out
	}
	raw, err := publicationCanonical(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func publicationNativeID(v string) bool {
	if len(v) < 1 || len(v) > 64 {
		return false
	}
	for _, c := range []byte(v) {
		if c < 33 || c > 126 || strings.ContainsRune("/?#%\\", rune(c)) {
			return false
		}
	}
	return true
}

func (r publicationReference) valid(tenant string) bool {
	if r.SchemaVersion != "publication-reference/v2" || !publicationIdentifier.MatchString(r.LogicalID) || !publicationNativeID(r.NativeAgentID) || !publicationIdentifier.MatchString(r.CapabilityVersion) || len(r.CapabilityVersion) > 16 || !publicationIdentifier.MatchString(r.DeploymentVersion) || len(r.DeploymentVersion) > 32 || !publicationIdentifier.MatchString(r.InstanceID) || r.Tenant != tenant || r.ContractVersion != "1.0.0" || r.ProtocolVersion != "1.0" || r.Governance != "unverified" || r.ExecutionAuthorized {
		return false
	}
	for _, d := range []string{r.ArtifactSHA256, r.CardDigest, r.ContractDigest, r.ManifestSHA256, r.WheelSHA256, r.AcceptanceSHA256, r.ClosureSHA256} {
		if !publicationDigestPattern.MatchString(d) {
			return false
		}
	}
	if len(r.SourceRevision) != 40 || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(r.SourceRevision) {
		return false
	}
	if r.ContractDigest != "f43c1b171d43cfb975684ed244bbb09ddd11308671f5d79fb268d2cf0c703c61" || r.Address != "127.0.0.1" || r.Port < 1024 || r.Port > 65535 || r.EndpointPath != "/a2a/jsonrpc/" {
		return false
	}
	var binding struct {
		Status    string `json:"status"`
		Candidate struct {
			SHA string `json:"sha256"`
		} `json:"candidate_ref"`
	}
	if json.Unmarshal(r.OriginalHandoff, &binding) != nil || binding.Status != "bound_to_original_projections" || binding.Candidate.SHA != r.ArtifactSHA256 {
		return false
	}
	name, _ := r.AgentCard["name"].(string)
	interfaces, ok := r.AgentCard["supportedInterfaces"].([]any)
	if !ok || len(interfaces) == 0 {
		return false
	}
	for _, x := range interfaces {
		m, ok := x.(map[string]any)
		if !ok || m["protocolVersion"] != r.ProtocolVersion || m["url"] != "http://127.0.0.1:"+strconv.Itoa(r.Port)+"/a2a/jsonrpc/" {
			return false
		}
	}
	digest, e := publicationCardDigest(r.AgentCard)
	return name == r.NativeAgentID && e == nil && digest == r.CardDigest && publicationProtocol(r.AgentCard, r.ProtocolVersion)
}

func publicationProtocol(card map[string]any, version string) bool {
	xs, ok := card["supportedInterfaces"].([]any)
	if !ok {
		return false
	}
	for _, x := range xs {
		if m, ok := x.(map[string]any); ok && m["protocolVersion"] == version && m["protocolBinding"] == "JSONRPC" {
			return true
		}
	}
	return false
}

func publicationLocalURL(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	return ip != nil && ip.IsLoopback() && u.Port() != ""
}

func (c *builderPublicationConfig) validate() error {
	if c == nil {
		return nil
	}
	if !publicationLocalURL(c.RegistryURL) || !publicationIdentifier.MatchString(c.Tenant) || len(c.Token) < 16 || strings.ContainsAny(c.Token, "\r\n") || len(c.Publications) < 1 || len(c.Publications) > 8 || len(c.RetainedPublications) > 32 || len(c.AllowedUsers) < 1 || len(c.AllowedUsers) > 64 {
		return errPublication
	}
	seen := map[string]bool{}
	for _, r := range c.Publications {
		if !r.valid(c.Tenant) || seen[r.LogicalID] {
			return errPublication
		}
		seen[r.LogicalID] = true
	}
	bindings := append(append([]publicationReference{}, c.Publications...), c.RetainedPublications...)
	for _, r := range c.RetainedPublications {
		if !r.valid(c.Tenant) {
			return errPublication
		}
	}
	if !publicationBindingsValid(bindings) {
		return errPublication
	}
	for _, u := range c.AllowedUsers {
		if _, ok := users[u]; !ok {
			return errPublication
		}
	}
	return nil
}

// The explicit retained set only detects conflicts; reference() never grants it reads.
func publicationBindingsValid(refs []publicationReference) bool {
	type capabilityKey struct{ Tenant, Native, Capability string }
	type instanceKey struct{ Tenant, Instance string }
	type instanceValue struct {
		Native, Artifact, Capability, Deployment, Contract, Protocol, Address string
		Port                                                                  int
		Path, CardDigest, ContractDigest                                      string
	}
	capabilities := map[capabilityKey]string{}
	instances := map[instanceKey]instanceValue{}
	for _, r := range refs {
		ck := capabilityKey{r.Tenant, r.NativeAgentID, r.CapabilityVersion}
		if previous, ok := capabilities[ck]; ok && previous != r.ArtifactSHA256 {
			return false
		}
		capabilities[ck] = r.ArtifactSHA256
		ik := instanceKey{r.Tenant, r.InstanceID}
		iv := instanceValue{r.NativeAgentID, r.ArtifactSHA256, r.CapabilityVersion, r.DeploymentVersion, r.ContractVersion, r.ProtocolVersion, r.Address, r.Port, r.EndpointPath, r.CardDigest, r.ContractDigest}
		if previous, ok := instances[ik]; ok && previous != iv {
			return false
		}
		instances[ik] = iv
	}
	return true
}

func builderPublicationFromEnv() (*builderPublicationConfig, error) {
	enabled := os.Getenv("BUILDER_PUBLICATION_ENABLED")
	if enabled == "" {
		return nil, nil
	}
	password := os.Getenv("CREDIT_DEMO_PASSWORD")
	if enabled != "1" || strings.TrimSpace(password) == "" || password == "demo" {
		return nil, errPublication
	}
	path := os.Getenv("BUILDER_PUBLICATION_CONFIG")
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Size() > 131072 {
		return nil, errPublication
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		return nil, errPublication
	}
	parsed, e := builderJSON(raw)
	if e != nil {
		return nil, errPublication
	}
	refs, ok := parsed["publications"].([]any)
	if !ok {
		return nil, errPublication
	}
	if retained, exists := parsed["retained_publications"]; exists {
		items, ok := retained.([]any)
		if !ok || len(items) > 32 {
			return nil, errPublication
		}
		refs = append(refs, items...)
	}
	for _, item := range refs {
		m, ok := item.(map[string]any)
		if !ok || len(m) != 24 {
			return nil, errPublication
		}
		for _, key := range []string{"schema_version", "logical_id", "native_agent_id", "artifact_sha256", "capability_version", "deployment_version", "instance_id", "contract_version", "protocol_version", "card_digest", "contract_digest", "tenant", "source_revision", "manifest_sha256", "wheel_sha256", "acceptance_sha256", "closure_sha256", "original_handoff", "address", "port", "endpoint_path", "agent_card", "governance", "execution_authorized"} {
			if _, ok := m[key]; !ok {
				return nil, errPublication
			}
		}
		if m["execution_authorized"] != false {
			return nil, errPublication
		}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var c builderPublicationConfig
	if dec.Decode(&c) != nil || dec.Decode(new(any)) != io.EOF {
		return nil, errPublication
	}
	c.Token = os.Getenv("BUILDER_PUBLICATION_RDC_TOKEN")
	if e = c.validate(); e != nil {
		return nil, e
	}
	return &c, nil
}

func (c *builderPublicationConfig) reference(id string) (publicationReference, bool) {
	for _, r := range c.Publications {
		if r.LogicalID == id {
			return r, true
		}
	}
	return publicationReference{}, false
}

func publicationGet(ctx context.Context, c *builderPublicationConfig, id, suffix string, out any) error {
	req, e := http.NewRequestWithContext(ctx, "GET", c.RegistryURL+"/v2/agents/"+url.PathEscape(id)+"/"+suffix, nil)
	if e != nil {
		return errPublication
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("X-Tenant-ID", c.Tenant)
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, e := client.Do(req)
	if e != nil {
		return errPublication
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errPublication
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if e != nil || len(raw) > 1<<20 {
		return errPublication
	}
	if _, e = builderJSON(raw); e != nil {
		return errPublication
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if d.Decode(out) != nil || d.Decode(new(any)) != io.EOF {
		return errPublication
	}
	return nil
}

// observePublication accepts the saved reference, never resolves it through a new alias.
func observePublication(ctx context.Context, c *builderPublicationConfig, r publicationReference) (publicationView, error) {
	v := publicationView{LogicalID: r.LogicalID, NativeAgentID: r.NativeAgentID, ArtifactSHA256: r.ArtifactSHA256, CapabilityVersion: r.CapabilityVersion, DeploymentVersion: r.DeploymentVersion, InstanceID: r.InstanceID, DeploymentVersionRegistration: "declared-only", ContractVersion: r.ContractVersion, ProtocolVersion: r.ProtocolVersion, CardDigest: r.CardDigest, ContractDigest: r.ContractDigest, SourceRevision: r.SourceRevision, Status: "indeterminate", Governance: "unverified"}
	if c == nil || c.validate() != nil || !r.valid(c.Tenant) {
		return v, errPublication
	}
	bindings := append(append(append([]publicationReference{}, c.Publications...), c.RetainedPublications...), r)
	if !publicationBindingsValid(bindings) {
		return v, errPublication
	}
	var card map[string]any
	if publicationGet(ctx, c, r.NativeAgentID, "card", &card) != nil {
		return v, errPublication
	}
	for _, k := range []string{"rawJson", "agentCardJson", "agentCard", "card"} {
		if raw, ok := card[k].(string); ok {
			if json.Unmarshal([]byte(raw), &card) != nil {
				return v, errPublication
			}
			break
		}
		if nested, ok := card[k].(map[string]any); ok {
			card = nested
			break
		}
	}
	digest, e := publicationCardDigest(card)
	if e != nil || digest != r.CardDigest || card["name"] != r.NativeAgentID || !publicationProtocol(card, r.ProtocolVersion) {
		return v, errPublication
	}
	var discovery struct {
		AgentID   string `json:"agentId"`
		Instances []struct {
			ServiceID         string `json:"serviceId"`
			InstanceID        string `json:"instanceId"`
			Health            string `json:"health"`
			ContractVersion   string `json:"contractVersion"`
			CapabilityVersion string `json:"capabilityVersion"`
		} `json:"instances"`
	}
	if publicationGet(ctx, c, r.NativeAgentID, "instances", &discovery) != nil || discovery.AgentID != r.NativeAgentID {
		return v, errPublication
	}
	for _, instance := range discovery.Instances {
		if instance.InstanceID == r.InstanceID && instance.ServiceID == r.InstanceID && instance.Health == "ONLINE" && instance.ContractVersion == r.ContractVersion && instance.CapabilityVersion == r.CapabilityVersion {
			v.Available = true
			v.Status = "registered-source-matched"
			return v, nil
		}
	}
	return v, errPublication
}
