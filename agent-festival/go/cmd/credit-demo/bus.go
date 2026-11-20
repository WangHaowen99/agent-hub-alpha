package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/openjiuwen/agent-festival/go/internal/rfc8785"
)

// Only logical capabilities are configured here. RDC owns cards and instances;
// neither the planner nor this control overlay supplies a runtime address.
// busAgents (expert id -> RDC agent id) comes from the capability cards'
// bus_agent field; see scenario.go.

type BusBinding struct {
	AgentID        string `json:"agent_id"`
	Version        string `json:"version"`
	CardDigest     string `json:"card_digest"`
	ContractDigest string `json:"contract_digest"`
}

type RemoteCall struct {
	NodeExecutionID  string                     `json:"node_execution_id"`
	GraphRevision    string                     `json:"graph_revision"`
	NodePath         string                     `json:"node_path"`
	LoopIteration    int                        `json:"loop_iteration"`
	Attempt          int                        `json:"attempt"`
	Binding          BusBinding                 `json:"binding"`
	InputDigest      string                     `json:"input_digest"`
	Input            string                     `json:"input"`
	Goal             string                     `json:"goal"`
	RequestID        string                     `json:"request_id"`
	MessageID        string                     `json:"message_id"`
	ContextID        string                     `json:"context_id"`
	RemoteTaskID     string                     `json:"remote_task_id,omitempty"`
	Status           string                     `json:"status"`
	Reason           string                     `json:"reason,omitempty"`
	Sequence         int64                      `json:"sequence"`
	CreatedAt        string                     `json:"created_at"`
	UpdatedAt        string                     `json:"updated_at"`
	AgentStartedAt   string                     `json:"agent_started_at,omitempty"`
	ArtifactID       string                     `json:"artifact_id,omitempty"`
	ArtifactDigest   string                     `json:"artifact_digest,omitempty"`
	ModelEvents      map[string]string          `json:"model_events"`
	ActualModels     map[string]string          `json:"actual_models,omitempty"` // model event id -> model the runtime reported
	Discovery        *DiscoverySnapshot         `json:"discovery,omitempty"`
	GatewayExchanges map[string]GatewayExchange `json:"gateway_exchanges,omitempty"`
	// This scoped credential is persisted privately and never projected to UI.
	CapabilityToken string `json:"capability_token"`
}

type CallCommand struct {
	RunID           string           `json:"run_id"`
	Token           string           `json:"token"`
	Step            string           `json:"step"`
	Input           string           `json:"input,omitempty"`
	Goal            string           `json:"goal,omitempty"`
	NodeExecutionID string           `json:"node_execution_id,omitempty"`
	InputDigest     string           `json:"input_digest,omitempty"`
	Attempt         int              `json:"attempt,omitempty"`
	RemoteTaskID    string           `json:"remote_task_id,omitempty"`
	ContextID       string           `json:"context_id,omitempty"`
	Status          string           `json:"status,omitempty"`
	Reason          string           `json:"reason,omitempty"`
	Sequence        int64            `json:"sequence,omitempty"`
	Output          string           `json:"output,omitempty"`
	ServiceID       string           `json:"service_id,omitempty"`
	ServiceVersion  string           `json:"service_version,omitempty"`
	EventID         string           `json:"event_id,omitempty"`
	Gateway         *GatewayExchange `json:"gateway,omitempty"`
	Model           string           `json:"model,omitempty"` // the model the remote runtime is about to call
}

var errCallConflict = errors.New("node input, contract or result conflict")

func canonicalHash(value any) (string, error) {
	raw, err := rfc8785.Canonicalize(jsonDoc(value))
	if err != nil {
		return "", err
	}
	return sha(string(raw)), nil
}

func (s *Service) readBinding(expert string) (BusBinding, error) {
	binding, _, err := s.observeBinding(expert)
	return binding, err
}

// discoveryClient is shared by all RDC lookups. Each request is bounded; the
// lookup runs outside the service lock (see BeginCall).
var discoveryClient = &http.Client{Timeout: 5 * time.Second}

