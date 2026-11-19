package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuilderPublicationRoutes(t *testing.T) {
	ref := publicationTestRef()
	calls := 0
	revoked := false
	up := publicationTestUpstream(t, ref, nil, func() { calls++ })
	defer up.Close()
	cfg := publicationTestConfig(up.URL, ref)
	cfg.LookupUser = func(string) bool { return !revoked }
	s := &Service{cfg: Config{BuilderPublication: cfg}}
	mux := http.NewServeMux()
	s.builderPublicationRoutes(mux)
	cookie := &http.Cookie{Name: "cd_session", Value: sign("alice")}
	for _, tc := range []struct {
		method, path, body string
		login              bool
		status             int
	}{{"GET", "/api/builder/publications/intent-gate", "", true, 200}, {"GET", "/api/builder/publications/intent-gate", "", false, 403}, {"POST", "/api/builder/publications/intent-gate", "", true, 405}, {"GET", "/api/builder/publications/intent-gate?actor=alice", "", true, 400}, {"GET", "/api/builder/publications/missing", "", true, 404}, {"GET", "/api/builder/publications/intent-gate", "x", true, 400}} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.login {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("%s: got %d want %d", tc.path, w.Code, tc.status)
		}
		if strings.Contains(w.Body.String(), up.URL) || strings.Contains(w.Body.String(), cfg.Token) || strings.Contains(w.Body.String(), "original_handoff") {
			t.Fatal("private data disclosure")
		}
	}
	if calls != 2 {
		t.Fatal("invalid requests reached RDC")
	}
	revoked = true
	req := httptest.NewRequest("GET", "/api/builder/publications/intent-gate", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 403 || calls != 2 {
		t.Fatal("revocation bypass")
	}
}
func TestBuilderPublicationRoutesRecheckAndUnknown(t *testing.T) {
	for _, revoke := range []bool{false, true} {
		t.Run(map[bool]string{true: "revoked", false: "unavailable"}[revoke], func(t *testing.T) {
			r := publicationTestRef()
			revoked := false
			up := publicationTestUpstream(t, r, func(_ string, m map[string]any) { m["name"] = "mismatch" }, func() { revoked = revoke })
			defer up.Close()
			c := publicationTestConfig(up.URL, r)
			c.LookupUser = func(string) bool { return !revoked }
			s := &Service{cfg: Config{BuilderPublication: c}}
			mux := http.NewServeMux()
			s.builderPublicationRoutes(mux)
			req := httptest.NewRequest("GET", "/api/builder/publications/intent-gate", nil)
			req.AddCookie(&http.Cookie{Name: "cd_session", Value: sign("alice")})
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if revoke {
				if w.Code != 403 {
					t.Fatal("disclosed after revocation")
				}
			} else if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"indeterminate"`) {
				t.Fatal("failed discovery not honest")
			}
		})
	}
}
func TestBuilderPublicationRoutesDisabled(t *testing.T) {
	s := &Service{}
	m := http.NewServeMux()
	s.builderPublicationRoutes(m)
	w := httptest.NewRecorder()
	m.ServeHTTP(w, httptest.NewRequest("GET", "/api/builder/publications/intent-gate", nil))
	if w.Code != 404 {
		t.Fatal("default route exposed")
	}
}

func TestBuilderPublicationRetainedDoesNotGrantRead(t *testing.T) {
	r := publicationTestRef()
	calls := 0
	up := publicationTestUpstream(t, r, nil, func() { calls++ })
	defer up.Close()
	c := publicationTestConfig(up.URL, r)
	retained := r
	retained.LogicalID = "retained-only"
	c.RetainedPublications = []publicationReference{retained}
	if c.validate() != nil {
		t.Fatal("identical retained binding rejected")
	}
	s := &Service{cfg: Config{BuilderPublication: c}}
	mux := http.NewServeMux()
	s.builderPublicationRoutes(mux)
	req := httptest.NewRequest("GET", "/api/builder/publications/retained-only", nil)
	req.AddCookie(&http.Cookie{Name: "cd_session", Value: sign("alice")})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 404 || calls != 0 {
		t.Fatal("retained binding granted a new read")
	}
}
