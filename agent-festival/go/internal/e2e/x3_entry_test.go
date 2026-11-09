package e2e

import (
	"os"
	"strings"
	"testing"

	"github.com/openjiuwen/agent-festival/go/internal/adopt"
	"github.com/openjiuwen/agent-festival/go/internal/ledger"
)

// X3 (60号 scope §三): the unified-entry acceptance matrix. Expectations
// FROZEN before implementation — each row cites xingchao 198789653 §3/§4/§五.5.
// The matrix IS the judgment truth source (frame-bench state-matrix form):
// new findings in review = new rows, not implementation changes.

// newCountingModel builds a ModelPort that returns the given verdicts
// (keyed by opID, "*" for default) and counts its own invocations.
func newCountingModel(verdicts map[string]creditEvidence) *countingModel {
	cm := &countingModel{verdicts: make(map[string]s1Verdict)}
	for op, ev := range verdicts {
		cm.verdicts[op] = s1Judge(ev)
	}
	return cm
}

// fullGrantsN3 returns grants that pass the E5 read authorization.
func fullGrantsN3() []ledger.Grant {
	return []ledger.Grant{{Kind: "data", Selector: "ledger://credit/e2e", Actions: []string{"read"}, Effect: "none"}}
}

func TestX3_BranchA_RetryZeroPortCalls(t *testing.T) {
	// X3-1: same-instance retry → original receipt + port calls = 0
	// Translation: "首次执行、重试与新任务经过同一入口" (198789653 §五.5)
	h := newT2Host(t, t.TempDir())
	if o := h.adoptAndDispatch("op-x3a", "er-1", "ev-1", t2Digest('e'), "br-x3a",
		"receipt:op-x3a:accepted", []string{"fact"}); o.Status != "dispatched" {
		t.Fatalf("X3-1 setup: dispatch = %q: %s", o.Status, o.Detail)
	}
	if o := h.completeNode("op-x3a"); o.Status != "completed" {
		t.Fatalf("X3-1 setup: complete = %q: %s", o.Status, o.Detail)
	}
	cm := newCountingModel(nil)
	out, err := h.nodeEntryExec("op-x3a", fullGrantsN3(), "history-verification", nil, nil, Ports{Model: cm})
	if err != nil {
		t.Fatalf("X3-1: entry error: %v", err)
	}
	if out.Branch != "A-retry" {
		t.Fatalf("X3-1: branch = %q, want A-retry", out.Branch)
	}
	if out.Result.Receipt != "receipt:op-x3a:accepted" {
		t.Fatalf("X3-1: receipt = %q, want the original — retry must serve the persisted receipt", out.Result.Receipt)
	}
	if cm.calls != 0 {
		t.Fatalf("X3-1: model port calls = %d, want 0 — retry must not invoke the model", cm.calls)
	}
}

func TestX3_BranchB_MethodZeroModelCalls(t *testing.T) {
	// X3-2: new task + valid method → method result + model calls = 0
	// Translation: "有效方法适用时产生新任务自己的结果" (198789653 §五.5)
	h := newT2Host(t, t.TempDir())
	h.registry.mustPublish(t, knowledgeMethod{
		ID: "method-x3", Version: "1.0.0", ServesNode: "history-verification",
		InputFields: []string{"overdue_months", "business_request_id"}, Valid: true,
	})
	cm := newCountingModel(nil)
	out, err := h.nodeEntryExec("op-x3b", fullGrantsN3(), "history-verification",
		map[string]string{"overdue_months": "0", "business_request_id": "br-x3b"}, nil, Ports{Model: cm})
	if err != nil {
		t.Fatalf("X3-2: entry error: %v", err)
	}
	if out.Branch != "B-method" {
		t.Fatalf("X3-2: branch = %q, want B-method", out.Branch)
	}
	if out.Result.BusinessReqID != "br-x3b" {
		t.Fatalf("X3-2: result business_request_id = %q, want br-x3b — must be the new task's own", out.Result.BusinessReqID)
	}
	if !strings.Contains(out.Result.Receipt, "method-x3") {
		t.Fatalf("X3-2: receipt = %q, want method-x3 — the result must come from the method", out.Result.Receipt)
	}
	if cm.calls != 0 {
		t.Fatalf("X3-2: model port calls = %d, want 0 — method execution is deterministic knowledge reuse, not a model call (60号 先验 #1 declared boundary)", cm.calls)
	}
}