func (s *Service) observeBinding(expert string) (BusBinding, *DiscoverySnapshot, error) {
	id := busAgents[expert]
	if id == "" || s.cfg.BusRegistryURL == "" || s.cfg.BusRegistryToken == "" {
		return BusBinding{}, nil, fmt.Errorf("real capability discovery is not configured for %s", expert)
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(s.cfg.BusRegistryURL, "/")+"/v2/agents/"+url.PathEscape(id)+"/card", nil)
	if err != nil {
		return BusBinding{}, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.BusRegistryToken)
	req.Header.Set("X-Tenant-ID", s.cfg.BusTenant)
	resp, err := discoveryClient.Do(req)
	if err != nil {
		return BusBinding{}, nil, fmt.Errorf("RDC discovery unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return BusBinding{}, nil, fmt.Errorf("RDC card %s unavailable: HTTP %d", id, resp.StatusCode)
	}
	var card map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&card); err != nil {
		return BusBinding{}, nil, err
	}
	for _, key := range []string{"rawJson", "agentCardJson", "agentCard", "card"} {
		if raw, ok := card[key].(string); ok {
			if err := json.Unmarshal([]byte(raw), &card); err != nil {
				return BusBinding{}, nil, err
			}
			break
		}
		if nested, ok := card[key].(map[string]any); ok {
			card = nested
			break
		}
	}
	name, _ := card["name"].(string)
	version, _ := card["version"].(string)
	if name != id || version == "" {
		return BusBinding{}, nil, fmt.Errorf("RDC card identity/version invalid for %s", id)
	}
	instancesRequest, _ := http.NewRequest(http.MethodGet, strings.TrimRight(s.cfg.BusRegistryURL, "/")+"/v2/agents/"+url.PathEscape(id)+"/instances", nil)
	instancesRequest.Header = req.Header.Clone()
	instancesResponse, err := discoveryClient.Do(instancesRequest)
	if err != nil {
		return BusBinding{}, nil, fmt.Errorf("RDC instances unavailable: %w", err)
	}
	defer instancesResponse.Body.Close()
	var discovery struct {
		Instances []struct {
			InstanceID        string `json:"instanceId"`
			Health            string `json:"health"`
			ContractVersion   string `json:"contractVersion"`
			CapabilityVersion string `json:"capabilityVersion"`
		} `json:"instances"`
	}
	if instancesResponse.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(instancesResponse.Body, 1<<20)).Decode(&discovery) != nil {
		return BusBinding{}, nil, fmt.Errorf("RDC instance discovery failed for %s", id)
	}
	snapshot := &DiscoverySnapshot{CheckedAt: now(), Candidates: []DiscoveredInstance{}}
	available := false
	for _, instance := range discovery.Instances {
		eligible := instance.Health == "ONLINE" && instance.ContractVersion == "1.0" && instance.CapabilityVersion == version
		snapshot.Candidates = append(snapshot.Candidates, DiscoveredInstance{InstanceID: instance.InstanceID, Health: instance.Health, ContractVersion: instance.ContractVersion, CapabilityVersion: instance.CapabilityVersion, Eligible: eligible})
		if instance.Health == "ONLINE" && instance.ContractVersion == "1.0" && instance.CapabilityVersion == version {
			available = true
		}
	}
	if !available {
		return BusBinding{}, nil, fmt.Errorf("no healthy A2A 1.0 instance matching capability %s version %s", id, version)
	}
	// Routing addresses may change; capability semantics may not silently change.
	semantic := map[string]any{}
	for _, key := range []string{"name", "version", "description", "skills", "capabilities", "defaultInputModes", "defaultOutputModes"} {
		semantic[key] = card[key]
	}
	digest, err := canonicalHash(semantic)
	if err != nil {
		return BusBinding{}, nil, err
	}
	return BusBinding{AgentID: id, Version: version, CardDigest: digest, ContractDigest: s.cat.Hashes[expert]}, snapshot, nil
}

func (s *Service) bindPlan(p *Plan) error {
	if s.cfg.BusRegistryURL == "" {
		return nil
	}
	p.BusBindings = map[string]BusBinding{}
	for _, a := range p.Agents {
		if busAgents[a.Expert] == "" {
			continue
		}
		b, err := s.readBinding(a.Expert)
		if err != nil {
			return err
		}
		p.BusBindings[a.ID] = b
	}
	return nil
}

