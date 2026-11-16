package com.openjiuwen.demo.report;

import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.HexFormat;

final class Json {
    static final ObjectMapper MAPPER = new ObjectMapper()
            .enable(JsonParser.Feature.STRICT_DUPLICATE_DETECTION)
            .enable(DeserializationFeature.FAIL_ON_TRAILING_TOKENS)
            .enable(DeserializationFeature.USE_BIG_DECIMAL_FOR_FLOATS);

    private Json() { }

    static JsonNode read(String input) {
        try { return MAPPER.readTree(input); }
        catch (Exception e) { throw new IllegalArgumentException("Invalid strict JSON", e); }
    }

    static String write(Object input) {
        try { return MAPPER.writeValueAsString(input); }
        catch (Exception e) { throw new IllegalArgumentException("Cannot encode JSON", e); }
    }

    static String digest(Object input) {
        try { return "sha256:" + HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256")
                .digest(write(input).getBytes(StandardCharsets.UTF_8))); }
        catch (Exception e) { throw new IllegalStateException(e); }
    }
}
