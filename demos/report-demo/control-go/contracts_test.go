package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExportedContractsMatchOwningTypes(t *testing.T) {
	temp := t.TempDir()
	if err := ExportContracts(temp); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(temp, "*.schema.json"))
	if err != nil || len(files) != 13 {
		t.Fatal("contract set incomplete", err)
	}
	for _, generated := range files {
		want, err := os.ReadFile(generated)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join("../contracts", filepath.Base(generated)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(want, got) {
			t.Fatal("contract drift; regenerate with -export-contracts", filepath.Base(generated))
		}
	}
}

func TestCanonicalGoldenVectors(t *testing.T) {
	raw, err := os.ReadFile("../contracts/canonical-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var suite struct {
		Vectors []struct {
			ID        string `json:"id"`
			Input     string `json:"input"`
			Canonical string `json:"canonical"`
			Digest    string `json:"digest"`
		} `json:"vectors"`
	}
	if err = json.Unmarshal(raw, &suite); err != nil {
		t.Fatal(err)
	}
	if len(suite.Vectors) < 7 {
		t.Fatal("canonical fixture matrix incomplete")
	}
	for _, v := range suite.Vectors {
		t.Run(v.ID, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.json")
			if err := os.WriteFile(path, []byte(v.Input), 0600); err != nil {
				t.Fatal(err)
			}
			out, err := CanonicalJSONFile(path)
			if err != nil {
				t.Fatal(err)
			}
			result := out.(map[string]any)
			if result["canonical"] != v.Canonical || result["digest"] != v.Digest {
				t.Fatal("cross-language canonical fixture differs", v.ID)
			}
		})
	}
}
