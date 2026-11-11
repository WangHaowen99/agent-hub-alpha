package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/openjiuwen/agent-festival/go/internal/adopt"
	"github.com/openjiuwen/agent-festival/go/internal/compose"
	"github.com/openjiuwen/agent-festival/go/internal/evidlog"
	"github.com/openjiuwen/agent-festival/go/internal/judgment"
	"github.com/openjiuwen/agent-festival/go/internal/ledger"
)

type Config struct {
	Addr            string
	DataDir         string
	OrchestratorURL string
	PublicURL       string   // where browsers reach this service: a path behind the front proxy, e.g. /credit
	Origins         []string // browser origins of the WorkSwarm UI; the MCP App may fetch /api/graph from them
	MemberMCPURL    string
	Model           string
	SpecPath        string
	LeaseSeconds    int
}

// StepRec is the authoritative record of one blueprint step in a run.
type StepRec struct {
	ID         string   `json:"id"`
	Expert     string   `json:"expert"`
	Status     string   `json:"status"` // pending running done skipped failed
	Output     string   `json:"output,omitempty"`
	Reused     int      `json:"reused,omitempty"`
	StartedAt  string   `json:"started_at,omitempty"`
	FinishedAt string   `json:"finished_at,omitempty"`
	Epoch      int      `json:"epoch,omitempty"`
	Branches   []Branch `json:"branches,omitempty"`
	Activity   string   `json:"activity,omitempty"` // what the step is doing right now, for the live graph
}

// Branch is one sub-goal a black-box (Loop Forest) step forked at run time.
type Branch struct {
	Goal     string `json:"goal"`
	Status   string `json:"status"` // started finished interrupted
	Chars    int    `json:"chars,omitempty"`
	At       string `json:"at"`
	Read     string `json:"read,omitempty"`     // the related company whose data this branch read
	Activity string `json:"activity,omitempty"` // what the branch is doing right now
}

type Run struct {
	ID          string              `json:"id"`
	PlanID      string              `json:"plan_id"`
	PlanVersion int                 `json:"plan_version"`
	PlanHash    string              `json:"plan_hash"`
	CaseID      string              `json:"case_id"`
	Status      string              `json:"status"` // queued running done failed
	ApprovedBy  string              `json:"approved_by"`
	CreatedAt   string              `json:"created_at"`
	FinishedAt  string              `json:"finished_at,omitempty"`
	Token       string              `json:"-"`
	TokenSaved  string              `json:"token,omitempty"`
	LeaseUntil  time.Time           `json:"lease_until"`
	Epoch       int                 `json:"epoch"`
	Leases      []string            `json:"leases,omitempty"`
	Steps       map[string]*StepRec `json:"steps"`
	Order       []string            `json:"order"`
	Violations  []string            `json:"violations,omitempty"`
	Decision    map[string]any      `json:"decision,omitempty"`
	DecisionOp  string              `json:"decision_op,omitempty"`
	BusinessReq string              `json:"business_request,omitempty"`
	Receipt     string              `json:"receipt,omitempty"`
	ReceiptOf   string              `json:"receipt_status,omitempty"` // approved declined cancelled unknown
	ReplaySteps []map[string]any    `json:"replay_steps,omitempty"`
	Scenario    string              `json:"scenario,omitempty"`
	Final       string              `json:"final_output,omitempty"`
}

type State struct {
	Plans    map[string][]*Plan `json:"plans"`
	Runs     map[string]*Run    `json:"runs"`
	NextPlan int                `json:"next_plan"`
	NextRun  int                `json:"next_run"`
}

type Service struct {
	cfg      Config
	mu       sync.Mutex
	st       *State
	cat      *Catalog
	bank     *Provider
	spec     map[string]any
	levels   []string
	machines map[string]*adopt.Machine
	logs     map[string]*evidlog.Log
}

