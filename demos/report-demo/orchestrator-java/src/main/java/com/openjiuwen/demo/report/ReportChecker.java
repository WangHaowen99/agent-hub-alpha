package com.openjiuwen.demo.report;

import com.fasterxml.jackson.databind.JsonNode;
import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Pattern;

/** Program checks actual source pages; acceptance answers are never runtime inputs. */
final class ReportChecker {
    static final String VERSION = "annual-report-checker/1";
    private static final Map<String, String> LABELS = Map.of(
            "revenue", "营业收入", "net_profit_attributable", "归属于本行股东的净利润",
            "net_interest_margin", "净利息收益率", "cost_to_income", "成本收入比",
            "npl_balance", "不良贷款", "npl_ratio", "不良贷款率",
            "provision_coverage", "拨备覆盖率", "cet1_ratio_advanced", "核心一级资本充足率");
    private static final Set<String> MONEY = Set.of("revenue", "net_profit_attributable", "npl_balance");
    private final ControlClient control;
    ReportChecker(ControlClient control) { this.control = control; }

    JsonNode check(JsonNode binding) {
        JsonNode report = null; String digest = null;
        for (JsonNode result : binding.path("upstream")) {
            if (result.path("output").has("indicators") && result.path("output").has("scope")) {
                report = result.path("output"); digest = result.path("artifact_digest").asText();
            }
        }
        if (report == null) { throw new ControlClient.ControlFailure("REPORT_SCHEMA_MISMATCH", 422); }
        JsonNode material = control.get("/v1/material");
        List<String> failures = new ArrayList<>(ReportSchema.validate(report)); List<String> human = new ArrayList<>();
        if (!report.path("snapshot").asText().equals(material.path("id").asText())) { failures.add("snapshot mismatch"); }
        JsonNode scope = report.path("scope");
        if (!scope.path("basis").asText().equals("本集团") || !scope.path("period").asText().equals("2025")
                || !scope.path("compare_period").asText().equals("2024")
                || !scope.path("currency_unit").asText().equals("人民币百万元")
                || !scope.path("entity").asText().equals("招商银行股份有限公司")) { failures.add("scope/period/currency mismatch"); }
        Set<String> seen = new HashSet<>();
        for (JsonNode indicator : report.path("indicators")) {
            if (!seen.add(indicator.path("id").asText())) { failures.add("duplicate indicator"); }
            failures.addAll(checkIndicator(indicator, material));
        }
        if (seen.isEmpty()) { failures.add("no indicators"); }
        if (binding.path("focus").asText().contains("quality") && !seen.containsAll(LABELS.keySet())) { failures.add("full report requires eight indicators"); }
        Set<String> sectionIds = new HashSet<>();
        for (JsonNode section : report.path("sections")) {
            if (!sectionIds.add(section.path("id").asText())) { failures.add("duplicate section"); }
            if (!Set.of("operating", "asset_quality", "risks").contains(section.path("id").asText())) { failures.add("unknown section"); }
            for (JsonNode finding : section.path("findings")) { failures.addAll(checkFinding(finding, material, seen, human)); }
        }
        if (!sectionIds.containsAll(Set.of("operating", "asset_quality", "risks"))) { failures.add("required sections missing"); }
        if (!report.path("limitations").isArray() || report.path("limitations").isEmpty()) { failures.add("limitations required"); }
        String outcome = !failures.isEmpty() ? "REVISE" : !human.isEmpty() ? "HUMAN" : "PASS";
        return Json.MAPPER.valueToTree(Map.of("checker_version", VERSION, "object_digest", digest, "outcome", outcome,
                "failures", failures, "human_review", human,
                "scope", "structure, source rows, units, periods, arithmetic and bounded numeric claims; no global semantic guarantee"));
    }

    JsonNode select(JsonNode binding) {
        JsonNode selected = null;JsonNode selectedReview=Json.MAPPER.nullNode();
        for (JsonNode upstream : binding.path("upstream")) {
            JsonNode review=Json.MAPPER.nullNode();
            for(JsonNode candidate:binding.path("reviews")){if(candidate.path("check_artifact").asText().equals(upstream.path("artifact_id").asText())){review=candidate;}}
            if(effectiveOutcome(upstream,review).equals("PASS")){selected=upstream.path("output");selectedReview=review;}
        }
        if (selected == null) { throw new ControlClient.ControlFailure("INPUT_REQUIRED", 409); }
        return Json.MAPPER.valueToTree(Map.of("artifact_digest", selected.path("object_digest").asText(),
                "check_version", selected.path("checker_version").asText(), "status", "CHECKED_NOT_USER_ACCEPTED",
                "qualification",selectedReview.isNull()?"PROGRAM_CHECK":"PROGRAM_CHECK_PLUS_HUMAN_REVIEW","human_review",selectedReview));
    }

    static String effectiveOutcome(JsonNode result,JsonNode review){
        String outcome=result.path("output").path("outcome").asText("UNKNOWN");
        if(outcome.equals("HUMAN")&&review.isObject()&&review.path("check_artifact").asText().equals(result.path("artifact_id").asText())
                &&review.path("object_digest").asText().equals(result.path("output").path("object_digest").asText())&&!review.path("receipt").asText().isBlank()){
            return switch(review.path("decision").asText()){case "ACCEPT"->"PASS";case "REVISE"->"REVISE";case "REJECT"->"FAIL";default->"HUMAN";};
        }
        return outcome;
    }

