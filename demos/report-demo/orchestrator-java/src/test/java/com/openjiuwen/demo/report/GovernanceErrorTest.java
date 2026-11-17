package com.openjiuwen.demo.report;

import static org.junit.jupiter.api.Assertions.*;
import org.junit.jupiter.api.Test;

class GovernanceErrorTest {
    @Test void knownIdentityFailuresRemainActionableWithoutCopyingMessages() {
        for (String code : new String[]{"AUTH_MISSING", "AUTH_INVALID", "AUTH_EXPIRED"}) {
            var body = Json.MAPPER.createObjectNode().put("code", code).put("message", "private provider diagnostic");
            assertEquals(code, RemoteExpertExecutor.remoteErrorCode(401, body));
            assertEquals("WAITING_AUTH", BoundNodeExecutor.waitingStatus(code));
        }
    }

    @Test void scopeDenialsBlockWhileDependencyFailuresRemainUnknown() {
        assertEquals("CALLER_NOT_AUTHORIZED", RemoteExpertExecutor.remoteErrorCode(403,
                Json.read("{\"error\":{\"code\":\"CALLER_NOT_AUTHORIZED\"}}")));
        for (String code : new String[]{"CALLER_NOT_AUTHORIZED", "SCOPE_VIOLATION", "AUTH_DISABLED"}) {
            assertEquals("BLOCKED", BoundNodeExecutor.waitingStatus(code));
        }
        for (String code : new String[]{"RDC_UNAVAILABLE", "IAM_UNAVAILABLE", "AUDIT_UNAVAILABLE"}) {
            assertEquals(code, RemoteExpertExecutor.remoteErrorCode(503,
                    Json.MAPPER.createObjectNode().put("code", code)));
            assertEquals("UNKNOWN", BoundNodeExecutor.waitingStatus(code));
        }
    }

    @Test void numericRpcErrorsAndUnknownFreeTextCannotSelectControlStates() {
        assertEquals("RDC_UNAVAILABLE", RemoteExpertExecutor.remoteErrorCode(200,
                Json.read("{\"error\":{\"code\":-32000,\"data\":{\"code\":\"RDC_UNAVAILABLE\"}}}")));
        assertEquals("REMOTE_UNKNOWN", RemoteExpertExecutor.remoteErrorCode(502,
                Json.read("{\"code\":\"COMPLETED\",\"message\":\"do not expose\"}")));
        assertEquals("AUTH_REQUIRED", RemoteExpertExecutor.remoteErrorCode(403, Json.read("{}")));
        assertEquals("REMOTE_UNKNOWN", RemoteExpertExecutor.remoteErrorCode(503, Json.read("{}")));
    }
}