func TestX3_BranchC_ModelExactlyOnce(t *testing.T) {
	// X3-3: first execution → model port exactly 1
	// Translation: "实际模型/工具调用通过可替换端口接入" (198789653 §五.5)
	// X3-9 (R74 B-1 清偿补行): the C branch runs the FULL machine flow —
	// 60号 §一.1 frozen spec "Invoke → Adopt→Dispatch→store.put 全流程".
	// The flow is what makes the C path's retry reachable (without it the
	// unified-entry premise is hollow on first executions).
	h := newT2Host(t, t.TempDir()) // no prior op, no method published
	// The evidence carries a NON-EMPTY overdue record (a 2-month overdue 10
	// months ago: outside the six-month window, under the 3-month veto line
	// — verdict stays grant-50000, but the veto-carrier leg is non-zero so
	// the flow pin can discriminate a surgical slice-swallow; R74 iter-3 M-1).
	evidence := creditEvidence{
		assetMillions: 35, assetReportOK: true,
		overdueOnRecord: []overdueEvent{{durationMonths: 2, monthsAgo: 10}},
	}
	cm := newCountingModel(map[string]creditEvidence{"*": evidence})
	first := &firstExecInput{
		req:      requestForT2("op-x3c", "er-1", "ev-1", t2Digest('e'), "br-x3c"),
		evidence: evidence,
	}
	// nil-guard probes (R74 iter-1 m-2 + R75 M-2): every fail-loud guard
	// DISJUNCT needs its own red face — branch C with a nil Model port, a
	// nil first-execution input, and a non-nil input with a nil req (the
	// panic-prevention half: without `first.req == nil`, machine.Adopt(nil)
	// nil-derefs). All refuse BEFORE the port fires, so cm stays untouched.
	if _, err := h.nodeEntryExec("op-x3c-nilmodel", fullGrantsN3(), "history-verification", nil, first, Ports{}); err == nil {
		t.Fatal("X3-3: nil-Model guard is dead — branch C accepted a nil Model port")
	}
	if _, err := h.nodeEntryExec("op-x3c-nilfirst", fullGrantsN3(), "history-verification", nil, nil, Ports{Model: cm}); err == nil {
		t.Fatal("X3-3: nil-first guard is dead — branch C accepted a nil first-execution input")
	}
	if _, err := h.nodeEntryExec("op-x3c-nilreq", fullGrantsN3(), "history-verification", nil, &firstExecInput{}, Ports{Model: cm}); err == nil {
		t.Fatal("X3-3: nil-req guard half is dead — branch C accepted a first-execution input with a nil adoption request")
	}
	if cm.calls != 0 {
		t.Fatalf("X3-3: nil-guard refusals fired the port %d times, want 0 — guards precede the port", cm.calls)
	}
	out, err := h.nodeEntryExec("op-x3c", fullGrantsN3(), "history-verification", nil, first, Ports{Model: cm})
	if err != nil {
		t.Fatalf("X3-3: entry error: %v", err)
	}
	if out.Branch != "C-model" {
		t.Fatalf("X3-3: branch = %q, want C-model", out.Branch)
	}
	if cm.calls != 1 {
		t.Fatalf("X3-3: model port calls = %d, want exactly 1 — first execution goes through the port once", cm.calls)
	}
	// evidence-flow pin (R74 iter-2 m-2 + iter-3 M-1): the caller's evidence
	// must reach the port on ALL THREE legs — an entry that swallows it (zero
	// value, or surgically the overdue slice alone) must not stay green
	if cm.lastAssetMillions != first.evidence.assetMillions || cm.lastReportOK != first.evidence.assetReportOK || cm.lastOverdueLen != len(first.evidence.overdueOnRecord) {
		t.Fatalf("X3-3: port saw evidence (%d, %v, len=%d), want the caller's (%d, %v, len=%d) — every evidence leg must flow through the entry", cm.lastAssetMillions, cm.lastReportOK, cm.lastOverdueLen, first.evidence.assetMillions, first.evidence.assetReportOK, len(first.evidence.overdueOnRecord))
	}
	// X3-9: the flow left machine state + a stored, verdict-derived artifact
	if s := h.machine.State("op-x3c"); s != adopt.StateDispatched {
		t.Fatalf("X3-9: machine state = %q, want dispatched — branch C must run the adopt→dispatch flow", s)
	}
	if r, ok := h.store.get("op-x3c"); !ok || r.Receipt != out.Result.Receipt || r.BusinessReqID != "br-x3c" {
		t.Fatalf("X3-9: stored result = (%v, %q, %q) — branch C must store its own artifact keyed to the task", ok, r.Receipt, r.BusinessReqID)
	}
	// verdict-authenticity anchor (R75 iter-3 B-1): the entry's returned
	// Verdict must be the port's ACTUAL return — a joint hardcode (fabricated
	// verdict + fabricated products on the same constant) survives every
	// product-only relative pin, because relative pins compare products to
	// the (fabricated) reference. The grant fixture carries the absolute
	// anchor; the veto fixture mirrors it below.
	if out.Verdict != cm.lastVerdict {
		t.Fatalf("X3-3: entry returned verdict %q, port actually returned %q — the verdict must be the port's, not a fabricated constant", out.Verdict.kind, cm.lastVerdict.kind)
	}
	// X3-9 verdict-derivation pin (R74 iter-1 M-2): the receipt must EMBED
	// the port verdict — out.Result.Receipt vs the stored copy alone is an
	// entry-self comparison and a fixed-string receipt passes it green.
	if !strings.Contains(out.Result.Receipt, out.Verdict.kind) {
		t.Fatalf("X3-9: receipt %q does not embed the port verdict kind %q — the stored artifact must be verdict-derived", out.Result.Receipt, out.Verdict.kind)
	}
	// Facts leg mirror (R75 iter-2 B-1): the Facts equality pin must hold on
	// EVERY discriminating fixture — a pin present only in the veto scenario
	// is defeated by hard-coding the fixture-predictable constant
	// ("model_port_verdict:deny"), which the grant fixture never observes.
	if len(out.Result.Facts) != 1 || out.Result.Facts[0] != "model_port_verdict:"+out.Verdict.kind {
		t.Fatalf("X3-9: facts=%v — the stored facts must embed the port verdict kind (mirrored on the grant fixture)", out.Result.Facts)
	}
	// X3-10 (R74 B-1 奇点推论补行): complete → re-enter → branch A serves the
	// receipt C stored. First execution and retry through ONE entry, closed.
	if o := h.completeNode("op-x3c"); o.Status != "completed" {
		t.Fatalf("X3-10 setup: complete = %q: %s", o.Status, o.Detail)
	}
	out2, err := h.nodeEntryExec("op-x3c", fullGrantsN3(), "history-verification", nil, nil, Ports{Model: cm})
	if err != nil || out2.Branch != "A-retry" {
		t.Fatalf("X3-10: re-entry = (%v, %q), want (nil, A-retry) — the C-stored artifact must be retry-reachable", err, out2.Branch)
	}
	if out2.Result.Receipt != out.Result.Receipt {
		t.Fatalf("X3-10: retry receipt = %q, want %q — retry serves the receipt the entry itself stored", out2.Result.Receipt, out.Result.Receipt)
	}
	if cm.calls != 1 {
		t.Fatalf("X3-10: model port calls = %d after retry, want still 1 — retry must not re-invoke the model", cm.calls)
	}
	// X3-9 discriminability fixture (R75 M-3 + iter-1 B-1): a SECOND verdict
	// value — the C branch's receipt must embed whatever verdict the port
	// returned, not a fixture-predictable constant. Every C-branch fixture
	// so far judged grant-50000, so a receipt hard-coded to that string
	// passed the pin green. The op is named "veto" (NOT "deny") and the
	// anchors are the UNPOLLUTED pair — verdict-kind equality + the Facts
	// leg — because a receipt containing the opID "op-x3c-deny" made
	// Contains("deny") trivially true (the substring-pollution form,
	// clause ⑯: the pinned literal must not be a substring of another
	// component of the checked string).
	vetoEv := creditEvidence{
		assetMillions: 35, assetReportOK: true,
		overdueOnRecord: []overdueEvent{{durationMonths: 4, monthsAgo: 24}},
	}
	cmVeto := newCountingModel(map[string]creditEvidence{"*": vetoEv})
	outVeto, err := h.nodeEntryExec("op-x3c-veto", fullGrantsN3(), "history-verification", nil,
		&firstExecInput{req: requestForT2("op-x3c-veto", "er-1", "ev-1", t2Digest('5'), "br-x3cv"), evidence: vetoEv},
		Ports{Model: cmVeto})
	if err != nil || outVeto.Branch != "C-model" || cmVeto.calls != 1 {
		t.Fatalf("X3-9 veto scenario: (%v, %q, %d calls), want (nil, C-model, 1)", err, outVeto.Branch, cmVeto.calls)
	}
	if outVeto.Verdict != cmVeto.lastVerdict {
		t.Fatalf("X3-9 veto scenario: entry returned verdict %q, port actually returned %q — the verdict must be the port's", outVeto.Verdict.kind, cmVeto.lastVerdict.kind)
	}
	if outVeto.Verdict.kind != "deny" || !strings.Contains(outVeto.Result.Receipt, outVeto.Verdict.kind) {
		t.Fatalf("X3-9 veto scenario: kind=%q receipt=%q — the pin must be discriminable across verdict values, not single-valued", outVeto.Verdict.kind, outVeto.Result.Receipt)
	}
	// the Facts leg (R75 iter-1 M-1): previously zero observation surface —
	// a hard-coded Facts value passed the whole suite green; equality here
	// is the unpolluted second pin
	if len(outVeto.Result.Facts) != 1 || outVeto.Result.Facts[0] != "model_port_verdict:"+outVeto.Verdict.kind {
		t.Fatalf("X3-9 veto scenario: facts=%v — the stored facts must embed the port verdict kind too", outVeto.Result.Facts)
	}
}