    private static Pattern rowPattern(String label) {
        return Pattern.compile("^\\s*" + Pattern.quote(label) + "(?:[（(]\\d+[）)])?\\s+([\\d,]+(?:\\.\\d+)?)\\s+([\\d,]+(?:\\.\\d+)?)", Pattern.MULTILINE);
    }

    static List<String> checkIndicator(JsonNode indicator, JsonNode material) {
        List<String> errors = new ArrayList<>(); String id = indicator.path("id").asText(); String label = LABELS.get(id);
        if (label == null) { errors.add("unsupported indicator " + id); return errors; }
        if (!indicator.path("unit").asText().equals(MONEY.contains(id) ? "人民币百万元" : "%")) { errors.add(id + ": unit mismatch"); }
        errors.addAll(checkCitation(indicator.path("citation"), material));
        var match = rowPattern(label).matcher(indicator.path("citation").path("quote").asText());
        if (!match.find()) { errors.add(id + ": citation is not the requested metric row"); return errors; }
        try {
            BigDecimal actual = new BigDecimal(match.group(1).replace(",", "")); BigDecimal previous = new BigDecimal(match.group(2).replace(",", ""));
            if (!indicator.path("values").path("2025").isNumber() || !indicator.path("values").path("2024").isNumber()
                    || actual.compareTo(indicator.path("values").path("2025").decimalValue()) != 0
                    || previous.compareTo(indicator.path("values").path("2024").decimalValue()) != 0) { errors.add(id + ": value/period differs from source row"); }
            BigDecimal expected = MONEY.contains(id) ? actual.subtract(previous).multiply(BigDecimal.valueOf(100)).divide(previous, 2, RoundingMode.HALF_UP)
                    : actual.subtract(previous).setScale(2, RoundingMode.HALF_UP);
            if (!indicator.path("change").path("kind").asText().equals(MONEY.contains(id) ? "pct" : "pp")
                    || !indicator.path("change").path("value").isNumber()
                    || expected.compareTo(indicator.path("change").path("value").decimalValue()) != 0) { errors.add(id + ": change kind/arithmetic mismatch"); }
        } catch (ArithmeticException | NumberFormatException e) { errors.add(id + ": invalid arithmetic basis"); }
        return errors;
    }

    static List<String> checkCitation(JsonNode citation, JsonNode material) {
        List<String> errors = new ArrayList<>();
        if (citation.has("snapshot") && !citation.path("snapshot").asText().equals(material.path("id").asText())) { errors.add("citation outside fixed snapshot"); }
        String quote = citation.path("quote").asText(); if (quote.isBlank()) { errors.add("empty citation"); return errors; }
        JsonNode page = null;
        for (JsonNode p : material.path("pages")) { if (p.path("pdf_page").asInt() == citation.path("pdf_page").asInt()) { page = p; } }
        if (page == null) { errors.add("citation page not in verified projection"); return errors; }
        if (!page.path("printed_page").asText().equals(citation.path("printed_page").asText())) { errors.add("printed page mismatch"); }
        if (!compact(page.path("text").asText()).contains(compact(quote))) { errors.add("quote not present on cited page"); }
        return errors;
    }

    static List<String> checkFinding(JsonNode finding, JsonNode material, Set<String> indicators, List<String> human) {
        List<String> errors = new ArrayList<>(); String text = finding.path("text").asText();
        if (text.isBlank() || !finding.path("evidence").isArray() || finding.path("evidence").isEmpty()) { errors.add("finding/evidence missing"); return errors; }
        boolean supported = false;
        for (JsonNode evidence : finding.path("evidence")) {
            if (evidence.has("indicator")) { if (!indicators.contains(evidence.path("indicator").asText())) { errors.add("finding references missing indicator"); } }
            else {
                errors.addAll(checkCitation(evidence, material)); String quote = evidence.path("quote").asText();
                if (compact(quote).equals(compact(text))) { supported = true; }
                if (text.contains("上升") && quote.contains("下降") || text.contains("下降") && quote.contains("上升")) { errors.add("finding direction contradicts source"); }
                // A deliberately bounded claim form is checked arithmetically.
                // Other prose remains HUMAN rather than delegated to an LLM judge.
                for (String label : LABELS.values()) {
                    var claim = Pattern.compile("^" + Pattern.quote(label) + "较上年(?:末)?(上升|下降)\\s*([\\d.]+)\\s*个百分点至\\s*([\\d.]+)%$").matcher(text);
                    var row = rowPattern(label).matcher(quote);
                    if (claim.matches() && row.find()) {
                        BigDecimal actual = new BigDecimal(row.group(1).replace(",", "")); BigDecimal prior = new BigDecimal(row.group(2).replace(",", ""));
                        BigDecimal change = actual.subtract(prior);
                        if (actual.compareTo(new BigDecimal(claim.group(3))) == 0 && change.abs().compareTo(new BigDecimal(claim.group(2))) == 0
                                && (change.signum() < 0 ? "下降" : "上升").equals(claim.group(1))) { supported = true; }
                        else { errors.add("finding numeric claim differs from source"); }
                    }
                }
            }
        }
        if (!supported) { human.add("Review finding support: " + text); }
        return errors;
    }
    private static String compact(String text) { return text.replaceAll("\\s+", ""); }
}
