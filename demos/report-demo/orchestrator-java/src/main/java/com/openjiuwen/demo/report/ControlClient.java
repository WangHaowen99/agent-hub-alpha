package com.openjiuwen.demo.report;

import com.fasterxml.jackson.databind.JsonNode;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

final class ControlClient {
    static final class ControlFailure extends RuntimeException {
        final String code;
        final int status;
        ControlFailure(String code, int status) { super(code + " HTTP " + status); this.code = code; this.status = status; }
    }

    private final String base;
    private final String token;
    private final HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(3)).build();

    ControlClient(String base, String token) { this.base = base; this.token = token; }

    JsonNode get(String path) { return request("GET", path, null); }
    JsonNode post(String path, Object body) { return request("POST", path, body); }

    private JsonNode request(String method, String path, Object body) {
        try {
            HttpRequest.Builder builder = HttpRequest.newBuilder(URI.create(base + path))
                    .timeout(Duration.ofSeconds(10)).header("Authorization", "Bearer " + token)
                    .header("Content-Type", "application/json");
            builder.method(method, body == null ? HttpRequest.BodyPublishers.noBody()
                    : HttpRequest.BodyPublishers.ofString(Json.write(body)));
            HttpResponse<String> response = http.send(builder.build(), HttpResponse.BodyHandlers.ofString());
            JsonNode value = Json.read(response.body());
            if (response.statusCode() >= 300) {
                throw new ControlFailure(value.path("error").path("code").asText("CONTROL_UNAVAILABLE"), response.statusCode());
            }
            return value;
        } catch (ControlFailure e) { throw e; }
        catch (Exception e) {
            if (e instanceof InterruptedException) { Thread.currentThread().interrupt(); }
            throw new ControlFailure("CONTROL_UNAVAILABLE", 503);
        }
    }
}
