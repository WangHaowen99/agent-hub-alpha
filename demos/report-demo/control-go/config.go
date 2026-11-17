package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Listen             string            `json:"listen"`
	PublicURL          string            `json:"public_url"`
	DSN                string            `json:"dsn"`
	MigratorDSN        string            `json:"migrator_dsn"`
	Scope              string            `json:"scope"`
	ArtifactRoot       string            `json:"artifact_root"`
	CatalogPath        string            `json:"catalog_path"`
	MaterialPath       string            `json:"material_path"`
	CompilerURL        string            `json:"compiler_url"`
	LeaseSeconds       int               `json:"lease_seconds"`
	ExecutionProfile   string            `json:"execution_profile"`
	ModelBindingDigest string            `json:"model_binding_digest,omitempty"`
	BusBinding         *BusBinding       `json:"bus_binding,omitempty"`
	Tokens             map[string]string `json:"tokens"`
	User               string            `json:"user"`
	UserPassword       string            `json:"user_password"`
	TestEffect         *TestEffectConfig `json:"test_effect,omitempty"`
}
type Material struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	Digest           string         `json:"digest"`
	ProjectionDigest string         `json:"projection_digest"`
	Pages            []MaterialPage `json:"pages"`
	Verification     string         `json:"verification"`
}
type MaterialPage struct {
	PDFPage     int    `json:"pdf_page"`
	PrintedPage string `json:"printed_page"`
	Text        string `json:"text"`
	Digest      string `json:"digest"`
}

func LoadConfig(path string) (Config, Catalog, Material, error) {
	var cfg Config
	var cat Catalog
	var m Material
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, cat, m, err
	}
	if err = json.Unmarshal(b, &cfg); err != nil {
		return cfg, cat, m, err
	}
	if cfg.DSN == "" || cfg.Scope == "" || cfg.Listen == "" || cfg.PublicURL == "" || cfg.CompilerURL == "" || cfg.User == "" || len(cfg.UserPassword) < 12 || cfg.LeaseSeconds < 3 {
		return cfg, cat, m, errors.New("required private configuration missing or invalid")
	}
	if cfg.ExecutionProfile != "" && cfg.ModelBindingDigest == "" {
		return cfg, cat, m, errors.New("published execution profile requires its reviewed actual-model binding")
	}
	if cfg.ExecutionProfile != "" && !cfg.BusBinding.valid() {
		return cfg, cat, m, errors.New("published execution profile requires an explicit Bus environment binding")
	}
	if cfg.TestEffect != nil {
		if err = cfg.TestEffect.validate(); err != nil {
			return cfg, cat, m, err
		}
	}
	seen := map[string]bool{}
	for _, role := range []string{"planner", "worker", "expert", "operator"} {
		token := cfg.Tokens[role]
		if len(token) < 24 || seen[token] {
			return cfg, cat, m, errors.New("unique scoped service tokens (24+ characters) required")
		}
		seen[token] = true
	}
	if cfg.TestEffect != nil && seen[cfg.TestEffect.Token] {
		return cfg, cat, m, errors.New("isolated provider credential must differ from model-facing service credentials")
	}
	for _, p := range []string{cfg.ArtifactRoot, cfg.CatalogPath, cfg.MaterialPath} {
		if !filepath.IsAbs(p) {
			return cfg, cat, m, errors.New("artifact/catalog/material paths must be absolute")
		}
	}
	b, err = os.ReadFile(cfg.CatalogPath)
	if err != nil {
		return cfg, cat, m, err
	}
	if err = DecodeStrict(b, &cat); err != nil {
		return cfg, cat, m, err
	}
	b, err = os.ReadFile(cfg.MaterialPath)
	if err != nil {
		return cfg, cat, m, err
	}
	if err = DecodeStrict(b, &m); err != nil {
		return cfg, cat, m, err
	}
	if m.ID == "" || m.Digest == "" || len(m.Pages) == 0 {
		return cfg, cat, m, errors.New("material snapshot missing")
	}
	for _, page := range m.Pages {
		if tokenDigest(page.Text) != strings.TrimPrefix(page.Digest, "sha256:") {
			return cfg, cat, m, errors.New("material page text digest differs")
		}
	}
	projection, err := Digest(m.Pages)
	if err != nil {
		return cfg, cat, m, err
	}
	if projection != m.ProjectionDigest {
		return cfg, cat, m, errors.New("material projection digest differs")
	}
	return cfg, cat, m, nil
}
