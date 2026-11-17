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
    private static volatile ModelBinding.Settings modelSettings;

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
        // Fail before any model client or listener is usable, including when
        // launched directly with java instead of the managed Python launcher.
        modelSettings = ModelBinding.read(System.getenv(), expertConfiguration);
        ReActAgent agent = createAgent(agentID, role);
        return new BoundExpertHandler(expertConfiguration, agent, new JiuwenCoreAgentHandler(agent), modelSettings);
    }

    @Bean IngressIdentityFilter ingressIdentityFilter(JsonNode expertConfiguration) {
        return new IngressIdentityFilter(expertConfiguration);
    }

    static ReActAgent createAgent(String id, String role) throws Exception {
        ModelBinding.Settings actual = modelSettings;
        if (actual == null) { throw new IllegalStateException("MODEL_BINDING_UNAVAILABLE"); }
        return configuredAgent(actual, id, role);
    }

    static ReActAgent configuredAgent(ModelBinding.Settings actual, String id, String role) throws Exception {
        String prompt;
        try (var input = ExpertApplication.class.getResourceAsStream("/prompts/" + role + ".md")) {
            if (input == null) { throw new IllegalStateException("Role prompt missing"); }
            prompt = new String(input.readAllBytes(), java.nio.charset.StandardCharsets.UTF_8);
        }
        ReActAgentConfig config = ReActAgentConfig.builder()
                .promptTemplate(List.of(Map.of("role", "system", "content", prompt)))
                .maxIterations(role.equals("quality") ? ModelBinding.QUALITY_ITERATIONS : ModelBinding.NORMAL_ITERATIONS).build()
                .configureModelClient("OpenAI", actual.key(), actual.base(), actual.model(), true)
                .configureContextEngine(240, 60, false);
        config.getModelConfigObj().setTemperature(ModelBinding.TEMPERATURE);
        config.getModelConfigObj().setMaxTokens(ModelBinding.MAX_TOKENS);
        ReActAgent agent = new ReActAgent(AgentCard.builder().id(id).name(id).description("Bound annual report " + role).build());
        agent.configure(config);
        return agent;
    }
}