func (s *Service) checkBindings(p *Plan) error {
	if s.cfg.BusRegistryURL != "" {
		for _, agent := range p.Agents {
			if busAgents[agent.Expert] != "" {
				if _, ok := p.BusBindings[agent.ID]; !ok {
					return fmt.Errorf("plan has no approved Agent Bus binding for %s; propose a new version", agent.ID)
				}
			}
		}
	}
	for step, expected := range p.BusBindings {
		a := p.agent(step)
		if a == nil {
			return errCallConflict
		}
		actual, err := s.readBinding(a.Expert)
		if err != nil {
			return err
		}
		if actual != expected {
			return fmt.Errorf("%w: capability changed for %s; propose a new plan", errCallConflict, step)
		}
	}
	return nil
}

func (s *Service) capabilityCards() []map[string]any {
	cards := s.cat.Cards()
	if s.cfg.BusRegistryURL == "" {
		return cards
	}
	for _, card := range cards {
		id, _ := card["id"].(string)
		if busAgents[id] == "" {
			continue
		}
		binding, err := s.readBinding(id)
		card["remote"] = true
		card["available"] = err == nil
		if err == nil {
			card["bus_binding"] = binding
		} else {
			card["unavailable_reason"] = err.Error()
		}
	}
	return cards
}

func copyCall(c *RemoteCall) *RemoteCall {
	raw, _ := json.Marshal(c)
	var result RemoteCall
	_ = json.Unmarshal(raw, &result)
	return &result
}

// beginSnapshot is what BeginCall checked under the lock before leaving it
// for capability discovery; the send intent is written only if it still holds.
type beginSnapshot struct {
	epoch   int
	plan    string
	binding BusBinding
	digest  string
	expert  string
}

// beginCheck validates the caller and the node under s.mu. If the node already
// has a call it answers from that call (reuse/query/unknown) and returns a
// non-nil result; otherwise it returns the snapshot discovery must confirm.
func (s *Service) beginCheck(in CallCommand) (map[string]any, *beginSnapshot, error) {
	r, err := s.fence(in.RunID, in.Token)
	if err != nil {
		return nil, nil, err
	}
	st := r.Steps[in.Step]
	if st == nil {
		return nil, nil, fmt.Errorf("node is not in the approved graph")
	}
	p := s.st.Plans[r.PlanID][r.PlanVersion-1]
	binding, ok := p.BusBindings[in.Step]
	if !ok {
		return nil, nil, fmt.Errorf("node has no approved remote capability")
	}
	digest, err := canonicalHash(map[string]any{"input": in.Input, "goal": in.Goal, "binding": binding, "model": s.runModel(r)})
	if err != nil {
		return nil, nil, err
	}
	if st.Call != nil {
		c := st.Call
		if c.InputDigest != digest || c.Binding != binding {
			return nil, nil, errCallConflict
		}
		action := "unknown"
		if st.Status == "done" {
			action = "reuse"
			st.Reused++
			if err := s.save(); err != nil {
				return nil, nil, err
			}
		} else if c.RemoteTaskID != "" {
			action = "query"
		}
		return map[string]any{"action": action, "call": copyCall(c), "output": st.Output}, nil, nil
	}
	return nil, &beginSnapshot{epoch: r.Epoch, plan: p.Key(), binding: binding, digest: digest, expert: st.Expert}, nil
}

