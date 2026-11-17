package com.openjiuwen.demo.report;

import com.fasterxml.jackson.databind.JsonNode;
import com.openjiuwen.agents.reactrails.loopforest.execution.SubAgentExecutor;
import java.util.Map;
import java.util.function.BooleanSupplier;

/** One immutable context per step; the scheduler's worker threads share no ThreadLocal identity. */
final class BoundNodeExecutor implements SubAgentExecutor {
    private final ControlClient control;
    private final String taskID;
    private final JsonNode node;
    private final String owner;
    private final String epoch;
    private final BooleanSupplier permitAlive;
    private final RemoteExpertExecutor remote;
    private final ReportChecker checker;

    BoundNodeExecutor(ControlClient control, String taskID, JsonNode node, String owner, String epoch,
            BooleanSupplier permitAlive, RemoteExpertExecutor remote, ReportChecker checker) {
        this.control = control; this.taskID = taskID; this.node = node; this.owner = owner;
        this.epoch = epoch; this.permitAlive = permitAlive; this.remote = remote; this.checker = checker;
    }

    @Override public String execute(String ignoredUserText, String ignoredJoinedText) {
        String nodeID = node.path("id").asText();
        String path = "/v1/tasks/" + taskID + "/nodes/" + nodeID;
        if (!permitAlive.getAsBoolean()) { throw new ControlClient.ControlFailure("LEASE_EXPIRED", 403); }
        JsonNode state = control.get("/v1/tasks/" + taskID);
        if (node.hasNonNull("condition")) {
            JsonNode condition = node.path("condition");
            JsonNode checkState = state.path("nodes").path(condition.path("step").asText());
            String outcome = ReportChecker.effectiveOutcome(checkState.path("result"),checkState.path("review"));
            if (!outcome.equals(condition.path("outcome").asText())) {
                control.post(path + "/skip", Map.of("owner", owner, "epoch", epoch));
                return "AUTHORIZED_SKIP";
            }
        }
        try {
            JsonNode permit = control.post(path + "/resolve", Map.of("owner", owner, "epoch", epoch));
            if (permit.path("status").asText().equals("REUSE")) { return Json.write(permit.path("result").path("output")); }
            String kind = permit.path("binding").path("capability").path("kind").asText();
            JsonNode output;
            if (kind.equals("check")) {
                if (!permit.path("status").asText().equals("GRANTED")) { throw new ControlClient.ControlFailure("WAIT", 409); }
                output = checker.check(permit.path("binding"));
            } else if (kind.equals("select")) {
                if (!permit.path("status").asText().equals("GRANTED")) { throw new ControlClient.ControlFailure("WAIT", 409); }
                output = checker.select(permit.path("binding"));
            } else {
                output = remote.execute(taskID, nodeID, owner, epoch, permit);
            }
            JsonNode artifact = control.post("/v1/artifacts", output);
            JsonNode result = control.post(path + "/commit", Map.of("owner", owner, "epoch", epoch,
                    "input_digest", permit.path("input_digest").asText(), "attempt", permit.path("attempt").asInt(),
                    "artifact_id", artifact.path("artifact_id").asText()));
            return Json.write(result.path("output"));
        } catch (ControlClient.ControlFailure e) {
            String status = waitingStatus(e.code);
            // A waiting update is best effort; a lost permit cannot be used to
            // invent a successful result. Trusted observations have their own API.
            try { control.post(path + "/waiting", Map.of("owner", owner, "epoch", epoch, "status", status, "reason", e.code)); }
            catch (ControlClient.ControlFailure ignored) { }
            throw e;
        }
    }

    static String waitingStatus(String code) {
        return switch (code) {
                case "INPUT_REQUIRED" -> "WAITING_INPUT";
                case "AUTH_REQUIRED", "AUTH_MISSING", "AUTH_INVALID", "AUTH_EXPIRED" -> "WAITING_AUTH";
                case "BINDING_CHANGED", "BUS_BINDING_CHANGED", "DISCOVERY_BINDING_CHANGED", "MODEL_BINDING_CHANGED", "AUTH_DISABLED",
                        "CALLER_NOT_AUTHORIZED", "SCOPE_VIOLATION", "TENANT_SCOPE_VIOLATION" -> "BLOCKED";
                case "REMOTE_UNKNOWN", "REMOTE_UNRESOLVED", "REMOTE_ASSOCIATION_CONFLICT", "WAIT", "DISCOVERY_UNAVAILABLE", "CONTROL_UNAVAILABLE",
                        "IAM_UNAVAILABLE", "RDC_UNAVAILABLE", "AUDIT_UNAVAILABLE" -> "UNKNOWN";
                default -> "FAILED";
            };
    }
}
