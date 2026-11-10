package main

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// oidcVerifier checks RS256 access tokens against one issuer's published
// keys. It answers "who signed this and is it still valid"; audience is
// reported, not enforced, because WorkSwarm forwards the user's own login
// token rather than one issued for this service.
type oidcVerifier struct {
	issuer string
	jwks   string
	client *http.Client

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

type tokenClaims struct {
	Subject  string `json:"sub"`
	Username string `json:"preferred_username"`
	Azp      string `json:"azp"`
	Issuer   string `json:"iss"`
	Exp      int64  `json:"exp"`
	Nbf      int64  `json:"nbf"`
	Aud      any    `json:"aud"`
}

func newOIDCVerifier(issuer, jwks string) *oidcVerifier {
	if jwks == "" {
		jwks = strings.TrimRight(issuer, "/") + "/protocol/openid-connect/certs"
	}
	return &oidcVerifier{issuer: issuer, jwks: jwks, client: &http.Client{Timeout: 5 * time.Second}}
}

// verify accepts the raw header value ("Bearer <jwt>" or bare jwt).
func (v *oidcVerifier) verify(header string) (*tokenClaims, error) {
	tok := strings.TrimSpace(header)
	if len(tok) > 7 && strings.EqualFold(tok[:7], "bearer ") {
		tok = strings.TrimSpace(tok[7:])
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return nil, errors.New("not a JWT")
	}
	var head struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodeSegment(parts[0], &head); err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	if head.Alg != "RS256" {
		return nil, fmt.Errorf("alg %q not accepted", head.Alg)
	}
	key, err := v.key(head.Kid)
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("signature encoding")
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig) != nil {
		return nil, errors.New("bad signature")
	}
	var c tokenClaims
	if err := decodeSegment(parts[1], &c); err != nil {
		return nil, fmt.Errorf("claims: %w", err)
	}
	now := time.Now().Unix()
	switch {
	case c.Issuer != v.issuer:
		return nil, fmt.Errorf("issuer %q not trusted", c.Issuer)
	case c.Exp == 0 || now > c.Exp+30:
		return nil, errors.New("expired")
	case c.Nbf != 0 && now+30 < c.Nbf:
		return nil, errors.New("not yet valid")
	}
	return &c, nil
}

// key returns the signing key for kid, refetching the key set at most every
// 10 seconds when an unknown kid shows up (key rotation).
func (v *oidcVerifier) key(kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	if time.Since(v.fetched) < 10*time.Second && v.keys != nil {
		return nil, fmt.Errorf("unknown key %q", kid)
	}
	resp, err := v.client.Get(v.jwks)
	if err != nil {
		return nil, fmt.Errorf("fetch keys: %w", err)
	}
	defer resp.Body.Close()
	var set struct {
		Keys []struct {
			Kid, Kty, Use, N, E string
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return nil, fmt.Errorf("decode keys: %w", err)
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" || (k.Use != "" && k.Use != "sig") {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(k.N)
		e, errE := base64.RawURLEncoding.DecodeString(k.E)
		if errN != nil || errE != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	v.keys, v.fetched = keys, time.Now()
	if k, ok := keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown key %q", kid)
}

func decodeSegment(seg string, out any) error {
	raw, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func audience(a any) string {
	switch x := a.(type) {
	case string:
		return x
	case []any:
		s := make([]string, 0, len(x))
		for _, v := range x {
			s = append(s, fmt.Sprint(v))
		}
		return strings.Join(s, ",")
	}
	return ""
}
