package main

import (
	"encoding/json"
	"os"
	"testing"
)

func sample(t *testing.T) (Catalog, Graph) {
	t.Helper()
	raw, err := os.ReadFile("../scenes/annual-report/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	var c Catalog
	if err = DecodeStrict(raw, &c); err != nil {
		t.Fatal(err)
	}
	cd, err := Digest(c)
	if err != nil {
		t.Fatal(err)
	}
	g := Graph{Goal: "经营分析", MaterialID: "cmb-2025-annual-report-a", Focus: "business", CatalogDigest: cd, PolicyVersion: c.PolicyVersion, Nodes: []Node{
		{ID: "extract", Expert: "extractor@1", Role: "资料", Task: "提取原文及来源", DependsOn: []string{}},
		{ID: "business", Expert: "business@1", Role: "经营", Task: "根据提取结果分析", DependsOn: []string{"extract"}},
		{ID: "draft", Expert: "writer@1", Role: "写作", Task: "按经营结果撰写", DependsOn: []string{"business"}},
	}}
	return c, g
}
func TestStrictJSONAndCanonicalBinding(t *testing.T) {
	for _, raw := range []string{`{"a":1,"a":2}`, `{"x":[{"a":1,"a":2}]}`, `{"x":NaN}`, `{} {}`, `{"x":Infinity}`, `{"x":"\ud800"}`, `{"\udfff":1}`, string([]byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'})} {
		var v any
		if DecodeStrict([]byte(raw), &v) == nil {
			t.Fatalf("accepted ambiguous JSON %s", raw)
		}
	}
	var a, b any
	if err := DecodeStrict([]byte(`{"z":"汉字","n":9007199254740991,"a":[null,""]}`), &a); err != nil {
		t.Fatal(err)
	}
	if err := DecodeStrict([]byte(`{"a":[null,""],"n":9007199254740991,"z":"汉字"}`), &b); err != nil {
		t.Fatal(err)
	}
	ha, err := Digest(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := Digest(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatal("same canonical input differs")
	}
	if _, ok := a.(map[string]any)["n"].(json.Number); !ok {
		t.Fatal("numeric precision lost")
	}
}
func TestAdmissionRejectsUnapprovedAndIncompatibleGraphs(t *testing.T) {
	c, g := sample(t)
	if err := ValidateGraph(g, c); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*Graph){
		"unknown capability":        func(g *Graph) { g.Nodes[1].Expert = "fake@1" },
		"wrong scope":               func(g *Graph) { g.Nodes[1].Expert = "parent-only@1" },
		"cycle":                     func(g *Graph) { g.Nodes[0].DependsOn = []string{"draft"} },
		"broken data edge":          func(g *Graph) { g.Nodes[1].DependsOn = []string{} },
		"duplicate node":            func(g *Graph) { g.Nodes[1].ID = "extract" },
		"changed policy":            func(g *Graph) { g.PolicyVersion = "new" },
		"condition from model text": func(g *Graph) { g.Nodes[2].Condition = &Condition{"business", "PASS"} },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			_, g := sample(t)
			change(&g)
			if ValidateGraph(g, c) == nil {
				t.Fatal("unsafe graph admitted")
			}
		})
	}
}
func TestImmutableContentAddressing(t *testing.T) {
	f := Files{t.TempDir()}
	raw := []byte(`{"answer":"source"}`)
	id, err := f.Put(raw)
	if err != nil {
		t.Fatal(err)
	}
	same, err := f.Put(raw)
	if err != nil || same != id {
		t.Fatal("not idempotent")
	}
	if _, err = f.Read("../secret"); err == nil {
		t.Fatal("path traversal accepted")
	}
	if err = os.WriteFile(f.Root+"/"+id+".json", []byte(`{"answer":"changed"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Read(id); err == nil {
		t.Fatal("corruption not detected")
	}
}
