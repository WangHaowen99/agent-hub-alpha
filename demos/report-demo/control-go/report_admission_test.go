package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func reportSample(t *testing.T) (Catalog, Graph) {
	c, g := sample(t)
	g.Nodes = append(g.Nodes, Node{ID: "check", Expert: "checker@1", Task: "检查同一报告", DependsOn: []string{"draft"}}, Node{ID: "select", Expert: "select@1", Task: "选择已核验报告", DependsOn: []string{"check"}})
	return c, g
}

func TestReportAdmissionRequiresCheckedDeliveryPath(t *testing.T) {
	c, g := reportSample(t)
	if err := ValidateReportGraph(g, c); err != nil {
		t.Fatal(err)
	}
	_, compute := sample(t)
	if err := ValidateGraph(compute, c); err != nil {
		t.Fatal("internal compute fixture rejected", err)
	}
	if err := ValidateReportGraph(compute, c); err == nil {
		t.Fatal("unchecked report admitted")
	}
	for _, kind := range []string{"no-selector", "selector-skips-initial-check", "conditional-selector", "unchecked-revision", "omitted-check", "wrong-check-branch"} {
		t.Run(kind, func(t *testing.T) {
			c, g := reportSample(t)
			switch kind {
			case "no-selector":
				g.Nodes = g.Nodes[:4]
			case "conditional-selector":
				g.Nodes[4].Condition = &Condition{"check", "PASS"}
			default:
				g.Nodes = append(g.Nodes, Node{ID: "revision", Expert: "writer@1", Task: "定向修订", DependsOn: []string{"draft", "check"}, Condition: &Condition{"check", "REVISE"}})
				if kind != "unchecked-revision" {
					g.Nodes = append(g.Nodes, Node{ID: "recheck", Expert: "checker@1", Task: "复检", DependsOn: []string{"revision", "check"}, Condition: &Condition{"check", "REVISE"}})
					if kind == "wrong-check-branch" {
						g.Nodes[6].Condition.Outcome = "PASS"
					}
					if kind != "omitted-check" {
						g.Nodes[4].DependsOn = append(g.Nodes[4].DependsOn, "recheck")
					}
					if kind == "selector-skips-initial-check" {
						g.Nodes[4].DependsOn = []string{"recheck"}
					}
				}
			}
			if ValidateReportGraph(g, c) == nil {
				t.Fatal("incomplete delivery graph admitted")
			}
		})
	}
	// Legal bounded revision and authorized skip paths remain admissible.
	c, g = reportSample(t)
	g.Nodes = append(g.Nodes, Node{ID: "revision", Expert: "writer@1", Task: "修订", DependsOn: []string{"draft", "check"}, Condition: &Condition{"check", "REVISE"}}, Node{ID: "recheck", Expert: "checker@1", Task: "复检", DependsOn: []string{"revision", "check"}, Condition: &Condition{"check", "REVISE"}})
	g.Nodes[4].DependsOn = append(g.Nodes[4].DependsOn, "recheck")
	if err := ValidateReportGraph(g, c); err != nil {
		t.Fatal("legal revision rejected", err)
	}
}

type admissionAuthTransport struct{ token string }

func (a admissionAuthTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	clone.Header = r.Header.Clone()
	clone.Header.Set("Authorization", "Bearer "+a.token)
	return http.DefaultTransport.RoundTrip(clone)
}

func TestReportPublicIngressRejectsBeforeCompilerOrPersistence(t *testing.T) {
	c, g := sample(t)
	var compiled atomic.Int64
	compiler := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compiled.Add(1)
		writeJSON(w, 200, map[string]string{"runtime_spec_digest": "compiler-fixture"})
	}))
	defer compiler.Close()
	cfg := Config{CompilerURL: compiler.URL, Tokens: map[string]string{"planner": "fixture-admission-planner", "worker": "fixture-admission-worker"}}
	s := NewServer(&Store{}, cfg, c, Material{ID: g.MaterialID})
	api := httptest.NewServer(s.Routes())
	defer api.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "admission-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: api.URL + "/mcp", HTTPClient: &http.Client{Transport: admissionAuthTransport{cfg.Tokens["planner"]}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, graph := range []Graph{g, func() Graph { _, r := reportSample(t); r.Nodes = r.Nodes[:4]; return r }()} {
		raw, _ := json.Marshal(ProposeInput{CommandID: "missing-delivery", Graph: graph})
		req, _ := http.NewRequestWithContext(ctx, "POST", api.URL+"/v1/commands", bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer "+cfg.Tokens["planner"])
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		json.NewDecoder(res.Body).Decode(&body)
		res.Body.Close()
		if res.StatusCode != 422 || body["error"].(map[string]any)["code"] != "POLICY_REJECTED" {
			t.Fatal("HTTP did not reject incomplete report", res.StatusCode, body)
		}
		out, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "propose_plan", Arguments: ProposeInput{CommandID: "missing-delivery", Graph: graph}})
		if err != nil || !out.IsError {
			t.Fatal("MCP did not reject incomplete report", err, out)
		}
		encoded, _ := json.Marshal(out.Content)
		if !strings.Contains(string(encoded), "POLICY_REJECTED") {
			t.Fatal("MCP lost policy reason", string(encoded))
		}
	}
	if compiled.Load() != 0 {
		t.Fatal("invalid graph reached Java compiler")
	}
	// This store has no Pool: reaching persistence would fail the test.
	_, valid := reportSample(t)
	if _, err := s.checkCandidate(ctx, valid); err != nil {
		t.Fatal("valid report preflight rejected", err)
	}
	if compiled.Load() != 1 {
		t.Fatal("valid report did not compile once")
	}
}
