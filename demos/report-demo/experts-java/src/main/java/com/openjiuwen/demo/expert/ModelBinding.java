package com.openjiuwen.demo.expert;

import com.fasterxml.jackson.databind.JsonNode;
import java.io.ByteArrayOutputStream;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;

/** Actual immutable client settings, checked inside the Runtime, not only by its launcher. */
final class ModelBinding {
    static final double TEMPERATURE = 0.1;
    static final int MAX_TOKENS = 12000, NORMAL_ITERATIONS = 4, QUALITY_ITERATIONS = 8;

    record Settings(String key, String base, String model, int maxTokens, int normalIterations, int qualityIterations) {
        Settings(String key, String base, String model) {
            this(key, base, model, MAX_TOKENS, NORMAL_ITERATIONS, QUALITY_ITERATIONS);
        }
        Settings {
            if (maxTokens < 1024 || maxTokens > 32768 || normalIterations < 1 || normalIterations > 32
                    || qualityIterations < 1 || qualityIterations > 32) {
                throw new IllegalArgumentException("Model budgets outside reviewed limits");
            }
        }
        @Override public String toString() { return "ModelSettings[credentials redacted]"; }
        String digest() {
            // Length-prefixed UTF-8 fields; same encoding in bind_profile.py.
            // No credentials, JSON serializer differences or source paths.
            var fields=List.of("report-model/1","OpenAI",base,model,"0.1",Integer.toString(maxTokens),
                    Integer.toString(normalIterations),Integer.toString(qualityIterations));
            var bytes=new ByteArrayOutputStream();
            for(String field:fields) {
                byte[] raw=field.getBytes(StandardCharsets.UTF_8);
                bytes.writeBytes((raw.length+":").getBytes(StandardCharsets.US_ASCII));bytes.writeBytes(raw);
            }
            try { return "sha256:"+HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(bytes.toByteArray())); }
            catch(Exception e) { throw new IllegalStateException("SHA-256 unavailable",e); }
        }
        void require(String expected) {
            if(expected==null || expected.isBlank()) { throw new IllegalStateException("MODEL_BINDING_UNAVAILABLE: publish a reviewed model binding before starting this Runtime"); }
            if(!digest().equals(expected)) { throw new IllegalStateException("MODEL_BINDING_CHANGED: actual Runtime model configuration differs from the approved release"); }
        }
    }

    static Settings read(Map<String,String> env, JsonNode configuration) {
        String key=env.get("DEMO_MODEL_API_KEY"),base=env.get("DEMO_MODEL_BASE_URL"),model=env.get("DEMO_MODEL_NAME");
        if(key==null || key.isBlank() || base==null || base.isBlank() || model==null || model.isBlank()) {
            throw new IllegalStateException("Real model environment required; no fixed-output fallback");
        }
        var settings=new Settings(key,base.replaceAll("/+$",""),model,
                budget(env,"DEMO_MODEL_MAX_TOKENS",MAX_TOKENS),
                budget(env,"DEMO_MODEL_NORMAL_ITERATIONS",NORMAL_ITERATIONS),
                budget(env,"DEMO_MODEL_QUALITY_ITERATIONS",QUALITY_ITERATIONS));
        settings.require(configuration.path("model_binding_digest").asText());
        return settings;
    }

    private static int budget(Map<String,String> env, String name, int fallback) {
        String value=env.get(name);
        if(value==null) {return fallback;}
        if(!value.matches("[0-9]+")) {throw new IllegalArgumentException("Invalid budget: "+name);}
        return Integer.parseInt(value);
    }
}
