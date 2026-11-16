package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// A separate authenticated HTTP server counts actual requests and counter
// effects. It is deliberately an isolated provider, not a bank or model expert.
type effectProvider struct {
	server                         *httptest.Server
	token                          string
	mu                             sync.Mutex
	posts, gets, effects, rejected int
	receipts                       map[string]EffectReceipt
	lost, deny                     bool
	entered, release               chan struct{}
}

func newEffectProvider(t *testing.T) *effectProvider {
	p := &effectProvider{token: newID("fixture-provider-secret-"), receipts: map[string]EffectReceipt{}}
	p.server = httptest.NewServer(http.HandlerFunc(p.serve))
	t.Cleanup(p.server.Close)
	return p
}

func (p *effectProvider) counts() map[string]int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return map[string]int{"authenticated_post": p.posts, "get": p.gets, "effects": p.effects, "rejected": p.rejected}
}
func (p *effectProvider) config() *TestEffectConfig {
	return &TestEffectConfig{URL: p.server.URL, Target: "isolated-report-sink", Token: p.token, TimeoutMS: 5000}
}
func (p *effectProvider) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+p.token {
		p.mu.Lock()
		p.rejected++
		p.mu.Unlock()
		w.WriteHeader(401)
		return
	}
	b := strings.TrimPrefix(r.URL.Path, "/effects/")
	if b == r.URL.Path || b == "" {
		w.WriteHeader(404)
		return
	}
	p.mu.Lock()
	if r.Method == "GET" {
		p.gets++
		receipt, ok := p.receipts[b]
		p.mu.Unlock()
		if !ok {
			w.WriteHeader(404)
			return
		}
		writeJSON(w, 200, receipt)
		return
	}
	if r.Method != "POST" {
		p.mu.Unlock()
		w.WriteHeader(405)
		return
	}
	p.posts++
	var wire map[string]string
	if json.NewDecoder(r.Body).Decode(&wire) != nil || wire["delivery_id"] != b {
		p.mu.Unlock()
		w.WriteHeader(400)
		return
	}
	digest, _ := Digest(map[string]string{"task_id": wire["task_id"], "object_digest": wire["object_digest"], "target": wire["target"], "origin": p.server.URL, "action": wire["action"], "renderer": wire["renderer"], "principal": wire["principal"]})
	if digest != wire["payload_digest"] || wire["target"] != "isolated-report-sink" || wire["action"] != "isolated-publish-report" || wire["renderer"] != "structured-json/1" {
		p.mu.Unlock()
		w.WriteHeader(409)
		return
	}
	receipt, exists := p.receipts[b]
	if exists && receipt.PayloadDigest != digest {
		p.mu.Unlock()
		w.WriteHeader(409)
		return
	}
	if !exists {
		status := "APPLIED"
		if p.deny {
			status = "DENIED"
		} else {
			p.effects++
		}
		receipt = EffectReceipt{ID: b, PayloadDigest: digest, Status: status, ReceiptID: newID("isolated-receipt-")}
		p.receipts[b] = receipt
	}
	lost, entered, release := p.lost, p.entered, p.release
	p.mu.Unlock()
	if entered != nil {
		close(entered)
		<-release
	}
	if lost {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			conn.Close()
		}
		return
	}
	writeJSON(w, 200, receipt)
}

