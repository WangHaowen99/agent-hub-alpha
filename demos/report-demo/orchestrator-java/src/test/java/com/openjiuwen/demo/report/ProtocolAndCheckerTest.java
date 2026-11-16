package com.openjiuwen.demo.report;

import static org.junit.jupiter.api.Assertions.*;
import com.fasterxml.jackson.databind.JsonNode;
import com.sun.net.httpserver.HttpServer;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashSet;
import org.junit.jupiter.api.Assumptions;
import org.junit.jupiter.api.Test;

class ProtocolAndCheckerTest {
    @Test void humanResolutionMustMatchBothImmutableCheckAndReport() {
        var check=Json.read("{\"artifact_id\":\"check-1\",\"output\":{\"outcome\":\"HUMAN\",\"object_digest\":\"report-1\"}}");
        var review=Json.read("{\"check_artifact\":\"check-1\",\"object_digest\":\"report-1\",\"receipt\":\"user-review-1\",\"decision\":\"ACCEPT\"}");
        assertEquals("PASS",ReportChecker.effectiveOutcome(check,review));
        assertEquals("HUMAN",check.path("output").path("outcome").asText());
        ((com.fasterxml.jackson.databind.node.ObjectNode)review).put("object_digest","other-report");
        assertEquals("HUMAN",ReportChecker.effectiveOutcome(check,review));
    }
    @Test void nativeV1ResultTaskAndLegacyDirectTaskAreBothRead() {
        var nested=Json.read("{\"result\":{\"task\":{\"id\":\"remote\",\"contextId\":\"ctx\",\"status\":{\"state\":\"TASK_STATE_WORKING\"}}}}");
        assertEquals("remote",RemoteExpertExecutor.taskResult(nested).path("id").asText());
        assertEquals("remote",RemoteExpertExecutor.taskResult(Json.read("{\"result\":{\"id\":\"remote\",\"status\":{\"state\":\"working\"}}}")).path("id").asText());
        assertThrows(ControlClient.ControlFailure.class,()->RemoteExpertExecutor.taskResult(Json.read("{\"result\":{\"message\":{}}}")));
    }
    @Test void discoveryRejectsWrongVersionAndEmptyCandidates() {
        var card=Json.read("""
                {"name":"report-extractor","rawJson":"{\\"name\\":\\"report-extractor\\",\\"version\\":\\"1.0.0\\"}"}
                """);
        var candidates=Json.read("""
                {"instances":[{"health":"ONLINE","contractVersion":"1.0.0","capabilityVersion":"1.0.0","card":{"name":"report-extractor"}}]}
                """);
        assertDoesNotThrow(()->RemoteExpertExecutor.validateDiscovery("report-extractor",card,candidates));
        assertThrows(ControlClient.ControlFailure.class,()->RemoteExpertExecutor.validateDiscovery("other-agent",card,candidates));
        assertThrows(ControlClient.ControlFailure.class,()->RemoteExpertExecutor.validateDiscovery("report-extractor",card,Json.read("{\"instances\":[]}")));
    }
    @Test void mixedArtifactPartsAreNotBlindlyConcatenated() {
        var envelope = RemoteExpertExecutor.resultEnvelope(Json.read("""
                {"artifacts":[{"parts":[{"text":"unrelated explanatory prose"},
                {"data":{"call_id":"c1","input_digest":"h1","attempt":1,"output":{"value":2}}}]}]}
                """));
        assertEquals(2, envelope.path("output").path("value").asInt());
        assertEquals("INPUT_REQUIRED", RemoteExpertExecutor.normalizeState("TASK_STATE_INPUT_REQUIRED"));
        assertEquals("AUTH_REQUIRED", RemoteExpertExecutor.normalizeState("auth-required"));
        assertThrows(ControlClient.ControlFailure.class, () -> RemoteExpertExecutor.normalizeState("invented-state"));
    }

    @Test void httpConflictRemainsInputConflict() throws Exception {
        var server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        server.createContext("/conflict", exchange -> {
            byte[] body = "{\"error\":{\"code\":\"INPUT_CONFLICT\"}}".getBytes(StandardCharsets.UTF_8);
            exchange.sendResponseHeaders(409, body.length); exchange.getResponseBody().write(body); exchange.close();
        });
        server.start();
        try {
            var client = new ControlClient("http://127.0.0.1:" + server.getAddress().getPort(), "isolated-test");
            var error = assertThrows(ControlClient.ControlFailure.class, () -> client.get("/conflict"));
            assertEquals("INPUT_CONFLICT", error.code);
        } finally { server.stop(0); }
    }

    @Test void w01CasesAreCheckedAgainstRealSourceProjection() throws Exception {
        String materialPath = System.getenv("REPORT_TEST_MATERIAL");
        Assumptions.assumeTrue(materialPath != null, "Fixed PDF source projection required");
        JsonNode material = Json.read(Files.readString(Path.of(materialPath)));
        Path scene = Path.of(System.getProperty("basedir"), "..", "scenes", "annual-report", "acceptance", "counterexamples.json");
        JsonNode corpus = Json.read(Files.readString(scene));
        int count = 0;
        for (JsonNode test : corpus.path("cases")) {
            var errors = new ArrayList<String>(); var human = new ArrayList<String>();
            JsonNode fragment = test.path("fragment");
            if (fragment.has("indicator")) { errors.addAll(ReportChecker.checkIndicator(fragment.path("indicator"), material)); }
            if (fragment.has("finding")) { errors.addAll(ReportChecker.checkFinding(fragment.path("finding"), material, new HashSet<>(), human)); }
            String actual = !errors.isEmpty() ? "fail" : !human.isEmpty() ? "human" : "pass";
            assertEquals(test.path("expected").asText(), actual, test.path("id").asText() + ": " + errors + human);
            count++;
        }
        assertEquals(12, count);
    }
}