func TestX3_PositiveControl_FailPortMustTrigger(t *testing.T) {
	// X3-4 POSITIVE CONTROL: a "must-reach-model" scenario with a port
	// that records its invocation — if the port is NOT reached, the test
	// must RED. This is the "确实需要模型时失败探针必触发" face (198789653 §五.5)
	// — the probe-registration-≠-probe-consumption killer.
	// Mutation probe: delete the cm.calls assertion → this test goes
	// silent-green (the X3-4 face dies).
	h := newT2Host(t, t.TempDir()) // no prior op, no method → must hit C
	cm := newCountingModel(map[string]creditEvidence{"*": {assetMillions: 35, assetReportOK: true}})
	first := &firstExecInput{
		req:      requestForT2("op-x3pc", "er-1", "ev-1", t2Digest('f'), "br-x3pc"),
		evidence: creditEvidence{assetMillions: 35, assetReportOK: true},
	}
	out, err := h.nodeEntryExec("op-x3pc", fullGrantsN3(), "history-verification", nil, first, Ports{Model: cm})
	if err != nil {
		t.Fatalf("X3-4: entry error: %v", err)
	}
	if out.Branch != "C-model" {
		t.Fatalf("X3-4: branch = %q, want C-model — the no-prior/no-method scenario MUST reach the model port", out.Branch)
	}
	if cm.calls < 1 {
		t.Fatalf("X3-4 POSITIVE CONTROL: the model port was NOT invoked — probe registration without consumption (the R/K overlay disease recurring in X3)")
	}
}

