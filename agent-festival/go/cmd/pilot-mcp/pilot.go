package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/openjiuwen/agent-festival/go/internal/adopt"
	"github.com/openjiuwen/agent-festival/go/internal/ledger"
)

// Pilot is the spike's control module: it owns command dedup, task and
// request records, and drives the reference adoption machine. Provider
// outcomes are stored as observations; the provider stays the source of
// business truth.
type Pilot struct {
	mu       sync.Mutex
	scope    string
	dir      string
	machine  *adopt.Machine
	provider *Provider

	Commands map[string]*CommandRecord `json:"commands"`
	Tasks    map[string]*Task          `json:"tasks"`
	Order    []string                  `json:"order"`
	Calls    []CallObservation         `json:"calls"`
}

type CommandRecord struct {
	Key       string     `json:"key"`
	CommandID string     `json:"command_id"`
	Source    string     `json:"source"`
	Issuer    string     `json:"issuer"`
	Digest    string     `json:"digest"`
	TaskID    string     `json:"task_id"`
	Receipt   Receipt    `json:"receipt"`
	Conflicts []Conflict `json:"conflicts,omitempty"`
}

type Conflict struct {
	At     time.Time `json:"at"`
	Digest string    `json:"digest"`
}

type Task struct {
	ID        string    `json:"task_id"`
	CommandID string    `json:"command_id"`
	Source    string    `json:"command_source"`
	Issuer    string    `json:"issuer"`
	Actor     string    `json:"actor,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Material  Material  `json:"material"`
	Judgment  Judgment  `json:"judgment"`
	Request   *Request  `json:"request,omitempty"`
}

type Request struct {
	ID             string        `json:"request_id"`
	OperationID    string        `json:"operation_id"`
	AmountMinor    int64         `json:"amount_minor"`
	Currency       string        `json:"currency"`
	Scenario       string        `json:"scenario"`
	ProviderStatus string        `json:"provider_status"` // "" = unknown to Go
	Attempts       []Attempt     `json:"attempts"`
	Observations   []Observation `json:"observations"`
	Cancel         string        `json:"cancel,omitempty"`
}

type Attempt struct {
	N      int       `json:"n"`
	At     time.Time `json:"at"`
	Result string    `json:"result"`
}

type Observation struct {
	At      time.Time `json:"at"`
	EventID string    `json:"event_id"`
	Status  string    `json:"status"`
	Applied string    `json:"applied"` // applied, duplicate, ignored-regression
}

type Receipt struct {
	Status         string `json:"status"` // accepted, duplicate, conflict
	CommandID      string `json:"command_id"`
	TaskID         string `json:"task_id,omitempty"`
	Judgment       string `json:"judgment,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
	AmountMinor    int64  `json:"amount_minor,omitempty"`
	ControlState   string `json:"control_state,omitempty"`
	ProviderStatus string `json:"provider_status,omitempty"`
	Detail         string `json:"detail,omitempty"`
}

// CallObservation records what Go could learn about the caller of one tool
// call (V1). Header values are never stored, only fingerprints.
type CallObservation struct {
	At       time.Time `json:"at"`
	Tool     string    `json:"tool"`
	Identity Identity  `json:"identity"`
}

type Identity struct {
	Issuer       string            `json:"issuer"`
	Basis        string            `json:"basis"`
	HeaderNames  []string          `json:"header_names"`
	Fingerprints map[string]string `json:"fingerprints,omitempty"`
	// Set only when an OIDC issuer is configured (PILOT_OIDC_ISSUER).
	UserVerified bool     `json:"user_verified,omitempty"`
	UserAudience string   `json:"user_audience,omitempty"`
	Actor        string   `json:"actor,omitempty"`
	Problems     []string `json:"problems,omitempty"`
}

var pilotGrants = []ledger.Grant{
	{Kind: "data", Selector: "data:credit/pilot/applications", Actions: []string{"read"}, Effect: "none"},
	{Kind: "tool", Selector: "tool:credit.test-api.submit", Actions: []string{"execute"}, Effect: "irreversible"},
}

