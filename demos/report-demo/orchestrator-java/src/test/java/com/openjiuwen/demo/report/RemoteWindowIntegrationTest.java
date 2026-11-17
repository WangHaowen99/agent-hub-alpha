package com.openjiuwen.demo.report;

import static org.junit.jupiter.api.Assertions.*;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sun.net.httpserver.HttpServer;
import com.openjiuwen.agents.orchestration.BlueprintScheduler;
import com.openjiuwen.agents.orchestration.BlueprintException;
import com.openjiuwen.agents.orchestration.BlueprintSpec;
import com.openjiuwen.agents.orchestration.StepSpec;
import com.openjiuwen.agents.orchestration.SkipPolicy;
import com.openjiuwen.agents.reactrails.loopforest.execution.SubAgentExecutor;
import com.openjiuwen.agents.reactrails.loopforest.observability.TraceForest;
import java.net.CookieManager;
import java.net.InetSocketAddress;
import java.net.ServerSocket;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.LinkedHashMap;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Assumptions;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/** Real Go process + real PG + actual Java adapter; isolated fake HTTP provider.
 * No model or real Gateway/Runtime identity claim is made by these fault tests.
 */
class RemoteWindowIntegrationTest {
    @TempDir Path temp;

    @Test void changedBusEnvironmentBlocksBothNewSendAndRecoveryQuery() throws Exception {
        for (boolean recovery : List.of(false, true)) {
            try (var h = new Harness(temp.resolve("bus-change-" + recovery))) {
                if (recovery) {
                    assertEquals("REMOTE_FAILED", assertThrows(ControlClient.ControlFailure.class, h::execute).code);
                    assertEquals(1, h.sends.get());
                }
                JsonNode frozen = h.task().path("bus_binding").deepCopy();
                var originalCalls = h.calls();
                int sends = h.sends.get(), gets = h.gets.get(), requests = h.providerRequests.get();
                JsonNode permit = h.control.post(h.path + "/nodes/" + h.node + "/resolve", Map.of("owner", h.owner, "epoch", h.epoch));
                for (String field : List.of("tenant", "environment_id", "issuer", "audience", "subject", "token_url")) {
                    ObjectNode changed = (ObjectNode) h.config.path("bus").deepCopy();
                    changed.put(field, "changed-fixture-domain");
                    var remote = new RemoteExpertExecutor(h.control, changed);
                    assertEquals("BUS_BINDING_CHANGED", assertThrows(ControlClient.ControlFailure.class,
                        () -> remote.execute(h.task().path("task_id").asText(), h.node, h.owner, h.epoch, permit)).code);
                }
                assertEquals(requests, h.providerRequests.get(), "environment mismatch made a discovery/token/Gateway request");
                assertEquals(sends, h.sends.get()); assertEquals(gets, h.gets.get());
                assertEquals(originalCalls, h.calls()); assertEquals(frozen, h.task().path("bus_binding"));
                h.saveEvidence("bus-change-" + recovery, Map.of("recovery", recovery, "old_binding_preserved", true,
                    "new_outbound_requests", 0, "same_call_identity", true, "result", "BUS_BINDING_CHANGED"));
            }
        }
    }

