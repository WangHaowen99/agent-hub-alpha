package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openjiuwen/agent-festival/go/internal/rfc8785"
)

// ExecBinding is the business semantics an approval covers besides the graph
// and member contracts (AR-04, #27): the judgment spec, by content digest, and
// the execution config. A run resolves its own binding for as long as it
// lives; a newly published spec or model only applies to plans proposed after
// it.
type ExecBinding struct {
	JudgmentSpec string `json:"judgment_spec"`          // sha256 of the RFC 8785 canonical spec
	SpecVersion  string `json:"spec_version,omitempty"` // question_id@spec_version, for people
	Model        string `json:"model"`
}

// boundSpec is one resolvable judgment spec and the levels it allows.
type boundSpec struct {
	spec   map[string]any
	levels []string
}

var errUnbound = errors.New("任务在版本绑定上线前采纳，没有可解析的规约绑定")

func specDigest(spec map[string]any) (string, []byte, error) {
	canon, err := rfc8785.Canonicalize(spec)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(canon)
	return "sha256:" + hex.EncodeToString(sum[:]), canon, nil
}

func specVersion(spec map[string]any) string {
	q, _ := spec["question"].(map[string]any)
	return fmt.Sprintf("%v@%v", q["question_id"], q["spec_version"])
}

func levelsOf(spec map[string]any) []string {
	var out []string
	if lv, ok := spec["levels"].([]any); ok {
		for _, l := range lv {
			if m, ok := l.(map[string]any); ok {
				out = append(out, fmt.Sprint(m["name"]))
			}
		}
	}
	return append(out, "on_missing_evidence", "on_undefined_rule")
}

func (s *Service) specArchive(digest string) string {
	return filepath.Join(s.cfg.DataDir, "specs", strings.TrimPrefix(digest, "sha256:")+".json")
}

// archiveSpec keeps the current spec under its content digest so that runs
// adopted under it can still resolve it after the spec file is replaced.
func (s *Service) archiveSpec(spec map[string]any) (string, error) {
	d, canon, err := specDigest(spec)
	if err != nil {
		return "", err
	}
	path := s.specArchive(d)
	if _, err := os.Stat(path); err == nil {
		return d, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	var doc any
	if err := json.Unmarshal(canon, &doc); err != nil {
		return "", err
	}
	return d, writeJSON(path, doc)
}

// specFor resolves a bound spec by digest: the current one, or an archived
// copy whose content still hashes to the same digest.
func (s *Service) specFor(digest string) (*boundSpec, error) {
	if digest == s.specDigest {
		return &boundSpec{spec: s.spec, levels: s.levels}, nil
	}
	if bs, ok := s.specCache[digest]; ok {
		return bs, nil
	}
	raw, err := os.ReadFile(s.specArchive(digest))
	if err != nil {
		return nil, fmt.Errorf("规约 %s 不可解析：%w", digest, err)
	}
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("规约 %s 存档损坏：%w", digest, err)
	}
	if d, _, err := specDigest(spec); err != nil || d != digest {
		return nil, fmt.Errorf("规约 %s 存档内容与摘要不符", digest)
	}
	bs := &boundSpec{spec: spec, levels: levelsOf(spec)}
	if s.specCache == nil {
		s.specCache = map[string]*boundSpec{}
	}
	s.specCache[digest] = bs
	return bs, nil
}

// runSpec is the spec a run was adopted under.
func (s *Service) runSpec(r *Run) (*boundSpec, error) {
	if r.Binding == nil {
		return nil, errUnbound
	}
	return s.specFor(r.Binding.JudgmentSpec)
}

// runModel is the execution config a run was adopted under. Runs adopted
// before bindings existed keep the configured model, as before the fix.
func (s *Service) runModel(r *Run) string {
	if r.Binding != nil {
		return r.Binding.Model
	}
	return s.cfg.Model
}

func (p *Plan) hasJudge() bool {
	for _, a := range p.Agents {
		if e := expertByID(a.Expert); e != nil && e.Kind == "judge" {
			return true
		}
	}
	return false
}
