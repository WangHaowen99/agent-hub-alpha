package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func (c *builderPublicationConfig) user(r *http.Request) (string, bool) {
	count := 0
	for _, cookie := range r.Cookies() {
		if cookie.Name == "cd_session" {
			count++
		}
	}
	user := currentUser(r)
	if count != 1 || user == "" {
		return "", false
	}
	if _, ok := users[user]; !ok {
		return "", false
	}
	allowed := false
	for _, u := range c.AllowedUsers {
		if u == user {
			allowed = true
		}
	}
	return user, allowed && (c.LookupUser == nil || c.LookupUser(user))
}

func (s *Service) builderPublicationRoutes(mux *http.ServeMux) {
	c := s.cfg.BuilderPublication
	if c == nil {
		return
	}
	mux.HandleFunc("/api/builder/publications/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != "GET" {
			builderHubError(w, 405)
			return
		}
		raw, e := io.ReadAll(io.LimitReader(r.Body, 1))
		if e != nil || len(raw) != 0 || r.URL.RawQuery != "" || len(r.Header.Values(builderContextHeader)) != 0 {
			builderHubError(w, 400)
			return
		}
		user, ok := c.user(r)
		if !ok {
			builderHubError(w, 403)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/builder/publications/")
		ref, ok := c.reference(id)
		if !ok {
			builderHubError(w, 404)
			return
		}
		view, err := observePublication(r.Context(), c, ref)
		after, ok := c.user(r)
		if !ok || after != user {
			builderHubError(w, 403)
			return
		}
		if err != nil {
			view.Available = false
			view.Status = "indeterminate"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(view)
	})
}

const builderPublicationBody = `<section><h2>场景产物与原 Bus 发现</h2><p>只读来源核对；治理未核验，不授予执行或生产权限。</p>{{range .Publications}}<button class="publication" data-id="{{.}}">查询 {{.}}</button>{{end}}<pre id="publication-result">尚未查询</pre></section><script>
document.querySelectorAll('.publication').forEach(b=>b.onclick=async()=>{const o=document.getElementById('publication-result');try{const r=await fetch('{{.Base}}/api/builder/publications/'+encodeURIComponent(b.dataset.id),{credentials:'same-origin',cache:'no-store'});o.textContent=JSON.stringify(await r.json(),null,2)}catch(e){o.textContent='查询未完成'}});
</script>`