    @Test void sameBusEnvironmentCanQueryThroughNewProxyWithoutResend() throws Exception {
        try (var h = new Harness(temp.resolve("bus-proxy"))) {
            assertEquals("REMOTE_FAILED", assertThrows(ControlClient.ControlFailure.class, h::execute).code);
            var original = h.calls().getFirst();
            String originalGateway = h.config.path("bus").path("gateway_url").asText();
            AtomicInteger proxyQueries = new AtomicInteger();
            var proxy = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            proxy.createContext("/a2a", exchange -> {
                try {
                    JsonNode request = Json.read(new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8));
                    if (!request.path("method").asText().equals("GetTask")) { exchange.sendResponseHeaders(400, -1); return; }
                    proxyQueries.incrementAndGet();
                    var target = HttpRequest.newBuilder(URI.create(originalGateway + "/a2a")).timeout(Duration.ofSeconds(5))
                        .header("Content-Type", "application/json").POST(HttpRequest.BodyPublishers.ofString(Json.write(request))).build();
                    var reply = HttpClient.newHttpClient().send(target, HttpResponse.BodyHandlers.ofByteArray());
                    exchange.sendResponseHeaders(reply.statusCode(), reply.body().length); exchange.getResponseBody().write(reply.body());
                } catch (InterruptedException failure) { Thread.currentThread().interrupt(); }
                finally { exchange.close(); }
            });
            proxy.start();
            try {
                ((ObjectNode) h.config.path("bus")).put("gateway_url", "http://127.0.0.1:" + proxy.getAddress().getPort());
                assertEquals("REMOTE_FAILED", assertThrows(ControlClient.ControlFailure.class, h::execute).code);
                assertEquals(1, proxyQueries.get()); assertEquals(1, h.sends.get()); assertEquals(1, h.gets.get());
                assertEquals(original, h.calls().getFirst());
                h.saveEvidence("bus-proxy-recovery", Map.of("proxy_queries", 1, "total_sends", 1, "same_call_identity", true));
            } finally { proxy.stop(0); }
        }
    }

    @Test void intentBeforeSendSurvivesControlProcessKillWithoutResend() throws Exception {
        try (var h = new Harness(temp.resolve("intent-before-send-restart"))) {
            var permit=h.control.post(h.path+"/nodes/"+h.node+"/resolve",Map.of("owner",h.owner,"epoch",h.epoch));
            var capability=permit.path("binding").path("capability");
            String agent=capability.path("agent_id").asText();
            var binding=capability.path("remote_binding");
            var route=Map.of("agent_id",agent,"card",Map.of("name",agent,"rawJson",Json.write(Map.of("name",agent,"version",binding.path("card_version").asText()))),
                "instances",Map.of("instances",List.of(Map.of("health","ONLINE","contractVersion",binding.path("contract_version").asText(),
                    "capabilityVersion",binding.path("capability_version").asText(),"card",Map.of("name",agent)))));
            assertEquals("SEND",h.control.post(h.path+"/nodes/"+h.node+"/calls",Map.of("owner",h.owner,"epoch",h.epoch,"route",route)).path("action").asText());
            var original=h.calls().getFirst();
            h.restartControl();
            h.epoch=h.control.post(h.path+"/claim",Map.of("owner",h.owner,"domain","graph")).path("epoch").asText();
            assertEquals("REMOTE_UNKNOWN",assertThrows(ControlClient.ControlFailure.class,h::execute).code);
            assertEquals(original,h.calls().getFirst());assertEquals(0,h.sends.get());assertEquals(0,h.gets.get());
            h.saveEvidence("intent-before-send-process-kill",Map.of("intent_persisted_before_kill",true,"resume","REMOTE_UNKNOWN","send_requests",0,"query_requests",0,"same_call_identity",true));
        }
    }

    @Test void changedProfileAfterProcessRestartDoesNotReplaceOldBinding() throws Exception {
        try (var h = new Harness(temp.resolve("profile-restart"))) {
            var old = h.task().path("execution_profile").asText();
            h.config.put("execution_profile", "sha256:changed-fixture-release");
            Files.writeString(h.configFile, Json.write(h.config));
            h.restartControl();
            h.epoch = h.control.post(h.path + "/claim", Map.of("owner", h.owner, "domain", "graph")).path("epoch").asText();
            assertEquals("BINDING_CHANGED", assertThrows(ControlClient.ControlFailure.class, h::execute).code);
            assertEquals(old, h.task().path("execution_profile").asText());
            assertEquals(0, h.sends.get()); assertEquals(0, h.calls().size());
            h.saveEvidence("changed-profile-process-restart", Map.of("old_profile_preserved",true,"resume","BINDING_CHANGED","send_requests",0));
        }
    }

    @Test void repeatedGrantedPermitDoesNotResendExistingIntent() throws Exception {
        for (boolean loseReply : List.of(false, true)) {
            try (var h = new Harness(temp.resolve("repeat-permit-" + loseReply))) {
                h.dropSendReply = loseReply;
                var permit = h.control.post(h.path + "/nodes/" + h.node + "/resolve", Map.of("owner", h.owner, "epoch", h.epoch));
                var remote = new RemoteExpertExecutor(h.control, h.config.path("bus"));
                var id = h.task().path("task_id").asText();
                assertThrows(ControlClient.ControlFailure.class, () -> remote.execute(id, h.node, h.owner, h.epoch, permit));
                var before = h.calls().getFirst();
                assertEquals(loseReply ? "REMOTE_UNKNOWN" : "REMOTE_FAILED", assertThrows(ControlClient.ControlFailure.class,
                    () -> remote.execute(id, h.node, h.owner, h.epoch, permit)).code);
                assertEquals(1, h.sends.get());
                assertEquals(loseReply ? 0 : 1, h.gets.get());
                assertEquals(before, h.calls().getFirst());
                h.saveEvidence("repeated-granted-permit-" + loseReply, Map.of("send_requests", h.sends.get(), "query_requests", h.gets.get(), "same_call", true));
            }
        }
    }

    @Test void restartDuringDiscoveryFencesOldWorkerWithoutSending() throws Exception {
        try (var h = new Harness(temp.resolve("discovery-restart"))) {
            h.blockDiscoveryAgent = "report-extractor";
            var old = CompletableFuture.supplyAsync(() -> {
                try { h.execute(); return "UNEXPECTED_SUCCESS"; }
                catch (ControlClient.ControlFailure failure) { return failure.code; }
            });
            try {
                assertTrue(h.discoveryEntered.await(3, TimeUnit.SECONDS));
                h.restartControl();
                var lease = h.control.post(h.path + "/claim", Map.of("owner", "replacement-worker", "domain", "graph"));
                assertNotEquals(h.epoch, lease.path("epoch").asText());
                h.discoveryReleased.countDown();
                assertEquals("LEASE_EXPIRED", old.get(8, TimeUnit.SECONDS));
                assertEquals(0, h.sends.get()); assertEquals(0, h.calls().size());
                h.saveEvidence("go-restart-during-discovery", Map.of("old_worker", "LEASE_EXPIRED", "send_requests", 0, "intents", 0, "new_epoch", lease.path("epoch").asText()));
            } finally { h.discoveryReleased.countDown(); }
        }
    }

    @Test void realWorkerStillRequestsGoCompletionAfterNativeSchedulerFailure() throws Exception {
        try(var h=new Harness(temp.resolve("worker-failure-completion"),false)) {
            Thread worker=new Thread(new DemoWorker(h.config),"isolated-fixture-worker");
            worker.setDaemon(true);worker.start();
            boolean checked=false;
            try {
                long deadline=System.nanoTime()+Duration.ofSeconds(20).toNanos();
                while(System.nanoTime()<deadline) {
                    for(var event:h.control.get(h.path+"/events")) {
                        if(event.path("kind").asText().equals("COMPLETION_CHECKED")&&event.path("document").path("status").asText().equals("FAILED")) {checked=true;break;}
                    }
                    if(checked)break;
                    Thread.sleep(100);
                }
                assertTrue(checked,"worker skipped the authoritative Go completion request after scheduler failure");
                assertEquals("FAILED",h.task().path("status").asText());
                assertEquals(1,h.sends.get());
                assertFalse(h.control.get("/v1/executions/pending").toString().contains(h.task().path("task_id").asText()));
                h.saveEvidence("worker-settled-failure-completion",Map.of("go_completion_checked",true,"task_status","FAILED","send_requests",h.sends.get(),"automatic_polling_stopped",true));
            } finally {worker.interrupt();worker.join(5000);assertFalse(worker.isAlive());}
        }
    }

    @Test void slowDiscoveryDoesNotHoldGoTransactionAndPauseRejectsStaleDispatch() throws Exception {
        try(var h=new Harness(temp.resolve("slow-discovery"))) {
            h.commitFixtureNode(h.node);
            h.blockDiscoveryAgent="business-analyst";
            var executing=CompletableFuture.supplyAsync(()->{
                try {h.execute(h.nodeOf("business@1"));return "UNEXPECTED_SUCCESS";}
                catch(ControlClient.ControlFailure failure){return failure.code;}
            });
            try {
                assertTrue(h.discoveryEntered.await(3,TimeUnit.SECONDS),"RDC barrier was not exercised");
                // These transactions touch the same Go task while the Java
                // discovery request remains blocked at an explicit barrier.
                var heartbeat=CompletableFuture.supplyAsync(()->h.control.post(h.path+"/heartbeat",Map.of("owner",h.owner,"epoch",h.epoch)));
                assertEquals(h.epoch,heartbeat.get(3,TimeUnit.SECONDS).path("epoch").asText());
                var otherCommit=CompletableFuture.runAsync(()->h.commitFixtureNode(h.nodeOf("quality@1")));
                otherCommit.get(3,TimeUnit.SECONDS);
                assertTrue(h.task().path("nodes").path(h.nodeOf("quality@1")).hasNonNull("result"));
                h.userPost(h.path+"/pause",Map.of());
                h.discoveryReleased.countDown();
                assertEquals("PAUSED",executing.get(5,TimeUnit.SECONDS));
                assertEquals(0,h.sends.get(),"stale discovery snapshot dispatched after current Go pause");
                assertEquals(0,h.calls().size());
                h.saveEvidence("slow-rdc-pause-barrier",Map.of("same_task_heartbeat_completed_while_rdc_blocked",true,"independent_quality_commit_completed_while_rdc_blocked",true,"stale_snapshot_dispatch_requests",0,"barriers_exercised",true));
            } finally {h.discoveryReleased.countDown();}
        }
    }

    @Test void nativeSchedulerContinuingAfterFailureCannotDispatchDownstream() throws Exception {
        try(var h=new Harness(temp.resolve("predecessor-failure"))) {
            var graph=h.task().path("graph");
            var compiled=new AdmittedGraphCompiler().compile(graph);
            assertEquals(h.task().path("runtime_spec_digest").asText(),compiled.digest());
            // Current TeamBlueprints normally skips when all dependencies are
            // inconclusive. Inject the adverse scheduler policy explicitly;
            // domain admission must still hold if those wrappers are invoked.
            // This is a fault fixture, not an admitted production spec change.
            var adverse=new BlueprintSpec(compiled.spec().id(),compiled.spec().type(),compiled.spec().members(),
                compiled.spec().steps().stream().map(step->new StepSpec(step.id(),step.member(),step.dependsOn(),SkipPolicy.NEVER,step.when())).toList());
            Map<String,SubAgentExecutor> bindings=new LinkedHashMap<>();
            var started=ConcurrentHashMap.<String>newKeySet();
            for(var node:graph.path("nodes")) {
                String id=node.path("id").asText();
                bindings.put(id,(user,joined)->{
                    started.add(id);
                    return new BoundNodeExecutor(h.control,h.path.substring("/v1/tasks/".length()),node,h.owner,h.epoch,()->true,
                        new RemoteExpertExecutor(h.control,h.config.path("bus")),new ReportChecker(h.control)).execute(user,joined);
                });
            }
            try(var scheduler=new BlueprintScheduler(new TraceForest(),3,null,null,Duration.ofSeconds(20))) {
                assertThrows(BlueprintException.class,()->scheduler.execute(adverse,bindings,"isolated adverse scheduler policy fixture"));
                assertTrue(scheduler.awaitMembersSettled(Duration.ofSeconds(5)));
            }
            assertTrue(started.contains(h.nodeOf("business@1"))&&started.contains(h.nodeOf("quality@1")),"native scheduler actually continued to the downstream wrappers");
            assertEquals(1,h.sends.get(),"only extractor request reached the provider");
            assertEquals(1,h.calls().size());
            var task=h.task();
            for(var node:graph.path("nodes")) {
                String id=node.path("id").asText();
                var actual=task.path("nodes").path(id);
                assertFalse(actual.hasNonNull("result"),"failed predecessor cannot yield a formal downstream result");
                if(!id.equals(h.node))assertEquals(0,actual.path("attempt").asInt(),id+" must not be admitted");
            }
            assertEquals("FAILED",h.control.post(h.path+"/complete",Map.of("owner",h.owner,"epoch",h.epoch)).path("status").asText());
            assertEquals("FAILED",h.task().path("status").asText());
            h.saveEvidence("predecessor-failed-scheduler-continued",Map.of("fault_injection","native scheduler with skip policy forced to NEVER; production spec not modified","wrappers_started",started,"send_requests",h.sends.get(),"downstream_provider_requests",0,"completion","FAILED"));
        }
    }

    @Test void acceptedRequestWithLostReplyDoesNotSendAgain() throws Exception {
        try(var h=new Harness(temp.resolve("lost-reply"))) {
            h.dropSendReply=true;
            var failure=assertThrows(ControlClient.ControlFailure.class,h::execute);
            assertEquals("REMOTE_UNKNOWN",failure.code);
            assertEquals(1,h.sends.get(),"provider accepted one request before dropping reply");
            JsonNode call=h.calls().getFirst();
            assertTrue(call.path("remote_task_id").asText().isBlank());
            var retry=assertThrows(ControlClient.ControlFailure.class,h::execute);
            assertEquals("REMOTE_UNKNOWN",retry.code);
            assertEquals(1,h.sends.get(),"unknown association must not create another SendMessage");
            assertEquals(call,h.calls().getFirst());
            assertEquals("UNKNOWN",h.task().path("nodes").path(h.node).path("status").asText());
            h.saveEvidence("accepted-lost-reply",Map.of("send_requests",h.sends.get(),"get_requests",h.gets.get(),"no_blind_resend",true));
        }
    }

    @Test void remoteTaskLostAfterAcceptanceKeepsOriginalAssociation() throws Exception {
        try(var h=new Harness(temp.resolve("task-lost"))) {
            h.state.set("WORKING");h.taskMissing=true;
            assertEquals("REMOTE_UNKNOWN",assertThrows(ControlClient.ControlFailure.class,h::execute).code);
            JsonNode call=h.calls().getFirst();
            assertEquals("fixture-remote-task",call.path("remote_task_id").asText());
            assertEquals("REMOTE_UNKNOWN",assertThrows(ControlClient.ControlFailure.class,h::execute).code);
            JsonNode after=h.calls().getFirst();
            assertEquals(call.path("call_id"),after.path("call_id"));
            assertEquals(call.path("message_id"),after.path("message_id"));
            assertEquals(call.path("remote_task_id"),after.path("remote_task_id"));
            assertEquals(1,h.sends.get());assertTrue(h.gets.get()>=2);
            h.saveEvidence("accepted-remote-task-lost",Map.of("send_requests",h.sends.get(),"get_requests",h.gets.get(),"same_remote_identity",true));
        }
    }

    @Test void inputAndAuthRequiredResumeOnlyPollExistingRemoteTask() throws Exception {
        for(String state:List.of("INPUT_REQUIRED","AUTH_REQUIRED")) {
            try(var h=new Harness(temp.resolve(state))) {
                h.state.set(state);
                assertEquals(state,assertThrows(ControlClient.ControlFailure.class,h::execute).code);
                String waiting=state.equals("INPUT_REQUIRED")?"WAITING_INPUT":"WAITING_AUTH";
                assertEquals(waiting,h.task().path("status").asText());
                JsonNode before=h.calls().getFirst();
                h.control.post(h.path+"/complete",Map.of("owner",h.owner,"epoch",h.epoch));
                h.userPost(h.path+"/resume",Map.of());
                var lease=h.control.post(h.path+"/claim",Map.of("owner",h.owner,"domain","graph"));
                h.epoch=lease.path("epoch").asText();
                // The provider is controlled out of band, as a supplemental
                // input/authorization service would be. FAILED ends this fixture
                // without inventing a successful business/model result.
                h.state.set("FAILED");
                assertEquals("REMOTE_FAILED",assertThrows(ControlClient.ControlFailure.class,h::execute).code);
                JsonNode after=h.calls().getFirst();
                for(String key:List.of("call_id","message_id","context_id","remote_task_id","attempt","input_digest")) {
                    assertEquals(before.path(key),after.path(key),key);
                }
                assertEquals(1,h.sends.get());assertEquals(1,h.gets.get());
                h.saveEvidence(state.toLowerCase()+"-resume",Map.of("send_requests",h.sends.get(),"get_requests",h.gets.get(),"same_remote_identity",true,"business_success_asserted",false));
            }
        }
    }

    @Test void typedGatewayFailuresReachAuthoritativeStateWithoutBlindResend() throws Exception {
        for (var entry : Map.of("AUTH_EXPIRED", "WAITING_AUTH", "CALLER_NOT_AUTHORIZED", "BLOCKED",
                "RDC_UNAVAILABLE", "WAITING_REMOTE").entrySet()) {
            try (var h = new Harness(temp.resolve("governance-" + entry.getKey()))) {
                h.governanceCode = entry.getKey();
                var failure = assertThrows(ControlClient.ControlFailure.class, h::execute);
                assertEquals(entry.getKey(), failure.code);
                assertEquals(entry.getValue(), h.task().path("status").asText());
                assertEquals(entry.getKey(), h.task().path("nodes").path(h.node).path("reason").asText());
                assertEquals(1, h.sends.get());
                assertTrue(h.calls().getFirst().path("remote_task_id").asText().isBlank());
                var observations = h.control.get("/v1/calls/" + h.calls().getFirst().path("call_id").asText() + "/observations");
                assertTrue(observations.toString().contains(entry.getKey()));
                assertFalse(observations.toString().contains("private fixture diagnostic"));
                assertFalse(h.task().path("nodes").path(h.node).hasNonNull("result"));
                assertThrows(ControlClient.ControlFailure.class, h::execute);
                assertEquals(1, h.sends.get(), "a classified failure must not authorize a second SendMessage");
                h.saveEvidence("governance-" + entry.getKey().toLowerCase(),
                        Map.of("code", entry.getKey(), "task_status", entry.getValue(), "send_requests", 1, "no_blind_resend", true));
            }
        }
    }

    static final class Harness implements AutoCloseable {
        final Path directory;
        final HttpClient http=HttpClient.newBuilder().cookieHandler(new CookieManager()).connectTimeout(Duration.ofSeconds(3)).build();
        final AtomicInteger sends=new AtomicInteger(),gets=new AtomicInteger();
        final AtomicInteger providerRequests=new AtomicInteger();
        final AtomicReference<String> state=new AtomicReference<>("FAILED");
        final CountDownLatch discoveryEntered=new CountDownLatch(1),discoveryReleased=new CountDownLatch(1);
        final HttpServer provider;
        Process go;
        Path configFile;
        String binary;
        Thread outputReader;
        java.io.ByteArrayOutputStream processOutput;
        int processNumber;
        final ControlClient control;
        final ObjectNode config;
        final String base,path,node,owner="fixture-worker";
        String epoch,csrf,context;
        volatile boolean dropSendReply,taskMissing;
        volatile String blockDiscoveryAgent;
        volatile String governanceCode;

        Harness(Path directory) throws Exception {this(directory,true);}
        Harness(Path directory,boolean claimInitial) throws Exception {
            this.directory=directory;
            String privatePath=System.getenv("REPORT_DEMO_CONFIG");binary=System.getenv("REPORT_CANONICAL_BINARY");
            Assumptions.assumeTrue(privatePath!=null&&binary!=null,"Actual private PG config and built Go executable required for isolated fault integration");
            Files.createDirectories(directory);
            config=(ObjectNode)Json.read(Files.readString(Path.of(privatePath)));
            String originalBase=config.path("public_url").asText();
            String source=System.getenv().getOrDefault("REPORT_FAULT_SOURCE_TASK","T-d2c511867ab929cbfe371cf5057de6b6");
            var original=new ControlClient(originalBase,config.path("tokens").path("worker").asText()).get("/v1/tasks/"+source);
            ObjectNode graph=(ObjectNode)original.path("graph").deepCopy();
            graph.put("goal","ISOLATED HTTP FAULT FIXTURE; not real model planning: "+graph.path("goal").asText());
            node=graph.path("nodes").get(0).path("id").asText();
            int port;
            try(var socket=new ServerSocket(0,0,java.net.InetAddress.getLoopbackAddress())) {port=socket.getLocalPort();}
            base="http://127.0.0.1:"+port;
            config.put("listen","127.0.0.1:"+port);config.put("public_url",base);
            config.put("scope","fault-"+UUID.randomUUID());config.put("artifact_root",directory.resolve("content").toAbsolutePath().toString());
            provider=HttpServer.create(new InetSocketAddress("127.0.0.1",0),0);
            String providerURL="http://127.0.0.1:"+provider.getAddress().getPort();
            String fixtureIssuer=providerURL+"/realms/fixture";
            config.set("bus",Json.MAPPER.valueToTree(Map.of("gateway_url",providerURL,"rdc_url",providerURL,"registry_token","isolated-registry-fixture",
                "token_url",fixtureIssuer+"/protocol/openid-connect/token","client_id","fixture","client_secret","fixture","subject","fixture-subject",
                "environment_id","isolated-"+UUID.randomUUID(),"tenant","fixture","issuer",fixtureIssuer)));
            ((ObjectNode)config.path("bus")).put("audience","fixture");
            ObjectNode busBinding=(ObjectNode)config.path("bus").deepCopy();
            busBinding.remove(List.of("gateway_url","rdc_url","registry_token","token_url","client_id","client_secret","subject"));
            busBinding.put("caller_subject","fixture-subject");config.set("bus_binding",busBinding);
            configFile=directory.resolve("config.json");Files.writeString(configFile,Json.write(config));
            go=startControl();
            control=new ControlClient(base,config.path("tokens").path("worker").asText());
            boolean ready=false;
            for(int attempt=0;attempt<100&&go.isAlive();attempt++) {
                try {control.get("/healthz");ready=true;break;}catch(ControlClient.ControlFailure ignored){Thread.sleep(100);}
            }
            if(!ready) {stopControl(false);throw new IllegalStateException("Isolated Go/PG fixture did not become ready");}
            provider.createContext("/",exchange->{
                try {
                    providerRequests.incrementAndGet();
                    Object body;
                    String requestPath=exchange.getRequestURI().getPath();
                    if(requestPath.equals("/realms/fixture/protocol/openid-connect/token")) {body=Map.of("access_token","isolated-fixture-token","expires_in",3600);}
                    else if(requestPath.endsWith("/card")) {
                        String agent=requestPath.split("/")[3];
                        if(agent.equals(blockDiscoveryAgent)) {
                            discoveryEntered.countDown();
                            try {if(!discoveryReleased.await(10,TimeUnit.SECONDS)){exchange.sendResponseHeaders(503,-1);return;}}
                            catch(InterruptedException failure){Thread.currentThread().interrupt();exchange.sendResponseHeaders(503,-1);return;}
                        }
                        body=Map.of("name",agent,"rawJson",Json.write(Map.of("name",agent,"version","1.0.0")));
                    } else if(requestPath.endsWith("/instances")) {
                        String agent=requestPath.split("/")[3];
                        body=Map.of("instances",List.of(Map.of("health","ONLINE","contractVersion","1.0.0","capabilityVersion","1.0.0","card",Map.of("name",agent))));
                    } else {
                        JsonNode rpc=Json.read(new String(exchange.getRequestBody().readAllBytes(),StandardCharsets.UTF_8));
                        boolean send=rpc.path("method").asText().equals("SendMessage");
                        if(send) {
                            sends.incrementAndGet();context=rpc.path("params").path("message").path("contextId").asText();
                            if(dropSendReply) {exchange.close();return;}
                        } else {gets.incrementAndGet();}
                        if (governanceCode != null) {
                            byte[] rejected = Json.write(Map.of("code", governanceCode, "message", "private fixture diagnostic")).getBytes(StandardCharsets.UTF_8);
                            exchange.getResponseHeaders().set("Content-Type", "application/json");
                            exchange.sendResponseHeaders(governanceCode.equals("RDC_UNAVAILABLE") ? 503 : 403, rejected.length);
                            exchange.getResponseBody().write(rejected);
                            return;
                        }
                        body=!send&&taskMissing?Map.of("jsonrpc","2.0","id",rpc.path("id").asText(),"error",Map.of("code",-32001,"message","fixture task lost")):
                            Map.of("jsonrpc","2.0","id",rpc.path("id").asText(),"result",Map.of("task",Map.of("id","fixture-remote-task","contextId",context,"status",Map.of("state","TASK_STATE_"+state.get()))));
                    }
                    byte[] bytes=Json.write(body).getBytes(StandardCharsets.UTF_8);
                    exchange.getResponseHeaders().set("Content-Type","application/json");exchange.sendResponseHeaders(200,bytes.length);exchange.getResponseBody().write(bytes);
                } finally {exchange.close();}
            });
            try {
            provider.start();
            JsonNode login=request("/auth/login",Map.of("user",config.path("user").asText(),"password",config.path("user_password").asText()),null);
            csrf=login.path("csrf").asText();
            // The source graph supplies topology only; admit this fixture under
            // the actual catalog served by its isolated Go process.
            graph.put("catalog_digest", control.get("/v1/catalog").path("catalog_digest").asText());
            JsonNode catalog=control.get("/v1/catalog");
            JsonNode selectionConfig=catalog.path("catalog").path("selection");
            ObjectNode query=Json.MAPPER.createObjectNode();
            for(String key:List.of("package","entity","basis","period","compare_period","deliverable"))query.set(key,selectionConfig.path(key));
            query.set("materials",Json.MAPPER.valueToTree(List.of(graph.path("material_id").asText())));
            query.set("focus",Json.MAPPER.valueToTree(graph.path("focus").asText().equals("business+quality")?List.of("经营","资产质量"):List.of("经营")));
            JsonNode selection=request("/v1/selections",query,config.path("tokens").path("planner").asText());
            graph.set("selection",Json.MAPPER.valueToTree(Map.of("id",selection.path("id").asText(),"digest",selection.path("digest").asText())));
            JsonNode created=request("/v1/commands",Map.of("command_id","fixture-"+UUID.randomUUID(),"graph",graph),config.path("tokens").path("planner").asText());
            path="/v1/tasks/"+created.path("task_id").asText();
            userPost(path+"/admit",Map.of("graph_digest",created.path("graph_digest").asText()));
            if(claimInitial)epoch=control.post(path+"/claim",Map.of("owner",owner,"domain","graph")).path("epoch").asText();
            } catch(Exception | Error failure) {
                provider.stop(0);stopControl(false);
                throw failure;
            }
        }

        Process startControl() throws Exception {
            var child=new ProcessBuilder(binary,"-config",configFile.toString()).redirectErrorStream(true).start();
            var output=new java.io.ByteArrayOutputStream();processOutput=output;
            outputReader=Thread.ofVirtual().start(()->{
                try(var input=child.getInputStream()){input.transferTo(output);}
                catch(java.io.IOException ignored){ /* process termination may close the pipe */ }
            });
            return child;
        }
        void stopControl(boolean force) throws Exception {
            if(force)go.destroyForcibly();else go.destroy();
            if(!go.waitFor(10,TimeUnit.SECONDS)){go.destroyForcibly();assertTrue(go.waitFor(10,TimeUnit.SECONDS));}
            assertFalse(go.isAlive(),"isolated control process leaked");
            outputReader.join(5000);assertFalse(outputReader.isAlive(),"process log reader leaked");
            Files.write(directory.resolve("go-"+(processNumber++)+".log"),processOutput.toByteArray());
        }
        void restartControl() throws Exception {
            stopControl(true);
            go=startControl();
            for(int i=0;i<100&&go.isAlive();i++) {
                try {control.get("/healthz");return;}catch(ControlClient.ControlFailure failure){Thread.sleep(100);}
            }
            throw new IllegalStateException("Restarted control did not become ready");
        }

        JsonNode request(String path,Object body,String token) throws Exception {
            var builder=HttpRequest.newBuilder(URI.create(base+path)).timeout(Duration.ofSeconds(15)).header("Content-Type","application/json").header("Origin",base);
            if(csrf!=null)builder.header("X-CSRF-Token",csrf);
            if(token!=null)builder.header("Authorization","Bearer "+token);
            var response=http.send(builder.POST(HttpRequest.BodyPublishers.ofString(Json.write(body))).build(),HttpResponse.BodyHandlers.ofString());
            assertEquals(200,response.statusCode(),"isolated fixture request rejected: "+path);
            return Json.read(response.body());
        }
        void userPost(String path,Object body) throws Exception {request(path,body,null);}
        JsonNode task() {return control.get(path);}
        List<JsonNode> calls() {return Json.MAPPER.convertValue(control.get(path+"/calls"),new com.fasterxml.jackson.core.type.TypeReference<List<JsonNode>>(){});}
        String nodeOf(String expert) {
            for(var n:task().path("graph").path("nodes")) {if(n.path("expert").asText().equals(expert))return n.path("id").asText();}
            throw new IllegalArgumentException("Fixture expert absent: "+expert);
        }
        void execute() {
            execute(node);
        }
        void execute(String nodeID) {
            JsonNode boundNode=null;
            for(var candidate:task().path("graph").path("nodes")){if(candidate.path("id").asText().equals(nodeID)){boundNode=candidate;break;}}
            if(boundNode==null)throw new IllegalArgumentException("Fixture node missing");
            new BoundNodeExecutor(control,path.substring("/v1/tasks/".length()),boundNode,owner,epoch,()->true,new RemoteExpertExecutor(control,config.path("bus")),new ReportChecker(control)).execute("","");
        }
        void commitFixtureNode(String nodeID) {
            JsonNode permit=control.post(path+"/nodes/"+nodeID+"/resolve",Map.of("owner",owner,"epoch",epoch));
            assertEquals("GRANTED",permit.path("status").asText());
            var candidate=control.post("/v1/artifacts",Map.of("fixture","isolated slow-RDC control transaction; not a model/business result","node",nodeID));
            control.post(path+"/nodes/"+nodeID+"/commit",Map.of("owner",owner,"epoch",epoch,"input_digest",permit.path("input_digest").asText(),"attempt",permit.path("attempt").asInt(),"artifact_id",candidate.path("artifact_id").asText()));
        }
        void saveEvidence(String name,Object checks) throws Exception {
            Path parent=Path.of(System.getenv("REPORT_DEMO_CONFIG")).toAbsolutePath().getParent().resolve("evidence");
            Files.writeString(parent.resolve("fault-"+name+".json"),Json.write(Map.of("provider","isolated fake HTTP; actual Java adapter, Go and PG","scope",config.path("scope").asText(),"task_id",task().path("task_id").asText(),"checks",checks,"calls",calls(),"model_calls",0)),StandardCharsets.UTF_8);
        }
        @Override public void close() throws Exception {provider.stop(0);stopControl(false);}
    }
}