func TestX3_Overlay_BreakEntry_RKMustGoRed(t *testing.T) {
	// X3-5 OVERLAY INTERNALIZATION: since N3, the R/K MAIN SCENARIOS route
	// through nodeEntryExec — breaking the entry makes TestT2R_/TestT2K_ RED
	// (the expectation FLIP, xingchao 198789653 §4: "新版本修复后应更新验收
	// 预期"). A green suite cannot contain a live broken entry, so the flip
	// proof is the MUTATION RECORD (60号 §五.3: mutate the entry → R/K red),
	// not this test. What this test pins instead is the CONTRACT the flip
	// rides on: (1) the R scenario's exact call (completed op) takes branch A
	// through the REAL entry — the path whose breakage flips R red; (2) the
	// entry's error face is exercised, so the error contract R/K fail on is
	// real, not unreachable.
	h := newT2Host(t, t.TempDir())
	if o := h.adoptAndDispatch("op-x3o", "er-1", "ev-1", t2Digest('f'), "br-x3o",
		"receipt:op-x3o:accepted", []string{"fact"}); o.Status != "dispatched" {
		t.Fatalf("X3-5 setup: dispatch = %q: %s", o.Status, o.Detail)
	}
	if o := h.completeNode("op-x3o"); o.Status != "completed" {
		t.Fatalf("X3-5 setup: complete = %q: %s", o.Status, o.Detail)
	}
	// (1) the R-path call shape through the REAL entry: branch A, zero port
	cm := newCountingModel(nil)
	out, err := h.nodeEntryExec("op-x3o", fullGrantsN3(), "history-verification", nil, nil, Ports{Model: cm})
	if err != nil || out.Branch != "A-retry" || cm.calls != 0 {
		t.Fatalf("X3-5: real entry R-path call = (%v, %q, %d calls), want (nil, A-retry, 0)", err, out.Branch, cm.calls)
	}
	// (2) the error face is reachable: an in-flight op (adopted+dispatched,
	// not completed) is refused — the same error channel R/K fail through
	// when a mutated entry returns errors
	if o := h.adoptAndDispatch("op-x3o-inflight", "er-1", "ev-1", t2Digest('3'), "br-x3oi",
		"receipt:unused", nil); o.Status != "dispatched" {
		t.Fatalf("X3-5 setup (in-flight): dispatch = %q: %s", o.Status, o.Detail)
	}
	if _, err := h.nodeEntryExec("op-x3o-inflight", fullGrantsN3(), "history-verification", nil, nil, Ports{Model: cm}); err == nil {
		t.Fatal("X3-5: the entry's error face is unreachable — the flip R/K rely on has no channel")
	}
	if cm.calls != 0 {
		t.Fatalf("X3-5: model port calls = %d across both faces, want 0", cm.calls)
	}
}

