package main

import "testing"

func TestExplainCompose(t *testing.T) {
	got := explainCompose("node related: max_downstream_calls 5 exceeds parent budget 4")
	want := "related 的下游调用次数上限 5 超过它在并行层里分到的预算 4（同层越宽，每个节点分到越少）"
	if got != want {
		t.Fatalf("got %q", got)
	}
	if other := "something compose said"; explainCompose(other) != other {
		t.Fatal("unknown errors must pass through unchanged")
	}
}

func TestStripStepTag(t *testing.T) {
	if got := stripStepTag("[step:judge]\nDECISION=GRANT-50000"); got != "DECISION=GRANT-50000" {
		t.Fatalf("got %q", got)
	}
}
