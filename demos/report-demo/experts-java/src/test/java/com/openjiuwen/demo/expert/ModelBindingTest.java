package com.openjiuwen.demo.expert;

import static org.junit.jupiter.api.Assertions.*;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.sun.net.httpserver.HttpServer;
import com.openjiuwen.agents.orchestration.ReActAgentMember;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CopyOnWriteArrayList;
import org.junit.jupiter.api.Test;

class ModelBindingTest {
    private static final ObjectMapper JSON=new ObjectMapper();

    @Test void incompleteAmbiguousAndTrailingModelOutputCannotBecomeAnArtifact() throws Exception {
        assertTrue(BoundExpertHandler.parseOutput("  \n```json\n{\"ok\":true}\n```\n").path("ok").asBoolean());
        for(String output:List.of("{\"truncated", "{\"a\":1,\"a\":2}", "{\"ok\":true} {}", "null", "[]", "")) {
            assertThrows(Exception.class,()->BoundExpertHandler.parseOutput(output));
        }
    }

    @Test void pythonAndJavaUseTheSameLengthPrefixedModelBinding() throws Exception {
        var suite=JSON.readTree(Files.readString(Path.of(System.getProperty("basedir"),"..","contracts","model-binding-vectors.json")));
        for(var vector:suite.path("vectors")) {
            var settings=new ModelBinding.Settings("fixture-secret",vector.path("base_url").asText(),vector.path("model").asText(),
                    vector.path("max_tokens").asInt(6000),vector.path("normal_iterations").asInt(4),vector.path("quality_iterations").asInt(8));
            assertEquals(vector.path("digest").asText(),settings.digest());
            assertFalse(settings.toString().contains("fixture-secret"));
        }
    }

    @Test void budgetDriftIsRejectedAndInvalidBudgetsCannotReachAClient() {
        var settings=new ModelBinding.Settings("fixture","https://example.invalid/v1","model-a");
        var configuration=JSON.valueToTree(Map.of("model_binding_digest",settings.digest()));
        var env=new java.util.HashMap<>(Map.of("DEMO_MODEL_API_KEY","fixture","DEMO_MODEL_BASE_URL",settings.base(),"DEMO_MODEL_NAME",settings.model()));
        env.put("DEMO_MODEL_MAX_TOKENS","16000");
        assertTrue(assertThrows(IllegalStateException.class,()->ModelBinding.read(env,configuration)).getMessage().startsWith("MODEL_BINDING_CHANGED"));
        env.put("DEMO_MODEL_MAX_TOKENS","0");
        assertThrows(IllegalArgumentException.class,()->ModelBinding.read(env,configuration));
    }

    @Test void actualEnvironmentDriftIsRejectedWithoutChangingStoredProfile() {
        var a=new ModelBinding.Settings("fixture","https://example.invalid/v1","model-a");
        var configuration=JSON.valueToTree(Map.of("execution_profile","unchanged-profile","model_binding_digest",a.digest()));
        var env=Map.of("DEMO_MODEL_API_KEY","fixture","DEMO_MODEL_BASE_URL",a.base(),"DEMO_MODEL_NAME","model-b");
        assertTrue(assertThrows(IllegalStateException.class,()->ModelBinding.read(env,configuration)).getMessage().startsWith("MODEL_BINDING_CHANGED"));
        assertEquals("unchanged-profile",configuration.path("execution_profile").asText());
    }

    @Test void missingBindingCannotBeFilledFromCurrentEnvironment() {
        var configuration=JSON.valueToTree(Map.of("execution_profile","legacy"));
        var env=Map.of("DEMO_MODEL_API_KEY","fixture","DEMO_MODEL_BASE_URL","http://localhost/v1","DEMO_MODEL_NAME","model-a");
        assertTrue(assertThrows(IllegalStateException.class,()->ModelBinding.read(env,configuration)).getMessage().startsWith("MODEL_BINDING_UNAVAILABLE"));
    }

    @Test void validatedSettingsAreUsedByTheActualCoreModelClient() throws Exception {
        var requests=new CopyOnWriteArrayList<String>();
        var budgets=new CopyOnWriteArrayList<Integer>();
        var server=HttpServer.create(new InetSocketAddress("127.0.0.1",0),0);
        server.createContext("/",exchange->{
            var request=JSON.readTree(exchange.getRequestBody().readAllBytes());requests.add(request.path("model").asText());
            budgets.add(request.path("max_tokens").asInt());
            String text="{\"fixture\":true}";byte[] body;
            if(request.path("stream").asBoolean()) {
                String chunk=JSON.writeValueAsString(Map.of("id","fixture","object","chat.completion.chunk","created",1,"model","model-a","choices",List.of(Map.of("index",0,"delta",Map.of("role","assistant","content",text),"finish_reason","stop"))));
                body=("data: "+chunk+"\n\ndata: [DONE]\n\n").getBytes(StandardCharsets.UTF_8);
                exchange.getResponseHeaders().set("Content-Type","text/event-stream");
            } else {
                body=JSON.writeValueAsBytes(Map.of("id","fixture","object","chat.completion","created",1,"model","model-a","choices",List.of(Map.of("index",0,"message",Map.of("role","assistant","content",text),"finish_reason","stop"))));
                exchange.getResponseHeaders().set("Content-Type","application/json");
            }
            exchange.sendResponseHeaders(200,body.length);exchange.getResponseBody().write(body);exchange.close();
        });server.start();
        try {
            var actual=new ModelBinding.Settings("fixture-no-external-key","http://127.0.0.1:"+server.getAddress().getPort()+"/v1","model-a");
            actual.require(actual.digest());
            var agent=ExpertApplication.configuredAgent(actual,"model-outlet-fixture","business");
            assertNotNull(new ReActAgentMember(agent).execute("Return a fixture JSON object.","model binding outlet test"));
            assertFalse(requests.isEmpty());assertTrue(requests.stream().allMatch("model-a"::equals));
            assertTrue(budgets.stream().allMatch(n->n==actual.maxTokens()));
        } finally {server.stop(0);}
    }
}
