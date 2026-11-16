package com.openjiuwen.demo.report;

import com.fasterxml.jackson.databind.JsonNode;
import com.openjiuwen.agents.orchestration.BlueprintScheduler;
import com.openjiuwen.agents.reactrails.loopforest.execution.SubAgentExecutor;
import com.openjiuwen.agents.reactrails.loopforest.observability.TraceForest;
import java.time.Duration;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicReference;

final class DemoWorker implements Runnable {
    private final ControlClient control;
    private final JsonNode config;
    private final String owner = "java-" + UUID.randomUUID();

    DemoWorker(JsonNode config) {
        this.config = config;
        control = new ControlClient(config.path("public_url").asText(), config.path("tokens").path("worker").asText());
    }

    @Override public void run() {
        while (!Thread.currentThread().isInterrupted()) {
            try {
                for (JsonNode id : control.get("/v1/executions/pending")) {
                    try { execute(control.post("/v1/tasks/" + id.asText() + "/claim", Map.of("owner", owner, "domain", "graph"))); }
                    catch (ControlClient.ControlFailure e) { System.err.println("worker task remains pending: " + e.code); }
                }
                Thread.sleep(1500);
            } catch (InterruptedException e) { Thread.currentThread().interrupt(); }
            catch (Exception e) {
                System.err.println("worker waits for control: " + e.getClass().getSimpleName());
                try { Thread.sleep(2000); } catch (InterruptedException interrupted) { Thread.currentThread().interrupt(); }
            }
        }
    }

    private void execute(JsonNode lease) {
        JsonNode task = lease.path("task");
        String id = task.path("task_id").asText();
        String epoch = lease.path("epoch").asText();
        var compiled = new AdmittedGraphCompiler().compile(task.path("graph"));
        if (!compiled.digest().equals(task.path("runtime_spec_digest").asText())) {
            throw new ControlClient.ControlFailure("BINDING_CHANGED", 409);
        }
        AtomicBoolean fenced = new AtomicBoolean();
        AtomicReference<Instant> expires = new AtomicReference<>(Instant.parse(lease.path("expires_at").asText()));
        var heartbeats = Executors.newSingleThreadScheduledExecutor();
        heartbeats.scheduleAtFixedRate(() -> {
            try {
                JsonNode renewed = control.post("/v1/tasks/" + id + "/heartbeat", Map.of("owner", owner, "epoch", epoch));
                expires.set(Instant.parse(renewed.path("expires_at").asText()));
            } catch (ControlClient.ControlFailure e) {
                if (e.code.equals("LEASE_EXPIRED") || e.code.equals("FENCED")) { fenced.set(true); }
            }
        }, 3, 3, TimeUnit.SECONDS);
        try {
            var remote = new RemoteExpertExecutor(control, config.path("bus"));
            var checker = new ReportChecker(control);
            Map<String, SubAgentExecutor> bindings = new LinkedHashMap<>();
            for (JsonNode node : task.path("graph").path("nodes")) {
                bindings.put(node.path("id").asText(), new BoundNodeExecutor(control, id, node, owner, epoch,
                        () -> !fenced.get() && Instant.now().isBefore(expires.get().minusSeconds(1)), remote, checker));
            }
            // This is the sole scheduler. Go decides admission/commit, never a ready queue.
            RuntimeException executionFailure = null;
            try (var scheduler = new BlueprintScheduler(new TraceForest(), 3, null, null, Duration.ofMinutes(5))) {
                try { scheduler.execute(compiled.spec(), bindings, task.path("graph").path("goal").asText()); }
                catch (RuntimeException failure) { executionFailure = failure; }
                finally {
                    if (!scheduler.awaitMembersSettled(Duration.ofSeconds(10))) {
                        throw new ControlClient.ControlFailure("IN_FLIGHT_UNKNOWN", 409);
                    }
                }
            }
            JsonNode completion = control.post("/v1/tasks/" + id + "/complete", Map.of("owner", owner, "epoch", epoch));
            System.out.println("task " + id + " completion=" + completion.path("status").asText());
            if (executionFailure != null) {
                System.err.println("scheduler settled with " + executionFailure.getClass().getSimpleName()
                        + "; authoritative completion=" + completion.path("status").asText());
            }
        } finally { heartbeats.shutdownNow(); }
    }
}
