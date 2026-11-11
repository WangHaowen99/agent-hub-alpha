package main

import (
	"strings"
	"testing"
)

func testPlan(steps ...Step) *Plan {
	p := &Plan{Steps: steps}
	for _, st := range steps {
		p.Agents = append(p.Agents, PlanAgent{ID: st.ID, Expert: st.Member})
		p.Steps[len(p.Agents)-1].Member = st.ID
	}
	return p
}

func goodSteps() []Step {
	grant := &RunIf{Step: "judge", Contains: "DECISION=GRANT"}
	return []Step{
		{ID: "intake", Member: "intake"},
		{ID: "financial", Member: "financial-analyst", DependsOn: []string{"intake"}},
		{ID: "industry", Member: "industry-researcher", DependsOn: []string{"intake"}},
		{ID: "bureau", Member: "bureau-summary", DependsOn: []string{"intake"}},
		{ID: "related", Member: "related-party", DependsOn: []string{"intake"}},
		{ID: "judge", Member: "risk-judge", DependsOn: []string{"financial", "industry", "bureau", "related"}},
		{ID: "report", Member: "report-writer", DependsOn: []string{"judge"}},
		{ID: "submit", Member: "credit-submitter", DependsOn: []string{"judge"}, When: grant},
	}
}

func TestRuleViolationsAcceptsFullPlan(t *testing.T) {
	if errs := ruleViolations(testPlan(goodSteps()...)); len(errs) != 0 {
		t.Fatalf("full plan rejected: %v", errs)
	}
}

func TestRuleViolationsRejectsMissingEvidence(t *testing.T) {
	var steps []Step
	for _, st := range goodSteps() {
		if st.ID == "related" || st.ID == "financial" {
			continue
		}
		if st.ID == "judge" {
			st.DependsOn = []string{"industry", "bureau"}
		}
		steps = append(steps, st)
	}
	errs := strings.Join(ruleViolations(testPlan(steps...)), "\n")
	for _, want := range []string{"缺少财务证据", "缺少关联方证据"} {
		if !strings.Contains(errs, want) {
			t.Errorf("want %q in:\n%s", want, errs)
		}
	}
}

func TestRuleViolationsRejectsUnguardedEffect(t *testing.T) {
	steps := goodSteps()
	steps[7].When = nil
	steps[5].DependsOn = []string{"financial", "industry", "bureau"} // judge skips the black box
	errs := strings.Join(ruleViolations(testPlan(steps...)), "\n")
	for _, want := range []string{"没有依赖调查步骤 related", "submit 有不可逆效果"} {
		if !strings.Contains(errs, want) {
			t.Errorf("want %q in:\n%s", want, errs)
		}
	}
}
