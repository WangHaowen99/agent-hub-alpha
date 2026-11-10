package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Material is the business content of one credit application command. Its
// digest is the h in (k, h): same command key with a different digest is a
// conflict, never a silent replacement.
type Material struct {
	CustomerID             string `json:"customer_id" jsonschema:"customer identifier"`
	AssetsWan              int    `json:"assets_wan" jsonschema:"total assets in units of 10,000 CNY"`
	AssetReportOK          bool   `json:"asset_report_ok" jsonschema:"whether the asset valuation report is available"`
	LongestOverdueMonths   int    `json:"longest_overdue_months" jsonschema:"longest overdue duration on record in months; 0 if none"`
	LatestOverdueMonthsAgo int    `json:"latest_overdue_months_ago" jsonschema:"how many months ago the most recent overdue happened; 0 if none"`
	Scenario               string `json:"scenario,omitempty" jsonschema:"test provider behaviour: normal, decline, timeout, lost_receipt, duplicate_receipt"`
}

// digest hashes the material with a fixed field order. A spike stand-in for
// the versioned canonical form the contract requires (RFC 8785 in the
// reference components).
func (m Material) digest() string {
	raw, _ := json.Marshal(m)
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Judgment is the S1 stand-in's typed output, in the judgment-spec-credit
// vocabulary: deny / grant-50000 / grant-10000 / on_missing_evidence /
// on_undefined_rule.
type Judgment struct {
	Kind    string `json:"kind"`
	Details string `json:"details"`
}

// judge mirrors the corrected credit rules of the reference e2e stand-in:
// the >3-month veto is unbounded and has precedence; the upper band needs a
// clean six-month window; uncovered cases are explicit, never defaulted.
func judge(m Material) Judgment {
	if m.LongestOverdueMonths > 3 {
		return Judgment{"deny", fmt.Sprintf("historical veto: a %d-month overdue is on record", m.LongestOverdueMonths)}
	}
	if !m.AssetReportOK {
		return Judgment{"on_missing_evidence", "asset valuation not available; request evidence"}
	}
	if m.AssetsWan > 3000 {
		if m.LatestOverdueMonthsAgo > 0 && m.LatestOverdueMonthsAgo <= 6 {
			return Judgment{"on_undefined_rule", "upper band premise unsatisfied: an overdue inside the six-month window"}
		}
		return Judgment{"grant-50000", "assets above the upper cutoff with a clean six-month window"}
	}
	if m.AssetsWan < 100 {
		return Judgment{"grant-10000", "assets below the lower cutoff"}
	}
	return Judgment{"on_undefined_rule", "assets between the bands: no rule covers this case"}
}

// grantAmountMinor maps a grant judgment to the fixed request amount in USD
// cents; zero means the judgment produces no business request.
func grantAmountMinor(kind string) int64 {
	switch kind {
	case "grant-50000":
		return 5_000_000
	case "grant-10000":
		return 1_000_000
	}
	return 0
}
