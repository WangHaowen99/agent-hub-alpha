package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-jose/go-jose/v4"
	"github.com/openjiuwen/agent-festival/go/internal/rfc8785"
	"github.com/openjiuwen/agent-festival/go/internal/validate"
)

// Expert is one registered capability x_i. The contract is the festival L3
// capability contract; the member fields tell the executor how to run it.
type Expert struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind"` // investigate | blackbox | judge | write | effect
	Summary     string   `json:"summary"`
	Tools       []string `json:"tools,omitempty"`
	Prompt      string   `json:"-"`
	MaxIter     int      `json:"-"`
	requires    []grant
	effectLevel string
	compensate  string
	costCents   int
	calls       int
	seconds     int
}

type grant struct {
	kind, selector string
	actions        []string
	effect         string
	purpose        string
}

func (g grant) doc() map[string]any {
	m := map[string]any{
		"resource": map[string]any{"kind": g.kind, "selector": g.selector},
		"actions":  toAny(g.actions),
		"effect":   g.effect,
	}
	if g.purpose != "" {
		m["purpose"] = g.purpose
	}
	return m
}

func read(sel, purpose string) grant {
	return grant{kind: "data", selector: sel, actions: []string{"read"}, effect: "none", purpose: purpose}
}

// experts is the demo catalog. Selectors follow the repo's credit slice
// fixture (compose/valid/credit-slice-a001.json).
var experts = []*Expert{
	{ID: "intake", Name: "材料受理", Kind: "investigate", Tools: []string{"get_application"},
		Summary:     "读取申请材料，整理成结构化字段",
		Prompt:      "你是授信材料受理员。必须调用 get_application 读取申请材料，再把申请人、申请额度、用途、资产申报等字段整理成清单。只依据工具返回的内容，不要编造。",
		requires:    []grant{read("data:credit/applications/*", "application materials")},
		effectLevel: "none", costCents: 20, calls: 1, seconds: 120},
	{ID: "financial-analyst", Name: "财务分析师", Kind: "investigate", Tools: []string{"get_financials"},
		Summary:     "读取资产与经营数据，计算资产规模和偿债指标",
		Prompt:      "你是财务分析师。必须调用 get_financials 读取该申请人的资产与经营数据，给出资产规模（USD-millions）、负债率、经营现金流等结论。只依据工具返回的内容。",
		requires:    []grant{read("data:credit/financials/*", "asset valuation and financials")},
		effectLevel: "none", costCents: 40, calls: 2, seconds: 180},
	{ID: "industry-researcher", Name: "行业研究员", Kind: "investigate", Tools: []string{"get_industry_report"},
		Summary:     "查询行业景气度与同业风险",
		Prompt:      "你是行业研究员。必须调用 get_industry_report（参数为案件号 case_id）读取申请人所在行业的报告，概括景气度和主要风险。只依据工具返回的内容。",
		requires:    []grant{read("data:industry/*", "industry report")},
		effectLevel: "none", costCents: 30, calls: 1, seconds: 120},
	{ID: "bureau-full", Name: "征信全量查询员", Kind: "investigate", Tools: []string{"query_bureau_full"},
		Summary:     "调取完整征信报告（逐笔还款记录）",
		Prompt:      "你是征信查询员。必须调用 query_bureau_full 调取完整征信报告，列出全部逾期记录（逾期时长、距今月数）。只依据工具返回的内容。",
		requires:    []grant{read("data:credit/bureau/full/*", "full credit bureau report")},
		effectLevel: "none", costCents: 50, calls: 1, seconds: 120},
	{ID: "bureau-summary", Name: "征信摘要查询员", Kind: "investigate", Tools: []string{"query_bureau_summary"},
		Summary:     "调取征信摘要（近六个月逾期、历史最长逾期）",
		Prompt:      "你是征信查询员。必须调用 query_bureau_summary 调取征信摘要，给出近六个月是否有逾期、历史最长逾期月数。只依据工具返回的内容。",
		requires:    []grant{read("data:credit/bureau/summary/*", "credit bureau summary")},
		effectLevel: "none", costCents: 20, calls: 1, seconds: 120},
	{ID: "related-party", Name: "关联方排查", Kind: "blackbox", Tools: []string{"get_related_parties", "query_company_bureau"},
		Summary:     "黑盒节点（Loop Forest）：自主拆分，先查关联方名单，再逐个分叉排查征信，运行时才展开",
		requires:    []grant{read("data:credit/related/*", "related-party list"), read("data:credit/related-bureau/*", "credit summary of related companies")},
		effectLevel: "none", costCents: 80, calls: 5, seconds: 300},
	{ID: "risk-judge", Name: "风险判断（S1）", Kind: "judge",
		Summary:     "按已发布的授信判断规约，对上游证据给出结构化判断",
		requires:    []grant{read("data:credit/evidence/*", "upstream evidence of this case")},
		effectLevel: "none", costCents: 30, calls: 0, seconds: 120},
	{ID: "report-writer", Name: "报告撰写人", Kind: "write",
		Summary:     "汇总各步结论，写出尽调报告",
		Prompt:      "你是授信尽调报告撰写人。根据上游各步产物写一份简明的尽调报告：申请概况、财务、行业、征信、判断结论与依据。不要引入上游没有的信息。",
		requires:    []grant{read("data:credit/evidence/*", "upstream evidence of this case")},
		effectLevel: "none", costCents: 30, calls: 0, seconds: 180},
	{ID: "credit-submitter", Name: "授信申请提交", Kind: "effect", Tools: []string{"credit.apply"},
		Summary:     "按已采纳的判断，经网关向银行提交授信申请（不可逆）",
		requires:    []grant{{kind: "tool", selector: "credit.apply", actions: []string{"execute"}, effect: "irreversible", purpose: "submit the credit application"}},
		effectLevel: "irreversible", compensate: "manual", costCents: 0, calls: 1, seconds: 60},
}