func TestX3_AuthorizationAndStateGates(t *testing.T) {
	// X3-6: unauthorized caller → all ports zero calls (E5 face preserved)
	h := newT2Host(t, t.TempDir())
	if o := h.adoptAndDispatch("op-x3g", "er-1", "ev-1", t2Digest('0'), "br-x3g",
		"receipt:op-x3g:accepted", []string{"fact"}); o.Status != "dispatched" {
		t.Fatalf("X3-6 setup: dispatch = %q: %s", o.Status, o.Detail)
	}
	if o := h.completeNode("op-x3g"); o.Status != "completed" {
		t.Fatalf("X3-6 setup: complete = %q: %s", o.Status, o.Detail)
	}
	cm := newCountingModel(nil)
	if _, err := h.nodeEntryExec("op-x3g", nil, "history-verification", nil, nil, Ports{Model: cm}); err == nil {
		t.Fatal("X3-6: nil-grants caller was accepted — the E5 face is not preserved")
	}
	if cm.calls != 0 {
		t.Fatalf("X3-6: model port calls = %d on unauthorized entry, want 0", cm.calls)
	}
	// X3-6b (R74 m-2 probe): completed but NOT stored — refused with a TRUE
	// diagnosis (its own face, not folded into the in-flight guard which
	// would label a completed op "in-flight")
	if o := h.machine.Adopt(requestForT2("op-x3g-nostore", "er-1", "ev-1", t2Digest('4'), "br-x3n")); o.Status != "adopted" {
		t.Fatalf("X3-6b setup: adopt = %q: %s", o.Status, o.Detail)
	}
	if o := h.machine.Dispatch("op-x3g-nostore", chainGrants()); o.Status != "dispatched" {
		t.Fatalf("X3-6b setup: dispatch = %q: %s", o.Status, o.Detail)
	}
	if o := h.completeNode("op-x3g-nostore"); o.Status != "completed" {
		t.Fatalf("X3-6b setup: complete = %q: %s", o.Status, o.Detail)
	}
	_, err := h.nodeEntryExec("op-x3g-nostore", fullGrantsN3(), "history-verification", nil, nil, Ports{Model: cm})
	if err == nil {
		t.Fatal("X3-6b: a completed-but-unstored op was accepted — the nothing-to-serve face is gone")
	}
	if !strings.Contains(err.Error(), "completed but no stored result") {
		t.Fatalf("X3-6b: refusal diagnosis = %q — must name the completed-but-unstored face, not mislabel it in-flight", err.Error())
	}
	if cm.calls != 0 {
		t.Fatalf("X3-6b: model port calls = %d, want 0", cm.calls)
	}
	// X3-7 (frozen matrix): an op that EXISTS but is not completed is
	// in-flight — the entry must refuse with zero port calls (falling to C
	// would double-fire the model on an already-dispatched op). The
	// never-adopted → C face is X3-3's, not this row's.
	if o := h.adoptAndDispatch("op-x3g-inflight", "er-1", "ev-1", t2Digest('2'), "br-x3i",
		"receipt:unused", nil); o.Status != "dispatched" {
		t.Fatalf("X3-7 setup: dispatch = %q: %s", o.Status, o.Detail)
	}
	cm2 := newCountingModel(map[string]creditEvidence{"*": {assetMillions: 35, assetReportOK: true}})
	if _, err := h.nodeEntryExec("op-x3g-inflight", fullGrantsN3(), "history-verification", nil, nil, Ports{Model: cm2}); err == nil {
		t.Fatal("X3-7: an in-flight (dispatched, not completed) op was accepted for re-execution — double-fire face")
	}
	if cm2.calls != 0 {
		t.Fatalf("X3-7: model port calls = %d on an in-flight op, want 0", cm2.calls)
	}
}

