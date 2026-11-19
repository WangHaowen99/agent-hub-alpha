package main

import "testing"

// Parent-module tests do not exercise this nested consumer's request version.
func TestReviewJudgmentConsumesCurrentAdoptionContract(t *testing.T) {
	e := newAR04(t, true)
	s := e.open()
	_, r := e.proposeAndApprove(s)
	job, err := s.LeaseAs("worker-a", "model-a")
	if err != nil || job == nil {
		t.Fatalf("lease: %v", err)
	}
	if err := s.StepPut(r.ID, job.Token, "related", "verified evidence"); err != nil {
		t.Fatal(err)
	}
	out, err := s.Judgment(r.ID, job.Token, "judge", `{"level":"grant-50000","evidence_steps":["related"],"rationale":"test"}`)
	if err != nil || out == "" {
		t.Fatalf("valid judgment must be adopted by current contract: %q, %v", out, err)
	}
	report, err := s.Verify(r.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, check := range report.Checks {
		if check.Name == "决策重算（replay）" {
			found = true
			if !check.OK {
				t.Fatalf("adopted decision must replay: %s", check.Detail)
			}
		}
	}
	if !found {
		t.Fatal("replay check missing")
	}
}
