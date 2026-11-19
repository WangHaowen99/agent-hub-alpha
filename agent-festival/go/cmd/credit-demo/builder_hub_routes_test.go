package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBuilderHubRoutesLoginAndQueries(t *testing.T) {
	t.Setenv("CREDIT_DEMO_PASSWORD", "explicit-local-password")
	calls := 0
	revoked := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.HasSuffix(r.URL.Path, "preflight") {
			_, _ = w.Write([]byte(builderTestPreflight))
		} else {
			_, _ = w.Write([]byte(builderTestCapabilities()))
		}
	}))
	defer upstream.Close()
	cfg := builderTestConfig(upstream.URL)
	cfg.LookupSubject = func(user string) (string, bool) { return user, user == "alice" && !revoked }
	c, _ := newBuilderHubClient(cfg)
	s := &Service{cfg: Config{PublicURL: "", Origins: []string{"http://127.0.0.1:8123"}}}
	mux := http.NewServeMux()
	s.routes(mux)
	s.builderHubRoutes(mux, c)
	form := url.Values{"user": {"alice"}, "password": {"explicit-local-password"}, "next": {"/ui/builder"}}
	login := httptest.NewRequest("POST", "/ui/login", strings.NewReader(form.Encode()))
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, login)
	if rec.Code != 303 || len(rec.Result().Cookies()) != 1 {
		t.Fatal("original login failed")
	}
	cookie := rec.Result().Cookies()[0]
	for _, action := range []string{"capabilities", "preflight"} {
		method := "GET"
		if action == "preflight" {
			method = "POST"
		}
		r := httptest.NewRequest(method, "/api/builder-hub/"+action, nil)
		r.AddCookie(cookie)
		r.Header.Set("Origin", "http://127.0.0.1:8123")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", action, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "kkkk") || strings.Contains(w.Body.String(), "tttt") {
			t.Fatal("secret leak")
		}
	}
	page := httptest.NewRequest("GET", "/ui/builder", nil)
	page.AddCookie(cookie)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, page)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "尚未查询") {
		t.Fatal("original-shell page missing")
	}
	before := calls
	for _, tc := range []struct{ method, path, body, origin, cookie, proof string }{{"GET", "/api/builder-hub/capabilities", "", "", "", ""}, {"GET", "/api/builder-hub/capabilities", "", "", "forged", ""}, {"GET", "/api/builder-hub/capabilities?actor=alice", "", "", "valid", ""}, {"POST", "/api/builder-hub/preflight", "", "http://evil.invalid", "valid", ""}, {"POST", "/api/builder-hub/preflight", `{"actor":"alice"}`, "http://127.0.0.1:8123", "valid", ""}, {"GET", "/api/builder-hub/capabilities", "", "", "valid", "forged"}, {"POST", "/api/builder-hub/capabilities", "", "", "valid", ""}} {
		r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.cookie == "valid" {
			r.AddCookie(cookie)
		} else if tc.cookie != "" {
			r.AddCookie(&http.Cookie{Name: "cd_session", Value: tc.cookie})
		}
		r.Header.Set("Origin", tc.origin)
		if tc.proof != "" {
			r.Header.Set(builderContextHeader, tc.proof)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code < 400 {
			t.Errorf("unsafe request accepted: %+v", tc)
		}
	}
	revoked = true
	r := httptest.NewRequest("GET", "/api/builder-hub/capabilities", nil)
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 403 || calls != before {
		t.Fatal("revoked user reached upstream")
	}
}
func TestBuilderHubRoutesRecheck(t *testing.T) {
	revoked := false
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		revoked = true
		_, _ = w.Write([]byte(builderTestCapabilities()))
	}))
	defer up.Close()
	cfg := builderTestConfig(up.URL)
	cfg.LookupSubject = func(user string) (string, bool) { return user, !revoked }
	c, _ := newBuilderHubClient(cfg)
	s := &Service{}
	mux := http.NewServeMux()
	s.builderHubRoutes(mux, c)
	r := httptest.NewRequest("GET", "/api/builder-hub/capabilities", nil)
	r.AddCookie(&http.Cookie{Name: "cd_session", Value: sign("alice")})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 403 || strings.Contains(w.Body.String(), "capabilities") {
		t.Fatal("revoked result disclosed")
	}
}
func TestBuilderHubRoutesConfiguration(t *testing.T) {
	t.Setenv("BUILDER_HUB_ENABLED", "")
	c, e := builderHubFromEnv()
	if c != nil || e != nil {
		t.Fatal("default not disabled")
	}
	mux := http.NewServeMux()
	(&Service{}).builderHubRoutes(mux, c)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/builder-hub/capabilities", nil))
	if w.Code != 404 {
		t.Fatal("default route exists")
	}
	t.Setenv("BUILDER_HUB_ENABLED", "1")
	t.Setenv("CREDIT_DEMO_PASSWORD", "demo")
	if _, e = builderHubFromEnv(); e == nil {
		t.Fatal("default login enabled bridge")
	}
	t.Setenv("CREDIT_DEMO_PASSWORD", "explicit-local-password")
	t.Setenv("BUILDER_HUB_KEY_HEX", strings.Repeat("6b", 32))
	t.Setenv("BUILDER_HUB_CHANNEL_TOKEN", strings.Repeat("t", 32))
	t.Setenv("BUILDER_HUB_URL", "http://127.0.0.1:8124")
	t.Setenv("BUILDER_HUB_ISSUER", "hub-local")
	t.Setenv("BUILDER_HUB_KEY_ID", "local-1")
	t.Setenv("BUILDER_HUB_AUDIENCE", "intent-host")
	t.Setenv("BUILDER_HUB_SCOPE", "intent-builder")
	t.Setenv("BUILDER_HUB_USERS_JSON", `{"alice":"alice"}`)
	if _, e = builderHubFromEnv(); e != nil {
		t.Fatal(e)
	}
	t.Setenv("BUILDER_HUB_USERS_JSON", `{"alice":"alice","alice":"owner"}`)
	if _, e = builderHubFromEnv(); e == nil {
		t.Fatal("duplicate mapping accepted")
	}
}
