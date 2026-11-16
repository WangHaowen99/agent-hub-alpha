package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestOfflineReplayDetectsTamperAndDoesNotDispatchRecordedRoutes(t *testing.T) {
	path := os.Getenv("REPORT_TEST_SNAPSHOT")
	if path == "" {
		t.Skip("actual scoped backup required")
	}
	snapshot, err := ReadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	var count atomic.Int64
	spy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { count.Add(1); w.WriteHeader(500) }))
	defer spy.Close()
	for i, raw := range snapshot.Payload.Tables["remote_calls"] {
		var row map[string]any
		if err = json.Unmarshal(raw, &row); err != nil {
			t.Fatal(err)
		}
		doc := row["document"].(map[string]any)
		doc["route"] = map[string]any{"gateway_url": spy.URL, "rdc_url": spy.URL, "publication_url": spy.URL}
		updated, _ := json.Marshal(row)
		snapshot.Payload.Tables["remote_calls"][i] = updated
	}
	snapshot.Digest, err = Digest(snapshot.Payload)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "snapshot.json")
	if err = WriteSnapshot(file, snapshot); err != nil {
		t.Fatal(err)
	}
	if _, err = ReplaySnapshot(file); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 0 {
		t.Fatal("offline replay invoked a recorded route")
	}
	for id, body := range snapshot.Payload.Contents {
		original := append([]byte(nil), body...)
		body[0] ^= 1
		snapshot.Payload.Contents[id] = body
		if snapshot.Validate() == nil {
			t.Fatal("manifest tamper undetected")
		}
		snapshot.Digest, _ = Digest(snapshot.Payload)
		if snapshot.Validate() == nil {
			t.Fatal("content tamper survived rehashed manifest")
		}
		snapshot.Payload.Contents[id] = original
		snapshot.Digest, _ = Digest(snapshot.Payload)
		delete(snapshot.Payload.Contents, id)
		snapshot.Digest, _ = Digest(snapshot.Payload)
		if snapshot.Validate() == nil {
			t.Fatal("missing content undetected")
		}
		snapshot.Payload.Contents[id] = original
		snapshot.Digest, _ = Digest(snapshot.Payload)
		break
	}
	for i, raw := range snapshot.Payload.Tables["tasks"] {
		var row map[string]any
		json.Unmarshal(raw, &row)
		doc := row["document"].(map[string]any)
		graph := doc["graph"].(map[string]any)
		graph["goal"] = "changed after admission"
		updated, _ := json.Marshal(row)
		snapshot.Payload.Tables["tasks"][i] = updated
		snapshot.Digest, _ = Digest(snapshot.Payload)
		if snapshot.Validate() == nil {
			t.Fatal("changed admitted graph accepted")
		}
		break
	}
}
