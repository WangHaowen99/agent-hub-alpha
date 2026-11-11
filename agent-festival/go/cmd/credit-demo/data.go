package main

import (
	"fmt"
	"path"
	"strings"
)

// Mock business data behind the read-only tools. C006 is a clean upper-band
// applicant; C007 has a historical four-month overdue (the veto case).
var cases = map[string]map[string]map[string]any{
	"C006": {
		"application": {
			"case_id": "C006", "applicant": "王某（个体经营，汽车零部件批发）", "product": "个人经营性授信",
			"requested_line_usd": 50000, "purpose": "补充流动资金", "declared_assets_usd_millions": 35,
			"industry": "auto-parts-wholesale", "submitted_at": "2026-11-01",
		},
		"financials": {
			"case_id": "C006", "asset_valuation_report": map[string]any{"assets_usd_millions": 35.2, "appraiser": "华衡资产评估", "valued_at": "2026-10-15"},
			"liability_ratio": 0.41, "operating_cashflow_usd_millions": 2.1, "revenue_trend": "近三年营收年均增长 6%",
		},
		"bureau_summary": {
			"case_id": "C006", "reference_time": "2026-11-01", "overdue_events_last_6_months": 0,
			"longest_overdue_months_on_record": 1, "overdue_events_on_record": 1,
		},
		"bureau_full": {
			"case_id": "C006", "reference_time": "2026-11-01",
			"overdue_events": []any{map[string]any{"duration_months": 1, "months_ago": 26}},
		},
	},
	"C007": {
		"application": {
			"case_id": "C007", "applicant": "李某（个体经营，建材零售）", "product": "个人经营性授信",
			"requested_line_usd": 50000, "purpose": "门店扩张", "declared_assets_usd_millions": 35,
			"industry": "building-materials-retail", "submitted_at": "2026-11-02",
		},
		"financials": {
			"case_id": "C007", "asset_valuation_report": map[string]any{"assets_usd_millions": 35.0, "appraiser": "华衡资产评估", "valued_at": "2026-10-20"},
			"liability_ratio": 0.55, "operating_cashflow_usd_millions": 1.2, "revenue_trend": "近三年营收持平",
		},
		"bureau_summary": {
			"case_id": "C007", "reference_time": "2026-11-02", "overdue_events_last_6_months": 0,
			"longest_overdue_months_on_record": 4, "overdue_events_on_record": 2,
		},
		"bureau_full": {
			"case_id": "C007", "reference_time": "2026-11-02",
			"overdue_events": []any{map[string]any{"duration_months": 4, "months_ago": 24}, map[string]any{"duration_months": 1, "months_ago": 13}},
		},
	},
}

// Related parties per case and a credit summary per related company.
var related = map[string]map[string]any{
	"C006": {"case_id": "C006", "related_parties": []any{
		map[string]any{"company_id": "R-601", "name": "王氏汽配贸易有限公司", "relation": "申请人持股 60%"},
		map[string]any{"company_id": "R-602", "name": "顺达物流有限公司", "relation": "申请人配偶任法人"},
		map[string]any{"company_id": "R-603", "name": "宏图机械配件厂", "relation": "申请人为其贷款提供担保"},
	}},
	"C007": {"case_id": "C007", "related_parties": []any{
		map[string]any{"company_id": "R-701", "name": "李记建材有限公司", "relation": "申请人持股 80%"},
	}},
}

var relatedBureau = map[string]map[string]any{
	"R-601": {"company_id": "R-601", "overdue_events_last_12_months": 0, "outstanding_loans_usd_millions": 1.2, "guarantees_given": 0},
	"R-602": {"company_id": "R-602", "overdue_events_last_12_months": 0, "outstanding_loans_usd_millions": 0.4, "guarantees_given": 1},
	"R-603": {"company_id": "R-603", "overdue_events_last_12_months": 1, "longest_overdue_months": 1, "outstanding_loans_usd_millions": 2.6, "guarantees_given": 0, "note": "担保链上一笔贷款曾逾期 1 个月，已结清"},
	"R-701": {"company_id": "R-701", "overdue_events_last_12_months": 2, "longest_overdue_months": 3, "outstanding_loans_usd_millions": 3.1},
}

