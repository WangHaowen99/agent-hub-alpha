package adopt

import (
	"math"
	"testing"
)

func TestAdoptFailureClassification(t *testing.T) {
	for _, mode := range []string{"schema", "capacity", "conflict", "first-write", "reuse-write", "conflict-write"} {
		t.Run(mode, func(t *testing.T) {
			m := freshMemory(t)
			r := testRequest("op", digestA, pinGrants())
			want := FailureStorage
			switch mode {
			case "schema":
				r.AdoptionVersion = "invalid"
				want = FailureRequest
			case "capacity":
				m.seq = math.MaxInt64
				want = FailureState
			case "conflict", "reuse-write", "conflict-write":
				if o := m.Adopt(r); o.Status != "adopted" {
					t.Fatal(o)
				}
				if mode != "reuse-write" {
					r.PayloadDigest = digestB
				}
				if mode == "conflict" {
					want = FailureConflict
				}
			}
			if want == FailureStorage {
				m.path = t.TempDir()
			} // a directory is not an appendable ledger
			out := m.Adopt(r)
			if out.Status != "rejected" || out.Failure != want {
				t.Fatalf("got %+v, want %s", out, want)
			}
		})
	}
}
