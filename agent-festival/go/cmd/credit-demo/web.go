package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

//go:embed web/*
var webFS embed.FS

var sessionKey = func() []byte { b := make([]byte, 32); _, _ = rand.Read(b); return b }()

var users = map[string]string{"alice": "客户经理 alice"}

func validLogin(user, password string) bool {
	_, ok := users[user]
	want := envOr("CREDIT_DEMO_PASSWORD", "demo")
	return ok && subtle.ConstantTimeCompare([]byte(password), []byte(want)) == 1
}

func sign(v string) string {
	m := hmac.New(sha256.New, sessionKey)
	m.Write([]byte(v))
	return v + "." + hex.EncodeToString(m.Sum(nil))
}

func currentUser(r *http.Request) string {
	c, err := r.Cookie("cd_session")
	if err != nil {
		return ""
	}
	i := strings.LastIndex(c.Value, ".")
	if i < 0 || sign(c.Value[:i]) != c.Value {
		return ""
	}
	return c.Value[:i]
}

var page = template.Must(template.New("page").Funcs(template.FuncMap{
	"expert": func(id string) *Expert { return expertByID(id) },
	"clip":   clip,
	"short":  func(h string) string { return clip(strings.TrimPrefix(h, "sha256:"), 12) },
	"join":   func(ss []string) string { return strings.Join(ss, "、") },
	"json":   compact,
}).Parse(`<!doctype html><html lang="zh"><head><meta charset="utf-8"><title>{{.Title}}</title>
<link rel="stylesheet" href="{{$.Base}}/static/style.css"><script src="{{$.Base}}/static/graph.js" defer></script></head><body>
<header><b>授信编排</b>{{if .User}}<span class="user">{{.User}}</span>{{else}}<a class="user" href="{{$.Base}}/ui/login?next={{.Path}}">登录</a>{{end}}</header>
<main>{{template "body" .}}</main>
</body></html>`))

func (s *Service) render(w http.ResponseWriter, body string, data map[string]any) {
	data["Base"] = s.base()
	t := template.Must(template.Must(page.Clone()).New("body").Parse(body))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "page", data); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

const loginBody = `<h1>演示登录</h1><p class="muted">演示环境未接 IdP，只用于区分"人在页面上批准"和"模型调用工具"。</p>
<form method="post" class="card"><input type="hidden" name="next" value="{{.Next}}">
<label>用户 <select name="user"><option value="alice">alice（客户经理）</option></select></label>
<label>密码 <input type="password" name="password"></label><button>登录</button>{{if .Err}}<p class="bad">{{.Err}}</p>{{end}}</form>`

const planBody = `{{with .Plan}}<h1>编排单 {{.ID}} · 第 {{.Version}} 版 <small>案件 {{.CaseID}}</small></h1>
<p class="goal">{{.Goal}}</p>
<nav class="versions">{{range $.Versions}}<a href="{{$.Base}}/ui/plan/{{.ID}}/v{{.Version}}" {{if eq .Version $.Plan.Version}}class="on"{{end}}>v{{.Version}}</a>{{end}}</nav>
<div class="graph" data-graph="{{$.Base}}/api/graph?plan={{.ID}}&v={{.Version}}"></div>
{{if .Precheck.OK}}<div class="ok">规约预检通过{{with .Precheck.Composed}} · 合成效果 {{json .effects}} · 合成上限 {{json .bounds}}{{end}}</div>
{{else if .Precheck.Errors}}<details><summary>compose 原文</summary><ul>{{range .Precheck.Errors}}<li><code>{{.}}</code></li>{{end}}</ul></details>{{end}}
<table><tr><th>步骤</th><th>专家</th><th>角色</th><th>子任务</th><th>依赖</th><th>契约</th></tr>
{{range .Agents}}{{$e := expert .Expert}}<tr><td><code>{{.ID}}</code></td><td>{{$e.Name}}</td><td>{{.Role}}</td><td>{{.Task}}</td><td>{{join .DependsOn}}{{with .RunIf}}<br><span class="muted">条件：{{.Step}} {{if .Negate}}不含{{else}}含{{end}} {{.Contains}}</span>{{end}}</td><td><code>{{short (index $.Hashes .Expert)}}</code> ✓</td></tr>{{end}}</table>
<p class="muted">编排单摘要 <code>{{.Hash}}</code></p>
{{if .RunID}}<p><a class="btn" href="{{$.Base}}/ui/run/{{.RunID}}">查看执行 {{.RunID}}</a></p>
{{else if .Precheck.OK}}{{if $.User}}<form method="post" action="{{$.Base}}/ui/plan/{{.ID}}/v{{.Version}}/approve"><input type="hidden" name="hash" value="{{.Hash}}"><button class="btn primary">批准并执行</button></form>
{{else}}<p><a class="btn" href="{{$.Base}}/ui/login?next={{$.Path}}">登录后批准</a></p>{{end}}{{end}}
<details><summary>蓝图 YAML（yaojun 编排模块编译）</summary><pre>{{.Blueprint}}</pre></details>{{end}}`

