//go:build linux

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestAR04_ProcessRestart is the process-level restart experiment requested in
// #27: phase 1 runs in its own process, adopts a run under judgment spec 1.1.0
// and is killed with SIGKILL (no shutdown path runs); phase 2 is a fresh
// process started with spec 2.0.0 on the same data directory. Nothing is
// shared between the processes except what is on disk.
func TestAR04_ProcessRestart(t *testing.T) {
	if phase := os.Getenv("AR04_PHASE"); phase != "" {
		ar04Child(t, phase)
		return
	}
	dir, specs := t.TempDir(), t.TempDir()
	v1 := ar04Spec(t, specs, "1.1.0", "RULE-V1")
	v2 := ar04Spec(t, specs, "2.0.0", "RULE-V2")
	orch := ar04Orchestrator(t)
	out := filepath.Join(t.TempDir(), "phase2.json")
	run := func(phase, spec string) (string, error) {
		cmd := exec.Command(os.Args[0], "-test.run", "^TestAR04_ProcessRestart$", "-test.count=1")
		cmd.Env = append(os.Environ(), "AR04_PHASE="+phase, "AR04_DIR="+dir, "AR04_SPEC="+spec, "AR04_ORCH="+orch, "AR04_OUT="+out)
		b, err := cmd.CombinedOutput()
		return string(b), err
	}
	log1, err := run("adopt-then-kill", v1)
	t.Logf("phase 1 (spec 1.1.0, killed with SIGKILL): exit=%v\n%s", err, lastLines(log1, 3))
	if ee, ok := err.(*exec.ExitError); !ok || ee.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
		t.Fatalf("phase 1 was not killed by SIGKILL: %v", err)
	}
	log2, err := run("restart-with-v2", v2)
	t.Logf("phase 2 (fresh process, spec 2.0.0): exit=%v\n%s", err, lastLines(log2, 3))
	if err != nil {
		t.Fatalf("phase 2 failed: %v\n%s", err, log2)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var res map[string]any
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatal(err)
	}
	t.Logf("phase 2 observations: %s", raw)
	if res["resumed_run"] != "R1" || res["resumed_prompt_has_v1"] != true || res["resumed_prompt_has_v2"] != false ||
		res["old_plan_approve_refused"] != true || res["new_run_prompt_has_v2"] != true {
		t.Fatalf("restart did not keep the bound spec: %s", raw)
	}
}

func ar04Child(t *testing.T, phase string) {
	e := &ar04Env{t: t, dir: os.Getenv("AR04_DIR"), spec: os.Getenv("AR04_SPEC"), orch: os.Getenv("AR04_ORCH"), mdl: "model-a"}
	s := e.open()
	switch phase {
	case "adopt-then-kill":
		_, r := e.proposeAndApprove(s)
		// a second plan is proposed under 1.1.0 but not approved before the kill
		if _, err := s.Propose(ProposeInput{CaseID: "C006", Goal: "credit due diligence", Agents: ar04Agents()}); err != nil {
			t.Fatal(err)
		}
		os.Stdout.WriteString("adopted " + r.ID + " under " + r.Binding.SpecVersion + "; killing\n")
		syscall.Kill(os.Getpid(), syscall.SIGKILL)
		select {}
	case "restart-with-v2":
		res := map[string]any{}
		job := mustLease(t, s)
		p := judgePromptOf(t, job)
		res["resumed_run"] = job.RunID
		res["resumed_prompt_has_v1"] = strings.Contains(p, "RULE-V1") && strings.Contains(p, "@1.1.0")
		res["resumed_prompt_has_v2"] = strings.Contains(p, "RULE-V2")
		res["resumed_binding"] = s.st.Runs[job.RunID].Binding
		old := s.st.Plans["P2"][0]
		_, err := s.Approve(old.ID, old.Version, old.Hash, "approver", nil)
		res["old_plan_approve_refused"] = err != nil
		if err != nil {
			res["old_plan_approve_error"] = err.Error()
		}
		s.st.Runs[job.RunID].Status = "done"
		_, r2 := e.proposeAndApprove(s)
		job2 := mustLease(t, s)
		res["new_run"] = r2.ID
		res["new_run_prompt_has_v2"] = job2.RunID == r2.ID && strings.Contains(judgePromptOf(t, job2), "RULE-V2")
		b, _ := json.Marshal(res)
		if err := os.WriteFile(os.Getenv("AR04_OUT"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
