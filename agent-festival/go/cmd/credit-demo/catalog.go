package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-jose/go-jose/v4"
	"github.com/openjiuwen/agent-festival/go/internal/rfc8785"
	"github.com/openjiuwen/agent-festival/go/internal/validate"
)

// Expert is one registered capability x_i. The contract is the festival L3
// capability contract; the member fields tell the executor how to run it.
type Expert struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind"` // investigate | blackbox | judge | write | effect
	Summary     string   `json:"summary"`
	Tools       []string `json:"tools,omitempty"`
	Prompt      string   `json:"-"`
	MaxIter     int      `json:"-"`
	Package     string   `json:"package,omitempty"` // scenario package that provides it
	requires    []grant
	effectLevel string
	compensate  string
	costCents   int
	calls       int
	seconds     int
}

type grant struct {
	kind, selector string
	actions        []string
	effect         string
	purpose        string
}

func (g grant) doc() map[string]any {
	m := map[string]any{
		"resource": map[string]any{"kind": g.kind, "selector": g.selector},
		"actions":  toAny(g.actions),
		"effect":   g.effect,
	}
	if g.purpose != "" {
		m["purpose"] = g.purpose
	}
	return m
}

// experts (the catalog) is derived from the approved scenario packages; see
// scenario.go. The credit due-diligence experts live in scenarios/credit-dd.json.

func expertByID(id string) *Expert {
	for _, e := range experts {
		if e.ID == id {
			return e
		}
	}
	return nil
}

// contract renders the festival L3 capability contract of an expert.
func (e *Expert) contract() map[string]any {
	reqs := make([]any, 0, len(e.requires))
	for _, g := range e.requires {
		reqs = append(reqs, g.doc())
	}
	effects := map[string]any{"level": e.effectLevel}
	if e.compensate != "" {
		effects["compensation"] = e.compensate
	}
	return jsonDoc(map[string]any{
		"contract_version": "0.1.0",
		"subject":          map[string]any{"id": e.ID, "version": "1.0.0"},
		"provides":         []any{map[string]any{"skill_id": e.ID, "description": e.Summary}},
		"requires":         reqs,
		"effects":          effects,
		"bounds": map[string]any{
			"max_cost":             map[string]any{"amount": e.costCents, "unit": "USD-cents"},
			"max_seconds":          e.seconds,
			"max_downstream_calls": e.calls,
		},
	})
}

// Catalog holds the signed envelopes and the trust store that verifies them.
type Catalog struct {
	Envelopes map[string]map[string]any
	Hashes    map[string]string
	trust     *validate.TrustStore
	kid       string
}

// LoadCatalog signs every contract with the publisher key (created on first
// start) and checks each envelope with the festival validator before use.
func LoadCatalog(dir string) (*Catalog, error) {
	key, err := loadOrCreateKey(filepath.Join(dir, "publisher.pem"))
	if err != nil {
		return nil, err
	}
	kid := "credit-demo-publisher"
	subjects := []any{}
	for _, e := range experts {
		subjects = append(subjects, e.ID)
	}
	jwks := map[string]any{
		"keys":       []any{publicJWK(key, kid)},
		"subject_of": map[string]any{kid: subjects},
	}
	jwksPath := filepath.Join(dir, "trusted.jwks.json")
	if err := writeJSON(jwksPath, jwks); err != nil {
		return nil, err
	}
	ts, err := validate.LoadTrustStore(jwksPath)
	if err != nil {
		return nil, fmt.Errorf("trust store: %w", err)
	}
	c := &Catalog{Envelopes: map[string]map[string]any{}, Hashes: map[string]string{}, trust: ts, kid: kid}
	for _, e := range experts {
		doc := e.contract()
		if errs := validate.ContractErrors(doc); len(errs) > 0 {
			return nil, fmt.Errorf("contract %s rejected: %s", e.ID, strings.Join(errs, "; "))
		}
		env, err := signEnvelope(doc, key, kid)
		if err != nil {
			return nil, err
		}
		c.Envelopes[e.ID] = env
		c.Hashes[e.ID], _ = env["contract_hash"].(string)
	}
	if problems := c.VerifyAll(); len(problems) > 0 {
		return nil, fmt.Errorf("catalog failed verification: %v", problems)
	}
	return c, nil
}

// Verify checks one envelope (signature, owner binding, contract hash).
func (c *Catalog) Verify(id string) []string {
	env, ok := c.Envelopes[id]
	if !ok {
		return []string{"unregistered capability " + id}
	}
	errs, _ := validate.EnvelopeErrors(validate.CloneDoc(env), c.trust, false)
	return errs
}

func (c *Catalog) VerifyAll() map[string][]string {
	out := map[string][]string{}
	for _, e := range experts {
		if errs := c.Verify(e.ID); len(errs) > 0 {
			out[e.ID] = errs
		}
	}
	return out
}

// Card is what the planner sees when choosing experts.
func (c *Catalog) Cards() []map[string]any {
	out := []map[string]any{}
	for _, e := range experts {
		doc := e.contract()
		out = append(out, map[string]any{
			"id": e.ID, "name": e.Name, "kind": e.Kind, "summary": e.Summary, "tools": e.Tools, "package": e.Package,
			"requires": doc["requires"], "effects": doc["effects"], "bounds": doc["bounds"],
			"contract_hash": c.Hashes[e.ID],
		})
	}
	return out
}

func signEnvelope(contract map[string]any, key *ecdsa.PrivateKey, kid string) (map[string]any, error) {
	canonical, err := rfc8785.Canonicalize(contract)
	if err != nil {
		return nil, err
	}
	opts := (&jose.SignerOptions{}).WithHeader("typ", "JOSE").WithHeader("kid", kid)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: key}, opts)
	if err != nil {
		return nil, err
	}
	obj, err := signer.Sign(canonical)
	if err != nil {
		return nil, err
	}
	compact, err := obj.CompactSerialize()
	if err != nil {
		return nil, err
	}
	parts := strings.Split(compact, ".")
	hash, err := validate.ContractHash(contract)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"envelope_version": "0.1.0",
		"contract":         contract,
		"contract_hash":    hash,
		"signatures":       []any{map[string]any{"protected": parts[0], "signature": parts[2]}},
	}, nil
}

func loadOrCreateKey(path string) (*ecdsa.PrivateKey, error) {
	if raw, err := os.ReadFile(path); err == nil {
		block, _ := pem.Decode(raw)
		if block == nil {
			return nil, fmt.Errorf("%s: not PEM", path)
		}
		return x509.ParseECPrivateKey(block.Bytes)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func publicJWK(key *ecdsa.PrivateKey, kid string) map[string]any {
	enc := func(v *big.Int) string {
		b := make([]byte, 32)
		v.FillBytes(b)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return map[string]any{"kty": "EC", "crv": "P-256", "alg": "ES256", "use": "sig", "kid": kid,
		"x": enc(key.PublicKey.X), "y": enc(key.PublicKey.Y)}
}

func toAny(ss []string) []any {
	out := make([]any, 0, len(ss))
	for _, s := range ss {
		out = append(out, s)
	}
	return out
}

func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

// jsonDoc round-trips v through JSON so numbers are float64, the shape the
// festival validators see when they load documents from disk.
func jsonDoc(v any) map[string]any {
	raw, _ := json.Marshal(v)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}
