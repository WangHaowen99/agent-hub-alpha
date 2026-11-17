package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/openjiuwen/agent-festival/go/cmd/report-demo/selection"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	Store    *Store
	Config   Config
	Catalog  Catalog
	Material Material
	Files    Files
	HTTP     *http.Client
}
type handler func(http.ResponseWriter, *http.Request) (any, error)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) wrap(roles []string, h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if len(roles) > 0 {
			if _, err := s.authorize(r, roles...); err != nil {
				writeJSON(w, 401, &Fault{"UNAUTHORIZED", "valid scoped identity required", 401})
				return
			}
		}
		v, err := h(w, r)
		if err != nil {
			var f *Fault
			if !errors.As(err, &f) {
				f = storageError(err).(*Fault)
			}
			writeJSON(w, f.HTTP, map[string]any{"schema_version": Protocol, "error": f})
			return
		}
		if v != nil {
			writeJSON(w, 200, v)
		}
	}
}
func readBody(r *http.Request, v any) error {
	b, err := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024+1))
	if err != nil {
		return err
	}
	if len(b) > 2*1024*1024 {
		return fault("INVALID_REQUEST", "body too large", 413)
	}
	if err = DecodeStrict(b, v); err != nil {
		return fault("INVALID_REQUEST", err.Error(), 400)
	}
	return nil
}
func tokenDigest(v string) string { b := sha256.Sum256([]byte(v)); return hex.EncodeToString(b[:]) }
func (s *Server) authorize(r *http.Request, roles ...string) (string, error) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	for _, role := range roles {
		if role == "user" {
			cookie, err := r.Cookie("report_session")
			if err != nil {
				continue
			}
			var principal string
			err = s.Store.Pool.QueryRow(r.Context(), "SELECT principal FROM sessions WHERE id_digest=$1 AND expires_at>clock_timestamp()", tokenDigest(cookie.Value)).Scan(&principal)
			if err == nil {
				return principal, nil
			}
			continue
		}
		if expected := s.Config.Tokens[role]; expected != "" && subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1 {
			return role, nil
		}
	}
	return "", fault("UNAUTHORIZED", "scoped identity required", 401)
}
func (s *Server) userWrite(r *http.Request) (string, error) {
	p, err := s.authorize(r, "user")
	if err != nil {
		return "", err
	}
	cookie, _ := r.Cookie("report_session")
	var ok bool
	if r.Header.Get("Origin") != "" && r.Header.Get("Origin") != s.Config.PublicURL {
		return "", fault("FORBIDDEN", "confirmation origin mismatch", 403)
	}
	err = s.Store.Pool.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM sessions WHERE id_digest=$1 AND csrf_digest=$2 AND expires_at>clock_timestamp())", tokenDigest(cookie.Value), tokenDigest(r.Header.Get("X-CSRF-Token"))).Scan(&ok)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fault("FORBIDDEN", "CSRF challenge required", 403)
	}
	return p, nil
}
func (s *Server) checkCandidate(ctx context.Context, g Graph) (string, error) {
	if g.MaterialID != s.Material.ID {
		return "", fault("POLICY_REJECTED", "unknown fixed material", 422)
	}
	if err := ValidateReportGraph(g, s.Catalog); err != nil {
		return "", err
	}
	return s.compile(ctx, g)
}

func (s *Server) checkPublicCandidate(ctx context.Context, g Graph) (string, error) {
	if err := ValidateReportGraph(g, s.Catalog); err != nil {
		return "", err
	}
	if s.Catalog.Selection != nil {
		if s.Config.ExecutionProfile == "" || s.Config.ModelBindingDigest == "" {
			return "", fault("BINDING_CHANGED", "publish a reviewed source and actual-model binding before proposing executable work", 409)
		}
		if !s.Config.BusBinding.valid() {
			return "", fault("BUS_BINDING_CHANGED", "publish an explicit Bus environment binding before proposing executable work", 409)
		}
		if err := s.Store.validateSelection(ctx, s.Store.Pool, g); err != nil {
			return "", err
		}
	}
	return s.checkCandidate(ctx, g)
}