const runBody = `{{with .Run}}<h1>执行 {{.ID}} <small>编排单 <a href="{{$.Base}}/ui/plan/{{.PlanID}}/v{{.PlanVersion}}">{{.PlanID}} v{{.PlanVersion}}</a> · {{.Status}}</small></h1>
<div class="graph" data-graph="{{$.Base}}/api/graph?run={{.ID}}"></div>
{{with .Decision}}<div class="ok">已采纳判断：<b>{{.level}}</b> · {{.rationale}}</div>{{end}}
{{if .Receipt}}<div class="ok">{{.Receipt}}</div>{{end}}
{{range .Violations}}<div class="bad">{{.}}</div>{{end}}
<details><summary>领取记录</summary><ul>{{range .Leases}}<li>{{.}}</li>{{end}}</ul></details>
{{if eq .Status "done"}}<p><a class="btn primary" href="{{$.Base}}/ui/run/{{.ID}}/verify">验证</a></p>{{end}}{{end}}`

const verifyBody = `<h1>验证 {{.Report.RunID}}</h1>
<div class="{{if .Report.OK}}ok big{{else}}bad big{{end}}">{{.Report.Summary}}</div>
{{with .Report.Tamper}}<div class="bad">{{.}}</div>{{end}}
{{range .Report.Checks}}<div class="check"><b class="{{if .OK}}y{{else}}n{{end}}">{{if .OK}}✓{{else}}✗{{end}}</b> <b>{{.Name}}</b> <span class="muted">{{.Detail}}</span>
{{if .Items}}<ul>{{range .Items}}<li>{{.}}</li>{{end}}</ul>{{end}}</div>{{end}}
<p>{{if .Tampered}}<a class="btn" href="{{$.Base}}/ui/run/{{.Report.RunID}}/verify">回到真实账本</a>{{else}}<a class="btn" href="{{$.Base}}/ui/run/{{.Report.RunID}}/verify?tamper=1">篡改演示</a>{{end}}
<a class="btn" href="{{$.Base}}/ui/run/{{.Report.RunID}}">回到执行</a></p>`

// base is the path prefix browsers see in front of every page and asset.
func (s *Service) base() string {
	if u, err := url.Parse(s.cfg.PublicURL); err == nil {
		return strings.TrimRight(u.Path, "/")
	}
	return ""
}

