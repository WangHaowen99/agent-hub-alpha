package com.openjiuwen.demo.credit;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.openjiuwen.agents.reactrails.loopforest.execution.SubAgentExecutor;
import java.net.URI;
import java.net.http.*;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.io.IOException;
import java.util.*;

/** Agent Bus A2A 1.0 JSON-RPC adapter; Graph remains the only scheduler. */
final class RemoteAgentMember implements SubAgentExecutor {
    private static final ObjectMapper JSON = new ObjectMapper();
    private final GoClient go;
    private final String run,token,step,gateway,credential;
    private Map<String,Object> observation;
    private final HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(5)).build();
    private final Clock clock;
    RemoteAgentMember(GoClient go,String run,String token,String step,Map<String,String> env) {
        this(go,run,token,step,env,Clock.systemUTC());
    }
    RemoteAgentMember(GoClient go,String run,String token,String step,Map<String,String> env,Clock clock) {
        this.go=go; this.run=run; this.token=token; this.step=step; this.clock=clock;
        gateway=env.getOrDefault("CREDIT_BUS_GATEWAY",""); credential=env.getOrDefault("CREDIT_BUS_TOKEN","");
        if (gateway.isBlank() || credential.isBlank()) throw new IllegalStateException("Agent Bus configuration missing; local fallback forbidden");
    }
    private Map<String,Object> base() { return new LinkedHashMap<>(Map.of("run_id",run,"token",token,"step",step)); }
    private Map<String,Object> identity(JsonNode call) {
        var body=base();
        body.put("node_execution_id",call.path("node_execution_id").asText());
        body.put("input_digest",call.path("input_digest").asText());
        body.put("attempt",call.path("attempt").asInt());
        return body;
    }
    private void update(JsonNode call,String task,String state,String reason,long seq) {
        var body=identity(call);
        body.put("remote_task_id",task);body.put("context_id",call.path("context_id").asText());
        body.put("status",state);body.put("reason",reason);body.put("sequence",seq);
        if (observation != null) body.put("gateway", new LinkedHashMap<>(observation));
        go.post("/exec/call/update",body,15);
    }
    private JsonNode rpc(String method,String id,Map<String,Object> params) throws Exception {
        var request=HttpRequest.newBuilder(URI.create(gateway.replaceAll("/$","")+"/a2a"))
            .timeout(Duration.ofSeconds(30)).header("Content-Type","application/json")
            .header("Authorization","Bearer "+credential)
            .POST(HttpRequest.BodyPublishers.ofString(JSON.writeValueAsString(Map.of("jsonrpc","2.0","id",id,"method",method,"params",params)))).build();
        observation = new LinkedHashMap<>();
        observation.put("rpc_id", id);
        observation.put("method", method);
        observation.put("started_at", clock.instant().toString());
        observation.put("outcome", "invalid_response");
        observation.put("error_code", "INVALID_A2A_RESPONSE");
        try {
            var response=http.send(request,HttpResponse.BodyHandlers.ofString());
            observation.put("http_status", response.statusCode());
            JsonNode body=JSON.readTree(response.body());
            if (response.statusCode()!=200 || body.has("error")) {
                String code=body.path("error").path("data").path("code").asText(body.path("code").asText("A2A_ERROR"));
                if (!code.matches("[A-Z][A-Z0-9_]{0,63}")) code="A2A_ERROR";
                observation.put("outcome", response.statusCode()==200 ? "protocol_error" : "http_error");
                observation.put("error_code", code);
                if (method.equals("SendMessage") && Set.of("AUTH_MISSING","AUTH_INVALID","TENANT_UNRESOLVED","VALIDATION_FAILED","ROUTE_NO_CANDIDATES","RDC_UNAVAILABLE","ROUTE_RESOLVE_FAILED").contains(code))
                    throw new RemoteFailed("调用被拒绝："+code);
                throw new IllegalStateException("Gateway HTTP "+response.statusCode()+" "+code);
            }
            if (!body.has("result")) throw new IllegalStateException("Missing A2A result");
            return body.path("result").has("task")?body.path("result").path("task"):body.path("result");
        } catch (IOException | InterruptedException e) {
            if (!observation.containsKey("http_status")) {
                observation.put("outcome", "transport_error");
                observation.put("error_code", e instanceof HttpTimeoutException ? "GATEWAY_TIMEOUT" : "GATEWAY_IO_ERROR");
            }
            throw e;
        } finally {
            observation.put("finished_at", clock.instant().toString());
        }
    }

    @Override public String execute(String userInput,String subGoal) {
        var begin=base();begin.put("input",userInput==null?"":userInput);begin.put("goal",subGoal==null?"":subGoal);
        JsonNode decision=go.post("/exec/call/begin",begin,30);
        JsonNode call=decision.path("call");
        String action=decision.path("action").asText();
        if (action.equals("reuse")) return decision.path("output").asText();
        String taskID=call.path("remote_task_id").asText();
        long seq=call.path("sequence").asLong();
        if (action.equals("unknown")) {
            update(call,taskID,"unknown","派发结果未知，需核对原调用；未重发",++seq);
            throw new IllegalStateException("Remote call unresolved; no resend");
        }
        JsonNode task;
        try {
            if (action.equals("send")) {
                String envelope=JSON.writeValueAsString(Map.of("run_id",run,"step",step,"token",call.path("capability_token").asText()));
                task=rpc("SendMessage",call.path("request_id").asText(),Map.of(
                    "message",Map.of("role","ROLE_USER","messageId",call.path("message_id").asText(),
                        "contextId",call.path("context_id").asText(),"parts",List.of(Map.of("kind","text","text",envelope))),
                    "configuration",Map.of("returnImmediately",true),
                    "metadata",Map.of("agentId",call.path("binding").path("agent_id").asText(),
                        "nodeExecutionId",call.path("node_execution_id").asText(),"attempt",call.path("attempt").asInt())));
            } else { task=rpc("GetTask",UUID.randomUUID().toString(),Map.of("id",taskID)); }
            long deadline=System.nanoTime()+Duration.ofMinutes(4).toNanos();
            while (true) {
                String received=task.path("id").asText();
                if (received.isBlank() || (!taskID.isBlank()&&!received.equals(taskID))) throw new IllegalStateException("Remote task identity mismatch");
                if (!task.path("contextId").asText().equals(call.path("context_id").asText())) throw new IllegalStateException("Remote context mismatch");
                taskID=received;
                String state=task.path("status").path("state").asText();
                if (!Set.of("TASK_STATE_SUBMITTED", "TASK_STATE_WORKING", "TASK_STATE_COMPLETED", "TASK_STATE_FAILED", "TASK_STATE_REJECTED", "TASK_STATE_CANCELED", "TASK_STATE_INPUT_REQUIRED", "TASK_STATE_AUTH_REQUIRED").contains(state))
                    throw new IllegalStateException("Unrecognized remote task state");
                observation.put("outcome", "task_received");
                observation.remove("error_code");
                update(call,taskID,"waiting","专业 Agent 执行中，查询原任务",++seq);
                if (state.equals("TASK_STATE_COMPLETED")) {
                    StringBuilder text=new StringBuilder();
                    for (JsonNode artifact:task.path("artifacts")) for (JsonNode part:artifact.path("parts")) if(part.has("text")) text.append(part.path("text").asText());
                    JsonNode artifact=JSON.readTree(text.toString());
                    if (!artifact.path("node_execution_id").asText().equals(call.path("node_execution_id").asText())
                        || !artifact.path("input_digest").asText().equals(call.path("input_digest").asText())
                        || artifact.path("attempt").asInt()!=call.path("attempt").asInt()) throw new IllegalStateException("Remote artifact identity mismatch");
                    var commit=identity(call);commit.put("remote_task_id",taskID);commit.put("context_id",call.path("context_id").asText());
                    commit.put("output",artifact.path("output").asText());
                    return go.post("/exec/call/commit",commit,15).path("output").asText();
                }
                if (Set.of("TASK_STATE_FAILED","TASK_STATE_REJECTED","TASK_STATE_CANCELED").contains(state)) {
                    throw new RemoteFailed("Remote task ended: "+state);
                }
                if (System.nanoTime()>deadline || Set.of("TASK_STATE_INPUT_REQUIRED","TASK_STATE_AUTH_REQUIRED").contains(state)) {
                    throw new IllegalStateException("Remote task requires waiting or input: "+state);
                }
                Thread.sleep(1000);
                task=rpc("GetTask",UUID.randomUUID().toString(),Map.of("id",taskID));
            }
        } catch (RemoteFailed e) { update(call,taskID,"failed",e.getMessage(),++seq); throw e; }
        catch (GoClient.FencedException | GoClient.InputConflictException | GoClient.StorageUnavailableException
               | GoClient.BindingRejectedException | GoClient.ControlRejectedException e) { throw e; } // control-plane facts, not an unknown remote outcome
        catch (Exception e) {
            if (e instanceof InterruptedException) Thread.currentThread().interrupt();
            String reason=e instanceof IllegalStateException?e.getMessage():e.getClass().getSimpleName();
            update(call,taskID,"unknown","远端结果待核对："+reason+"；未重发",++seq);
            throw new IllegalStateException("Remote outcome unresolved; original task retained",e);
        }
    }
    private static final class RemoteFailed extends RuntimeException { RemoteFailed(String message) { super(message); } }
}