func OpenPilot(dir, scope string) (*Pilot, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	m, err := adopt.Open(filepath.Join(dir, "adopt.jsonl"))
	if err != nil {
		return nil, err
	}
	p := &Pilot{scope: scope, dir: dir, machine: m,
		Commands: map[string]*CommandRecord{}, Tasks: map[string]*Task{}}
	if raw, err := os.ReadFile(filepath.Join(dir, "state.json")); err == nil {
		if err := json.Unmarshal(raw, p); err != nil {
			return nil, fmt.Errorf("state.json: %w", err)
		}
	}
	p.provider = NewProvider(p.observe)
	return p, nil
}

func (p *Pilot) save() {
	raw, _ := json.MarshalIndent(p, "", "  ")
	tmp := filepath.Join(p.dir, "state.json.tmp")
	if os.WriteFile(tmp, raw, 0o644) == nil {
		_ = os.Rename(tmp, filepath.Join(p.dir, "state.json"))
	}
}

// identify derives the command issuer from what actually arrived over the
// connection. An explicit end-user header wins; a bearer token is a service
// credential unless shown otherwise; nothing means anonymous.
func identify(h http.Header) Identity {
	id := Identity{Issuer: "anonymous", Basis: "none", Fingerprints: map[string]string{}}
	for name := range h {
		id.HeaderNames = append(id.HeaderNames, name)
		l := strings.ToLower(name)
		if l == "authorization" || l == "cookie" || strings.Contains(l, "user") ||
			strings.Contains(l, "token") || strings.Contains(l, "auth") || strings.Contains(l, "tenant") {
			id.Fingerprints[name] = fingerprint(strings.Join(h.Values(name), ","))
		}
	}
	sort.Strings(id.HeaderNames)
	if verifier != nil {
		verifyTokens(h, &id)
		return id
	}
	switch {
	case h.Get("X-Pilot-User") != "":
		id.Issuer, id.Basis = "user:"+h.Get("X-Pilot-User"), "x-pilot-user header"
	case h.Get("Authorization") != "":
		id.Issuer, id.Basis = "bearer:"+id.Fingerprints["Authorization"], "authorization header"
	}
	return id
}

// verifyTokens reads WorkSwarm enterprise's two tokens: Authorization carries
// the end user's login token, X-Actor-Token the agent instance's own token.
// Only a verified user token names a user; a service-account token in
// Authorization does not.
func verifyTokens(h http.Header, id *Identity) {
	switch raw := h.Get("Authorization"); {
	case raw == "":
		id.Problems = append(id.Problems, "user token: missing")
	default:
		c, err := verifier.verify(raw)
		switch {
		case err != nil:
			id.Problems = append(id.Problems, "user token: "+err.Error())
		case c.Username == "" || strings.HasPrefix(c.Username, "service-account-"):
			id.Problems = append(id.Problems, "user token: not a user ("+c.Azp+")")
		default:
			id.Issuer, id.Basis = "user:"+c.Username, "verified user token"
			id.UserVerified, id.UserAudience = true, audience(c.Aud)
		}
	}
	switch raw := h.Get("X-Actor-Token"); {
	case raw == "" || strings.TrimSpace(strings.TrimPrefix(raw, "Bearer")) == "":
		id.Problems = append(id.Problems, "actor token: missing")
	default:
		if c, err := verifier.verify(raw); err != nil {
			id.Problems = append(id.Problems, "actor token: "+err.Error())
		} else {
			id.Actor = c.Azp
		}
	}
	if !id.UserVerified {
		id.Basis = "no verified user token"
	}
}

func fingerprint(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])[:12]
}

func (p *Pilot) recordCall(tool string, id Identity) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Calls = append(p.Calls, CallObservation{At: time.Now().UTC(), Tool: tool, Identity: id})
	if len(p.Calls) > 200 {
		p.Calls = p.Calls[len(p.Calls)-200:]
	}
}

