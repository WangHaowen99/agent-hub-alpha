package com.openjiuwen.demo.report;

import static org.junit.jupiter.api.Assertions.*;
import org.junit.jupiter.api.Test;

class AdmittedGraphCompilerTest {
    private static final String GRAPH = """
            {"goal":"report","nodes":[
              {"id":"extract","role":"extract","task":"read source","expert":"extractor@1","depends_on":[]},
              {"id":"business","role":"business","task":"analyze","expert":"business@1","depends_on":["extract"]},
              {"id":"draft","role":"writer","task":"write","expert":"writer@1","depends_on":["business"]}
            ]}
            """;
    @Test void nativeGraphPreservesDependenciesAndDigest() {
        var compiler = new AdmittedGraphCompiler();
        var compiled = compiler.compile(Json.read(GRAPH));
        assertEquals(compiled.digest(), compiler.compile(Json.read(GRAPH)).digest());
        assertEquals("extract", compiled.spec().steps().get(1).dependsOn().getFirst());
        assertNotEquals(compiled.digest(), compiler.compile(Json.read(GRAPH.replace("analyze", "changed instruction"))).digest());
    }
    @Test void cyclesAndDuplicateJsonKeysAreRejected() {
        assertThrows(RuntimeException.class, () -> new AdmittedGraphCompiler().compile(Json.read(
                GRAPH.replace("\"depends_on\":[]", "\"depends_on\":[\"draft\"]"))));
        assertThrows(RuntimeException.class, () -> Json.read("{\"x\":1,\"x\":2}"));
    }
}
