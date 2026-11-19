package adopt

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/openjiuwen/agent-festival/go/internal/ledger"
)

// A schema-approved identity must survive the actual consumer representation.
func TestReviewIdentityContractRoundTrip(t *testing.T) {
	r := testRequest("review-op", digestA, pinGrants())
	r.Identity.LogicalExecution = &ledger.LogicalExecution{TaskID: "T", GraphVersion: "g", NodePath: "n", IterationPath: "0"}
	b, _ := json.Marshal(r)
	var doc map[string]any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if errs := CheckRequest(doc); len(errs) != 0 {
		t.Fatal(errs)
	}
	var consumed Request
	if err := json.Unmarshal(b, &consumed); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Identity, consumed.Identity) {
		t.Fatal("identity lost during decode")
	}
	for _, key := range []string{"logical_execution", "execution_generation"} {
		t.Run(key, func(t *testing.T) {
			copyDoc := make(map[string]any)
			for k, v := range doc {
				copyDoc[k] = v
			}
			if key == "logical_execution" {
				copyDoc[key] = doc["identity"].(map[string]any)[key]
			} else {
				copyDoc[key] = json.Number("7")
			}
			if errs := CheckRequest(copyDoc); len(errs) == 0 {
				t.Fatal("accepted a top-level identity field that Request silently discards")
			}
		})
	}
}
