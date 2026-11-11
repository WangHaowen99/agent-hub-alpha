package com.openjiuwen.demo.credit;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import com.fasterxml.jackson.dataformat.yaml.YAMLGenerator;
import com.openjiuwen.agents.orchestration.BlueprintSpec;
import com.openjiuwen.agents.orchestration.MemberSpec;
import com.openjiuwen.agents.orchestration.StepSpec;
import com.openjiuwen.agents.orchestration.TeamBlueprints;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Compiles a planner's team into a blueprint with agent-solution's {@link TeamBlueprints} and renders
 * it as data. The digest is what the plan approval binds; the executor recompiles the approved team
 * and refuses to run when the digest differs.
 */
final class Blueprints {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final ObjectMapper YAML = new ObjectMapper(new YAMLFactory()
            .disable(YAMLGenerator.Feature.WRITE_DOC_START_MARKER)
            .enable(YAMLGenerator.Feature.MINIMIZE_QUOTES));

    private Blueprints() {
    }

    static BlueprintSpec compile(String mode, Object team) {
        return TeamBlueprints.compile(mode, TeamBlueprints.parseAgents(team), false);
    }

    /** The blueprint as plain data (member instructions, steps, dependencies, conditions). */
    static Map<String, Object> data(BlueprintSpec spec) {
        List<Object> members = new ArrayList<>();
        for (MemberSpec m : spec.members()) {
            Map<String, Object> mm = new LinkedHashMap<>();
            mm.put("id", m.id());
            mm.put("instruction", m.instruction());
            members.add(mm);
        }
        List<Object> steps = new ArrayList<>();
        for (StepSpec s : spec.steps()) {
            Map<String, Object> sm = new LinkedHashMap<>();
            sm.put("id", s.id());
            sm.put("member", s.member());
            sm.put("depends_on", s.dependsOn() == null ? List.of() : s.dependsOn());
            if (s.skipIf() != null) {
                sm.put("skip_if", s.skipIf().name());
            }
            if (s.when() != null) {
                Map<String, Object> w = new LinkedHashMap<>();
                w.put("step", s.when().step());
                w.put("contains", s.when().text());
                w.put("negate", !s.when().isContains());
                sm.put("when", w);
            }
            steps.add(sm);
        }
        Map<String, Object> bp = new LinkedHashMap<>();
        bp.put("id", spec.id());
        bp.put("type", spec.type().name().toLowerCase());
        bp.put("members", members);
        bp.put("steps", steps);
        return Map.of("blueprint", bp);
    }

    static String yaml(BlueprintSpec spec) throws Exception {
        return YAML.writeValueAsString(data(spec));
    }

    static String digest(BlueprintSpec spec) throws Exception {
        byte[] raw = JSON.writeValueAsString(data(spec)).getBytes(StandardCharsets.UTF_8);
        try {
            return "sha256:" + HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(raw));
        } catch (NoSuchAlgorithmException ex) {
            throw new IllegalStateException(ex);
        }
    }

    @SuppressWarnings("unchecked")
    static List<Map<String, Object>> steps(BlueprintSpec spec) {
        Map<String, Object> bp = (Map<String, Object>) data(spec).get("blueprint");
        return (List<Map<String, Object>>) bp.get("steps");
    }
}
