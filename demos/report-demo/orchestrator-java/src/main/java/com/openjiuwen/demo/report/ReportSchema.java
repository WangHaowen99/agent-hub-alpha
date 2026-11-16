package com.openjiuwen.demo.report;

import com.fasterxml.jackson.databind.JsonNode;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;

/** Validator for every assertion keyword used by the fixed W01 report schema.
 * This deliberately rejects unknown assertion keywords at load time; it is not
 * advertised as a general JSON Schema implementation.
 */
final class ReportSchema {
    private static final JsonNode ROOT = load();
    private ReportSchema() { }

    private static JsonNode load() {
        try (var resource = ReportSchema.class.getResourceAsStream("/report.schema.json")) {
            if (resource == null) { throw new IllegalStateException("Fixed report schema missing from JAR"); }
            JsonNode schema = Json.read(new String(resource.readAllBytes(), StandardCharsets.UTF_8));
            verifyKeywords(schema);
            return schema;
        } catch (Exception e) { throw new ExceptionInInitializerError(e); }
    }

    private static void verifyKeywords(JsonNode schema) {
        var allowed = java.util.Set.of("$schema", "$id", "$ref", "$defs", "title", "description", "type", "const", "enum",
                "required", "additionalProperties", "properties", "items", "oneOf", "minLength", "minItems", "minimum", "maximum");
        schema.fieldNames().forEachRemaining(key -> {
            if (!allowed.contains(key)) { throw new IllegalStateException("Unsupported schema assertion: " + key); }
        });
        for (String key : List.of("properties", "$defs")) { schema.path(key).forEach(ReportSchema::verifyKeywords); }
        if (schema.has("items")) { verifyKeywords(schema.path("items")); }
        schema.path("oneOf").forEach(ReportSchema::verifyKeywords);
    }

    static List<String> validate(JsonNode value) {
        List<String> errors = new ArrayList<>(); validate(ROOT, value, "$", errors); return errors;
    }

    private static void validate(JsonNode schema, JsonNode value, String path, List<String> errors) {
        if (schema.has("$ref")) {
            String ref = schema.path("$ref").asText();
            if (!ref.startsWith("#/")) { throw new IllegalStateException("Only fixed local schema references supported"); }
            validate(ROOT.at(ref.substring(1)), value, path, errors); return;
        }
        if (schema.has("oneOf")) {
            int matches = 0;
            for (JsonNode branch : schema.path("oneOf")) {
                List<String> branchErrors = new ArrayList<>(); validate(branch, value, path, branchErrors);
                if (branchErrors.isEmpty()) { matches++; }
            }
            if (matches != 1) { errors.add(path + ": oneOf must match exactly one schema"); }
        }
        if (schema.has("const") && !schema.path("const").equals(value)) { errors.add(path + ": const mismatch"); }
        if (schema.has("enum")) {
            boolean found = false;
            for (JsonNode option : schema.path("enum")) { if (option.equals(value)) { found = true; } }
            if (!found) { errors.add(path + ": enum mismatch"); }
        }
        if (schema.has("type")) {
            JsonNode type = schema.path("type");
            List<String> types = new ArrayList<>();
            if (type.isArray()) { type.forEach(t -> types.add(t.asText())); } else { types.add(type.asText()); }
            boolean good = types.stream().anyMatch(t -> switch (t) {
                case "object" -> value.isObject(); case "array" -> value.isArray(); case "string" -> value.isTextual();
                case "number" -> value.isNumber(); case "integer" -> value.isNumber() && value.decimalValue().stripTrailingZeros().scale() <= 0;
                case "null" -> value.isNull(); default -> throw new IllegalStateException("Unsupported schema type");
            });
            if (!good) { errors.add(path + ": type mismatch"); return; }
        }
        if (value.isObject()) {
            for (JsonNode required : schema.path("required")) { if (!value.has(required.asText())) { errors.add(path + ": missing " + required.asText()); } }
            value.fields().forEachRemaining(entry -> {
                JsonNode property = schema.path("properties").get(entry.getKey());
                if (property != null) { validate(property, entry.getValue(), path + "." + entry.getKey(), errors); }
                else if (schema.has("additionalProperties") && !schema.path("additionalProperties").asBoolean()) { errors.add(path + ": unknown " + entry.getKey()); }
            });
        }
        if (value.isArray()) {
            if (value.size() < schema.path("minItems").asInt(0)) { errors.add(path + ": too few items"); }
            for (int i = 0; i < value.size(); i++) { if (schema.has("items")) { validate(schema.path("items"), value.get(i), path + "[" + i + "]", errors); } }
        }
        if (value.isTextual() && value.asText().codePointCount(0, value.asText().length()) < schema.path("minLength").asInt(0)) { errors.add(path + ": string too short"); }
        if (value.isNumber()) {
            if (schema.has("minimum") && value.decimalValue().compareTo(schema.path("minimum").decimalValue()) < 0) { errors.add(path + ": below minimum"); }
            if (schema.has("maximum") && value.decimalValue().compareTo(schema.path("maximum").decimalValue()) > 0) { errors.add(path + ": above maximum"); }
        }
    }
}