// Submit accepts one credit application command. Same (scope, issuer,
// command_id) with the same digest returns the original receipt; with a
// different digest it is a conflict and has no new effect.
func (p *Pilot) Submit(commandID, source string, id Identity, m Material) Receipt {
	if m.Scenario == "" {
		m.Scenario = "normal"
	}
	if commandID == "" {
		commandID, source = fmt.Sprintf("srv-%d", time.Now().UnixNano()), "server"
	}
	if source == "" {
		source = "model"
	}
	key := p.scope + "|" + id.Issuer + "|" + commandID
	digest := m.digest()

	p.mu.Lock()
	if rec, ok := p.Commands[key]; ok {
		defer p.mu.Unlock()
		if rec.Digest != digest {
			rec.Conflicts = append(rec.Conflicts, Conflict{At: time.Now().UTC(), Digest: digest})
			p.save()
			return Receipt{Status: "conflict", CommandID: commandID, TaskID: rec.TaskID,
				Detail: "same command_id with different material; no new effect"}
		}
		r := p.receiptFor(p.Tasks[rec.TaskID])
		r.Status = "duplicate"
		return r
	}
	t := &Task{ID: fmt.Sprintf("T%d", len(p.Tasks)+1), CommandID: commandID, Source: source,
		Issuer: id.Issuer, Actor: id.Actor, CreatedAt: time.Now().UTC(), Material: m, Judgment: judge(m)}
	p.Tasks[t.ID] = t
	p.Order = append(p.Order, t.ID)
	p.Commands[key] = &CommandRecord{Key: key, CommandID: commandID, Source: source,
		Issuer: id.Issuer, Digest: digest, TaskID: t.ID}

	amount := grantAmountMinor(t.Judgment.Kind)
	if amount == 0 {
		r := p.receiptFor(t)
		p.Commands[key].Receipt = r
		p.save()
		p.mu.Unlock()
		return r
	}
	req := &Request{ID: "B" + t.ID[1:], OperationID: "op-" + t.ID, AmountMinor: amount,
		Currency: "USD", Scenario: m.Scenario}
	t.Request = req
	payload := fmt.Sprintf(`{"customer_id":%q,"amount_minor":%d,"currency":"USD"}`, m.CustomerID, amount)
	sum := sha256.Sum256([]byte(payload))
	out := p.machine.Adopt(&adopt.Request{
		AdoptionVersion: "0.1.0",
		Identity:        ledger.Identity{BusinessRequestID: req.ID, EvaluationRevision: "D1", OperationID: req.OperationID},
		Versions: ledger.Versions{Evidence: "ev-" + digest[7:19], Rule: "credit-pilot-rule-1",
			Model: "s1-standin", Readout: "r-1", Policy: "pilot-policy-1"},
		PayloadDigest:    "sha256:" + hex.EncodeToString(sum[:]),
		GrantsAtDecision: pilotGrants,
		Accountability:   &ledger.Accountability{Session: commandID, Skill: "credit-pilot", Role: id.Issuer},
	})
	if out.Status == "adopted" {
		out = p.machine.Dispatch(req.OperationID, pilotGrants)
	}
	if out.Status != "dispatched" {
		req.Attempts = append(req.Attempts, Attempt{N: 1, At: time.Now().UTC(), Result: "not dispatched: " + out.Detail})
		r := p.receiptFor(t)
		p.Commands[key].Receipt = r
		p.save()
		p.mu.Unlock()
		return r
	}
	req.Attempts = append(req.Attempts, Attempt{N: 1, At: time.Now().UTC(), Result: "sending"})
	p.save()
	p.mu.Unlock()

	status, err := p.provider.Submit(req.ID, m.Scenario)

	p.mu.Lock()
	defer p.mu.Unlock()
	a := &req.Attempts[len(req.Attempts)-1]
	if err != nil {
		a.Result = "unknown: " + err.Error()
		p.machine.MarkUnresolved(req.OperationID)
	} else {
		a.Result = "receipt: " + status
		if req.ProviderStatus == "" {
			req.ProviderStatus = status
		}
	}
	r := p.receiptFor(t)
	p.Commands[key].Receipt = r
	p.save()
	return r
}

func (p *Pilot) receiptFor(t *Task) Receipt {
	r := Receipt{Status: "accepted", CommandID: t.CommandID, TaskID: t.ID, Judgment: t.Judgment.Kind}
	if t.Request != nil {
		r.RequestID = t.Request.ID
		r.AmountMinor = t.Request.AmountMinor
		r.ControlState = string(p.machine.State(t.Request.OperationID))
		r.ProviderStatus = t.Request.ProviderStatus
	}
	return r
}

func terminal(s string) bool { return s == "approved" || s == "declined" || s == "cancelled" }

// observe ingests one provider event. Duplicates are recorded but not
// re-applied; a non-terminal event after a terminal one never regresses
// the observed state.
func (p *Pilot) observe(requestID, status, eventID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.applyObservation(requestID, status, eventID)
	p.save()
}