func expertByID(id string) *Expert {
	for _, e := range experts {
		if e.ID == id {
			return e
		}
	}
	return nil
}

// contract renders the festival L3 capability contract of an expert.
func (e *Expert) contract() map[string]any {
	reqs := make([]any, 0, len(e.requires))
	for _, g := range e.requires {
		reqs = append(reqs, g.doc())
	}
	effects := map[string]any{"level": e.effectLevel}
	if e.compensate != "" {
		effects["compensation"] = e.compensate
	}
	return jsonDoc(map[string]any{
		"contract_version": "0.1.0",
		"subject":          map[string]any{"id": e.ID, "version": "1.0.0"},
		"provides":         []any{map[string]any{"skill_id": e.ID, "description": e.Summary}},
		"requires":         reqs,
		"effects":          effects,
		"bounds": map[string]any{
			"max_cost":             map[string]any{"amount": e.costCents, "unit": "USD-cents"},
			"max_seconds":          e.seconds,
			"max_downstream_calls": e.calls,
		},
	})
}

// Catalog holds the signed envelopes and the trust store that verifies them.
type Catalog struct {
	Envelopes map[string]map[string]any
	Hashes    map[string]string
	trust     *validate.TrustStore
	kid       string
}

// LoadCatalog signs every contract with the publisher key (created on first
// start) and checks each envelope with the festival validator before use.
func LoadCatalog(dir string) (*Catalog, error) {
	key, err := loadOrCreateKey(filepath.Join(dir, "publisher.pem"))
	if err != nil {
		return nil, err
	}
	kid := "credit-demo-publisher"
	subjects := []any{}
	for _, e := range experts {
		subjects = append(subjects, e.ID)
	}
	jwks := map[string]any{
		"keys":       []any{publicJWK(key, kid)},
		"subject_of": map[string]any{kid: subjects},
	}
	jwksPath := filepath.Join(dir, "trusted.jwks.json")
	if err := writeJSON(jwksPath, jwks); err != nil {
		return nil, err
	}
	ts, err := validate.LoadTrustStore(jwksPath)
	if err != nil {
		return nil, fmt.Errorf("trust store: %w", err)
	}
	c := &Catalog{Envelopes: map[string]map[string]any{}, Hashes: map[string]string{}, trust: ts, kid: kid}
	for _, e := range experts {
		doc := e.contract()
		if errs := validate.ContractErrors(doc); len(errs) > 0 {
			return nil, fmt.Errorf("contract %s rejected: %s", e.ID, strings.Join(errs, "; "))
		}
		env, err := signEnvelope(doc, key, kid)
		if err != nil {
			return nil, err
		}
		c.Envelopes[e.ID] = env
		c.Hashes[e.ID], _ = env["contract_hash"].(string)
	}
	if problems := c.VerifyAll(); len(problems) > 0 {
		return nil, fmt.Errorf("catalog failed verification: %v", problems)
	}
	return c, nil
}