func OpenService(cfg Config) (*Service, error) {
	compose.RegisterGraph()
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "runs"), 0o755); err != nil {
		return nil, err
	}
	cat, err := LoadCatalog(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(cfg.SpecPath)
	if err != nil {
		return nil, fmt.Errorf("judgment spec: %w", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, err
	}
	for k := range spec {
		if strings.HasPrefix(k, "_") {
			delete(spec, k)
		}
	}
	if errs := judgment.CheckSpec(spec); len(errs) > 0 {
		return nil, fmt.Errorf("judgment spec rejected: %v", errs)
	}
	s := &Service{cfg: cfg, cat: cat, spec: spec, machines: map[string]*adopt.Machine{}, logs: map[string]*evidlog.Log{},
		st: &State{Plans: map[string][]*Plan{}, Runs: map[string]*Run{}}}
	if lv, ok := spec["levels"].([]any); ok {
		for _, l := range lv {
			if m, ok := l.(map[string]any); ok {
				s.levels = append(s.levels, fmt.Sprint(m["name"]))
			}
		}
	}
	s.levels = append(s.levels, "on_missing_evidence", "on_undefined_rule")
	if raw, err := os.ReadFile(filepath.Join(cfg.DataDir, "state.json")); err == nil {
		if err := json.Unmarshal(raw, s.st); err != nil {
			return nil, fmt.Errorf("state.json: %w", err)
		}
		for _, r := range s.st.Runs {
			r.Token = r.TokenSaved
		}
	}
	s.bank = NewProvider(func(string, string, string) {})
	s.bank.TerminalDelay = 4 * time.Second
	return s, nil
}

// save must be called with s.mu held.
func (s *Service) save() {
	for _, r := range s.st.Runs {
		r.TokenSaved = r.Token
	}
	if err := writeJSON(filepath.Join(s.cfg.DataDir, "state.json"), s.st); err != nil {
		fmt.Fprintln(os.Stderr, "save state:", err)
	}
}

func (s *Service) machine(runID string) (*adopt.Machine, error) {
	if m, ok := s.machines[runID]; ok {
		return m, nil
	}
	dir := filepath.Join(s.cfg.DataDir, "runs", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	m, err := adopt.Open(filepath.Join(dir, "adopt.jsonl"))
	if err != nil {
		return nil, err
	}
	s.machines[runID] = m
	return m, nil
}

func (s *Service) evidLog(runID, step string) (*evidlog.Log, error) {
	key := runID + "/" + step
	if l, ok := s.logs[key]; ok {
		return l, nil
	}
	dir := filepath.Join(s.cfg.DataDir, "runs", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	l, err := evidlog.Open(filepath.Join(dir, "evid-"+step+".jsonl"))
	if err != nil {
		return nil, err
	}
	s.logs[key] = l
	return l, nil
}

// ---------- plans ----------

type ProposeInput struct {
	CaseID string      `json:"case_id" jsonschema:"case id such as C006"`
	Goal   string      `json:"goal" jsonschema:"what the team must deliver"`
	PlanID string      `json:"plan_id,omitempty" jsonschema:"set to revise an existing plan (creates a new version)"`
	Agents []PlanAgent `json:"agents" jsonschema:"team members, each bound to a registered expert"`
}

func (s *Service) Propose(in ProposeInput) (*Plan, error) {
	in.CaseID = strings.ToUpper(strings.TrimSpace(in.CaseID))
	if _, ok := cases[in.CaseID]; !ok {
		return nil, fmt.Errorf("unknown case %q", in.CaseID)
	}
	if err := checkAgents(in.Agents); err != nil {
		return nil, err
	}
	res, team, err := s.compile(in.Agents)
	if err != nil {
		return nil, err
	}
	p := &Plan{CaseID: in.CaseID, Goal: in.Goal, Agents: in.Agents, Blueprint: res.Blueprint, SpecDigest: res.SpecDigest,
		Team: team, Type: res.Type, Steps: res.Steps, ProposedAt: now()}
	if p.Layers, err = layers(p.Steps); err != nil {
		return nil, err
	}
	p.Precheck = precheck(p)
	p.Hash = planHash(p, s.cat.Hashes)
	s.mu.Lock()
	defer s.mu.Unlock()
	if in.PlanID != "" {
		if _, ok := s.st.Plans[in.PlanID]; !ok {
			return nil, fmt.Errorf("unknown plan %q", in.PlanID)
		}
		p.ID = in.PlanID
	} else {
		s.st.NextPlan++
		p.ID = fmt.Sprintf("P%d", s.st.NextPlan)
	}
	p.Version = len(s.st.Plans[p.ID]) + 1
	s.st.Plans[p.ID] = append(s.st.Plans[p.ID], p)
	s.save()
	return p, nil
}

func (s *Service) Plan(id string, version int) (*Plan, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vs := s.st.Plans[id]
	if len(vs) == 0 {
		return nil, false
	}
	if version <= 0 || version > len(vs) {
		return vs[len(vs)-1], true
	}
	return vs[version-1], true
}

// Approve is only reachable from the signed-in confirm page: the planner
// has no tool for it. The approved hash must be the one shown.
func (s *Service) Approve(planID string, version int, hash, user string) (*Run, error) {
	p, ok := s.Plan(planID, version)
	if !ok || p.Version != version {
		return nil, fmt.Errorf("unknown plan version")
	}
	if p.Hash != hash || planHash(p, s.cat.Hashes) != hash {
		return nil, fmt.Errorf("the plan changed after it was shown — reload and review again")
	}
	if pc := precheck(p); !pc.OK {
		return nil, fmt.Errorf("precheck rejects this plan")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.RunID != "" {
		return s.st.Runs[p.RunID], nil
	}
	s.st.NextRun++
	r := &Run{ID: fmt.Sprintf("R%d", s.st.NextRun), PlanID: p.ID, PlanVersion: p.Version, PlanHash: p.Hash, CaseID: p.CaseID,
		Status: "queued", ApprovedBy: user, CreatedAt: now(), Steps: map[string]*StepRec{}, Scenario: os.Getenv("CREDIT_BANK_SCENARIO")}
	for _, l := range p.Layers {
		for _, id := range l {
			a := p.agent(id)
			r.Steps[id] = &StepRec{ID: id, Expert: a.Expert, Status: "pending"}
			r.Order = append(r.Order, id)
		}
	}
	p.ApprovedBy, p.RunID = user, r.ID
	s.st.Runs[r.ID] = r
	s.save()
	return r, nil
}

func (s *Service) RunByID(id string) (*Run, *Plan, bool) {
	s.mu.Lock()
	r, ok := s.st.Runs[id]
	s.mu.Unlock()
	if !ok {
		return nil, nil, false
	}
	p, _ := s.Plan(r.PlanID, r.PlanVersion)
	return r, p, true
}

func (s *Service) RunForPlan(planID string) (*Run, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vs := s.st.Plans[planID]
	for i := len(vs) - 1; i >= 0; i-- {
		if vs[i].RunID != "" {
			return s.st.Runs[vs[i].RunID], true
		}
	}
	return nil, false
}

// ---------- executor protocol (lease, steps, judgment, effect) ----------

type Job struct {
	RunID        string                    `json:"run_id"`
	Token        string                    `json:"token"`
	Epoch        int                       `json:"epoch"`
	Mode         string                    `json:"mode"`
	Team         []map[string]any          `json:"team"`
	SpecDigest   string                    `json:"spec_digest"`
	UserInput    string                    `json:"user_input"`
	Members      map[string]map[string]any `json:"members"`
	MemberMCPURL string                    `json:"member_mcp_url"`
	LeaseSeconds int                       `json:"lease_seconds"`
}

func newToken() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Lease hands one runnable run to an executor. A run whose lease expired is
// taken over with a new token; the old executor's writes are then fenced.
func (s *Service) Lease(worker string) (*Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.st.Runs))
	for id := range s.st.Runs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		r := s.st.Runs[id]
		if r.Status != "queued" && !(r.Status == "running" && time.Now().After(r.LeaseUntil)) {
			continue
		}
		vs := s.st.Plans[r.PlanID]
		p := vs[r.PlanVersion-1]
		r.Epoch++
		r.Token = newToken()
		r.LeaseUntil = time.Now().Add(time.Duration(s.cfg.LeaseSeconds) * time.Second)
		r.Status = "running"
		r.Leases = append(r.Leases, fmt.Sprintf("%s 第 %d 次领取（%s）", now(), r.Epoch, worker))
		for _, st := range r.Steps {
			if st.Status == "running" {
				st.Status = "pending"
			}
			if st.Status != "done" {
				for i := range st.Branches {
					if st.Branches[i].Status == "started" {
						st.Branches[i].Status = "interrupted" // forked by the executor that lost its lease
					}
				}
			}
		}
		s.save()
		members := map[string]map[string]any{}
		for _, a := range p.Agents {
			e := expertByID(a.Expert)
			m := map[string]any{"kind": e.Kind, "expert": e.ID, "name": e.Name, "tools": e.Tools}
			if e.Kind == "judge" {
				var deps []string
				for _, st := range p.Steps {
					if st.ID == a.ID {
						deps = st.DependsOn
					}
				}
				m["judge_prompt"] = s.judgePrompt() + fmt.Sprintf("\nevidence_steps 只能填上游产物开头 [step:xxx] 标注的步骤 id，可选：%s。\n", strings.Join(deps, "、"))
			}
			members[a.ID] = m
		}
		return &Job{RunID: r.ID, Token: r.Token, Epoch: r.Epoch, Mode: "graph", Team: p.Team, SpecDigest: p.SpecDigest,
			UserInput: fmt.Sprintf("案件号：%s。任务：%s", r.CaseID, p.Goal),
			Members:   members, MemberMCPURL: s.cfg.MemberMCPURL, LeaseSeconds: s.cfg.LeaseSeconds}, true
	}
	return nil, false
}

var errFenced = fmt.Errorf("fenced: this executor's lease was taken over")

// fence checks the caller's token; must be called with s.mu held.
func (s *Service) fence(runID, token string) (*Run, error) {
	r, ok := s.st.Runs[runID]
	if !ok {
		return nil, fmt.Errorf("unknown run")
	}
	if r.Token != token {
		return nil, errFenced
	}
	return r, nil
}

func (s *Service) Heartbeat(runID, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.fence(runID, token)
	if err != nil {
		return err
	}
	r.LeaseUntil = time.Now().Add(time.Duration(s.cfg.LeaseSeconds) * time.Second)
	return nil
}

// StepGet returns the recorded output if the step already ran (the replay
// reuses it, no model call); otherwise marks it running.
func (s *Service) StepGet(runID, token, step string) (bool, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.fence(runID, token)
	if err != nil {
		return false, "", err
	}
	st, ok := r.Steps[step]
	if !ok {
		return false, "", fmt.Errorf("step %q is not in the approved plan", step)
	}
	if st.Status == "done" {
		st.Reused++
		s.save()
		return true, st.Output, nil
	}
	st.Status, st.StartedAt, st.Epoch = "running", now(), r.Epoch
	st.Activity = startActivity[expertByID(st.Expert).Kind]
	s.save()
	return false, "", nil
}

// toolActs are short enough to fit under a node in the live graph.
var toolActs = map[string]string{
	"get_application":      "读申请材料",
	"get_financials":       "读财务数据",
	"get_industry_report":  "读行业报告",
	"query_bureau_summary": "查征信摘要",
	"query_bureau_full":    "查完整征信",
	"get_related_parties":  "读关联方名单",
	"query_company_bureau": "查关联企业征信",
}

var startActivity = map[string]string{
	"investigate": "开始调查",
	"blackbox":    "拆分子目标",
	"judge":       "按规约判断",
	"write":       "撰写尽调报告",
	"effect":      "准备经网关提交",
}

func (s *Service) StepPut(runID, token, step, output string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.fence(runID, token)
	if err != nil {
		return err
	}
	st, ok := r.Steps[step]
	if !ok {
		return fmt.Errorf("step %q is not in the approved plan", step)
	}
	if st.Status == "done" {
		return nil // first committed result wins
	}
	st.Status, st.Output, st.FinishedAt = "done", output, now()
	s.save()
	return nil
}

// BranchEvent records a fork of a black-box step: the inside of the box is
// not planned ahead, but what it did is still on the record.
func (s *Service) BranchEvent(runID, token, step, goal, status string, chars int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.fence(runID, token)
	if err != nil {
		return err
	}
	st, ok := r.Steps[step]
	if !ok {
		return fmt.Errorf("step %q is not in the approved plan", step)
	}
	if status == "finished" {
		st.Activity = "分支完成：" + goal
		for i := len(st.Branches) - 1; i >= 0; i-- {
			if st.Branches[i].Goal == goal && st.Branches[i].Status == "started" {
				st.Branches[i].Status, st.Branches[i].Chars = "finished", chars
				s.save()
				return nil
			}
		}
	}
	st.Activity = "分叉：" + goal
	st.Branches = append(st.Branches, Branch{Goal: goal, Status: status, Chars: chars, At: now()})
	s.save()
	return nil
}

func (s *Service) Done(runID, token string, skipped []string, final string, failed string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.fence(runID, token)
	if err != nil {
		return err
	}
	for _, id := range skipped {
		if st, ok := r.Steps[id]; ok && st.Status != "done" {
			st.Status = "skipped"
		}
	}
	r.Final, r.FinishedAt = final, now()
	r.Status = "done"
	if failed != "" {
		r.Status = "failed"
		r.Final = failed
	}
	r.Token = ""
	s.save()
	return nil
}

func (s *Service) judgePrompt() string {
	var b strings.Builder
	q, _ := s.spec["question"].(map[string]any)
	fmt.Fprintf(&b, "你是风险判断员（S1）。只按已发布的判断规约 %v@%v 作判断，不得自行补充规则。\n\n规则（precedence 越小越优先）：\n", q["question_id"], q["spec_version"])
	if rules, ok := s.spec["rules"].([]any); ok {
		for _, r := range rules {
			if m, ok := r.(map[string]any); ok {
				fmt.Fprintf(&b, "- precedence %v：%v；例外：%v\n", m["precedence"], m["coverage"], m["exceptions"])
			}
		}
	}
	if res, ok := s.spec["resolution"].(map[string]any); ok {
		fmt.Fprintf(&b, "\n证据缺失时：%v\n规则未覆盖时：%v\n", res["on_missing_evidence"], res["on_undefined_rule"])
	}
	fmt.Fprintf(&b, "\n术语：资产单位为 USD-millions；逾期按征信记录中的逾期事件计，近六个月窗口只约束上档规则。\n")
	fmt.Fprintf(&b, "规则里的资产和逾期都只指申请人本人：关联方、行业等其它上游产物写进报告，不作为规则事实。\n")
	fmt.Fprintf(&b, "\n可选结论：%s。\n", strings.Join(s.levels, " | "))
	b.WriteString("\n只输出一个 JSON 对象，不要输出其它文字：\n{\"level\": \"<可选结论之一>\", \"rationale\": \"<一句话依据，引用具体数字>\", \"evidence_steps\": [\"<你依据的上游步骤 id>\"]}\n")
	return b.String()
}

func extractJSON(raw string) (map[string]any, error) {
	i, j := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	if i < 0 || j <= i {
		return nil, fmt.Errorf("no JSON object in the judgment")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw[i:j+1]), &m); err != nil {
		return nil, fmt.Errorf("judgment is not valid JSON: %v", err)
	}
	return m, nil
}

