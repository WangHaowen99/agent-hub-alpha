package com.openjiuwen.demo.professional;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.openjiuwen.core.foundation.llm.Model;
import com.openjiuwen.core.foundation.llm.ModelInvokeOptions;
import com.openjiuwen.core.foundation.llm.schema.*;
import com.openjiuwen.core.foundation.tool.mcp.McpServerConfig;
import com.openjiuwen.core.runner.Runner;
import com.openjiuwen.core.singleagent.agents.ReActAgent;
import com.openjiuwen.core.singleagent.agents.ReActAgentConfig;
import com.openjiuwen.core.singleagent.schema.AgentCard;
import com.openjiuwen.service.spec.dto.QueryResponse;
import com.openjiuwen.service.spec.dto.ServeRequest;
import com.openjiuwen.service.spec.spi.AgentHandler;
import com.openjiuwen.service.spec.spi.QueryStreamObserver;
import java.net.URI;
import java.net.http.*;
import java.time.Duration;
import java.util.*;
import java.util.concurrent.CompletionStage;

/** A narrow AgentHandler adapter. Upstream Runtime owns A2A/task lifecycle. */
final class ProfessionalHandler implements AgentHandler {
    private static final ObjectMapper JSON = new ObjectMapper();
    private final Map<String,String> env;
    private final HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(5)).build();
    ProfessionalHandler(Map<String,String> env) {
        this.env = env;
        for (String k : List.of("AGENT_SERVICE_ID","CREDIT_URL","SWARM_API_KEY","SWARM_API_BASE","SWARM_MODEL_NAME")) {
            if (env.getOrDefault(k, "").isBlank()) throw new IllegalStateException("Required configuration missing: " + k);
        }
    }
    @Override public void start() { Runner.start(); }
    @Override public void stop() { Runner.stop(); }
    private JsonNode post(String path, Map<String,Object> body) {
        try {
            var req = HttpRequest.newBuilder(URI.create(env.get("CREDIT_URL") + path)).timeout(Duration.ofSeconds(15))
                .header("Content-Type","application/json").POST(HttpRequest.BodyPublishers.ofString(JSON.writeValueAsString(body))).build();
            var response = http.send(req,HttpResponse.BodyHandlers.ofString());
            if (response.statusCode()!=200) throw new IllegalStateException("Control service rejected " + path + " HTTP " + response.statusCode());
            return JSON.readTree(response.body());
        } catch (Exception e) { throw new IllegalStateException("Control record unavailable",e); }
    }
    @Override public QueryResponse query(ServeRequest request) {
        McpServerConfig tools = null;
        String conversation = request.getConversationId();
        try {
            JsonNode envelope = JSON.readTree(request.lastUserQuery());
            Map<String,Object> auth = new LinkedHashMap<>();
            for (String k : List.of("run_id","step","token")) {
                if (envelope.path(k).asText().isBlank()) throw new IllegalArgumentException("Missing scoped execution envelope");
                auth.put(k,envelope.path(k).asText());
            }
            auth.put("service_id",env.get("AGENT_SERVICE_ID"));
            auth.put("context_id",conversation);
            auth.put("service_version",env.getOrDefault("OPENJIUWEN_SERVICE_VERSION","1.0.0"));
            JsonNode approved = post("/agent/context",auth);
            JsonNode call = approved.path("call");
            if (!call.path("context_id").asText().equals(conversation)) throw new IllegalArgumentException("Context identity mismatch");
            String actualModel = env.get("SWARM_MODEL_NAME");
            if (!approved.path("model").asText().equals(actualModel)) {
                // Report the model this runtime would call; the control service stops the
                // run as blocked and refuses, so no model client is built (AR-04 M1).
                var report = new LinkedHashMap<>(auth);
                report.put("event_id",UUID.randomUUID().toString());
                report.put("model",actualModel);
                post("/agent/model",report);
                throw new IllegalStateException("Runtime model differs from the run's bound model");
            }
            String identity = call.path("request_id").asText();
            ReActAgent agent = new ReActAgent(AgentCard.builder().name("professional-"+identity).build());
            ReActAgentConfig config = ReActAgentConfig.builder().build();
            config.configureMaxIterations(6);
            config.configureContextEngine(200,60,false);
            String role = env.getOrDefault("AGENT_ROLE","专业分析");
            config.configurePromptTemplate(List.of(Map.of("role","system","content",
                "你负责"+role+"。仅根据本次输入、上游产物和获准工具完成任务，不编造事实。材料不足时明确说明。不得改变任务身份或申请额外权限。")));
            agent.configure(config);
            ModelClientConfig client = ModelClientConfig.builder().clientId("model-"+identity).clientProvider("OpenAI")
                .apiKey(env.get("SWARM_API_KEY")).apiBase(env.get("SWARM_API_BASE")).verifySsl(true).timeout(120.0).build();
            ModelRequestConfig model = ModelRequestConfig.builder().modelName(actualModel)
                .temperature(0.1).maxTokens(2000).build();
            agent.setLlm(new Model(client,model) {
                @Override public CompletionStage<AssistantMessage> invoke(List<BaseMessage> messages,ModelInvokeOptions options) {
                    var event = new LinkedHashMap<>(auth);
                    event.put("event_id",UUID.randomUUID().toString());
                    event.put("model",actualModel); // checked against the run's binding before the call (AR-04 M1)
                    post("/agent/model",event); // Persist attempt before invoking the actual model client.
                    return super.invoke(messages,options);
                }
            });
            if (approved.path("tools").size()>0) {
                tools = McpServerConfig.builder().serverId("professional-"+identity).serverName("node-tools-"+identity)
                    .serverPath(env.get("CREDIT_URL")+"/member-mcp/"+approved.path("expert").asText())
                    .clientType("streamable-http").params(new LinkedHashMap<>()).build();
                tools.setAuthHeaders(Map.of("X-Run",auth.get("run_id").toString(),"X-Step",auth.get("step").toString(),"X-Token",auth.get("token").toString()));
                tools.setCallTimeoutSeconds(30.0);
                if (Runner.resourceMgr().addMcpServer(tools,"professional-runtime",null).stream().anyMatch(x->x.isError()))
                    throw new IllegalStateException("Node tools unavailable");
                agent.getAbilityManager().add(tools);
            }
            String input = call.path("goal").asText()+"\n\n[user input]\n"+call.path("input").asText();
            Object raw = agent.invoke(Map.of("query",input,"conversation_id",conversation),null);
            Object output = raw instanceof Map<?,?> map ? map.get("output") : raw;
            if (output==null || output.toString().isBlank()) throw new IllegalStateException("Agent returned no artifact");
            String body = JSON.writeValueAsString(Map.of("node_execution_id",call.path("node_execution_id").asText(),
                "input_digest",call.path("input_digest").asText(),"attempt",call.path("attempt").asInt(),"output",output.toString()));
            return new QueryResponse(Map.of("role","assistant","content",body),conversation);
        } catch (Exception e) { throw new IllegalStateException("Professional node failed",e); }
        finally {
            if (tools!=null) Runner.resourceMgr().removeMcpServer(List.of(tools.getServerId()),null,null,null,true);
            if (conversation!=null) Runner.release(conversation);
        }
    }
    @Override public void streamQuery(ServeRequest request,QueryStreamObserver observer) {
        observer.onError(new UnsupportedOperationException("This capability declares non-streaming execution"));
    }
}