// relatedTo reports whether company belongs to the case's related parties.
func relatedTo(caseID, company string) bool {
	rp, ok := related[strings.ToUpper(caseID)]
	if !ok {
		return false
	}
	list, _ := rp["related_parties"].([]any)
	for _, item := range list {
		if m, ok := item.(map[string]any); ok && m["company_id"] == strings.ToUpper(company) {
			return true
		}
	}
	return false
}

var industries = map[string]map[string]any{
	"auto-parts-wholesale":      {"industry": "auto-parts-wholesale", "name": "汽车零部件批发", "outlook": "稳定", "risks": []any{"整车厂账期拉长", "价格竞争"}, "default_rate_pct": 1.8},
	"building-materials-retail": {"industry": "building-materials-retail", "name": "建材零售", "outlook": "承压", "risks": []any{"地产下行", "回款周期长"}, "default_rate_pct": 3.6},
}

// memberTool describes one read-only data tool and the selector it touches.
type memberTool struct {
	name, desc, arg string
	selector        func(arg string) string
	fetch           func(arg string) (map[string]any, bool)
}

func caseDoc(kind string) func(string) (map[string]any, bool) {
	return func(id string) (map[string]any, bool) {
		c, ok := cases[strings.ToUpper(id)]
		if !ok {
			return nil, false
		}
		d, ok := c[kind]
		return d, ok
	}
}

var memberTools = []memberTool{
	{"get_application", "读取授信申请材料", "case_id", func(a string) string { return "data:credit/applications/" + strings.ToUpper(a) }, caseDoc("application")},
	{"get_financials", "读取资产评估与经营财务数据", "case_id", func(a string) string { return "data:credit/financials/" + strings.ToUpper(a) }, caseDoc("financials")},
	// The industry is resolved from the case's application: the researcher only
	// knows the case, and a guessed industry code would read nothing.
	{"get_industry_report", "读取该案申请人所在行业的行业报告", "case_id", func(a string) string { return "data:industry/" + industryOf(a) },
		func(a string) (map[string]any, bool) { d, ok := industries[industryOf(a)]; return d, ok }},
	{"query_bureau_summary", "调取征信摘要", "case_id", func(a string) string { return "data:credit/bureau/summary/" + strings.ToUpper(a) }, caseDoc("bureau_summary")},
	{"query_bureau_full", "调取完整征信报告", "case_id", func(a string) string { return "data:credit/bureau/full/" + strings.ToUpper(a) }, caseDoc("bureau_full")},
	{"get_related_parties", "读取申请人的关联方名单（关联企业、担保关系）", "case_id", func(a string) string { return "data:credit/related/" + strings.ToUpper(a) },
		func(a string) (map[string]any, bool) { d, ok := related[strings.ToUpper(a)]; return d, ok }},
	{"query_company_bureau", "调取一家关联企业的征信摘要（参数为 company_id）", "company_id", func(a string) string { return "data:credit/related-bureau/" + strings.ToUpper(a) },
		func(a string) (map[string]any, bool) { d, ok := relatedBureau[strings.ToUpper(a)]; return d, ok }},
}

func industryOf(caseID string) string {
	if app, ok := caseDoc("application")(caseID); ok {
		return fmt.Sprint(app["industry"])
	}
	return ""
}

func memberToolByName(name string) *memberTool {
	for i := range memberTools {
		if memberTools[i].name == name {
			return &memberTools[i]
		}
	}
	return nil
}

// allowedBy reports whether an expert's contract grants a read on sel.
func allowedBy(e *Expert, sel string) bool {
	for _, g := range e.requires {
		if g.kind != "data" {
			continue
		}
		if ok, _ := path.Match(strings.TrimPrefix(g.selector, "data:"), strings.TrimPrefix(sel, "data:")); ok {
			return true
		}
	}
	return false
}

// declaredSelector is what an investigate step is expected to have read for
// its case: the evidence declaration checked against its evidence log.
func declaredSelector(tool, caseID string) string {
	t := memberToolByName(tool)
	if t == nil {
		return ""
	}
	return t.selector(caseID)
}