func TestX3_OldEntryCompat(t *testing.T) {
	// X3-8: the OLD nodeEntry (read-only result serving) still works —
	// the E5/T2R_PermissionRevoked callers are unchanged.
	h := newT2Host(t, t.TempDir())
	if o := h.adoptAndDispatch("op-x3oc", "er-1", "ev-1", t2Digest('1'), "br-x3oc",
		"receipt:op-x3oc:accepted", []string{"fact"}); o.Status != "dispatched" {
		t.Fatalf("X3-8 setup: dispatch = %q: %s", o.Status, o.Detail)
	}
	if o := h.completeNode("op-x3oc"); o.Status != "completed" {
		t.Fatalf("X3-8 setup: complete = %q: %s", o.Status, o.Detail)
	}
	r, err := h.nodeEntry("op-x3oc", fullGrantsN3())
	if err != nil {
		t.Fatalf("X3-8: old entry error: %v", err)
	}
	if r.Receipt != "receipt:op-x3oc:accepted" {
		t.Fatalf("X3-8: receipt = %q", r.Receipt)
	}
	// and the old entry's authorization face (E5) still refuses
	if _, err := h.nodeEntry("op-x3oc", nil); err == nil {
		t.Fatal("X3-8: old entry accepted nil grants — E5 face lost")
	}
	_ = adopt.StateAdopted // import anchor
}

