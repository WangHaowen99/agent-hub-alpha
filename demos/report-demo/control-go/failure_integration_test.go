package main

import "testing"

func TestPGFailedGraphStopsPollingAndExplicitRetryPreservesBinding(t *testing.T) {
	ctx, s, _, cat, material, task, request, _ := commitFixture(t)
	if _, err := s.MarkWaiting(ctx, task.ID, "extract", request.Owner, request.Epoch, "FAILED", "REMOTE_FAILED"); err != nil {
		t.Fatal(err)
	}
	after, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != "FAILED" || after.Nodes["extract"].Result != nil {
		t.Fatal("failure was shown as running or successful")
	}
	pending, err := s.Pending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range pending {
		if id == task.ID {
			t.Fatal("failed task remained in automatic polling queue")
		}
	}
	v, err := s.Complete(ctx, task.ID, request.Owner, request.Epoch)
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["status"] != "FAILED" {
		t.Fatal("failed graph was not authoritatively closed")
	}
	_, err = s.Heartbeat(ctx, task.ID, request.Owner, request.Epoch, "graph")
	requireCode(t, err, "LEASE_EXPIRED")
	if _, err = s.SetPause(ctx, task.ID, false); err != nil {
		t.Fatal(err)
	}
	v, err = s.ClaimRun(ctx, task.ID, "explicit-retry-worker", "graph")
	if err != nil {
		t.Fatal(err)
	}
	lease := leaseFrom(t, v)
	if lease.Epoch == request.Epoch {
		t.Fatal("explicit retry revived the old epoch")
	}
	v, err = s.ResolveNode(ctx, task.ID, "extract", lease.Owner, lease.Epoch, "", cat, material.Digest)
	if err != nil {
		t.Fatal(err)
	}
	permit := v.(map[string]any)
	if permit["input_digest"] != request.InputDigest || permit["attempt"] != request.Attempt+1 {
		t.Fatal("explicit retry changed h or did not use a new bounded pure attempt")
	}
}