func sha(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

var effectGrants = []ledger.Grant{{Kind: "tool", Selector: "credit.apply", Actions: []string{"execute"}, Effect: "irreversible"}}

func liveGrants() []ledger.Grant {
	out := []ledger.Grant{}
	for _, g := range callerHolds {
		out = append(out, ledger.Grant{Kind: g.kind, Selector: g.selector, Actions: g.actions, Effect: g.effect})
	}
	return out
}

// Judgment validates the S1 output against the published spec and adopts it
// through the festival adoption machine. Returns the normalized step output.
func (s *Service) Judgment(runID, token, step, raw string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.fence(runID, token)
	if err != nil {
		return "", err
	}
	j, err := extractJSON(raw)
	if err != nil {
		return "", err
	}
	level := fmt.Sprint(j["level"])
	if !contains(s.levels, level) {
		return "", fmt.Errorf("level %q is not one of %v", level, s.levels)
	}
	var evidence []string
	if ev, ok := j["evidence_steps"].([]any); ok {
		for _, e := range ev {
			id := strings.Trim(strings.TrimSpace(fmt.Sprint(e)), "[]")
			id = strings.TrimPrefix(id, "step:")
			st, ok := r.Steps[id]
			if !ok || st.Status != "done" {
				return "", fmt.Errorf("evidence step %q did not run in this case", id)
			}
			evidence = append(evidence, id+"="+st.Output)
		}
	}
	if len(evidence) == 0 {
		return "", fmt.Errorf("evidence_steps is empty")
	}
	m, err := s.machine(runID)
	if err != nil {
		return "", err
	}
	decision := map[string]any{"level": level, "rationale": j["rationale"], "evidence_steps": j["evidence_steps"], "case": r.CaseID}
	digest, err := adopt.Digest(decision)
	if err != nil {
		return "", err
	}
	grants := []ledger.Grant{{Kind: "data", Selector: "data:credit/evidence/*", Actions: []string{"read"}, Effect: "none"}}
	if strings.HasPrefix(level, "grant-") {
		grants = effectGrants
	}
	q, _ := s.spec["question"].(map[string]any)
	op := "op-" + r.ID
	req := &adopt.Request{
		AdoptionVersion: "0.1.0",
		Identity:        ledger.Identity{BusinessRequestID: "br-" + strings.ToLower(r.CaseID) + "-" + strings.ToLower(r.ID), EvaluationRevision: "er-1", OperationID: op},
		Versions: ledger.Versions{Evidence: sha(evidence...), Rule: fmt.Sprintf("%v@%v", q["question_id"], q["spec_version"]),
			Model: s.cfg.Model, Readout: "s1-readout-1", Policy: r.PlanHash},
		PayloadDigest:    digest,
		GrantsAtDecision: grants,
		Accountability:   &ledger.Accountability{Session: r.ID, Skill: "risk-judge", Role: "customer-manager"},
	}
	out := m.Adopt(req)
	reqDoc := jsonDoc(req)
	r.ReplaySteps = append(r.ReplaySteps, map[string]any{"op": "adopt", "request": reqDoc})
	if out.Status != "adopted" && out.Status != "reused" {
		s.save()
		return "", fmt.Errorf("adoption %s: %s", out.Status, out.Detail)
	}
	r.Decision, r.DecisionOp = decision, op
	r.BusinessReq = req.Identity.BusinessRequestID
	if st := r.Steps[step]; st != nil {
		st.Activity = "判断已采纳：" + level
	}
	s.save()
	return fmt.Sprintf("DECISION=%s\n依据：%v\n证据步骤：%v", strings.ToUpper(level), j["rationale"], j["evidence_steps"]), nil
}

func (s *Service) record(r *Run, op string, extra map[string]any) {
	step := map[string]any{"op": op, "op_id": r.DecisionOp}
	for k, v := range extra {
		step[k] = v
	}
	r.ReplaySteps = append(r.ReplaySteps, jsonDoc(step))
}

// Effect submits the credit application through the gateway: the adoption
// machine must permit the dispatch, the bank is called with the business
// request id as idempotency key, and an unknown outcome is queried, never
// blindly resent.
func (s *Service) Effect(runID, token, step string) (string, error) {
	s.mu.Lock()
	r, err := s.fence(runID, token)
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	if r.Decision == nil {
		s.mu.Unlock()
		return "", fmt.Errorf("no adopted judgment: nothing may be dispatched")
	}
	level := fmt.Sprint(r.Decision["level"])
	if !strings.HasPrefix(level, "grant-") {
		s.mu.Unlock()
		return "", fmt.Errorf("adopted judgment is %s: no credit application is allowed", level)
	}
	m, err := s.machine(runID)
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	op, br := r.DecisionOp, r.BusinessReq
	state := m.State(op)
	if state == adopt.StateAdopted {
		out := m.Dispatch(op, liveGrants())
		s.record(r, "dispatch", map[string]any{"grants": liveGrants()})
		if st := r.Steps[step]; st != nil {
			st.Activity = "已派发，等银行回执"
		}
		if out.Status != "dispatched" {
			s.save()
			s.mu.Unlock()
			return "", fmt.Errorf("dispatch %s: %s", out.Status, out.Detail)
		}
		s.save()
		s.mu.Unlock()
		if _, err := s.bank.Submit(br, r.Scenario); err != nil {
			s.mu.Lock()
			m.MarkUnresolved(op)
			s.record(r, "mark_unresolved", nil)
			r.Receipt = "结果未知：" + err.Error() + "（不重发，改为查询）"
			r.ReceiptOf = "unknown"
			s.save()
			s.mu.Unlock()
		}
	} else {
		s.mu.Unlock()
	}
	// Wait for the provider's terminal answer by querying it.
	deadline := time.Now().Add(40 * time.Second)
	status := ""
	for time.Now().Before(deadline) {
		status = s.bank.Query(br)
		if status == "approved" || status == "declined" || status == "cancelled" {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch m.State(op) {
	case adopt.StateDispatched, adopt.StateUnresolved:
		switch status {
		case "approved":
			m.MarkCompleted(op)
			s.record(r, "mark_completed", nil)
		case "declined", "cancelled":
			m.MarkFailed(op)
			s.record(r, "mark_failed", nil)
		}
	}
	line := strings.TrimPrefix(level, "grant-")
	r.Receipt = fmt.Sprintf("银行回执：%s（业务请求 %s，额度 %s USD）", status, br, line)
	r.ReceiptOf = status
	s.save()
	return fmt.Sprintf("RECEIPT=%s\n业务请求：%s\n额度：%s USD", strings.ToUpper(status), br, line), nil
}

// ToolCall serves a member's read-only data tool: the step's contract must
// grant the selector and the case must be this run's; permitted reads go to
// the step's evidence log, refused ones are recorded as violations.
func (s *Service) ToolCall(runID, token, step, tool, arg string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.fence(runID, token)
	if err != nil {
		return nil, err
	}
	st, ok := r.Steps[step]
	if !ok {
		return nil, fmt.Errorf("step %q is not in the approved plan", step)
	}
	e := expertByID(st.Expert)
	t := memberToolByName(tool)
	if t == nil {
		return nil, fmt.Errorf("unknown tool %s", tool)
	}
	sel := t.selector(arg)
	refuse := func(why string) (map[string]any, error) {
		st.Activity = "调用被拒：" + why
		r.Violations = append(r.Violations, fmt.Sprintf("%s %s（%s）调用 %s(%s) 被拒绝：%s", now(), step, e.Name, tool, arg, why))
		s.save()
		return nil, fmt.Errorf("refused by contract: %s", why)
	}
	if !contains(e.Tools, tool) {
		return refuse("该专家的契约没有这个工具")
	}
	if !allowedBy(e, sel) {
		return refuse("契约未授权读取 " + sel)
	}
	if t.arg == "case_id" && !strings.EqualFold(arg, r.CaseID) {
		return refuse("只能读取本案件 " + r.CaseID + " 的数据")
	}
	if t.arg == "company_id" && !relatedTo(r.CaseID, arg) {
		return refuse(arg + " 不是本案件的关联方")
	}
	doc, ok := t.fetch(arg)
	if !ok {
		return nil, fmt.Errorf("no data for %s", sel)
	}
	l, err := s.evidLog(runID, step)
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(doc)
	if err := l.Append(evidlog.Entry{Tool: tool, Selector: sel, Caller: step, At: now(), ResultRef: sha(string(raw))}); err != nil {
		return nil, err
	}
	st.Activity = toolActs[tool] + " · " + arg
	if t.arg == "company_id" {
		// Branches run side by side under one step; the company id in the
		// sub-goal tells which branch made this call.
		for i := len(st.Branches) - 1; i >= 0; i-- {
			b := &st.Branches[i]
			if id, _, _ := relatedParty(r.CaseID, b.Goal); b.Status == "started" && strings.EqualFold(id, arg) {
				b.Read, b.Activity = id, toolActs[tool]
				break
			}
		}
	}
	return doc, nil
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

// branches must be called with s.mu held.
