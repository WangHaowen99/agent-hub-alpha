package main

import "testing"

func TestReconciliationSeparatesInputReceiptAndImmutableBody(t *testing.T) {
	stored := &Record[string]{InputDigest: "input-1", ContentDigest: "body-1", Value: "receipt-1"}
	if c, s, _ := QueryForReconciliation[string](nil, "input-1"); c || s {
		t.Fatal("absent result reported committed")
	}
	if c, s, v := QueryForReconciliation(stored, "input-1"); !c || !s || v != "receipt-1" {
		t.Fatal("same input did not reuse original result")
	}
	if c, s, _ := QueryForReconciliation(stored, "receipt-1"); !c || s {
		t.Fatal("receipt was treated as the input binding")
	}
	if !CanReuseCommit(stored, "input-1", "body-1") || CanReuseCommit(stored, "input-1", "body-2") || CanReuseCommit(stored, "input-2", "body-1") {
		t.Fatal("immutable commit policy violated")
	}
	if stored.InputDigest != "input-1" || stored.Value != "receipt-1" {
		t.Fatal("read policy modified stored result")
	}
}
