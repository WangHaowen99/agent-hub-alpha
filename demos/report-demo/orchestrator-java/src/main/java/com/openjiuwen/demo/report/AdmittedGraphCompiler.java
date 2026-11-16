package com.openjiuwen.demo.report;

import com.fasterxml.jackson.databind.JsonNode;
import com.openjiuwen.agents.orchestration.BlueprintSpec;
import com.openjiuwen.agents.orchestration.MemberSpec;
import com.openjiuwen.agents.orchestration.StepSpec;
import com.openjiuwen.agents.orchestration.TeamBlueprints;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * TeamBlueprints compilation/data projection adapted from WangHaowen99's prototype,
 * inherited by rt77789 W05 a4d20c3, credit-orchestrator/Blueprints.java.
 * Keeps the native JSON digest distinct from Go's admitted JCS binding digest.
 */
final class AdmittedGraphCompiler {
    record Compiled(BlueprintSpec spec, Map<String, Object> projection, String digest) { }

    Compiled compile(JsonNode graph) {
        if (!graph.path("nodes").isArray() || graph.path("nodes").size() < 3 || graph.path("nodes").size() > 12) {
            throw new IllegalArgumentException("Expected bounded graph nodes");
        }
        List<Map<String, Object>> team = new ArrayList<>();
        for (JsonNode node : graph.path("nodes")) {
            Map<String, Object> item = new LinkedHashMap<>();
            item.put("id", node.path("id").asText());
            item.put("role", node.path("role").asText());
            item.put("task", node.path("task").asText());
            item.put("dependsOn", Json.MAPPER.convertValue(node.path("depends_on"), List.class));
            // Conditions are checked by the bound executor against committed typed
            // checker output. Every branch therefore records a Go-authorized skip.
            team.add(item);
        }
        BlueprintSpec spec = TeamBlueprints.compile("graph", TeamBlueprints.parseAgents(team), false);
        List<Object> members = new ArrayList<>();
        for (MemberSpec m : spec.members()) {
            Map<String, Object> value = new LinkedHashMap<>();
            value.put("id", m.id()); value.put("instruction", m.instruction()); members.add(value);
        }
        List<Object> steps = new ArrayList<>();
        for (StepSpec step : spec.steps()) {
            Map<String, Object> value = new LinkedHashMap<>();
            value.put("id", step.id()); value.put("member", step.member());
            value.put("depends_on", step.dependsOn() == null ? List.of() : step.dependsOn()); steps.add(value);
        }
        Map<String, Object> blueprint = new LinkedHashMap<>();
        blueprint.put("id", spec.id()); blueprint.put("type", spec.type().name());
        blueprint.put("members", members); blueprint.put("steps", steps);
        Map<String, Object> projection = new LinkedHashMap<>();
        projection.put("compiler_version", "report-compiler/1"); projection.put("blueprint", blueprint);
        // Include adapter-controlled conditions and role/capability/data binding,
        // not just scheduler edges, in the native runtime contract.
        projection.put("graph", graph);
        return new Compiled(spec, projection, Json.digest(projection));
    }
}
