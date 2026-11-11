package com.openjiuwen.demo.credit;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.openjiuwen.agents.orchestration.BlueprintSpec;
import com.openjiuwen.core.foundation.llm.modelclients.DefaultModelClientFactories;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.Executors;

/**
 * Executor for the credit orchestration demo: {@code POST /compile} turns a planner's team into a blueprint
 * (agent-solution TeamBlueprints), and a worker thread leases approved runs from credit-demo and executes them.
 */
public final class Main {
    private static final Logger LOG = LoggerFactory.getLogger(Main.class);
    private static final ObjectMapper JSON = new ObjectMapper();

    private Main() {
    }

    public static void main(String[] args) throws IOException {
        DefaultModelClientFactories.ensureRegistered();
        Map<String, String> env = System.getenv();
        int port = Integer.parseInt(env.getOrDefault("PORT", "8195"));
        GoClient go = new GoClient(env.getOrDefault("CREDIT_URL", "http://credit:8091"));
        HttpServer http = HttpServer.create(new InetSocketAddress(port), 16);
        http.createContext("/compile", Main::compile);
        http.createContext("/healthz", ex -> reply(ex, 200, Map.of("ok", true)));
        http.setExecutor(Executors.newCachedThreadPool());
        http.start();
        Thread worker = new Thread(new Worker(go, env.getOrDefault("WORKER_NAME", "orchestrator-1"), env), "worker");
        worker.setDaemon(false);
        worker.start();
        LOG.info("credit orchestrator on :{}, record at {}", port, env.getOrDefault("CREDIT_URL", "http://credit:8091"));
    }

    private static void compile(HttpExchange ex) throws IOException {
        try {
            JsonNode in = JSON.readTree(ex.getRequestBody());
            Object team = JSON.convertValue(in.path("agents"), List.class);
            BlueprintSpec spec = Blueprints.compile(in.path("mode").asText("graph"), team);
            Map<String, Object> out = new LinkedHashMap<>();
            out.put("blueprint_yaml", Blueprints.yaml(spec));
            out.put("spec_digest", Blueprints.digest(spec));
            out.put("type", spec.type().name().toLowerCase());
            out.put("steps", Blueprints.steps(spec));
            reply(ex, 200, out);
        } catch (Exception err) {
            reply(ex, 200, Map.of("error", String.valueOf(err.getMessage())));
        }
    }

    private static void reply(HttpExchange ex, int code, Object body) throws IOException {
        byte[] raw = JSON.writeValueAsString(body).getBytes(StandardCharsets.UTF_8);
        ex.getResponseHeaders().set("Content-Type", "application/json");
        ex.sendResponseHeaders(code, raw.length);
        ex.getResponseBody().write(raw);
        ex.close();
    }
}