// BeginCall records the send intent of a remote node (AR-05, #27). Capability
// discovery talks to RDC outside the service lock, so a slow or unreachable
// registry never holds up heartbeats, other nodes' results or reads; after
// discovery everything checked before is checked again under the lock, and
// any drift (lease taken over, plan or binding changed, another request
// already wrote the intent) prevents a send from a stale snapshot.
func (s *Service) BeginCall(in CallCommand) (map[string]any, error) {
	s.mu.Lock()
	out, snap, err := s.beginCheck(in)
	s.mu.Unlock()
	if err != nil || out != nil {
		return out, err
	}

	actual, discovery, err := s.observeBinding(snap.expert)

	s.mu.Lock()
	defer s.mu.Unlock()
	out, again, cerr := s.beginCheck(in)
	if cerr != nil || out != nil {
		// fenced, conflicting, or another request recorded the intent meanwhile
		return out, cerr
	}
	if err != nil {
		return nil, err
	}
	if *again != *snap {
		return nil, fmt.Errorf("%w: run state changed during capability discovery", errCallConflict)
	}
	if actual != snap.binding {
		return nil, errCallConflict
	}
	r := s.st.Runs[in.RunID]
	st := r.Steps[in.Step]
	p := s.st.Plans[r.PlanID][r.PlanVersion-1]
	binding, digest := snap.binding, snap.digest
	node := sha(r.ID, p.Key(), in.Step, "iteration:0")
	c := &RemoteCall{NodeExecutionID: node, GraphRevision: p.Key(), NodePath: in.Step, LoopIteration: 0, Attempt: 1, Binding: binding,
		InputDigest: digest, Input: in.Input, Goal: in.Goal, RequestID: newToken(), MessageID: newToken(), ContextID: "ctx-" + newToken(),
		Status: "dispatching", CreatedAt: now(), UpdatedAt: now(), CapabilityToken: newToken() + newToken(), ModelEvents: map[string]string{}, Discovery: discovery}
	st.Call, st.Status, st.StartedAt, st.Epoch = c, "running", now(), r.Epoch
	st.Activity = "经 Agent Bus 调用专业 Agent"
	if err := s.save(); err != nil {
		return nil, err
	}
	return map[string]any{"action": "send", "call": copyCall(c)}, nil
}

func (s *Service) callFor(in CallCommand) (*Run, *StepRec, error) {
	r, err := s.fence(in.RunID, in.Token)
	if err != nil {
		return nil, nil, err
	}
	st := r.Steps[in.Step]
	if st == nil || st.Call == nil {
		return nil, nil, fmt.Errorf("remote call does not exist")
	}
	c := st.Call
	if in.NodeExecutionID != c.NodeExecutionID || in.InputDigest != c.InputDigest || in.Attempt != c.Attempt {
		return nil, nil, errCallConflict
	}
	return r, st, nil
}

func (s *Service) UpdateCall(in CallCommand) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, st, err := s.callFor(in)
	if err != nil {
		return err
	}
	c := st.Call
	if in.RemoteTaskID != "" && c.RemoteTaskID != "" && in.RemoteTaskID != c.RemoteTaskID {
		return errCallConflict
	}
	if in.ContextID != "" && in.ContextID != c.ContextID {
		return errCallConflict
	}
	if st.Status == "done" || st.Status == "failed" || in.Sequence <= c.Sequence {
		return nil
	}
	if !contains([]string{"waiting", "unknown", "failed"}, in.Status) {
		return fmt.Errorf("invalid remote status")
	}
	if err := recordGateway(c, in); err != nil {
		return err
	}
	if in.RemoteTaskID != "" {
		c.RemoteTaskID = in.RemoteTaskID
	}
	c.Status, c.Reason, c.Sequence, c.UpdatedAt = in.Status, clip(in.Reason, 400), in.Sequence, now()
	st.Status, st.Activity = in.Status, c.Reason
	return s.save()
}

func (s *Service) CommitCall(in CallCommand) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, st, err := s.callFor(in)
	if err != nil {
		return "", err
	}
	c := st.Call
	if st.Status == "failed" {
		return "", errCallConflict
	}
	if in.ContextID != c.ContextID || in.RemoteTaskID == "" || (c.RemoteTaskID != "" && c.RemoteTaskID != in.RemoteTaskID) {
		return "", errCallConflict
	}
	if strings.TrimSpace(in.Output) == "" {
		return "", fmt.Errorf("empty remote artifact")
	}
	digest := sha(in.Output)
	if st.Status == "done" {
		if c.ArtifactDigest != digest {
			return "", errCallConflict
		}
		return st.Output, nil
	}
	c.RemoteTaskID, c.Status, c.UpdatedAt = in.RemoteTaskID, "completed", now()
	c.Reason = ""
	c.ArtifactID, c.ArtifactDigest = "artifact-"+strings.TrimPrefix(c.NodeExecutionID, "sha256:"), digest
	st.Output, st.Status, st.FinishedAt = "[step:"+in.Step+"]\n"+in.Output, "done", now()
	st.Activity = "专业 Agent 结果已持久提交"
	if err := s.save(); err != nil {
		return "", err
	}
	return st.Output, nil
}

