package main

import (
	"bytes"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func builderHubFromEnv() (*builderHubClient, error) {
	if os.Getenv("BUILDER_HUB_ENABLED") == "" {
		return nil, nil
	}
	if os.Getenv("BUILDER_HUB_ENABLED") != "1" {
		return nil, errBuilderHub
	}
	password := os.Getenv("CREDIT_DEMO_PASSWORD")
	if strings.TrimSpace(password) == "" || password == "demo" {
		return nil, errBuilderHub
	}
	key, e := hex.DecodeString(os.Getenv("BUILDER_HUB_KEY_HEX"))
	if e != nil || bytes.Equal(key, sessionKey) || bytes.Equal(key, []byte(password)) {
		return nil, errBuilderHub
	}
	raw := []byte(os.Getenv("BUILDER_HUB_USERS_JSON"))
	m, e := builderJSON(raw)
	if e != nil || len(m) == 0 || len(m) > 64 {
		return nil, errBuilderHub
	}
	mapping := map[string]string{}
	for user, v := range m {
		subject, ok := v.(string)
		if !ok || !builderIdentifier.MatchString(user) || !builderIdentifier.MatchString(subject) {
			return nil, errBuilderHub
		}
		if _, ok := users[user]; !ok {
			return nil, errBuilderHub
		}
		mapping[user] = subject
	}
	return newBuilderHubClient(builderHubConfig{Target: os.Getenv("BUILDER_HUB_URL"), ChannelToken: os.Getenv("BUILDER_HUB_CHANNEL_TOKEN"), Issuer: os.Getenv("BUILDER_HUB_ISSUER"), KeyID: os.Getenv("BUILDER_HUB_KEY_ID"), Audience: os.Getenv("BUILDER_HUB_AUDIENCE"), Scope: os.Getenv("BUILDER_HUB_SCOPE"), Key: key, LookupSubject: func(user string) (string, bool) { s, ok := mapping[user]; return s, ok }})
}

func builderHubError(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, `{"error":{"code":"builder-query-unavailable","message":"Builder query unavailable."}}`)
}
func (c *builderHubClient) user(r *http.Request) (string, string, bool) {
	count := 0
	for _, cookie := range r.Cookies() {
		if cookie.Name == "cd_session" {
			count++
		}
	}
	if count != 1 {
		return "", "", false
	}
	user := currentUser(r)
	if user == "" {
		return "", "", false
	}
	if _, ok := users[user]; !ok {
		return "", "", false
	}
	subject, ok := c.cfg.LookupSubject(user)
	return user, subject, ok && builderIdentifier.MatchString(subject)
}
func builderOrigin(origin string, allowed []string) bool {
	u, e := url.Parse(origin)
	if e != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	for _, s := range allowed {
		if origin == s {
			return true
		}
	}
	return false
}

// The original mux and original page shell own the user-facing entrypoint.
func (s *Service) builderHubRoutes(mux *http.ServeMux, c *builderHubClient) {
	if c == nil {
		return
	}
	mux.HandleFunc("/ui/builder", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.RawQuery != "" {
			builderHubError(w, 400)
			return
		}
		user, _, ok := c.user(r)
		if !ok {
			builderHubError(w, 403)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		body := builderHubBody
		ids := []string{}
		if p := s.cfg.BuilderPublication; p != nil {
			if after, ok := p.user(r); ok && after == user {
				for _, ref := range p.Publications {
					ids = append(ids, ref.LogicalID)
				}
				body += builderPublicationBody
			}
		}
		s.render(w, body, map[string]any{"Title": "Builder 支持与前置", "User": users[user], "Path": r.URL.Path, "Publications": ids})
	})
	for _, action := range []string{"capabilities", "preflight"} {
		action := action
		mux.HandleFunc("/api/builder-hub/"+action, func(w http.ResponseWriter, r *http.Request) {
			expected := "GET"
			if action == "preflight" {
				expected = "POST"
			}
			if r.Method != expected {
				builderHubError(w, 405)
				return
			}
			if r.URL.RawQuery != "" || len(r.Header.Values(builderContextHeader)) != 0 {
				builderHubError(w, 400)
				return
			}
			user, subject, ok := c.user(r)
			if !ok {
				builderHubError(w, 403)
				return
			}
			if action == "preflight" && (len(r.Header.Values("Origin")) != 1 || !builderOrigin(r.Header.Get("Origin"), s.cfg.Origins)) {
				builderHubError(w, 403)
				return
			}
			// Browser supplies no operation, principal, target, scope or body override.
			body, e := io.ReadAll(io.LimitReader(r.Body, 1))
			if e != nil || len(body) != 0 {
				builderHubError(w, 400)
				return
			}
			raw, e := c.query(r.Context(), subject, action)
			if e != nil {
				builderHubError(w, 502)
				return
			}
			after, afterSubject, ok := c.user(r)
			if !ok || after != user || afterSubject != subject {
				builderHubError(w, 403)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_, _ = w.Write(raw)
		})
	}
}

const builderHubBody = `<h1>Builder 支持与前置</h1><p>Intent / 1.0 / GATE · 只读查询。本地参考接线不表示正式 Hub 或生产资格。</p><button id="support">查询支持</button> <button id="preflight">检查构建前置</button><pre id="result">尚未查询</pre><script>
const result=document.getElementById('result');
async function query(action){const r=await fetch('{{.Base}}/api/builder-hub/'+action,{method:action==='preflight'?'POST':'GET',credentials:'same-origin',cache:'no-store'});result.textContent=JSON.stringify(await r.json(),null,2);}
document.getElementById('support').onclick=()=>query('capabilities').catch(()=>result.textContent='查询未完成');
document.getElementById('preflight').onclick=()=>query('preflight').catch(()=>result.textContent='查询未完成');
</script>`
