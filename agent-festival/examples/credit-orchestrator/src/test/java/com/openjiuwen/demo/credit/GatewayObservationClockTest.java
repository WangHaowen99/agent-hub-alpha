package com.openjiuwen.demo.credit;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.time.Clock;
import java.time.Instant;
import java.time.ZoneId;
import java.time.ZoneOffset;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CopyOnWriteArrayList;

import static org.junit.jupiter.api.Assertions.*;

/**
 * A wall-clock step back during a Gateway call (WSL2/VM time correction) must
 * not produce an observation whose finished_at precedes started_at: Go rejects
 * it (time_order) together with the update that carries the real task state,
 * and the node then needs a second manual resume (W02 D2, R9 report 400).
 */
class GatewayObservationClockTest {
    private static final ObjectMapper JSON = new ObjectMapper();

    /** Every read is 50 ms earlier than the previous one. */
    private static final class SteppingBackClock extends Clock {
        private final Instant base = Instant.parse("2026-11-18T07:58:03.700Z");
        private int reads;
        @Override public ZoneId getZone() { return ZoneOffset.UTC; }
        @Override public Clock withZone(ZoneId zone) { return this; }
        @Override public synchronized Instant instant() { return base.minusMillis(50L * reads++); }
    }

    private static void reply(HttpExchange exchange, Object body) throws IOException {
        byte[] raw = JSON.writeValueAsBytes(body);
        exchange.getResponseHeaders().add("Content-Type", "application/json");
        exchange.sendResponseHeaders(200, raw.length);
        exchange.getResponseBody().write(raw);
        exchange.close();
    }

    @Test void observationIntervalSurvivesWallClockStepBack() throws Exception {
        List<JsonNode> observations = new CopyOnWriteArrayList<>();
        Map<String, Object> task = Map.of("id", "task-1", "contextId", "ctx-1");
        String artifact = JSON.writeValueAsString(Map.of("node_execution_id", "o-1", "input_digest", "h-1", "attempt", 1, "output", "report text"));
        HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        server.createContext("/exec/call/begin", exchange -> {
            exchange.getRequestBody().readAllBytes();
            reply(exchange, Map.of("action", "send", "call", Map.of(
                    "remote_task_id", "", "sequence", 0, "capability_token", "cap", "request_id", "req-1",
                    "message_id", "msg-1", "context_id", "ctx-1", "binding", Map.of("agent_id", "credit-writer"),
                    "node_execution_id", "o-1", "input_digest", "h-1", "attempt", 1)));
        });
        server.createContext("/exec/call/update", exchange -> {
            JsonNode body = JSON.readTree(exchange.getRequestBody().readAllBytes());
            if (body.has("gateway")) observations.add(body.path("gateway"));
            reply(exchange, Map.of("ok", true));
        });
        server.createContext("/exec/call/commit", exchange -> {
            exchange.getRequestBody().readAllBytes();
            reply(exchange, Map.of("output", "report text"));
        });
        server.createContext("/a2a", exchange -> {
            JsonNode rpc = JSON.readTree(exchange.getRequestBody().readAllBytes());
            boolean send = rpc.path("method").asText().equals("SendMessage");
            Map<String, Object> state = send
                    ? Map.of("state", "TASK_STATE_WORKING")
                    : Map.of("state", "TASK_STATE_COMPLETED");
            Map<String, Object> result = new java.util.LinkedHashMap<>(task);
            result.put("status", state);
            if (!send) result.put("artifacts", List.of(Map.of("parts", List.of(Map.of("text", artifact)))));
            reply(exchange, Map.of("jsonrpc", "2.0", "id", rpc.path("id").asText(), "result", Map.of("task", result)));
        });
        server.start();
        try {
            String base = "http://127.0.0.1:" + server.getAddress().getPort();
            var member = new RemoteAgentMember(new GoClient(base), "R1", "lease", "report",
                    Map.of("CREDIT_BUS_GATEWAY", base, "CREDIT_BUS_TOKEN", "bus"), new SteppingBackClock());
            assertEquals("report text", member.execute("input", "goal"));
        } finally { server.stop(0); }

        assertTrue(observations.size() >= 2, "a SendMessage and a GetTask observation were reported: " + observations);
        for (JsonNode o : observations) {
            Instant started = Instant.parse(o.path("started_at").asText());
            Instant finished = Instant.parse(o.path("finished_at").asText());
            assertFalse(finished.isBefore(started), "finished_at before started_at in " + o);
        }
    }
}
