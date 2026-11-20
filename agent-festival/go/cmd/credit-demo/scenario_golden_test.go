package main

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/openjiuwen/agent-festival/go/internal/rfc8785"
)

func goldenDigest(v any) string {
	b, err := rfc8785.Canonicalize(jsonDoc(v))
	if err != nil {
		panic(err)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// golden snapshot of everything the scenario package must reproduce exactly
func goldenSnapshot() map[string]string {
	out := map[string]string{}
	for _, e := range experts {
		out["contract/"+e.ID] = goldenDigest(e.contract())
		out["card/"+e.ID] = goldenDigest(map[string]any{"id": e.ID, "name": e.Name, "kind": e.Kind, "summary": e.Summary, "tools": toAny(e.Tools), "instruction": expertInstruction(e), "max_iter": e.MaxIter})
	}
	pkg := scenarios.get(defaultScenario)
	holds := []any{}
	for _, g := range pkg.holds() {
		holds = append(holds, g.doc())
	}
	out["caller_holds"] = goldenDigest(map[string]any{"h": holds})
	cov := []any{}
	for _, c := range pkg.Rules.Coverage {
		cov = append(cov, map[string]any{"name": c.Name, "selectors": toAny(c.Selectors)})
	}
	out["coverage"] = goldenDigest(map[string]any{"c": cov})
	out["rules"] = goldenDigest(map[string]any{"r": toAny(pkg.planRules())})
	out["parent_bounds"] = goldenDigest(pkg.ParentBounds)
	ba := map[string]any{}
	for k, v := range busAgents {
		ba[k] = v
	}
	out["bus_agents"] = goldenDigest(ba)
	ids := []any{}
	for _, e := range experts {
		ids = append(ids, e.ID)
	}
	out["order"] = goldenDigest(map[string]any{"ids": ids})
	return out
}

// builtinCatalogGolden are the digests of the catalog data as it was
// hard-coded in 7dfd061, before it moved into scenarios/credit-dd.json.
var builtinCatalogGolden = map[string]string{
	"bus_agents":                   "946b453b48958b5bf4a906569ba08546ea051b157d90d888eca4a35cbb0bbdee",
	"caller_holds":                 "0ca2e77739aa6b75009c0f416e8d02320b9bd3c68bd7231ac75872862aa58c0c",
	"card/bureau-full":             "b6ed5d31e5b43fc0ef505bf1112ea41d8a78f1bab3746630dcef3402aabd9f98",
	"card/bureau-summary":          "db9a5b737d49470f57cd17b5cceddd9cd403e95b460ee6be8dbfa6d930a265e4",
	"card/credit-submitter":        "d31479c1488cf1e2386b557ba324bcb2bfb8efaa62883b8bde6c01091c060e9a",
	"card/financial-analyst":       "6e9c14e86a42dc730a69e04a31fcb2be4f866cb23ec118553f97ace5b27f4de0",
	"card/industry-researcher":     "e8f7fda499eaf2e6905a29f4d7fbae355ba8be9868a3c046a78e0cafb565da11",
	"card/intake":                  "c027c3dfd8346e107655784f306fa95480555eff205e0aacdf21d083b203ef85",
	"card/related-party":           "c5383e6eebccf0d6ee755e0432c71c6983ea3c07afb7552c9d52afe1dd02fe8e",
	"card/report-writer":           "f8df9edb3331877e20f20d96c714a67a866274907ead2d3609b8400357d78943",
	"card/risk-judge":              "f3a57707304166b6c8ecb665e2e71dad9db241ea79118df35fb4aeedca1ba3f3",
	"contract/bureau-full":         "16ff0fe14bd50e8bbbbff003d805213cea57272f36c8b60615193293c0ee32aa",
	"contract/bureau-summary":      "255383d9e0e3e5d5b0a5952afc6e5e8a3c26ccb07a4bc671a6b85c115427cee1",
	"contract/credit-submitter":    "49c5a553d57134a2c60132b1908ee6cbc4437c788c7c4d0da91008faf150b7ce",
	"contract/financial-analyst":   "b433290ae68a411e090e9b54bf8607a1d08aeb9bdf8efe4461735137505b1a1f",
	"contract/industry-researcher": "e1622a9ca8e50c2281568d16e46c5518f157c5d6d6991097220aa3c7726d52b8",
	"contract/intake":              "a9da2fc2917a4d425c353be9bfb1df58af30f88c7d5d07e5234a77541324e98e",
	"contract/related-party":       "58bf67d11161cc14f1d3b3cc5ae66d16a74aeea929f299435f1edf606fbc34fe",
	"contract/report-writer":       "28ec6a969fcbb53626da96e7cc6f26cf1d5ffef0e12c021757121a096eef253f",
	"contract/risk-judge":          "bc9a7cfd42ef2cf682cd5b254a302fa9be84a51ab2bae28c0e85419ae9bbc213",
	"coverage":                     "72e0de72901513c636902d83465d2475cee86e2deb62ff0e2abef2cb5cd3d07c",
	"order":                        "2e3536d1f59fa1a84ea6c1fa68c593519c8742520a0bbcf0ae99c2dd779de398",
	"parent_bounds":                "2aaf969cb277156f1f9e1a7d81ba408321655ae9fef5bd7c6e2f83d802d0b296",
	"rules":                        "b8abfa3e178d904e5280b9cd3de4474c6bfb851e57e276688b8712f5c6dd7b41",
}

// The credit scenario package reproduces the former built-in catalog exactly:
// contracts (and so signatures and contract hashes), instructions, the
// approver's grants, the evidence rules, the budget, the Bus mapping and the
// catalog order (AR-06 moves data, it does not change behaviour).
func TestScenarioPackageReproducesBuiltinCatalog(t *testing.T) {
	got := goldenSnapshot()
	if len(got) != len(builtinCatalogGolden) {
		t.Fatalf("snapshot has %d entries, golden %d", len(got), len(builtinCatalogGolden))
	}
	for k, want := range builtinCatalogGolden {
		if got[k] != want {
			t.Errorf("%s: digest %s, want %s", k, got[k], want)
		}
	}
}