func (s *Service) routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/graph", s.serveGraph)
	mux.HandleFunc("/api/approve", s.serveApprove)
	static := http.FileServerFS(webFS)
	mux.Handle("/static/", http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "web/" + r.URL.Path
		static.ServeHTTP(w, r)
	})))
	mux.HandleFunc("/ui/login", func(w http.ResponseWriter, r *http.Request) {
		next := r.FormValue("next")
		if !strings.HasPrefix(next, "/ui/") {
			next = "/ui/"
		}
		if r.Method == http.MethodPost {
			u := r.FormValue("user")
			if validLogin(u, r.FormValue("password")) {
				http.SetCookie(w, &http.Cookie{Name: "cd_session", Value: sign(u), Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
				http.Redirect(w, r, s.base()+next, http.StatusSeeOther)
				return
			}
			s.render(w, loginBody, map[string]any{"Title": "登录", "Next": next, "Err": "用户名或密码不对", "Path": r.URL.Path})
			return
		}
		s.render(w, loginBody, map[string]any{"Title": "登录", "Next": next, "Path": r.URL.Path})
	})
	mux.HandleFunc("/ui/plan/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/ui/plan/"), "/"), "/")
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}
		ver, _ := strconv.Atoi(strings.TrimPrefix(parts[1], "v"))
		p, ok := s.Plan(parts[0], ver)
		if !ok {
			http.NotFound(w, r)
			return
		}
		user := currentUser(r)
		if len(parts) == 3 && parts[2] == "approve" && r.Method == http.MethodPost {
			if user == "" {
				http.Redirect(w, r, s.base()+"/ui/login?next="+fmt.Sprintf("/ui/plan/%s/v%d", p.ID, p.Version), http.StatusSeeOther)
				return
			}
			run, err := s.Approve(p.ID, p.Version, r.FormValue("hash"), user)
			if err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			http.Redirect(w, r, s.base()+"/ui/run/"+run.ID, http.StatusSeeOther)
			return
		}
		s.mu.Lock()
		versions := append([]*Plan(nil), s.st.Plans[p.ID]...)
		s.mu.Unlock()
		s.render(w, planBody, map[string]any{"Title": "编排单 " + p.Key(), "Plan": p, "Versions": versions,
			"Hashes": s.cat.Hashes, "User": users[user], "Path": r.URL.Path})
	})
	mux.HandleFunc("/ui/run/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/ui/run/"), "/"), "/")
		run, _, ok := s.RunByID(parts[0])
		if !ok {
			http.NotFound(w, r)
			return
		}
		user := users[currentUser(r)]
		if len(parts) == 2 && parts[1] == "verify" {
			tamper := r.URL.Query().Get("tamper") == "1"
			rep, err := s.Verify(run.ID, tamper)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			s.render(w, verifyBody, map[string]any{"Title": "验证 " + run.ID, "Report": rep, "Tampered": tamper, "User": user, "Path": r.URL.Path})
			return
		}
		s.mu.Lock()
		view := *run
		s.mu.Unlock()
		s.render(w, runBody, map[string]any{"Title": "执行 " + run.ID, "Run": &view, "User": user, "Path": r.URL.Path})
	})
	mux.HandleFunc("/ui/", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		var latest []*Plan
		for _, vs := range s.st.Plans {
			latest = append(latest, vs[len(vs)-1])
		}
		s.mu.Unlock()
		s.render(w, `<h1>编排单</h1><ul>{{range .Plans}}<li><a href="/ui/plan/{{.ID}}/v{{.Version}}">{{.ID}} v{{.Version}} · {{.CaseID}} · {{.Goal}}</a>{{if .RunID}} → <a href="/ui/run/{{.RunID}}">{{.RunID}}</a>{{end}}</li>{{end}}</ul>`,
			map[string]any{"Title": "授信编排", "Plans": latest, "User": users[currentUser(r)], "Path": r.URL.Path})
	})

	// executor protocol (internal network only)
	jsonIn := func(r *http.Request, v any) error { return json.NewDecoder(r.Body).Decode(v) }
	jsonOut := func(w http.ResponseWriter, code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	errOut := func(w http.ResponseWriter, err error) {
		code := http.StatusBadRequest
		if errors.Is(err, errFenced) {
			code = http.StatusConflict
		}
		jsonOut(w, code, map[string]string{"error": err.Error()})
	}
	type call struct {
		RunID   string   `json:"run_id"`
		Token   string   `json:"token"`
		Step    string   `json:"step"`
		Output  string   `json:"output"`
		Raw     string   `json:"raw"`
		Worker  string   `json:"worker"`
		Skipped []string `json:"skipped"`
		Final   string   `json:"final"`
		Failed  string   `json:"failed"`
	}
	mux.HandleFunc("/exec/lease", func(w http.ResponseWriter, r *http.Request) {
		var c call
		_ = jsonIn(r, &c)
		deadline := time.Now().Add(20 * time.Second)
		for {
			if job, ok := s.Lease(c.Worker); ok {
				jsonOut(w, 200, job)
				return
			}
			if time.Now().After(deadline) {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
	})
	mux.HandleFunc("/exec/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		var c call
		_ = jsonIn(r, &c)
		if err := s.Heartbeat(c.RunID, c.Token); err != nil {
			errOut(w, err)
			return
		}
		jsonOut(w, 200, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/exec/step/get", func(w http.ResponseWriter, r *http.Request) {
		var c call
		_ = jsonIn(r, &c)
		rec, out, err := s.StepGet(c.RunID, c.Token, c.Step)
		if err != nil {
			errOut(w, err)
			return
		}
		jsonOut(w, 200, map[string]any{"recorded": rec, "output": out})
	})
	mux.HandleFunc("/exec/step/put", func(w http.ResponseWriter, r *http.Request) {
		var c call
		_ = jsonIn(r, &c)
		if err := s.StepPut(c.RunID, c.Token, c.Step, c.Output); err != nil {
			errOut(w, err)
			return
		}
		jsonOut(w, 200, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/exec/judgment", func(w http.ResponseWriter, r *http.Request) {
		var c call
		_ = jsonIn(r, &c)
		out, err := s.Judgment(c.RunID, c.Token, c.Step, c.Raw)
		if err != nil {
			errOut(w, err)
			return
		}
		jsonOut(w, 200, map[string]string{"output": out})
	})
	mux.HandleFunc("/exec/effect", func(w http.ResponseWriter, r *http.Request) {
		var c call
		_ = jsonIn(r, &c)
		out, err := s.Effect(c.RunID, c.Token, c.Step)
		if err != nil {
			errOut(w, err)
			return
		}
		jsonOut(w, 200, map[string]string{"output": out})
	})
	mux.HandleFunc("/exec/branch", func(w http.ResponseWriter, r *http.Request) {
		var c struct {
			RunID  string `json:"run_id"`
			Token  string `json:"token"`
			Step   string `json:"step"`
			Goal   string `json:"goal"`
			Status string `json:"status"`
			Chars  int    `json:"chars"`
		}
		_ = jsonIn(r, &c)
		if err := s.BranchEvent(c.RunID, c.Token, c.Step, c.Goal, c.Status, c.Chars); err != nil {
			errOut(w, err)
			return
		}
		jsonOut(w, 200, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/exec/done", func(w http.ResponseWriter, r *http.Request) {
		var c call
		_ = jsonIn(r, &c)
		if err := s.Done(c.RunID, c.Token, c.Skipped, c.Final, c.Failed); err != nil {
			errOut(w, err)
			return
		}
		jsonOut(w, 200, map[string]bool{"ok": true})
	})
	mux.HandleFunc("/debug/state", func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		jsonOut(w, 200, s.st)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