// extractTestFuncBody returns the source span of one test function (from
// its "func <name>(" line to the next top-level "\nfunc " line). The
// tripwire below uses it to scope symbol bans to specific scenarios.
// X3-11 (hub-R1 M-1 junction pins): the most common production shape — a
// node kind WITH a published method that also has historical ops — was
// uncovered (branch-reorder mutant survived the whole suite green). Retry
// precedence and the double-fire guard must win over an applicable method.
func TestX3_JunctionRetryAndMethodPrecedence(t *testing.T) {
	h := newT2Host(t, t.TempDir())
	h.registry.mustPublish(t, knowledgeMethod{
		ID: "method-junction", Version: "1.0.0", ServesNode: "history-verification",
		InputFields: []string{"overdue_months", "business_request_id"}, Valid: true,
	})
	// X3-11a: completed op + method present → A wins (original receipt, zero
	// model calls) — never re-execute the method on a retry
	if o := h.adoptAndDispatch("op-x3j", "er-1", "ev-1", t2Digest('6'), "br-x3j",
		"receipt:op-x3j:original", []string{"fact"}); o.Status != "dispatched" {
		t.Fatalf("X3-11a setup: dispatch = %q: %s", o.Status, o.Detail)
	}
	if o := h.completeNode("op-x3j"); o.Status != "completed" {
		t.Fatalf("X3-11a setup: complete = %q: %s", o.Status, o.Detail)
	}
	cm := newCountingModel(nil)
	out, err := h.nodeEntryExec("op-x3j", fullGrantsN3(), "history-verification",
		map[string]string{"overdue_months": "0", "business_request_id": "br-x3j"}, nil, Ports{Model: cm})
	if err != nil || out.Branch != "A-retry" {
		t.Fatalf("X3-11a: junction = (%v, %q), want (nil, A-retry) — retry precedence must win over an applicable method", err, out.Branch)
	}
	if out.Result.Receipt != "receipt:op-x3j:original" {
		t.Fatalf("X3-11a: receipt = %q, want the ORIGINAL — a method-armed node's retry must not re-execute", out.Result.Receipt)
	}
	if cm.calls != 0 {
		t.Fatalf("X3-11a: model calls = %d, want 0", cm.calls)
	}
	// X3-11b: in-flight op + method present → the double-fire guard refuses
	// BEFORE method execution; zero port calls
	if o := h.adoptAndDispatch("op-x3j2", "er-1", "ev-1", t2Digest('7'), "br-x3j2",
		"receipt:unused", nil); o.Status != "dispatched" {
		t.Fatalf("X3-11b setup: dispatch = %q: %s", o.Status, o.Detail)
	}
	cm2 := newCountingModel(nil)
	// VALID method inputs (code-review B-1): with nil inputs, a method-first
	// reorder only fails on the input CONTRACT — indistinguishable from the
	// guard's refusal, so the pin was fake (guard-after-method mutant stayed
	// green). Valid inputs make the method SUCCEED if it runs first, so
	// err == nil becomes the red signal for "guard not preceding method".
	if _, err := h.nodeEntryExec("op-x3j2", fullGrantsN3(), "history-verification",
		map[string]string{"overdue_months": "0", "business_request_id": "br-x3j2"}, nil, Ports{Model: cm2}); err == nil {
		t.Fatal("X3-11b: in-flight op with an applicable method was executed — the guard must precede method dispatch")
	}
	if cm2.calls != 0 {
		t.Fatalf("X3-11b: model calls = %d, want 0", cm2.calls)
	}
}

func extractTestFuncBody(t *testing.T, src, fn string) string {
	t.Helper()
	start := strings.Index(src, "func "+fn+"(")
	if start < 0 {
		t.Fatalf("tripwire: function %s not found in source", fn)
	}
	rest := src[start:]
	end := strings.Index(rest[len("func "):], "\nfunc ")
	if end < 0 {
		t.Fatalf("tripwire: function %s has no terminator (next func)", fn)
	}
	return rest[:len("func ")+end]
}

// TestX3_NoBypassInRKScenarios is the RESIDENT bypass tripwire (R74 M-2):
// 60号 §六.3 once claimed "the overlay defense is resident" while the only
// live detector was the one-shot M5 mutation record — a bypass mutant (the
// R scenario reverted to a direct store.get, the Xingchao overlay disease
// form) survived the whole suite GREEN. This guard makes the claim true:
// the R/K MAIN scenario bodies must contain no direct store.get /
// registry.findForNode / method-execute calls. Reverting either scenario
// to a bypass turns this red.
//
// FORM-LIMITED (R74 iter-1 M-1, the corpus open-set-of-forms peg law):
// this is a source-substring TEXT peg — bypass forms that move the direct
// call into a helper (body keeps only the helper call) or decoy the
// first-match anchor (a comment line containing the function name before
// the real func) pass it green. No stronger BEHAVIOURAL face exists for
// scenario-source bypass (a resident behavioural face can prove the entry
// WAS called, never that nothing else was) — boundary recorded, peg kept:
// it catches the accident form (careless direct revert), which the
// verification mutations prove.
func TestX3_NoBypassInRKScenarios(t *testing.T) {
	b, err := os.ReadFile("t2host_rk_test.go")
	if err != nil {
		t.Fatalf("tripwire: cannot read t2host_rk_test.go: %v", err)
	}
	src := string(b)
	for _, fn := range []string{"TestT2R_PersistedReceiptAcrossRestart", "TestT2K_NewTaskZeroModelCalls"} {
		body := extractTestFuncBody(t, src, fn)
		for _, sym := range []string{".store.get(", ".registry.findForNode(", ".execute("} {
			if strings.Contains(body, sym) {
				t.Fatalf("%s contains a direct %q call — the scenario bypassed the unified entry (overlay disease form; resident face of R74 M-2)", fn, sym)
			}
		}
	}
}
