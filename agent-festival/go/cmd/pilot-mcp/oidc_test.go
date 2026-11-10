package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testIssuer = "http://keycloak:8080/realms/aha"

func signToken(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		raw, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	input := enc(map[string]string{"alg": "RS256", "kid": kid, "typ": "JWT"}) + "." + enc(claims)
	sum := sha256.Sum256([]byte(input))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func withVerifier(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "k1", "kty": "RSA", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(srv.Close)
	verifier = newOIDCVerifier(testIssuer, srv.URL)
	t.Cleanup(func() { verifier = nil })
	return key
}

func TestIdentifyVerifiesUserAndActorTokens(t *testing.T) {
	key := withVerifier(t)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	exp := time.Now().Add(5 * time.Minute).Unix()
	user := func(k *rsa.PrivateKey, iss string, exp int64) string {
		return "Bearer " + signToken(t, k, "k1", map[string]any{"iss": iss, "exp": exp,
			"preferred_username": "alice", "azp": "workswarm-web", "aud": "account"})
	}
	actor := "Bearer " + signToken(t, key, "k1", map[string]any{"iss": testIssuer, "exp": exp,
		"preferred_username": "service-account-workswarm-agent", "azp": "workswarm-agent"})

	cases := []struct {
		name, auth, actor string
		wantUser          bool
		wantIssuer        string
		wantActor         string
	}{
		{"both valid", user(key, testIssuer, exp), actor, true, "user:alice", "workswarm-agent"},
		{"forged user token", user(other, testIssuer, exp), actor, false, "anonymous", "workswarm-agent"},
		{"other issuer", user(key, "http://evil/realms/aha", exp), actor, false, "anonymous", "workswarm-agent"},
		{"expired", user(key, testIssuer, time.Now().Add(-time.Hour).Unix()), actor, false, "anonymous", "workswarm-agent"},
		{"service account as user", actor, actor, false, "anonymous", "workswarm-agent"},
		{"no tokens, empty actor header", "", "", false, "anonymous", ""},
	}
	for _, c := range cases {
		h := http.Header{}
		if c.auth != "" {
			h.Set("Authorization", c.auth)
		}
		h.Set("X-Actor-Token", c.actor)
		id := identify(h)
		if id.UserVerified != c.wantUser || id.Issuer != c.wantIssuer || id.Actor != c.wantActor {
			t.Errorf("%s: got verified=%v issuer=%s actor=%s problems=%v", c.name, id.UserVerified, id.Issuer, id.Actor, id.Problems)
		}
	}
}
