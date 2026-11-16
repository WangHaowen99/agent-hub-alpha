package com.openjiuwen.demo.report;

import static org.junit.jupiter.api.Assertions.*;
import org.junit.jupiter.api.Test;

class ReportSchemaTest {
    @Test void rejectsMalformedReportAndEvidenceWithValidLookingValues() {
        var valid = Json.read("""
            {"snapshot":"cmb-2025-annual-report-a","scope":{"entity":"招商银行股份有限公司","basis":"本集团","currency_unit":"人民币百万元","period":"2025","compare_period":"2024"},"indicators":[],"sections":[{"id":"operating","findings":[{"text":"example","evidence":[{"indicator":"revenue"}]}]}],"limitations":["example"]}
            """);
        assertTrue(ReportSchema.validate(valid).isEmpty());
        var modified = valid.deepCopy();
        ((com.fasterxml.jackson.databind.node.ObjectNode) modified).put("unapproved", true);
        assertFalse(ReportSchema.validate(modified).isEmpty());
        modified = valid.deepCopy();
        ((com.fasterxml.jackson.databind.node.ObjectNode) modified.path("scope")).put("period", 2025);
        assertFalse(ReportSchema.validate(modified).isEmpty());
        modified = valid.deepCopy();
        ((com.fasterxml.jackson.databind.node.ObjectNode) modified.path("sections").get(0).path("findings").get(0).path("evidence").get(0)).put("quote", "untrusted mixed reference");
        assertFalse(ReportSchema.validate(modified).isEmpty());
    }
}