// Verify checks one envelope (signature, owner binding, contract hash).
func (c *Catalog) Verify(id string) []string {
	env, ok := c.Envelopes[id]
	if !ok {
		return []string{"unregistered capability " + id}
	}
	errs, _ := validate.EnvelopeErrors(validate.CloneDoc(env), c.trust, false)
	return errs
}

func (c *Catalog) VerifyAll() map[string][]string {
	out := map[string][]string{}
	for _, e := range experts {
		if errs := c.Verify(e.ID); len(errs) > 0 {
			out[e.ID] = errs
		}
	}
	return out
}

// Card is what the planner sees when choosing experts.
func (c *Catalog) Cards() []map[string]any {
	out := []map[string]any{}
	for _, e := range experts {
		doc := e.contract()
		out = append(out, map[string]any{
			"id": e.ID, "name": e.Name, "kind": e.Kind, "summary": e.Summary, "tools": e.Tools,
			"requires": doc["requires"], "effects": doc["effects"], "bounds": doc["bounds"],
			"contract_hash": c.Hashes[e.ID],
		})
	}
	return out
}

func signEnvelope(contract map[string]any, key *ecdsa.PrivateKey, kid string) (map[string]any, error) {
	canonical, err := rfc8785.Canonicalize(contract)
	if err != nil {
		return nil, err
	}
	opts := (&jose.SignerOptions{}).WithHeader("typ", "JOSE").WithHeader("kid", kid)
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: key}, opts)
	if err != nil {
		return nil, err
	}
	obj, err := signer.Sign(canonical)
	if err != nil {
		return nil, err
	}
	compact, err := obj.CompactSerialize()
	if err != nil {
		return nil, err
	}
	parts := strings.Split(compact, ".")
	hash, err := validate.ContractHash(contract)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"envelope_version": "0.1.0",
		"contract":         contract,
		"contract_hash":    hash,
		"signatures":       []any{map[string]any{"protected": parts[0], "signature": parts[2]}},
	}, nil
}

func loadOrCreateKey(path string) (*ecdsa.PrivateKey, error) {
	if raw, err := os.ReadFile(path); err == nil {
		block, _ := pem.Decode(raw)
		if block == nil {
			return nil, fmt.Errorf("%s: not PEM", path)
		}
		return x509.ParseECPrivateKey(block.Bytes)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func publicJWK(key *ecdsa.PrivateKey, kid string) map[string]any {
	enc := func(v *big.Int) string {
		b := make([]byte, 32)
		v.FillBytes(b)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return map[string]any{"kty": "EC", "crv": "P-256", "alg": "ES256", "use": "sig", "kid": kid,
		"x": enc(key.PublicKey.X), "y": enc(key.PublicKey.Y)}
}

func toAny(ss []string) []any {
	out := make([]any, 0, len(ss))
	for _, s := range ss {
		out = append(out, s)
	}
	return out
}

func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// jsonDoc round-trips v through JSON so numbers are float64, the shape the
// festival validators see when they load documents from disk.
func jsonDoc(v any) map[string]any {
	raw, _ := json.Marshal(v)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}