// Agent authorization survives a host lease change, but only grants this
// node's read-only tools and model accounting, never executor/control writes.
func (s *Service) agentCall(in CallCommand) (*Run, *StepRec, error) {
	if s.storageErr != nil {
		return nil, nil, s.storageErr
	}
	r := s.st.Runs[in.RunID]
	if r == nil {
		return nil, nil, errFenced
	}
	if r.Status == "blocked" {
		// A blocked run grants no new execution start or model permit to any of
		// its nodes; withdrawing the host lease alone does not stop remote
		// runtimes that still hold a node credential (AR-04 review P1). Reads
		// and audit go through other endpoints and are not affected.
		return nil, nil, errFenced
	}
	st := r.Steps[in.Step]
	if st == nil || st.Call == nil || in.Token == "" || st.Call.CapabilityToken != in.Token {
		return nil, nil, errFenced
	}
	if st.Status == "done" || st.Call.Status == "failed" {
		return nil, nil, errFenced
	}
	return r, st, nil
}

func (s *Service) AgentContext(in CallCommand) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, st, err := s.agentCall(in)
	if err != nil {
		return nil, err
	}
	c := st.Call
	if in.ServiceID != c.Binding.AgentID || in.ContextID != c.ContextID || in.ServiceVersion != c.Binding.Version {
		return nil, errFenced
	}
	if c.AgentStartedAt != "" {
		return nil, fmt.Errorf("remote execution already started; query original task")
	}
	c.AgentStartedAt = now()
	if err := s.save(); err != nil {
		return nil, err
	}
	return map[string]any{"call": copyCall(c), "expert": st.Expert, "tools": expertByID(st.Expert).Tools, "model": s.runModel(r)}, nil
}

// ModelCall is the remote runtime's record of a model call, made before the
// call. The runtime reports the model it is about to use; anything but the
// run's bound model stops the run as blocked and is refused, so the call is
// not made (AR-04 M1).
func (s *Service) ModelCall(in CallCommand) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, st, err := s.agentCall(in)
	if err != nil {
		return err
	}
	if in.EventID == "" || len(in.EventID) > 150 {
		return fmt.Errorf("model event id required")
	}
	if bound := s.runModel(r); in.Model != bound {
		reason := modelMismatch(bound, "远端 "+in.ServiceID, in.Model)
		st.Call.Reason = reason
		s.blockRun(r, reason)
		if err := s.save(); err != nil {
			return err
		}
		return fmt.Errorf("%w: %s", errModelBinding, reason)
	}
	if st.Call.ModelEvents[in.EventID] != "" {
		return nil
	}
	st.Call.ModelEvents[in.EventID] = now()
	if st.Call.ActualModels == nil {
		st.Call.ActualModels = map[string]string{}
	}
	st.Call.ActualModels[in.EventID] = in.Model
	return s.save()
}

func (s *Service) ResumeRun(runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.storageErr != nil {
		return s.storageErr
	}
	r := s.st.Runs[runID]
	if r == nil || r.Status != "waiting" {
		return fmt.Errorf("run is not waiting")
	}
	r.Status, r.Final = "queued", ""
	return s.save()
}

func (s *Service) busRoutes(mux *http.ServeMux) {
	for _, operation := range []string{"begin", "update", "commit", "context", "model"} {
		prefix := "/exec/call/"
		if operation == "context" || operation == "model" {
			prefix = "/agent/"
		}
		mux.HandleFunc("POST "+prefix+operation, func(w http.ResponseWriter, r *http.Request) {
			var in CallCommand
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
			decoder.DisallowUnknownFields()
			err := decoder.Decode(&in)
			var out any = map[string]bool{"ok": true}
			if err == nil {
				switch operation {
				case "begin":
					out, err = s.BeginCall(in)
				case "update":
					err = s.UpdateCall(in)
				case "commit":
					var result string
					result, err = s.CommitCall(in)
					out = map[string]string{"output": result}
				case "context":
					out, err = s.AgentContext(in)
				case "model":
					err = s.ModelCall(in)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			if err != nil {
				status, code := errorCode(err)
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error(), "code": code})
				return
			}
			_ = json.NewEncoder(w).Encode(out)
		})
	}
}
