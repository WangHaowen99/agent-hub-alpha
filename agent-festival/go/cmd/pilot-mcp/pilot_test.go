package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newTestPilot(t *testing.T) *Pilot {
	t.Helper()
	p, err := OpenPilot(t.TempDir(), "credit-pilot")
	if err != nil {
		t.Fatal(err)
	}
	p.provider.AcceptDelay = 0
	p.provider.TerminalDelay = 150 * time.Millisecond
	p.provider.CallTimeout = 30 * time.Millisecond
	return p
}

var upperBand = Material{CustomerID: "C001", AssetsWan: 3500, AssetReportOK: true}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 60; i++ {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestSameCommandReturnsOriginalReceiptAndChangedMaterialConflicts(t *testing.T) {
	p := newTestPilot(t)
	user := identify(http.Header{"X-Pilot-User": {"alice"}})
	first := p.Submit("cmd-1", "app", user, upperBand)
	if first.Status != "accepted" || first.RequestID == "" || first.AmountMinor != 5_000_000 {
		t.Fatalf("first submit = %+v", first)
	}
	again := p.Submit("cmd-1", "app", user, upperBand)
	if again.Status != "duplicate" || again.TaskID != first.TaskID {
		t.Fatalf("retry = %+v, want duplicate of %s", again, first.TaskID)
	}
	changed := upperBand
	changed.AssetsWan++
	if c := p.Submit("cmd-1", "app", user, changed); c.Status != "conflict" {
		t.Fatalf("changed material = %+v, want conflict", c)
	}
	if len(p.Tasks) != 1 {
		t.Fatalf("tasks = %d, want 1", len(p.Tasks))
	}
	// The same command id from another user is a different command.
	bob := identify(http.Header{"X-Pilot-User": {"bob"}})
	if other := p.Submit("cmd-1", "app", bob, upperBand); other.Status != "accepted" || other.TaskID == first.TaskID {
		t.Fatalf("other issuer = %+v", other)
	}
}

func TestTimeoutIsUnknownUntilProviderIsQueried(t *testing.T) {
	p := newTestPilot(t)
	m := upperBand
	m.Scenario = "timeout"
	r := p.Submit("cmd-t", "app", identify(http.Header{}), m)
	if r.ControlState != "unresolved" || r.ProviderStatus != "" {
		t.Fatalf("timeout receipt = %+v, want unresolved with unknown provider status", r)
	}
	eventually(t, "provider terminal via query", func() bool {
		v, _ := p.Get(r.TaskID, "")
		return v.ControlState == "completed" && v.Task.Request.ProviderStatus == "approved"
	})
}

func TestDuplicateAndLateReceiptsDoNotRegress(t *testing.T) {
	p := newTestPilot(t)
	m := upperBand
	m.Scenario = "duplicate_receipt"
	r := p.Submit("cmd-d", "app", identify(http.Header{}), m)
	eventually(t, "three observations", func() bool {
		v, _ := p.Get(r.TaskID, "")
		return len(v.Task.Request.Observations) == 3
	})
	v, _ := p.Get(r.TaskID, "")
	got := []string{}
	for _, o := range v.Task.Request.Observations {
		got = append(got, o.Applied)
	}
	if strings.Join(got, ",") != "applied,duplicate,ignored-regression" || v.Task.Request.ProviderStatus != "approved" {
		t.Fatalf("observations = %v, status %s", got, v.Task.Request.ProviderStatus)
	}
}

func TestCancelNeedsProviderConfirmation(t *testing.T) {
	p := newTestPilot(t)
	p.provider.TerminalDelay = time.Hour
	r := p.Submit("cmd-c", "app", identify(http.Header{}), upperBand)
	if ok, _ := p.RequestCancel(r.RequestID); !ok {
		t.Fatal("cancel of a pending request was not confirmed")
	}
	v, _ := p.Get(r.TaskID, "")
	if v.Task.Request.ProviderStatus != "cancelled" || v.Task.Request.Cancel != "confirmed" || v.ControlState != "failed" {
		t.Fatalf("after cancel: %+v control=%s", v.Task.Request, v.ControlState)
	}
	if ok, detail := p.RequestCancel(r.RequestID); ok || detail != "already cancelled" {
		t.Fatalf("second cancel = %v %q", ok, detail)
	}
}

func TestDenyAndUncoveredProduceNoBusinessRequest(t *testing.T) {
	p := newTestPilot(t)
	veto := upperBand
	veto.LongestOverdueMonths = 4
	if r := p.Submit("cmd-v", "model", identify(http.Header{}), veto); r.Judgment != "deny" || r.RequestID != "" {
		t.Fatalf("veto = %+v", r)
	}
	between := Material{CustomerID: "C2", AssetsWan: 500, AssetReportOK: true}
	if r := p.Submit("cmd-b", "model", identify(http.Header{}), between); r.Judgment != "on_undefined_rule" || r.RequestID != "" {
		t.Fatalf("between bands = %+v", r)
	}
}

func TestMCPToolsDeclareConsoleAndResourceIsAnApp(t *testing.T) {
	p := newTestPilot(t)
	ctx := context.Background()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := newServer(p).Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	withUI := map[string]bool{}
	for _, tl := range tools.Tools {
		if ui, ok := tl.Meta["ui"].(map[string]any); ok && ui["resourceUri"] == consoleURI {
			withUI[tl.Name] = true
		}
	}
	if !withUI["pilot_submit_application"] || !withUI["pilot_open_console"] || withUI["pilot_get_task"] {
		t.Fatalf("tools with console ui = %v", withUI)
	}
	res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: consoleURI})
	if err != nil {
		t.Fatal(err)
	}
	if c := res.Contents[0]; c.MIMEType != appMIME || !strings.Contains(c.Text, "<script>") {
		t.Fatalf("console resource mime=%q len=%d", c.MIMEType, len(c.Text))
	}
	out, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "pilot_submit_application", Arguments: map[string]any{
		"command_id": "cmd-m", "customer_id": "C9", "assets_wan": 50}})
	if err != nil || out.IsError {
		t.Fatalf("call = %v %s", err, out.Content[0].(*mcp.TextContent).Text)
	}
	if !strings.Contains(out.Content[0].(*mcp.TextContent).Text, `"amount_minor":1000000`) {
		t.Fatalf("lower band receipt = %s", out.Content[0].(*mcp.TextContent).Text)
	}
}