func (s *Server) compile(ctx context.Context, g Graph) (string, error) {
	b, err := json.Marshal(g)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", s.Config.CompilerURL+"/compile", strings.NewReader(string(b)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.Config.Tokens["worker"])
	res, err := s.HTTP.Do(req)
	if err != nil {
		return "", fault("COMPILER_UNAVAILABLE", "Java compilation service not ready", 503)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fault("POLICY_REJECTED", "Java compiler rejected candidate", 422)
	}
	var out struct {
		Digest string `json:"runtime_spec_digest"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 1024*1024)).Decode(&out); err != nil {
		return "", err
	}
	if out.Digest == "" {
		return "", fault("COMPILER_UNAVAILABLE", "compiler did not provide digest", 503)
	}
	return out.Digest, nil
}

func (s *Server) Routes() http.Handler {
	m := http.NewServeMux()
	s.uiRoutes(m)
	s.deliveryRoutes(m)
	s.testEffectRoutes(m)
	s.expertRoutes(m)
	s.verifyRoutes(m)
	s.humanRoutes(m)
	s.workbenchRoutes(m)
	m.HandleFunc("POST /v1/selections", s.wrap([]string{"planner"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var q selection.SelectionQuery
		if err := readBody(r, &q); err != nil {
			return nil, err
		}
		return s.selectCapabilities(r.Context(), q)
	}))
	m.HandleFunc("GET /healthz", s.wrap(nil, func(w http.ResponseWriter, r *http.Request) (any, error) {
		if err := s.Store.Pool.Ping(r.Context()); err != nil {
			return nil, err
		}
		return map[string]string{"status": "ok", "schema_version": Protocol, "control_session_id": s.Store.Session}, nil
	}))
	m.HandleFunc("GET /v1/catalog", s.wrap([]string{"planner", "worker", "operator", "user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		h, err := Digest(s.Catalog)
		return map[string]any{"catalog": s.Catalog, "catalog_digest": h, "material": map[string]string{"id": s.Material.ID, "title": s.Material.Title, "digest": s.Material.Digest}}, err
	}))
	m.HandleFunc("GET /v1/material", s.wrap([]string{"worker", "operator"}, func(w http.ResponseWriter, r *http.Request) (any, error) { return s.Material, nil }))
	m.HandleFunc("POST /v1/commands", s.wrap([]string{"planner", "user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		p, _ := s.authorize(r, "planner", "user")
		if p != "planner" {
			var err error
			p, err = s.userWrite(r)
			if err != nil {
				return nil, err
			}
		}
		var v struct {
			CommandID string `json:"command_id"`
			Graph     Graph  `json:"graph"`
		}
		if err := readBody(r, &v); err != nil {
			return nil, err
		}
		digest, err := s.checkPublicCandidate(r.Context(), v.Graph)
		if err != nil {
			return nil, err
		}
		return s.Store.Propose(r.Context(), p, v.CommandID, v.Graph, digest)
	}))
	m.HandleFunc("GET /v1/tasks/{id}", s.wrap([]string{"planner", "worker", "operator", "user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.Store.GetTask(r.Context(), r.PathValue("id"))
	}))
	m.HandleFunc("GET /v1/tasks/{id}/nodes/{node}/result", s.wrap([]string{"worker", "operator", "user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.Store.ReconcileResult(r.Context(), r.PathValue("id"), r.PathValue("node"), r.URL.Query().Get("input_digest"))
	}))
	m.HandleFunc("GET /v1/tasks/{id}/events", s.wrap([]string{"planner", "worker", "operator", "user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		after, err := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		if r.URL.Query().Get("after") == "" {
			after = 0
			err = nil
		}
		if err != nil || after < 0 {
			return nil, fault("INVALID_REQUEST", "invalid cursor", 400)
		}
		return s.Store.Events(r.Context(), r.PathValue("id"), after)
	}))
	m.HandleFunc("POST /v1/tasks/{id}/admit", s.wrap([]string{"user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		p, err := s.userWrite(r)
		if err != nil {
			return nil, err
		}
		var v struct {
			Digest string `json:"graph_digest"`
		}
		if err = readBody(r, &v); err != nil {
			return nil, err
		}
		return s.Store.Admit(r.Context(), r.PathValue("id"), v.Digest, p)
	}))
	for _, action := range []string{"pause", "resume"} {
		action := action
		m.HandleFunc("POST /v1/tasks/{id}/"+action, s.wrap([]string{"user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
			if _, err := s.userWrite(r); err != nil {
				return nil, err
			}
			return s.Store.SetPause(r.Context(), r.PathValue("id"), action == "pause")
		}))
	}
	m.HandleFunc("GET /v1/executions/pending", s.wrap([]string{"worker"}, func(w http.ResponseWriter, r *http.Request) (any, error) { return s.Store.Pending(r.Context()) }))
	m.HandleFunc("POST /v1/tasks/{id}/claim", s.wrap([]string{"worker"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var v struct {
			Owner  string `json:"owner"`
			Domain string `json:"domain"`
		}
		if err := readBody(r, &v); err != nil {
			return nil, err
		}
		if v.Domain != "graph" {
			return nil, fault("FORBIDDEN", "worker cannot claim delivery domain", 403)
		}
		return s.Store.ClaimRun(r.Context(), r.PathValue("id"), v.Owner, v.Domain)
	}))
	m.HandleFunc("POST /v1/tasks/{id}/heartbeat", s.wrap([]string{"worker"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var v struct {
			Owner string `json:"owner"`
			Epoch string `json:"epoch"`
		}
		if err := readBody(r, &v); err != nil {
			return nil, err
		}
		return s.Store.Heartbeat(r.Context(), r.PathValue("id"), v.Owner, v.Epoch, "graph")
	}))
	m.HandleFunc("POST /v1/tasks/{id}/nodes/{node}/resolve", s.wrap([]string{"worker"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var v struct {
			Owner  string `json:"owner"`
			Epoch  string `json:"epoch"`
			Digest string `json:"input_digest,omitempty"`
		}
		if err := readBody(r, &v); err != nil {
			return nil, err
		}
		return s.Store.ResolveNode(r.Context(), r.PathValue("id"), r.PathValue("node"), v.Owner, v.Epoch, v.Digest, s.Catalog, s.Material.Digest)
	}))
	m.HandleFunc("POST /v1/tasks/{id}/nodes/{node}/commit", s.wrap([]string{"worker"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var v CommitRequest
		if err := readBody(r, &v); err != nil {
			return nil, err
		}
		raw, err := s.Files.Read(v.ArtifactID)
		if err != nil {
			return nil, err
		}
		return s.Store.Commit(r.Context(), r.PathValue("id"), r.PathValue("node"), v, raw, s.Catalog, s.Material.Digest)
	}))
	m.HandleFunc("POST /v1/tasks/{id}/nodes/{node}/waiting", s.wrap([]string{"worker"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var v struct {
			Owner  string `json:"owner"`
			Epoch  string `json:"epoch"`
			Status string `json:"status"`
			Reason string `json:"reason"`
		}
		if err := readBody(r, &v); err != nil {
			return nil, err
		}
		return s.Store.MarkWaiting(r.Context(), r.PathValue("id"), r.PathValue("node"), v.Owner, v.Epoch, v.Status, v.Reason)
	}))
	m.HandleFunc("POST /v1/tasks/{id}/nodes/{node}/skip", s.wrap([]string{"worker"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var v struct {
			Owner string `json:"owner"`
			Epoch string `json:"epoch"`
		}
		if err := readBody(r, &v); err != nil {
			return nil, err
		}
		return s.Store.AuthorizeSkip(r.Context(), r.PathValue("id"), r.PathValue("node"), v.Owner, v.Epoch)
	}))
	m.HandleFunc("POST /v1/tasks/{id}/complete", s.wrap([]string{"worker"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var v struct {
			Owner string `json:"owner"`
			Epoch string `json:"epoch"`
		}
		if err := readBody(r, &v); err != nil {
			return nil, err
		}
		return s.Store.Complete(r.Context(), r.PathValue("id"), v.Owner, v.Epoch)
	}))
	m.HandleFunc("GET /v1/tasks/{id}/calls", s.wrap([]string{"worker", "planner", "user", "operator"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.Store.Calls(r.Context(), r.PathValue("id"), r.URL.Query().Get("node_id"))
	}))
	m.HandleFunc("POST /v1/tasks/{id}/nodes/{node}/calls", s.wrap([]string{"worker"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var v struct {
			Owner string          `json:"owner"`
			Epoch string          `json:"epoch"`
			Route json.RawMessage `json:"route"`
		}
		if err := readBody(r, &v); err != nil {
			return nil, err
		}
		return s.Store.BeginCall(r.Context(), r.PathValue("id"), r.PathValue("node"), v.Owner, v.Epoch, v.Route, s.Catalog)
	}))
	m.HandleFunc("POST /v1/calls/{call}/observations", s.wrap([]string{"worker"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var v Observation
		if err := readBody(r, &v); err != nil {
			return nil, err
		}
		return s.Store.Observe(r.Context(), r.PathValue("call"), "worker-adapter", v)
	}))
	m.HandleFunc("GET /v1/calls/{call}/observations", s.wrap([]string{"worker", "operator", "user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		return s.Store.Observations(r.Context(), r.PathValue("call"))
	}))
	m.HandleFunc("POST /v1/artifacts", s.wrap([]string{"worker"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		raw, err := io.ReadAll(io.LimitReader(r.Body, 4*1024*1024+1))
		if err != nil {
			return nil, err
		}
		if len(raw) > 4*1024*1024 {
			return nil, fault("INVALID_ARTIFACT", "artifact too large", 413)
		}
		id, err := s.Files.Put(raw)
		return map[string]string{"artifact_id": id, "digest": "sha256:" + id}, err
	}))
	m.HandleFunc("GET /v1/artifacts/{artifact}", s.wrap([]string{"worker", "operator", "user"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		b, err := s.Files.Read(r.PathValue("artifact"))
		if err != nil {
			return nil, err
		}
		return json.RawMessage(b), nil
	}))
	m.HandleFunc("POST /v1/artifacts/{artifact}/invalidate", s.wrap([]string{"operator"}, func(w http.ResponseWriter, r *http.Request) (any, error) {
		var v struct {
			Reason string `json:"reason"`
		}
		if err := readBody(r, &v); err != nil {
			return nil, err
		}
		return s.Store.Invalidate(r.Context(), r.PathValue("artifact"), v.Reason, "operator")
	}))
	m.HandleFunc("POST /auth/login", s.wrap(nil, func(w http.ResponseWriter, r *http.Request) (any, error) {
		if r.Header.Get("Origin") != "" && r.Header.Get("Origin") != s.Config.PublicURL {
			return nil, fault("FORBIDDEN", "login origin mismatch", 403)
		}
		var v struct {
			User     string `json:"user"`
			Password string `json:"password"`
		}
		if err := readBody(r, &v); err != nil {
			return nil, err
		}
		got := tokenDigest(v.Password)
		expected := tokenDigest(s.Config.UserPassword)
		if v.User != s.Config.User || subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
			return nil, fault("UNAUTHORIZED", "invalid login", 401)
		}
		token, csrf := newID("session-"), newID("csrf-")
		_, err := s.Store.Pool.Exec(r.Context(), "INSERT INTO sessions(id_digest,principal,csrf_digest,expires_at) VALUES($1,$2,$3,clock_timestamp()+interval '8 hours')", tokenDigest(token), s.Config.User, tokenDigest(csrf))
		if err != nil {
			return nil, err
		}
		http.SetCookie(w, &http.Cookie{Name: "report_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: strings.HasPrefix(s.Config.PublicURL, "https://"), MaxAge: 28800})
		return map[string]string{"user": s.Config.User, "csrf": csrf}, nil
	}))
	return m
}

func NewServer(s *Store, c Config, cat Catalog, material Material) *Server {
	s.MaterialProjectionDigest = material.ProjectionDigest
	s.ExecutionProfile = c.ExecutionProfile
	s.ModelBindingDigest = c.ModelBindingDigest
	s.BusBinding = c.BusBinding
	s.CurrentCatalogDigest, _ = Digest(cat)
	s.DeliveryPolicyVersion = cat.PolicyVersion
	s.ReuseFiles = &Files{Root: c.ArtifactRoot}
	return &Server{s, c, cat, material, Files{c.ArtifactRoot}, &http.Client{Timeout: 5 * time.Second}}
}