func effectFixture(t *testing.T) (context.Context, *Server, *Task, string, *effectProvider) {
	t.Helper()
	ctx, s, cfg, cat := integration(t)
	_, g := sample(t)
	g.Nodes = append(g.Nodes, Node{ID: "check", Expert: "checker@1", Task: "control fixture check; not model or business acceptance", DependsOn: []string{"draft"}})
	task, err := s.Propose(ctx, "fixture-planner", "isolated-effect", g, "compiler-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Admit(ctx, task.ID, task.GraphDigest, "reviewer"); err != nil {
		t.Fatal(err)
	}
	v, err := s.ClaimRun(ctx, task.ID, "fixture-worker", "graph")
	if err != nil {
		t.Fatal(err)
	}
	lease := leaseFrom(t, v)
	cfg.ArtifactRoot = t.TempDir()
	files := Files{cfg.ArtifactRoot}
	artifact := ""
	for _, node := range g.Nodes {
		v, err = s.ResolveNode(ctx, task.ID, node.ID, lease.Owner, lease.Epoch, "", cat, "fixture-material")
		if err != nil {
			t.Fatal(err)
		}
		permit := v.(map[string]any)
		var raw []byte
		if node.ID == "check" {
			raw, _ = json.Marshal(map[string]string{"outcome": "PASS", "object_digest": "sha256:" + artifact, "checker_version": "isolated-control-fixture/1"})
		} else {
			raw, _ = json.Marshal(map[string]string{"fixture": "isolated protocol control; not a real report", "node": node.ID})
		}
		id, err := files.Put(raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.Commit(ctx, task.ID, node.ID, CommitRequest{lease.Owner, lease.Epoch, permit["input_digest"].(string), permit["attempt"].(int), id}, raw, cat, "fixture-material"); err != nil {
			t.Fatal(err)
		}
		if node.ID == "draft" {
			artifact = id
		}
	}
	if _, err = s.Complete(ctx, task.ID, lease.Owner, lease.Epoch); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApproveReport(ctx, task.ID, artifact, "reviewer"); err != nil {
		t.Fatal(err)
	}
	p := newEffectProvider(t)
	cfg.TestEffect = p.config()
	s.DeliveryPolicyVersion = cat.PolicyVersion
	server := &Server{Store: s, Config: cfg, Catalog: cat, Files: files}
	return ctx, server, task, artifact, p
}

func assertEffect(t *testing.T, v any, err error, state string) *EffectIntent {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := v.(*EffectIntent)
	if !ok || e.State != state {
		t.Fatalf("expected %s, got %#v", state, v)
	}
	return e
}

func TestPGIsolatedEffectLostReplyReconcilesOriginalIdentity(t *testing.T) {
	ctx, s, task, artifact, p := effectFixture(t)
	p.lost = true
	v, err := s.publishTestEffect(ctx, task.ID, artifact, "lost-reply", "reviewer")
	first := assertEffect(t, v, err, "UNKNOWN")
	originalFiles := s.Files
	s.Files = Files{Root: t.TempDir()}
	v, err = s.publishTestEffect(ctx, task.ID, artifact, "lost-reply", "reviewer")
	second := assertEffect(t, v, err, "APPLIED")
	s.Files = originalFiles
	if first.ID != second.ID || first.PayloadDigest != second.PayloadDigest || p.counts()["authenticated_post"] != 1 || p.counts()["effects"] != 1 || p.counts()["get"] != 1 {
		t.Fatal("unknown window blindly resent or changed identity", p.counts())
	}
	_, err = s.publishTestEffect(ctx, task.ID, artifact, "new-b-same-effect", "reviewer")
	requireCode(t, err, "INPUT_CONFLICT")
	changed := *s.Config.TestEffect
	changed.Target = "another-sink"
	s.Config.TestEffect = &changed
	_, err = s.publishTestEffect(ctx, task.ID, artifact, "lost-reply", "reviewer")
	requireCode(t, err, "INPUT_CONFLICT")
	if p.counts()["authenticated_post"] != 1 {
		t.Fatal("conflicting retry sent")
	}
	after, err := s.Store.GetTask(ctx, task.ID)
	if err != nil || after.Status != "AWAITING_ACCEPTANCE" {
		t.Fatal("test effect fabricated report delivery", err)
	}
	saveCommitEvidence(t, "effect-lost-reply", map[string]any{"counts": p.counts(), "intent": second, "task_status": after.Status, "reconciliation_with_missing_local_content": true, "scope": "real PG/HTTP; isolated counter provider and control fixtures, no bank or model"})
	before := p.counts()
	snapshot, err := s.Store.Backup(ctx, s.Files, s.Catalog, s.Material)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Payload.Tables["delivery_intents"]) != 1 || len(snapshot.Payload.Tables["observations"]) != 2 {
		t.Fatal("network effect facts omitted from scoped backup")
	}
	path := filepath.Join(t.TempDir(), "effect-snapshot.json")
	if err = WriteSnapshot(path, snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err = ReplaySnapshot(path); err != nil {
		t.Fatal(err)
	}
	current := p.counts()
	for k, v := range before {
		if current[k] != v {
			t.Fatal("offline replay sent a network request")
		}
	}
	saveCommitEvidence(t, "effect-snapshot-replay", map[string]any{"snapshot_digest": snapshot.Digest, "intent_rows": 1, "observation_rows": 2, "provider_counts_unchanged": true})
}

func TestPGIsolatedEffectActualTimeoutIsUnknownThenQueriesOriginalB(t *testing.T) {
	ctx, s, task, artifact, p := effectFixture(t)
	s.Config.TestEffect.TimeoutMS = 100
	p.entered = make(chan struct{})
	p.release = make(chan struct{})
	start := time.Now()
	v, err := s.publishTestEffect(ctx, task.ID, artifact, "actual-timeout", "reviewer")
	first := assertEffect(t, v, err, "UNKNOWN")
	select {
	case <-p.entered:
	default:
		close(p.release)
		t.Fatal("timeout happened before provider accepted request")
	}
	if time.Since(start) < 100*time.Millisecond || p.counts()["effects"] != 1 {
		close(p.release)
		t.Fatal("actual accepted-request timeout not exercised")
	}
	close(p.release)
	v, err = s.publishTestEffect(ctx, task.ID, artifact, "actual-timeout", "reviewer")
	second := assertEffect(t, v, err, "APPLIED")
	if first.PayloadDigest != second.PayloadDigest || p.counts()["authenticated_post"] != 1 || p.counts()["get"] != 1 || p.counts()["effects"] != 1 {
		t.Fatal("timeout caused a replacement effect", p.counts())
	}
	saveCommitEvidence(t, "effect-actual-timeout", map[string]any{"counts": p.counts(), "initial_state": "UNKNOWN", "reconciled_state": "APPLIED", "same_b_and_payload": true, "client_timeout_ms": 100})
}

func TestPGIsolatedEffectLateReceiptSurvivesPauseAndNewEpoch(t *testing.T) {
	ctx, s, task, artifact, p := effectFixture(t)
	p.entered = make(chan struct{})
	p.release = make(chan struct{})
	type outcome struct {
		v   any
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		v, err := s.publishTestEffect(ctx, task.ID, artifact, "late-receipt", "reviewer")
		done <- outcome{v, err}
	}()
	select {
	case <-p.entered:
	case got := <-done:
		t.Fatal("publisher returned before reaching the provider barrier", got.err)
	case <-time.After(15 * time.Second):
		close(p.release)
		t.Fatal("provider barrier not reached")
	}
	if _, err := s.Store.SetPause(ctx, task.ID, true); err != nil {
		close(p.release)
		t.Fatal(err)
	}
	_, err := s.Store.Pool.Exec(ctx, "UPDATE run_leases SET epoch=epoch+1,owner='new-current-owner',control_session='new-current-session',expires_at=clock_timestamp()+interval '1 minute',status='ACTIVE' WHERE scope=$1 AND task_id=$2 AND domain='delivery'", s.Store.Scope, task.ID)
	if err != nil {
		close(p.release)
		t.Fatal(err)
	}
	close(p.release)
	got := <-done
	e := assertEffect(t, got.v, got.err, "APPLIED")
	var owner, status string
	if err = s.Store.Pool.QueryRow(ctx, "SELECT owner,status FROM run_leases WHERE scope=$1 AND task_id=$2 AND domain='delivery'", s.Store.Scope, task.ID).Scan(&owner, &status); err != nil {
		t.Fatal(err)
	}
	if owner != "new-current-owner" || status != "ACTIVE" {
		t.Fatal("late observation released a newer lease")
	}
	observations, err := s.Store.Observations(ctx, "effect:"+e.ID)
	if err != nil || len(observations) != 1 || p.counts()["effects"] != 1 {
		t.Fatal("true late effect fact discarded", err)
	}
	saveCommitEvidence(t, "effect-late-receipt", map[string]any{"counts": p.counts(), "intent": e, "new_owner_status": status, "late_observations": len(observations), "scope": "effect happened before paused/new epoch; retained fact does not authorize new effects"})
}

func TestPGIsolatedEffectPreSendFencesAndRevocation(t *testing.T) {
	for _, mode := range []string{"expired", "pause", "approval-revoked", "artifact-revoked", "policy-changed"} {
		t.Run(mode, func(t *testing.T) {
			ctx, s, task, artifact, p := effectFixture(t)
			e, created, err := s.Store.prepareTestEffect(ctx, task.ID, artifact, "before-send", "reviewer", *s.Config.TestEffect)
			if err != nil || !created {
				t.Fatal(err)
			}
			want := ""
			switch mode {
			case "expired":
				_, err = s.Store.Pool.Exec(ctx, "UPDATE run_leases SET expires_at=clock_timestamp()-interval '1 second' WHERE scope=$1 AND task_id=$2 AND domain='delivery'", s.Store.Scope, task.ID)
				want = "LEASE_EXPIRED"
			case "pause":
				_, err = s.Store.SetPause(ctx, task.ID, true)
				want = "PAUSED"
			case "approval-revoked":
				_, err = s.Store.Pool.Exec(ctx, "DELETE FROM approvals WHERE scope=$1 AND task_id=$2", s.Store.Scope, task.ID)
				want = "APPROVAL_REQUIRED"
			case "artifact-revoked":
				_, err = s.Store.Invalidate(ctx, artifact, "isolated pre-send test", "operator")
				want = "POLICY_REJECTED"
			case "policy-changed":
				s.Store.DeliveryPolicyVersion = "different-current-policy"
				want = "BINDING_CHANGED"
			}
			if err != nil {
				t.Fatal(err)
			}
			err = s.Store.dispatchTestEffect(ctx, e)
			requireCode(t, err, want)
			if p.counts()["authenticated_post"] != 0 || p.counts()["effects"] != 0 {
				t.Fatal("pre-send rejected request reached provider")
			}
		})
	}
}

// Fail an actual mandatory audit INSERT only in this test's generated scope.
func effectAuditFault(t *testing.T, s *Server, kind string) func() {
	t.Helper()
	ctx := context.Background()
	admin, err := OpenStore(ctx, s.Config.MigratorDSN, s.Store.Scope, 30)
	if err != nil {
		t.Fatal(err)
	}
	name := strings.ReplaceAll(newID("effect_fault_"), "-", "_")
	quoted := pgx.Identifier{name}.Sanitize()
	scope := strings.ReplaceAll(s.Store.Scope, "'", "''")
	_, err = admin.Pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated required audit failure'; END $$;
	 CREATE TRIGGER %s BEFORE INSERT ON audit_outbox FOR EACH ROW WHEN (NEW.scope='%s' AND NEW.kind='%s') EXECUTE FUNCTION %s()`, quoted, quoted, scope, kind, quoted))
	if err != nil {
		admin.Pool.Close()
		t.Fatal(err)
	}
	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			_, err := admin.Pool.Exec(ctx, "DROP TRIGGER "+quoted+" ON audit_outbox; DROP FUNCTION "+quoted+"()")
			admin.Pool.Close()
			if err != nil {
				t.Error("failed to remove isolated audit trigger", err)
			}
		})
	}
	t.Cleanup(cleanup)
	return cleanup
}

func TestPGIsolatedEffectRequiredAuditFailureWindows(t *testing.T) {
	for _, kind := range []string{"TEST_EFFECT_INTENT", "TEST_EFFECT_DISPATCH", "TEST_EFFECT_OBSERVATION"} {
		t.Run(kind, func(t *testing.T) {
			ctx, s, task, artifact, p := effectFixture(t)
			remove := effectAuditFault(t, s, kind)
			_, err := s.publishTestEffect(ctx, task.ID, artifact, "audit-window", "reviewer")
			if err == nil {
				t.Fatal("required audit failure ignored")
			}
			if kind != "TEST_EFFECT_OBSERVATION" {
				if p.counts()["authenticated_post"] != 0 || p.counts()["effects"] != 0 {
					t.Fatal("uncommitted intent/dispatch sent an effect")
				}
			} else {
				if p.counts()["authenticated_post"] != 1 || p.counts()["effects"] != 1 {
					t.Fatal("post-effect receipt failure not exercised")
				}
				remove()
				v, err := s.publishTestEffect(ctx, task.ID, artifact, "audit-window", "reviewer")
				e := assertEffect(t, v, err, "APPLIED")
				if p.counts()["authenticated_post"] != 1 || p.counts()["get"] != 1 || p.counts()["effects"] != 1 {
					t.Fatal("receipt save failure caused a resend")
				}
				saveCommitEvidence(t, "effect-receipt-audit-failure", map[string]any{"intent": e, "counts": p.counts(), "required_audit_failure_exercised": true})
			}
		})
	}
}

func TestPGIsolatedEffectBusinessDenyAndConflictingReceipt(t *testing.T) {
	ctx, s, task, artifact, p := effectFixture(t)
	p.deny = true
	v, err := s.publishTestEffect(ctx, task.ID, artifact, "business-deny", "reviewer")
	first := assertEffect(t, v, err, "DENIED")
	v, err = s.publishTestEffect(ctx, task.ID, artifact, "business-deny", "reviewer")
	second := assertEffect(t, v, err, "DENIED")
	var firstReceipt, secondReceipt EffectReceipt
	json.Unmarshal(first.Receipt, &firstReceipt)
	json.Unmarshal(second.Receipt, &secondReceipt)
	if firstReceipt != secondReceipt || p.counts()["authenticated_post"] != 1 || p.counts()["effects"] != 0 {
		t.Fatal("business refusal not reused")
	}
	p.mu.Lock()
	r := p.receipts[first.ID]
	r.ReceiptID = "contradictory-provider-receipt"
	p.receipts[first.ID] = r
	p.mu.Unlock()
	v, err = s.publishTestEffect(ctx, task.ID, artifact, "business-deny", "reviewer")
	conflict := assertEffect(t, v, err, "CONFLICT")
	var original EffectReceipt
	json.Unmarshal(conflict.Receipt, &original)
	if original != firstReceipt {
		t.Fatal("conflict overwrote original receipt")
	}
	p.mu.Lock()
	r.Status = "APPLIED"
	p.receipts[first.ID] = r
	p.mu.Unlock()
	v, err = s.publishTestEffect(ctx, task.ID, artifact, "business-deny", "reviewer")
	assertEffect(t, v, err, "CONFLICT")
	observations, err := s.Store.Observations(ctx, "effect:"+first.ID)
	if err != nil || len(observations) != 4 {
		t.Fatal("conflicting fact was discarded", err)
	}
	saveCommitEvidence(t, "effect-business-deny-conflict", map[string]any{"counts": p.counts(), "state": conflict.State, "observations": len(observations)})
}

func TestPGIsolatedEffectTechnicalAuthErrorRemainsUnknown(t *testing.T) {
	ctx, s, task, artifact, p := effectFixture(t)
	s.Config.TestEffect.Token = newID("deliberately-unrecognized-fixture-token-")
	v, err := s.publishTestEffect(ctx, task.ID, artifact, "technical-auth", "reviewer")
	assertEffect(t, v, err, "UNKNOWN")
	v, err = s.publishTestEffect(ctx, task.ID, artifact, "technical-auth", "reviewer")
	assertEffect(t, v, err, "UNKNOWN")
	if p.counts()["rejected"] != 2 || p.counts()["effects"] != 0 || p.counts()["authenticated_post"] != 0 {
		t.Fatal("technical authentication failure converted to an applied effect or business refusal")
	}
	// Fix transport authentication, then query the original b. Provider 404
	// remains uncertain and never grants an automatic replacement POST.
	s.Config.TestEffect.Token = p.token
	v, err = s.publishTestEffect(ctx, task.ID, artifact, "technical-auth", "reviewer")
	assertEffect(t, v, err, "UNKNOWN")
	if p.counts()["get"] != 1 || p.counts()["authenticated_post"] != 0 {
		t.Fatal("provider not-found caused an automatic resend")
	}
	saveCommitEvidence(t, "effect-technical-auth-unknown", map[string]any{"counts": p.counts(), "state": "UNKNOWN", "no_business_deny_or_blind_resend": true})
}

func TestIsolatedEffectConfigOnlyAcceptsExplicitLoopbackOrigins(t *testing.T) {
	c := TestEffectConfig{URL: "http://127.0.0.1:8100", Target: "isolated-report-sink", Token: "fixture-only-provider-token-long-enough", TimeoutMS: 1000}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"https://example.com", "http://example.com:8100", "http://localhost:8100", "http://127.0.0.1:8100/path", "http://127.0.0.1:8100?target=other", "http://user:password@127.0.0.1:8100", "http://127.0.0.1"} {
		bad := c
		bad.URL = origin
		if bad.validate() == nil {
			t.Fatal("untrusted effect origin accepted", origin)
		}
	}
}

func TestPGDeliveryPolicyChangeBlocksNewActionButPreservesAuthorizedRead(t *testing.T) {
	ctx, s, task, artifact, p := effectFixture(t)
	if _, err := s.Store.Deliver(ctx, task.ID, artifact, "before-policy-change", "reviewer"); err != nil {
		t.Fatal(err)
	}
	s.Store.DeliveryPolicyVersion = "withdrawn-and-replaced-policy"
	_, err := s.Store.ApproveReport(ctx, task.ID, artifact, "reviewer")
	requireCode(t, err, "BINDING_CHANGED")
	_, err = s.Store.Deliver(ctx, task.ID, artifact, "after-policy-change", "reviewer")
	requireCode(t, err, "BINDING_CHANGED")
	// A currently authorized reader still reads the original immutable object.
	// This does not use a new approval, a new intent, or any provider request.
	session := newID("fixture-session-")
	_, err = s.Store.Pool.Exec(ctx, "INSERT INTO sessions(id_digest,principal,csrf_digest,expires_at) VALUES($1,'reviewer',$2,clock_timestamp()+interval '1 hour')", tokenDigest(session), tokenDigest("fixture-read-csrf"))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/v1/tasks/"+task.ID+"/artifacts/"+artifact+"/download", nil)
	req.AddCookie(&http.Cookie{Name: "report_session", Value: session})
	response := httptest.NewRecorder()
	s.Routes().ServeHTTP(response, req)
	raw, err := s.Files.Read(artifact)
	if err != nil {
		t.Fatal(err)
	}
	var received, want any
	if DecodeStrict(response.Body.Bytes(), &received) != nil || DecodeStrict(raw, &want) != nil || response.Code != 200 {
		t.Fatal("current authorized read was treated as a new effect", response.Code)
	}
	h1, _ := Digest(received)
	h2, _ := Digest(want)
	if h1 != h2 || !bytes.Equal(response.Body.Bytes(), raw) || p.counts()["authenticated_post"] != 0 {
		t.Fatal("read changed the stored report or emitted a network effect")
	}
	saveCommitEvidence(t, "delivery-policy-change", map[string]any{"new_approval_and_delivery_rejected": true, "current_authorized_download_http": response.Code, "provider_counts": p.counts(), "original_bytes_digest": "sha256:" + artifact})
}

func TestPGIsolatedEffectEndpointRequiresTrustedConfirmation(t *testing.T) {
	ctx, s, task, artifact, p := effectFixture(t)
	disabled := *s
	disabled.Config = s.Config
	disabled.Config.TestEffect = nil
	request := httptest.NewRequest("POST", "/test/effects/tasks/"+task.ID+"/artifacts/"+artifact, strings.NewReader(`{"delivery_id":"http-confirmed"}`))
	response := httptest.NewRecorder()
	disabled.Routes().ServeHTTP(response, request)
	if response.Code != 404 {
		t.Fatal("test endpoint registered in default demo")
	}
	api := httptest.NewServer(s.Routes())
	defer api.Close()
	path := api.URL + "/test/effects/tasks/" + task.ID + "/artifacts/" + artifact
	for _, role := range []string{"planner", "worker", "expert", "operator"} {
		req, _ := http.NewRequest("POST", path, strings.NewReader(`{"delivery_id":"http-confirmed"}`))
		req.Header.Set("Authorization", "Bearer "+s.Config.Tokens[role])
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatal("service/model role gained user effect authority", role)
		}
	}
	for _, credential := range []string{"", "Bearer forged-provider-credential", "Bearer " + s.Config.Tokens["worker"]} {
		req, _ := http.NewRequest("POST", p.server.URL+"/effects/bypass", strings.NewReader(`{}`))
		req.Header.Set("Authorization", credential)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 401 {
			t.Fatal("provider accepted unauthorized bypass")
		}
	}
	// Exercise real Go session + CSRF API with a fixture principal owning approval.
	if _, err := s.Store.ApproveReport(ctx, task.ID, artifact, s.Config.User); err != nil {
		t.Fatal(err)
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	loginBody, _ := json.Marshal(map[string]string{"user": s.Config.User, "password": s.Config.UserPassword})
	res, err := client.Post(api.URL+"/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	var login struct {
		CSRF string `json:"csrf"`
	}
	json.NewDecoder(res.Body).Decode(&login)
	res.Body.Close()
	if login.CSRF == "" {
		t.Fatal("trusted test login failed")
	}
	req, _ := http.NewRequest("POST", path, strings.NewReader(`{"delivery_id":"http-confirmed"}`))
	req.Header.Set("Origin", s.Config.PublicURL)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("effect accepted missing CSRF")
	}
	req, _ = http.NewRequest("POST", path, strings.NewReader(`{"delivery_id":"http-confirmed"}`))
	req.Header.Set("Origin", s.Config.PublicURL)
	req.Header.Set("X-CSRF-Token", login.CSRF)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var intent EffectIntent
	json.NewDecoder(res.Body).Decode(&intent)
	res.Body.Close()
	if res.StatusCode != 200 || intent.State != "APPLIED" {
		t.Fatal("valid confirmation failed", res.StatusCode, intent.State)
	}
	res, err = client.Get(api.URL + "/test/effects/http-confirmed")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || p.counts()["effects"] != 1 || p.counts()["authenticated_post"] != 1 || p.counts()["rejected"] != 3 || p.counts()["get"] != 0 {
		t.Fatal("read created another network action or bypass passed", p.counts())
	}
	saveCommitEvidence(t, "effect-http-permissions", map[string]any{"counts": p.counts(), "user_confirmation_positive": true, "service_roles_denied": 4, "missing_csrf_denied": true, "default_route_disabled": true, "limits": "HTTP provider boundary; not an OS sandbox or exhaustive Loop/Workflow/remote bypass proof"})
}
