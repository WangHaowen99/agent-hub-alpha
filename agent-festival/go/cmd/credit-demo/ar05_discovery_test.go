package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// AR-05 (#27): capability discovery must not run inside the service-wide
// critical section, and must not let a stale snapshot dispatch. These are
// target assertions replacing the review counterexample
// TestArchitectureReviewDiscoveryBlocksHeartbeat (PASS there meant the defect
// existed).
//
// Required shape of BeginCall: take a snapshot under the lock, query RDC
// outside it, then re-check epoch/token, approved binding, run state and any
// existing call under the lock before writing the send intent.

const ar05Window = 500 * time.Millisecond

// slowRDC puts a gate in front of the fixture registry: every card request
// signals entered and waits until release is called (or a safety timeout).
type slowRDC struct {
	entered chan struct{}
	once    sync.Once
	gate    chan struct{}
}

func (g *slowRDC) release() { g.once.Do(func() { close(g.gate) }) }

func withSlowRDC(t *testing.T, s *Service) *slowRDC {
	t.Helper()
	g := &slowRDC{entered: make(chan struct{}, 16), gate: make(chan struct{})}
	original := s.cfg.BusRegistryURL
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/card") {
			g.entered <- struct{}{}
			select {
			case <-g.gate:
			case <-time.After(4 * time.Second):
			}
		}
		req, _ := http.NewRequest("GET", original+r.URL.Path, nil)
		req.Header = r.Header.Clone()
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, "fixture unavailable", 500)
			return
		}
		defer resp.Body.Close()
		var v any
		_ = json.NewDecoder(resp.Body).Decode(&v)
		_ = json.NewEncoder(w).Encode(v)
	}))
	t.Cleanup(func() { g.release(); proxy.Close() })
	s.cfg.BusRegistryURL = proxy.URL
	return g
}

// within reports whether fn finishes inside ar05Window; it keeps running in the
// background otherwise, and done is closed when it eventually returns.
func within(fn func() error) (ok bool, err error, done chan struct{}) {
	done = make(chan struct{})
	res := make(chan error, 1)
	go func() { res <- fn(); close(done) }()
	select {
	case err = <-res:
		return true, err, done
	case <-time.After(ar05Window):
		return false, nil, done
	}
}

func beginAsync(s *Service, in CallCommand) chan error {
	ch := make(chan error, 1)
	go func() { _, err := s.BeginCall(in); ch <- err }()
	return ch
}

// addLocalRun adds a second, unrelated run R2 with one local node.
func addLocalRun(t *testing.T, s *Service) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Plans["P2"] = []*Plan{{ID: "P2", Version: 1, Agents: []PlanAgent{{ID: "related", Expert: "related-party"}}, Layers: [][]string{{"related"}}}}
	s.st.Runs["R2"] = &Run{ID: "R2", PlanID: "P2", PlanVersion: 1, Status: "running", Token: "lease-r2", Epoch: 1,
		LeaseUntil: time.Now().Add(time.Hour), Steps: map[string]*StepRec{"related": {ID: "related", Expert: "related-party", Status: "pending"}}}
	if err := s.save(); err != nil {
		t.Fatal(err)
	}
}

func TestAR05_SlowDiscoveryDoesNotBlockHeartbeat(t *testing.T) {
	s, in, _ := busFixture(t)
	g := withSlowRDC(t, s)
	begin := beginAsync(s, in)
	<-g.entered
	ok, err, _ := within(func() error { return s.Heartbeat(in.RunID, in.Token) })
	g.release()
	if berr := <-begin; berr != nil {
		t.Fatalf("BeginCall after slow discovery: %v", berr)
	}
	if !ok {
		t.Fatal("heartbeat waited for an unrelated RDC request")
	}
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
}

func TestAR05_SlowDiscoveryDoesNotBlockOtherRunsOrReads(t *testing.T) {
	s, in, _ := busFixture(t)
	addLocalRun(t, s)
	g := withSlowRDC(t, s)
	begin := beginAsync(s, in)
	<-g.entered
	okRead, _, _ := within(func() error { s.RunByID("R1"); return nil })
	okPut, errPut, _ := within(func() error {
		if _, _, err := s.StepGet("R2", "lease-r2", "related"); err != nil {
			return err
		}
		return s.StepPut("R2", "lease-r2", "related", "related parties checked")
	})
	g.release()
	<-begin
	if !okRead {
		t.Error("read-only projection waited for an RDC request")
	}
	if !okPut {
		t.Error("another run's result commit waited for an RDC request")
	} else if errPut != nil {
		t.Errorf("other run's result commit failed: %v", errPut)
	}
}

// The lease is taken over while discovery is in flight: the old executor's
// BeginCall must be fenced on re-check and write no send intent.
func TestAR05_TakeoverDuringDiscoveryWritesNoIntent(t *testing.T) {
	s, in, _ := busFixture(t)
	g := withSlowRDC(t, s)
	begin := beginAsync(s, in)
	<-g.entered
	var job *Job
	ok, err, leaseDone := within(func() error {
		var err error
		job, err = s.Lease("new-worker")
		return err
	})
	g.release()
	berr := <-begin
	<-leaseDone
	if !ok {
		t.Fatal("lease takeover waited for the old executor's RDC request")
	}
	if err != nil || job == nil || job.RunID != in.RunID {
		t.Fatalf("takeover lease: job=%v err=%v", job, err)
	}
	if berr == nil {
		t.Fatal("old executor's BeginCall succeeded after its lease was taken over during discovery")
	}
	s.mu.Lock()
	call := s.st.Runs[in.RunID].Steps[in.Step].Call
	s.mu.Unlock()
	if call != nil {
		t.Fatalf("a send intent from the fenced executor was written: %+v", call)
	}
}

// Positive control and TOCTOU guard: two concurrent BeginCall requests for
// the same node yield exactly one send intent, whatever the interleaving.
func TestAR05_ConcurrentBeginCallsCreateOneIntent(t *testing.T) {
	s, in, _ := busFixture(t)
	g := withSlowRDC(t, s)
	var mu sync.Mutex
	var ids []string
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := s.BeginCall(in)
			if err != nil {
				t.Errorf("BeginCall: %v", err)
				return
			}
			mu.Lock()
			ids = append(ids, out["call"].(*RemoteCall).RequestID)
			mu.Unlock()
		}()
	}
	<-g.entered
	select { // with discovery outside the lock both requests reach RDC; today the second waits on the lock
	case <-g.entered:
	case <-time.After(300 * time.Millisecond):
	}
	g.release()
	wg.Wait()
	if len(ids) != 2 || ids[0] != ids[1] {
		t.Fatalf("concurrent BeginCall produced request ids %v, want one shared id", ids)
	}
}

// Positive control: with a responsive registry BeginCall behaves as before.
func TestAR05_NormalDiscoveryUnchanged(t *testing.T) {
	s, in, _ := busFixture(t)
	out, err := s.BeginCall(in)
	if err != nil || out["action"] != "send" {
		t.Fatalf("BeginCall = %v, %v", out, err)
	}
	again, err := s.BeginCall(in)
	if err != nil || again["call"].(*RemoteCall).RequestID != out["call"].(*RemoteCall).RequestID {
		t.Fatalf("repeat BeginCall = %v, %v", again, err)
	}
}
