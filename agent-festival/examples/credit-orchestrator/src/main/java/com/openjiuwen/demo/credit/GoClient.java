package com.openjiuwen.demo.credit;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.List;
import java.util.Map;

/** The executor's only line to the authoritative record (credit-demo, Go). */
final class GoClient {
    /** Thrown when the run's lease was taken over: this executor must stop writing. */
    static final class FencedException extends RuntimeException {
        FencedException(String message) {
            super(message);
        }
    }

    private static final ObjectMapper JSON = new ObjectMapper();
    private final HttpClient http = HttpClient.newBuilder().version(HttpClient.Version.HTTP_1_1)
            .connectTimeout(Duration.ofSeconds(5)).build();
    private final String base;

    GoClient(String base) {
        this.base = base;
    }

    JsonNode post(String path, Map<String, Object> body, int timeoutSeconds) {
        try {
            HttpRequest req = HttpRequest.newBuilder(URI.create(base + path))
                    .timeout(Duration.ofSeconds(timeoutSeconds))
                    .header("Content-Type", "application/json")
                    .POST(HttpRequest.BodyPublishers.ofString(JSON.writeValueAsString(body))).build();
            HttpResponse<String> resp = http.send(req, HttpResponse.BodyHandlers.ofString());
            if (resp.statusCode() == 204) {
                return null;
            }
            JsonNode node = resp.body().isBlank() ? JSON.createObjectNode() : JSON.readTree(resp.body());
            if (resp.statusCode() == 409) {
                throw new FencedException(node.path("error").asText("fenced"));
            }
            if (resp.statusCode() >= 300) {
                throw new IllegalStateException(path + " -> " + resp.statusCode() + " " + node.path("error").asText(resp.body()));
            }
            return node;
        } catch (IOException ex) {
            throw new IllegalStateException(path + " unreachable: " + ex.getMessage(), ex);
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
            throw new IllegalStateException(path + " interrupted", ex);
        }
    }

    JsonNode lease(String worker) {
        return post("/exec/lease", Map.of("worker", worker), 30);
    }

    void heartbeat(String run, String token) {
        post("/exec/heartbeat", Map.of("run_id", run, "token", token), 10);
    }

    JsonNode stepGet(String run, String token, String step) {
        return post("/exec/step/get", Map.of("run_id", run, "token", token, "step", step), 10);
    }

    void stepPut(String run, String token, String step, String output) {
        post("/exec/step/put", Map.of("run_id", run, "token", token, "step", step, "output", output), 10);
    }

    String judgment(String run, String token, String step, String raw) {
        return post("/exec/judgment", Map.of("run_id", run, "token", token, "step", step, "raw", raw), 20)
                .path("output").asText();
    }

    String effect(String run, String token, String step) {
        return post("/exec/effect", Map.of("run_id", run, "token", token, "step", step), 90).path("output").asText();
    }

    void branch(String run, String token, String step, String goal, String status, int chars) {
        post("/exec/branch", Map.of("run_id", run, "token", token, "step", step, "goal", goal, "status", status, "chars", chars), 10);
    }

    void done(String run, String token, List<String> skipped, String finalOutput, String failed) {
        post("/exec/done", Map.of("run_id", run, "token", token, "skipped", skipped,
                "final", finalOutput == null ? "" : finalOutput, "failed", failed == null ? "" : failed), 10);
    }
}
