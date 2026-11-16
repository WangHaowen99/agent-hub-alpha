package com.openjiuwen.demo.expert;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.openjiuwen.core.singleagent.agents.ReActAgent;
import com.openjiuwen.core.singleagent.agents.ReActAgentConfig;
import com.openjiuwen.core.singleagent.schema.AgentCard;
import com.openjiuwen.service.adapters.agentcore.agentfw.JiuwenCoreAgentHandler;
import com.openjiuwen.service.spec.spi.AgentHandler;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.context.annotation.Bean;

@SpringBootApplication(scanBasePackages = "com.openjiuwen.service.app")
public class ExpertApplication {
    private static final ObjectMapper JSON = new ObjectMapper();

    public static void main(String[] args) { SpringApplication.run(ExpertApplication.class, args); }

    @Bean JsonNode expertConfiguration() throws Exception {
        String path = System.getenv("REPORT_EXPERT_CONFIG");
        if (path == null) { throw new IllegalStateException("REPORT_EXPERT_CONFIG required"); }
        return JSON.readTree(Files.readString(Path.of(path)));
    }

    @Bean AgentHandler reportExpertHandler(JsonNode expertConfiguration) throws Exception {
        String agentID = expertConfiguration.path("agent_id").asText();
        String role = expertConfiguration.path("role").asText();
        if (agentID.isBlank() || !List.of("extract", "business", "quality", "write").contains(role)) {
            throw new IllegalStateException("Unknown configured expert role");
        }
        ReActAgent agent = createAgent(agentID, role);
        return new BoundExpertHandler(expertConfiguration, agent, new JiuwenCoreAgentHandler(agent));
    }

    @Bean IngressIdentityFilter ingressIdentityFilter(JsonNode expertConfiguration) {
        return new IngressIdentityFilter(expertConfiguration);
    }

    static ReActAgent createAgent(String id, String role) throws Exception {
        String key = System.getenv("DEMO_MODEL_API_KEY");
        String base = System.getenv("DEMO_MODEL_BASE_URL");
        String model = System.getenv("DEMO_MODEL_NAME");
        if (key == null || key.isBlank() || base == null || model == null) {
            throw new IllegalStateException("Real model environment required; no fixed-output fallback");
        }
        String prompt;
        try (var input = ExpertApplication.class.getResourceAsStream("/prompts/" + role + ".md")) {
            if (input == null) { throw new IllegalStateException("Role prompt missing"); }
            prompt = new String(input.readAllBytes(), java.nio.charset.StandardCharsets.UTF_8);
        }
        ReActAgentConfig config = ReActAgentConfig.builder()
                .promptTemplate(List.of(Map.of("role", "system", "content", prompt)))
                .maxIterations(role.equals("quality") ? 8 : 4).build()
                .configureModelClient("OpenAI", key, base, model, true)
                .configureContextEngine(240, 60, false);
        config.getModelConfigObj().setTemperature(0.1);
        config.getModelConfigObj().setMaxTokens(6000);
        ReActAgent agent = new ReActAgent(AgentCard.builder().id(id).name(id).description("Bound annual report " + role).build());
        agent.configure(config);
        return agent;
    }
}
