package com.openjiuwen.demo.report;

import com.fasterxml.jackson.databind.JsonNode;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.UUID;

/** Narrow W05-style A2A adapter. No local expert fallback and no ambiguous resend. */
final class RemoteExpertExecutor {
    private static final Set<String> GOVERNANCE_CODES = Set.of("AUTH_MISSING", "AUTH_INVALID", "AUTH_EXPIRED",
            "AUTH_DISABLED", "CALLER_NOT_AUTHORIZED", "SCOPE_VIOLATION", "TENANT_SCOPE_VIOLATION",
            "IAM_UNAVAILABLE", "RDC_UNAVAILABLE", "AUDIT_UNAVAILABLE");
    private final ControlClient control;
    private final JsonNode bus;
    private final HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(5)).build();
    private String cachedToken;
    private long tokenExpires;

    RemoteExpertExecutor(ControlClient control, JsonNode bus) { this.control = control; this.bus = bus.deepCopy(); }

    JsonNode execute(String taskID, String nodeID, String owner, String epoch, JsonNode permit) {
        String path = "/v1/tasks/" + taskID + "/nodes/" + nodeID;
        JsonNode binding = permit.path("binding");
        if (!binding.isObject()) { throw new ControlClient.ControlFailure("WAIT", 409); }
        // Check before discovery, persisting an intent, or querying an old task.
        // Old records without an environment are readable but cannot dispatch.
        requireBusBinding(binding.path("bus_binding"), bus);
        JsonNode call;
        boolean send = false;
        if (permit.path("status").asText().equals("GRANTED")) {
            // Discovery is outside every Go transaction/lock. Store the actual
            // read snapshot, not a model-supplied service address.
            JsonNode route = discover(binding.path("capability"));
            call = control.post(path + "/calls", Map.of("owner", owner, "epoch", epoch, "route", route));
            String action = call.path("action").asText();
            if (!List.of("SEND", "QUERY", "UNKNOWN").contains(action)) { throw new ControlClient.ControlFailure("REMOTE_UNKNOWN", 409); }
            send = action.equals("SEND");
            if (!send && !action.equals("QUERY")) { throw new ControlClient.ControlFailure("REMOTE_UNKNOWN", 409); }
        } else {
            JsonNode calls = control.get("/v1/tasks/" + taskID + "/calls?node_id=" + nodeID);
            if (!calls.isArray() || calls.isEmpty()) { throw new ControlClient.ControlFailure("WAIT", 409); }
            call = calls.get(calls.size() - 1);
            if (call.path("remote_task_id").asText().isBlank()) {
                throw new ControlClient.ControlFailure("REMOTE_UNKNOWN", 409);
            }
        }
        String remoteID = call.path("remote_task_id").asText();
        if (!send && (remoteID.isBlank() || call.path("conflicted").asBoolean())) { throw new ControlClient.ControlFailure("REMOTE_UNKNOWN", 409); }
        JsonNode remote;
        try {
            if (send) {
                Object envelope = Map.of("schema_version", "node-execution/1", "call_id", call.path("call_id").asText(),
                        "task_id", taskID, "node_id", nodeID, "input_digest", permit.path("input_digest").asText(),
                        "attempt", permit.path("attempt").asInt());
                remote = rpc("SendMessage", Map.of("message", Map.of("role", "ROLE_USER",
                        "messageId", call.path("message_id").asText(), "contextId", call.path("context_id").asText(),
                        "parts", List.of(Map.of("kind", "text", "text", Json.write(envelope)))),
                        "configuration", Map.of("returnImmediately", true),
                        "metadata", Map.of("agentId", call.path("agent_id").asText())),call);
            } else {
                remote = rpc("GetTask", Map.of("id", remoteID),call);
            }
            long deadline = System.nanoTime() + Duration.ofMinutes(4).toNanos();
            while (true) {
                String received = remote.path("id").asText();
                if (received.isBlank() || (!remoteID.isBlank() && !received.equals(remoteID))
                        || !call.path("context_id").asText().equals(remote.path("contextId").asText())) {
                    throw new ControlClient.ControlFailure("REMOTE_ASSOCIATION_CONFLICT", 409);
                }
                remoteID = received;
                String state = normalizeState(remote.path("status").path("state").asText());
                observe(call, state, remoteID, remote);
                if (state.equals("COMPLETED")) {
                    JsonNode envelope = resultEnvelope(remote);
                    if (!envelope.path("input_digest").asText().equals(permit.path("input_digest").asText())
                            || !envelope.path("call_id").asText().equals(call.path("call_id").asText())
                            || envelope.path("attempt").asInt() != call.path("attempt").asInt()) {
                        throw new ControlClient.ControlFailure("REMOTE_ASSOCIATION_CONFLICT", 409);
                    }
                    return envelope.path("output");
                }
                if (state.equals("INPUT_REQUIRED")) { throw new ControlClient.ControlFailure("INPUT_REQUIRED", 409); }
                if (state.equals("AUTH_REQUIRED")) { throw new ControlClient.ControlFailure("AUTH_REQUIRED", 403); }
                if (List.of("FAILED", "REJECTED", "CANCELED").contains(state)) {
                    throw new ControlClient.ControlFailure("REMOTE_" + state, 409);
                }
                if (System.nanoTime() > deadline) { throw new ControlClient.ControlFailure("REMOTE_UNKNOWN", 504); }
                Thread.sleep(1000);
                remote = rpc("GetTask", Map.of("id", remoteID),call);
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new ControlClient.ControlFailure("REMOTE_UNKNOWN", 503);
        }
    }

    static void requireBusBinding(JsonNode expected, JsonNode actual) {
        for (String field : List.of("environment_id", "tenant", "issuer", "audience", "caller_subject")) {
            String current = actual.path(field.equals("caller_subject") ? "subject" : field).asText();
            if (current.isBlank() || !current.equals(expected.path(field).asText())) {
                throw new ControlClient.ControlFailure("BUS_BINDING_CHANGED", 409);
            }
        }
        if (!actual.path("token_url").asText().equals(actual.path("issuer").asText() + "/protocol/openid-connect/token")) {
            throw new ControlClient.ControlFailure("BUS_BINDING_CHANGED", 409);
        }
    }

    private void observe(JsonNode call, String state, String remoteID, JsonNode remote) {
        control.post("/v1/calls/" + call.path("call_id").asText() + "/observations",
                Map.of("event_id", UUID.randomUUID().toString(), "state", state, "remote_task_id", remoteID,
                        "context_id", call.path("context_id").asText(), "payload", remote));
    }

    private JsonNode discover(JsonNode capability) {
        String agentID = capability.path("agent_id").asText();
        if (bus.path("gateway_url").asText().isBlank() || bus.path("rdc_url").asText().isBlank()
                || bus.path("registry_token").asText().isBlank()) {
            throw new ControlClient.ControlFailure("AUTH_REQUIRED", 503);
        }
        JsonNode card = getRdc("/v2/agents/" + agentID + "/card");
        JsonNode instances = getRdc("/v2/agents/" + agentID + "/instances");
        validateDiscovery(capability,card,instances);
        return Json.MAPPER.valueToTree(Map.of("agent_id", agentID, "card", card, "instances", instances,
                "gateway_mode", "DIRECT", "observed_at", java.time.Instant.now().toString(),
                "actual_execution_instance", "not_collected"));
    }

    static void validateDiscovery(JsonNode capability,JsonNode card,JsonNode instances) {
        String agentID = capability.path("agent_id").asText();
        // Compatibility only for catalogs adopted before explicit remote bindings.
        JsonNode expected = capability.path("remote_binding");
        if (expected.isMissingNode()) { expected = Json.read("{\"card_version\":\"1.0.0\",\"contract_version\":\"1.0.0\",\"capability_version\":\"1.0.0\"}"); }
        for (String field : List.of("card_version", "contract_version", "capability_version")) {
            if (expected.path(field).asText().isBlank()) { throw new ControlClient.ControlFailure("BINDING_CHANGED",409); }
        }
        if (!card.path("name").asText().equals(agentID)) {throw new ControlClient.ControlFailure("DISCOVERY_BINDING_CHANGED",409);}
        JsonNode raw;
        try {raw=Json.read(card.path("rawJson").asText());}catch(Exception e){throw new ControlClient.ControlFailure("DISCOVERY_BINDING_CHANGED",409);}
        if (!raw.path("name").asText().equals(agentID)||!raw.path("version").asText().equals(expected.path("card_version").asText())) {throw new ControlClient.ControlFailure("DISCOVERY_BINDING_CHANGED",409);}
        if (!expected.path("card_digest").asText().isBlank() && !expected.path("card_digest").asText().equals(rawDigest(card.path("rawJson").asText()))) { throw new ControlClient.ControlFailure("DISCOVERY_BINDING_CHANGED",409); }
        boolean healthy=false;
        for(JsonNode candidate:instances.path("instances")) {
            if(candidate.path("health").asText().equals("ONLINE")&&candidate.path("contractVersion").asText().equals(expected.path("contract_version").asText())
                    &&candidate.path("capabilityVersion").asText().equals(expected.path("capability_version").asText())&&candidate.path("card").path("name").asText().equals(agentID)) {healthy=true;}
        }
        if(!healthy){throw new ControlClient.ControlFailure("DISCOVERY_UNAVAILABLE",503);}
    }

    static String rawDigest(String raw) {
        try { return "sha256:" + java.util.HexFormat.of().formatHex(java.security.MessageDigest.getInstance("SHA-256").digest(raw.getBytes(java.nio.charset.StandardCharsets.UTF_8))); }
        catch (java.security.NoSuchAlgorithmException e) { throw new IllegalStateException(e); }
    }

    private JsonNode getRdc(String path) {
        try {
            var request = HttpRequest.newBuilder(URI.create(bus.path("rdc_url").asText() + path))
                    .timeout(Duration.ofSeconds(5)).header("Authorization", "Bearer " + bus.path("registry_token").asText())
                    .header("X-Tenant-ID", bus.path("tenant").asText("demo")).GET().build();
            var response = http.send(request, HttpResponse.BodyHandlers.ofString());
            if (response.statusCode() != 200) { throw new ControlClient.ControlFailure("DISCOVERY_UNAVAILABLE", response.statusCode()); }
            return Json.read(response.body());
        } catch (ControlClient.ControlFailure e) { throw e; }
        catch (Exception e) { throw new ControlClient.ControlFailure("DISCOVERY_UNAVAILABLE", 503); }
    }

    static JsonNode taskResult(JsonNode response) {
        JsonNode result=response.path("result");
        JsonNode task=result.has("task")?result.path("task"):result;
        if(response.has("error")||!task.isObject()||!task.has("id")||!task.has("status")) {throw new ControlClient.ControlFailure("REMOTE_UNKNOWN",409);}
        return task;
    }

    private JsonNode rpc(String method, Object params,JsonNode call) {
        try {
            var request = HttpRequest.newBuilder(URI.create(bus.path("gateway_url").asText() + "/a2a"))
                    .timeout(Duration.ofSeconds(30)).header("Authorization", "Bearer " + bearer())
                    .header("X-User-ID", bus.path("subject").asText())
                    .header("Content-Type", "application/json")
                    .POST(HttpRequest.BodyPublishers.ofString(Json.write(Map.of("jsonrpc", "2.0", "id", UUID.randomUUID().toString(),
                            "method", method, "params", params)))).build();
            var response = http.send(request, HttpResponse.BodyHandlers.ofString());
            JsonNode value;
            try {value=Json.read(response.body());}catch(Exception e){value=Json.MAPPER.valueToTree(Map.of("unparseable_body",true));}
            String remoteID="";
            try {JsonNode snapshot=taskResult(value);if(snapshot.path("contextId").asText().equals(call.path("context_id").asText())){remoteID=snapshot.path("id").asText();}}catch(ControlClient.ControlFailure ignored) { }
            // A receipt is not a business state transition. Save a validated
            // task association before later polling/parsing can fail; no headers.
            boolean failed = response.statusCode() != 200 || value.has("error");
            Object safeResponse = failed ? Map.of("code", remoteErrorCode(response.statusCode(), value)) : value;
            control.post("/v1/calls/"+call.path("call_id").asText()+"/observations",Map.of("event_id",UUID.randomUUID().toString(),"state","TRANSPORT",
                    "remote_task_id",remoteID,"context_id",call.path("context_id").asText(),
                    "payload",Map.of("method",method,"http_status",response.statusCode(),"response",safeResponse)));
            if (failed) {
                throw new ControlClient.ControlFailure(remoteErrorCode(response.statusCode(), value), response.statusCode());
            }
            return taskResult(value);
        } catch (ControlClient.ControlFailure e) { throw e; }
        catch (Exception e) { throw new ControlClient.ControlFailure("REMOTE_UNKNOWN", 503); }
    }

    /** Preserve only known machine codes; never expose remote free text or turn a failure into success. */
    static String remoteErrorCode(int status, JsonNode response) {
        String code = response.path("code").asText();
        if (code.isBlank()) { code = response.path("error").path("code").asText(); }
        if (code.isBlank() || !GOVERNANCE_CODES.contains(code)) {
            code = response.path("error").path("data").path("code").asText();
        }
        if (GOVERNANCE_CODES.contains(code)) { return code; }
        return status == 401 || status == 403 ? "AUTH_REQUIRED" : "REMOTE_UNKNOWN";
    }

    static String normalizeState(String state) {
        String value = state.replace("TASK_STATE_", "").replace('-', '_').toUpperCase(java.util.Locale.ROOT);
        return switch (value) {
            case "SUBMITTED" -> "ACCEPTED";
            case "WORKING", "COMPLETED", "FAILED", "REJECTED", "CANCELED", "INPUT_REQUIRED", "AUTH_REQUIRED" -> value;
            default -> throw new ControlClient.ControlFailure("REMOTE_UNKNOWN", 409);
        };
    }

    private synchronized String bearer() {
        if (cachedToken != null && System.currentTimeMillis() < tokenExpires - 30000) { return cachedToken; }
        try {
            String body = "grant_type=client_credentials&client_id=" + java.net.URLEncoder.encode(bus.path("client_id").asText(), java.nio.charset.StandardCharsets.UTF_8)
                    + "&client_secret=" + java.net.URLEncoder.encode(bus.path("client_secret").asText(), java.nio.charset.StandardCharsets.UTF_8);
            var request = HttpRequest.newBuilder(URI.create(bus.path("token_url").asText())).timeout(Duration.ofSeconds(5))
                    .header("Content-Type", "application/x-www-form-urlencoded").POST(HttpRequest.BodyPublishers.ofString(body)).build();
            var response = http.send(request, HttpResponse.BodyHandlers.ofString());
            if (response.statusCode() != 200) { throw new IllegalStateException("Token issuance failed"); }
            JsonNode issued = Json.read(response.body()); cachedToken = issued.path("access_token").asText();
            if (cachedToken.isBlank()) { throw new IllegalStateException("Token missing"); }
            tokenExpires = System.currentTimeMillis() + issued.path("expires_in").asLong(60) * 1000;
            return cachedToken;
        } catch (Exception e) { throw new ControlClient.ControlFailure("AUTH_REQUIRED", 503); }
    }

    static JsonNode resultEnvelope(JsonNode remote) {
        List<JsonNode> candidates = new ArrayList<>();
        for (JsonNode artifact : remote.path("artifacts")) {
            for (JsonNode part : artifact.path("parts")) {
                if (part.has("data")) {
                    JsonNode data = part.path("data");
                    if (data.isObject()) { candidates.add(data); }
                } else if (part.has("text")) {
                    // Text is parsed independently; unrelated parts are never concatenated.
                    try { candidates.add(Json.read(part.path("text").asText())); }
                    catch (IllegalArgumentException ignored) { /* non-contract text is not a result */ }
                }
            }
        }
        List<JsonNode> matches = candidates.stream().filter(c -> c.has("call_id") && c.has("input_digest") && c.has("output")).toList();
        if (matches.size() != 1) { throw new ControlClient.ControlFailure("RESULT_SCHEMA_MISMATCH", 422); }
        return matches.getFirst();
    }
}
