package main

import (
	"strconv"
	"testing"
	"time"
)

func TestPGEventCursorOrdersNumbersAcrossDigitWidths(t *testing.T) {
	ctx, s, cfg, _ := integration(t)
	// Explicit sequence fixtures live only in this test scope. They do not
	// advance the shared identity sequence or alter any existing event.
	admin, err := OpenStore(ctx, cfg.MigratorDSN, s.Scope, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Pool.Close()
	large := time.Now().UnixNano()
	small := large / 2
	for _, seq := range []int64{large, small} {
		_, err = admin.Pool.Exec(ctx, "INSERT INTO audit_outbox(sequence,scope,aggregate,event_id,kind,document) OVERRIDING SYSTEM VALUE VALUES($1,$2,'cursor-fixture',$3,'TEST','{}')", seq, s.Scope, newID("fixture-event-"))
		if err != nil {
			t.Fatal(err)
		}
	}
	rows, err := s.Events(ctx, "cursor-fixture", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0]["sequence"] != strconv.FormatInt(small, 10) || rows[1]["sequence"] != strconv.FormatInt(large, 10) {
		t.Fatal("event cursor sorted textual aliases instead of numeric PG sequence")
	}
	tail, err := s.Events(ctx, "cursor-fixture", small)
	if err != nil || len(tail) != 1 || tail[0]["sequence"] != strconv.FormatInt(large, 10) {
		t.Fatal("cursor skipped or duplicated an event", err)
	}
}