func (p *Pilot) applyObservation(requestID, status, eventID string) {
	var req *Request
	for _, t := range p.Tasks {
		if t.Request != nil && t.Request.ID == requestID {
			req = t.Request
		}
	}
	if req == nil {
		return
	}
	o := Observation{At: time.Now().UTC(), EventID: eventID, Status: status, Applied: "applied"}
	for _, prev := range req.Observations {
		if prev.EventID == eventID {
			o.Applied = "duplicate"
		}
	}
	if o.Applied == "applied" && terminal(req.ProviderStatus) && !terminal(status) {
		o.Applied = "ignored-regression"
	}
	req.Observations = append(req.Observations, o)
	if o.Applied != "applied" {
		return
	}
	req.ProviderStatus = status
	if terminal(status) {
		state := p.machine.State(req.OperationID)
		if state == adopt.StateDispatched || state == adopt.StateUnresolved {
			if status == "cancelled" {
				p.machine.MarkFailed(req.OperationID)
			} else {
				p.machine.MarkCompleted(req.OperationID)
			}
		}
	}
}

// reconcile queries the provider for every request whose outcome Go does
// not know. The adoption machine has no unresolved -> pending edge, so a
// "still pending" answer is kept as an observation only.
func (p *Pilot) reconcile() {
	p.mu.Lock()
	var unknown []string
	for _, t := range p.Tasks {
		if t.Request != nil && p.machine.State(t.Request.OperationID) == adopt.StateUnresolved {
			unknown = append(unknown, t.Request.ID)
		}
	}
	p.mu.Unlock()
	for _, id := range unknown {
		if s := p.provider.Query(id); s != "" {
			p.mu.Lock()
			p.applyObservation(id, s, id+"-query-"+s)
			p.save()
			p.mu.Unlock()
		}
	}
}

func (p *Pilot) RequestCancel(requestID string) (bool, string) {
	p.mu.Lock()
	var req *Request
	for _, t := range p.Tasks {
		if t.Request != nil && t.Request.ID == requestID {
			req = t.Request
		}
	}
	if req == nil {
		p.mu.Unlock()
		return false, "unknown request"
	}
	req.Cancel = "requested"
	p.save()
	p.mu.Unlock()

	ok, detail := p.provider.Cancel(requestID)
	p.mu.Lock()
	defer p.mu.Unlock()
	if ok {
		req.Cancel = "confirmed"
		p.applyObservation(requestID, "cancelled", requestID+"-cancel")
	} else {
		req.Cancel = "rejected: " + detail
	}
	p.save()
	return ok, detail
}

// View is the authoritative projection a client may display.
type View struct {
	Task         *Task  `json:"task"`
	ControlState string `json:"control_state,omitempty"`
	LedgerEvents int    `json:"ledger_events"`
	Conflicts    int    `json:"conflicts"`
}

// view returns a copy, so callers never read records the provider
// callbacks are still writing.
func (p *Pilot) view(t *Task) View {
	c := *t
	if t.Request != nil {
		r := *t.Request
		r.Attempts = append([]Attempt(nil), t.Request.Attempts...)
		r.Observations = append([]Observation(nil), t.Request.Observations...)
		c.Request = &r
	}
	v := View{Task: &c}
	if t.Request != nil {
		v.ControlState = string(p.machine.State(t.Request.OperationID))
	}
	if ev, err := p.machine.Events(); err == nil {
		v.LedgerEvents = len(ev)
	}
	for _, c := range p.Commands {
		if c.TaskID == t.ID {
			v.Conflicts = len(c.Conflicts)
		}
	}
	return v
}

func (p *Pilot) Get(taskID, commandID string) (View, bool) {
	p.reconcile()
	p.mu.Lock()
	defer p.mu.Unlock()
	if taskID == "" {
		for _, c := range p.Commands {
			if c.CommandID == commandID {
				taskID = c.TaskID
			}
		}
	}
	t, ok := p.Tasks[taskID]
	if !ok {
		return View{}, false
	}
	return p.view(t), true
}

func (p *Pilot) List(limit int) []View {
	p.reconcile()
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []View
	for i := len(p.Order) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, p.view(p.Tasks[p.Order[i]]))
	}
	return out
}

func (p *Pilot) RecentCalls(n int) []CallObservation {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.Calls) <= n {
		return append([]CallObservation(nil), p.Calls...)
	}
	return append([]CallObservation(nil), p.Calls[len(p.Calls)-n:]...)
}
