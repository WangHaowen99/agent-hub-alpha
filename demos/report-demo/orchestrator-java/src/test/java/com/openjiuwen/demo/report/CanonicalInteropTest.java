package com.openjiuwen.demo.report;

import static org.junit.jupiter.api.Assertions.*;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Assumptions;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class CanonicalInteropTest {
    @TempDir Path temp;

    @Test void javaSerializationPreservesAuthoritativeGoCanonicalIdentity() throws Exception {
        String binary=System.getenv("REPORT_CANONICAL_BINARY");
        Assumptions.assumeTrue(binary!=null&&!binary.isBlank(),"Build actual offline Go CLI for the cross-language contract gate");
        var suite=Json.read(Files.readString(Path.of("../contracts/canonical-vectors.json"),StandardCharsets.UTF_8));
        assertTrue(suite.path("vectors").size()>=7);
        for(var vector:suite.path("vectors")) {
            // The actual Java JSON boundary parses and writes; it must preserve
            // Go's semantic identity, not recreate Go's JCS in application code.
            var value=Json.read(vector.path("input").asText());
            Path input=temp.resolve(vector.path("id").asText()+".json");
            Files.writeString(input,Json.write(value),StandardCharsets.UTF_8);
            Process process=new ProcessBuilder(binary,"-config",temp.resolve("no-config.json").toString(),"-canonical-json",input.toString()).start();
            assertTrue(process.waitFor(10,TimeUnit.SECONDS),"offline canonical CLI timeout");
            assertEquals(0,process.exitValue(),vector.path("id").asText());
            var actual=Json.read(new String(process.getInputStream().readAllBytes(),StandardCharsets.UTF_8));
            assertEquals(vector.path("canonical").asText(),actual.path("canonical").asText(),vector.path("id").asText());
            assertEquals(vector.path("digest").asText(),actual.path("digest").asText(),vector.path("id").asText());
            assertEquals(0,actual.path("database_calls").asInt());
        }
    }
}
