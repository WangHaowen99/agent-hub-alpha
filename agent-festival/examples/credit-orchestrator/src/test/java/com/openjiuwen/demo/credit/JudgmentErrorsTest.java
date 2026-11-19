package com.openjiuwen.demo.credit;

import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.Test;

import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

class JudgmentErrorsTest {
    private void exercise(String code, int status, Class<? extends RuntimeException> expected,
            boolean alwaysReject, int expectedCalls) throws Exception {
        AtomicInteger requests = new AtomicInteger(), modelCalls = new AtomicInteger();
        HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1",0),0);
        server.createContext("/exec/judgment", exchange -> {
            exchange.getRequestBody().readAllBytes();
            boolean reject = requests.incrementAndGet()==1 || alwaysReject;
            byte[] body = (reject ? "{\"error\":\"failure\",\"code\":\""+code+"\"}" : "{\"output\":\"accepted\"}")
                    .getBytes(StandardCharsets.UTF_8);
            exchange.sendResponseHeaders(reject?status:200,body.length);
            exchange.getResponseBody().write(body); exchange.close();
        });
        server.start();
        try {
            GoClient go = new GoClient("http://127.0.0.1:"+server.getAddress().getPort());
            org.junit.jupiter.api.function.Executable invoke = () -> {
                String result = Worker.judgeWithRevision(go,"r","token","judge",(u,g)->{
                    modelCalls.incrementAndGet(); return "model answer";
                },"input","goal");
                assertEquals("accepted",result);
            };
            if(expected==null) assertDoesNotThrow(invoke); else assertThrows(expected,invoke);
            assertEquals(expectedCalls,modelCalls.get(),"model calls for "+code);
            assertEquals(expectedCalls,requests.get(),"submission calls for "+code);
        } finally {server.stop(0);}
    }

    @Test void onlyContentRejectionCanRevise() throws Exception {
        exercise("POLICY_REJECTED",422,null,false,2);
        exercise("POLICY_REJECTED",422,GoClient.PolicyRejectedException.class,true,2);
        exercise("STORAGE_UNAVAILABLE",503,GoClient.StorageUnavailableException.class,true,1);
        exercise("INPUT_CONFLICT",409,GoClient.InputConflictException.class,true,1);
        exercise("FENCED",409,GoClient.FencedException.class,true,1);
        exercise("MODEL_BINDING_MISMATCH",409,GoClient.BindingRejectedException.class,true,1);
        exercise("CONTROL_REJECTED",409,GoClient.ControlRejectedException.class,true,1);
        exercise("INVALID_REQUEST",400,IllegalStateException.class,true,1);
    }

    @Test void transportFailureDoesNotRevise() throws Exception {
        HttpServer server=HttpServer.create(new InetSocketAddress("127.0.0.1",0),0);
        server.start(); int port=server.getAddress().getPort();server.stop(0);
        AtomicInteger calls=new AtomicInteger();
        assertThrows(IllegalStateException.class,()->Worker.judgeWithRevision(new GoClient("http://127.0.0.1:"+port),
                "r","t","judge",(u,g)->{calls.incrementAndGet();return "answer";},"input","goal"));
        assertEquals(1,calls.get());
    }
}
