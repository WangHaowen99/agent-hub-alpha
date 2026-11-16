package com.openjiuwen.demo.expert;

import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.openjiuwen.agents.orchestration.ReActAgentMember;
import com.openjiuwen.agents.reactrails.loopforest.LoopForestSuite;
import com.openjiuwen.core.singleagent.agents.ReActAgent;
import com.openjiuwen.service.spec.dto.QueryResponse;
import com.openjiuwen.service.spec.dto.ServeRequest;
import com.openjiuwen.service.spec.spi.AgentHandler;
import com.openjiuwen.service.spec.spi.QueryStreamObserver;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.Semaphore;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.CopyOnWriteArrayList;
import java.security.MessageDigest;
import java.util.HexFormat;

/** Platform binding/lifecycle only. Business experts see report I/O, never control credentials. */
final class BoundExpertHandler implements AgentHandler {
    private static final ObjectMapper JSON = new ObjectMapper().enable(JsonParser.Feature.STRICT_DUPLICATE_DETECTION);
    private final JsonNode config;
    private final ReActAgent agent;
    private final AgentHandler delegate;
    private final HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(3)).build();
    private final ConcurrentHashMap<String, Context> contexts = new ConcurrentHashMap<>();
    private final Semaphore capacity = new Semaphore(1);
    private LoopForestSuite.Installation forest;
    private final AtomicInteger branchCount = new AtomicInteger();
    private final List<Map<String,Object>> branchRecords = new CopyOnWriteArrayList<>();

    private record Context(JsonNode original, JsonNode approved, ServeRequest request) { }
    private record Token(BoundExpertHandler owner, String conversation, Optional<Object> delegateToken) { }

    BoundExpertHandler(JsonNode config, ReActAgent agent, AgentHandler delegate) {
        this.config = config; this.agent = agent; this.delegate = delegate;
    }

    @Override public void start() {
        delegate.start();
        if (config.path("role").asText().equals("quality")) {
            forest = LoopForestSuite.builder().forkEnabled(true)
                    .forkDescription("在本节点输入范围内核对资产质量指标；最多两项只读子任务，不新增外部动作")
                    .executor((input, goal) -> {
                        if (branchCount.incrementAndGet() > 2) { throw new IllegalStateException("Node branch budget exhausted"); }
                        try {
                            // Native ForkTool carries only sub_goal. It does
                            // not inherit the parent's messages/upstream data.
                            // The single-capacity Runtime supplies the already
                            // authorized business input, never model-supplied I/O.
                            if(contexts.size()!=1){throw new IllegalStateException("Exactly one bound node context required");}
                            Context parent=contexts.values().iterator().next();
                            String businessInput=parent.request().lastUserQuery();
                            ReActAgent branch = ExpertApplication.createAgent("quality-branch-" + UUID.randomUUID(), "quality-branch");
                            String output=new ReActAgentMember(branch).execute(businessInput, goal);
                            branchRecords.add(Map.of("branch_id",UUID.randomUUID().toString(),"goal",goal,
                                    "parent_input_digest",parent.approved().path("input_digest").asText(),
                                    "input_digest",sha(businessInput),"output_digest",sha(output),"output",output,"completed",true));
                            return output;
                        } catch (Exception e) { throw new IllegalStateException("Bound quality branch failed", e); }
                    }).build().install(agent);
        }
    }
    @Override public void stop() { if (forest != null) { forest.close(); } delegate.stop(); }
    @Override public void clearSession(String id) { delegate.clearSession(id); }

    @Override public Optional<Object> prepareTask(ServeRequest request) {
        if (!capacity.tryAcquire()) { throw new IllegalStateException("Expert process is busy; retain original task identity"); }
        Optional<Object> delegateToken = Optional.empty();
        try {
            JsonNode input = JSON.readTree(request.lastUserQuery());
            String call = input.path("call_id").asText();
            if (call.isBlank()) { throw new IllegalArgumentException("Bound call envelope required"); }
            String url = config.path("control_url").asText() + "/v1/experts/" + config.path("agent_id").asText() + "/calls/" + call + "/authorize";
            var auth = Map.of("input_digest", input.path("input_digest").asText(), "attempt", input.path("attempt").asInt(),
                    "context_id", request.getConversationId());
            var post = HttpRequest.newBuilder(URI.create(url)).timeout(Duration.ofSeconds(10))
                    .header("Authorization", "Bearer " + config.path("expert_token").asText()).header("Content-Type", "application/json")
                    .POST(HttpRequest.BodyPublishers.ofString(JSON.writeValueAsString(auth))).build();
            var response = http.send(post, HttpResponse.BodyHandlers.ofString());
            if (response.statusCode() != 200) { throw new IllegalStateException("Control rejected expert input HTTP " + response.statusCode()); }
            JsonNode approved = JSON.readTree(response.body());
            if (!config.path("execution_profile").asText().isBlank()
                    && !config.path("execution_profile").asText().equals(approved.path("binding").path("execution_profile").asText())) {
                throw new IllegalArgumentException("Source/model execution profile mismatch");
            }
            branchCount.set(0);
            branchRecords.clear();
            if (!approved.path("binding").path("capability").path("agent_id").asText().equals(config.path("agent_id").asText())) {
                throw new IllegalArgumentException("Capability identity mismatch");
            }
            ServeRequest actual = new ServeRequest();
            actual.setConversationId(request.getConversationId()); actual.setStream(false);
            actual.setTenantId(request.getTenantId()); actual.setUserId(request.getUserId()); actual.setSpaceId(request.getSpaceId());
            Object material = config.path("role").asText().equals("extract") ? approved.path("material")
                    : Map.of("id", approved.path("material").path("id").asText(), "digest", approved.path("material").path("digest").asText());
            JsonNode businessBinding = approved.path("binding").deepCopy();
            if (config.path("role").asText().equals("extract")) {
                ((com.fasterxml.jackson.databind.node.ObjectNode) businessBinding).remove(List.of("task_id", "graph_revision", "node_id", "goal", "focus"));
            }
            actual.setMessages(List.of(Map.of("role", "user", "content", JSON.writeValueAsString(
                    Map.of("binding", businessBinding, "material", material)))));
            delegateToken = delegate.prepareTask(actual);
            if (contexts.putIfAbsent(request.getConversationId(), new Context(input, approved, actual)) != null) {
                throw new IllegalStateException("Conversation already in flight");
            }
            return Optional.of(new Token(this, request.getConversationId(), delegateToken));
        } catch (Exception e) {
            delegate.completeTask(delegateToken); capacity.release();
            throw new IllegalStateException("Bound expert preparation failed", e);
        }
    }

    @Override public QueryResponse query(ServeRequest request) {
        Context ctx = contexts.get(request.getConversationId());
        if (ctx == null) { throw new IllegalStateException("prepareTask must authorize this request first"); }
        QueryResponse response = delegate.query(ctx.request());
        try {
            JsonNode result = JSON.valueToTree(response.getResult());
            String text = result.path("content").asText();
            if (text.startsWith("```")) { text = text.replaceFirst("^```(?:json)?\\s*", "").replaceFirst("\\s*```$", ""); }
            JsonNode output = JSON.readTree(text);
            if (!output.isObject()) { throw new IllegalArgumentException("Expert output must be a JSON object"); }
            String body = JSON.writeValueAsString(Map.of("call_id", ctx.original().path("call_id").asText(),
                    "input_digest", ctx.approved().path("input_digest").asText(), "attempt", ctx.original().path("attempt").asInt(), "output", output,
                    "runtime_evidence", Map.of("agent_id",config.path("agent_id").asText(),"role",config.path("role").asText(),"loop_branches",branchCount.get(),
                            "completed_loop_branches",branchRecords.size(),"branch_records",List.copyOf(branchRecords))));
            return new QueryResponse(Map.of("role", "assistant", "content", body), request.getConversationId());
        } catch (Exception e) { throw new IllegalStateException("Expert did not produce declared JSON output", e); }
    }

    @Override public void streamQuery(ServeRequest request, QueryStreamObserver observer) {
        try { observer.onComplete(query(request)); } catch (Exception e) { observer.onError(e); }
    }

    @Override public void completeTask(Optional<Object> token) {
        if (token.isPresent() && token.get() instanceof Token owned && owned.owner() == this) {
            if (contexts.remove(owned.conversation()) == null) { return; }
            try { delegate.completeTask(owned.delegateToken()); }
            finally { capacity.release(); }
        }
    }

    private static String sha(String value) throws Exception {return "sha256:"+HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(value.getBytes(java.nio.charset.StandardCharsets.UTF_8)));}
}
