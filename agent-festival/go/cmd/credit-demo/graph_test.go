package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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

func TestValidLogin(t *testing.T) {
	t.Setenv("CREDIT_DEMO_PASSWORD", "s3cret")
	if !validLogin("alice", "s3cret") {
		t.Fatal("alice with the right password must pass")
	}
	for _, c := range [][2]string{{"alice", "demo"}, {"mallory", "s3cret"}, {"alice", ""}} {
		if validLogin(c[0], c[1]) {
			t.Fatalf("%v must be refused", c)
		}
	}
}

func TestGraphBranches(t *testing.T) {
	got := graphBranches("C006", []Branch{
		{Goal: "company_id: R-601，查征信摘要", Status: "interrupted"},
		{Goal: "company_id: R-603，查征信摘要", Status: "finished", Read: "R-603"},
		{Goal: "company_id: R-601，查征信摘要", Status: "started", Read: "R-601", Activity: "查关联企业征信 · R-601"},
		{Goal: "查一家不认识的企业", Status: "finished"},
	})
	if len(got) != 3 {
		t.Fatalf("want 3 branches (R-601 once), got %d: %+v", len(got), got)
	}
	if b := got[0]; b.Title != "王氏汽配贸易" || b.Status != "started" || !b.Retried || b.Activity == "" || b.Relation != "持股 60%" {
		t.Errorf("R-601: %+v", b)
	}
	if b := got[1]; b.Finding != "近一年逾期 1 次" || b.Tone != "warn" {
		t.Errorf("R-603: %+v", b)
	}
	if b := got[2]; b.Title != "查一家不认识的企业" || b.Finding != "" {
		t.Errorf("unknown: %+v", b)
	}
	if f, tone := bureauFinding(relatedBureau["R-601"]); f != "无逾期" || tone != "ok" {
		t.Errorf("R-601 finding %q %q", f, tone)
	}
}

// A12: the state revision only grows and survives a restart, so a front end
// can drop any snapshot older than one it has already shown.
func TestStateRevisionPersists(t *testing.T) {
	dir := t.TempDir()
	s := &Service{cfg: Config{DataDir: dir}, st: &State{Plans: map[string][]*Plan{}, Runs: map[string]*Run{}}}
	s.save()
	s.save()
	if s.st.Rev != 2 {
		t.Fatalf("rev after two saves = %d", s.st.Rev)
	}
	var back State
	raw, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &back); err != nil || back.Rev != 2 {
		t.Fatalf("persisted rev = %d (%v)", back.Rev, err)
	}
}
