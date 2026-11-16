package com.openjiuwen.demo.report;

import com.fasterxml.jackson.databind.JsonNode;
import com.sun.net.httpserver.HttpServer;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.util.Map;
import java.util.concurrent.Executors;

/** Compiler starts independently; the worker (when enabled) backs off waiting for Go. */
public final class Main {
    private Main() { }

    public static void main(String[] args) throws Exception {
        String configPath = System.getenv("REPORT_DEMO_CONFIG");
        if (args.length > 0) { configPath = args[0]; }
        if (configPath == null) { throw new IllegalArgumentException("Private config path required"); }
        JsonNode config = Json.read(Files.readString(Path.of(configPath)));
        String token = config.path("tokens").path("worker").asText();
        if (token.length() < 24) { throw new IllegalArgumentException("Worker identity missing"); }
        HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1", 8195), 0);
        server.setExecutor(Executors.newVirtualThreadPerTaskExecutor());
        AdmittedGraphCompiler compiler = new AdmittedGraphCompiler();
        server.createContext("/healthz", exchange -> {
            byte[] body = Json.write(Map.of("status", "ok", "compiler_version", "report-compiler/1")).getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().set("Content-Type", "application/json");
            exchange.sendResponseHeaders(200, body.length); exchange.getResponseBody().write(body); exchange.close();
        });
        server.createContext("/compile", exchange -> {
            int status = 200;
            Object result;
            try {
                String authorization = exchange.getRequestHeaders().getFirst("Authorization");
                if (!MessageDigest.isEqual(("Bearer " + token).getBytes(StandardCharsets.UTF_8),
                        String.valueOf(authorization).getBytes(StandardCharsets.UTF_8))) {
                    status = 401; result = Map.of("error", "UNAUTHORIZED");
                } else if (!exchange.getRequestMethod().equals("POST")) {
                    status = 405; result = Map.of("error", "METHOD_NOT_ALLOWED");
                } else {
                    byte[] raw = exchange.getRequestBody().readNBytes(2 * 1024 * 1024 + 1);
                    if (raw.length > 2 * 1024 * 1024) { throw new IllegalArgumentException("Graph too large"); }
                    var compiled = compiler.compile(Json.read(new String(raw, StandardCharsets.UTF_8)));
                    result = Map.of("runtime_spec_digest", compiled.digest(), "runtime_spec", compiled.projection());
                }
            } catch (Exception e) { status = 422; result = Map.of("error", "INVALID_GRAPH"); }
            byte[] body = Json.write(result).getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().set("Content-Type", "application/json");
            exchange.sendResponseHeaders(status, body.length); exchange.getResponseBody().write(body); exchange.close();
        });
        Runtime.getRuntime().addShutdownHook(new Thread(() -> server.stop(1)));
        server.start();
        System.out.println("report compiler listening at 127.0.0.1:8195");
        if (config.path("worker_enabled").asBoolean(false)) {
            Thread worker = new Thread(new DemoWorker(config), "report-worker");
            worker.setDaemon(true);
            Runtime.getRuntime().addShutdownHook(new Thread(worker::interrupt));
            worker.start();
        }
    }
}
