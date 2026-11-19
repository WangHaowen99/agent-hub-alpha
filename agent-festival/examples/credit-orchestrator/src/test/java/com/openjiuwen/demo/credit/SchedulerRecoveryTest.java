package com.openjiuwen.demo.credit;

import com.openjiuwen.agents.orchestration.*;
import com.openjiuwen.agents.reactrails.loopforest.LoopForestSuite;
import com.openjiuwen.agents.reactrails.loopforest.execution.SubAgentExecutor;
import com.openjiuwen.agents.reactrails.loopforest.observability.TraceForest;
import com.openjiuwen.core.runner.Runner;
import com.openjiuwen.core.singleagent.agents.ReActAgent;
import com.openjiuwen.core.singleagent.schema.AgentCard;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import java.util.List;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

import static org.junit.jupiter.api.Assertions.*;

class SchedulerRecoveryTest {
    private static ReActAgent host(String name) {
        return new ReActAgent(AgentCard.builder().name(name).build());
    }

    @Test @Timeout(15)
    void failedPeerMustDrainInstalledLoopForestBeforeSameProcessResume() throws Exception {
        CountDownLatch installed = new CountDownLatch(1);
        CountDownLatch release = new CountDownLatch(1);
        CountDownLatch cleaned = new CountDownLatch(1);
        AtomicBoolean interrupted = new AtomicBoolean();
        LoopForestSuite suite = LoopForestSuite.builder().executor((u,g)->"branch").build();
        BlueprintSpec spec = new BlueprintSpec("resume", BlueprintType.PARALLEL,
                List.of(new MemberSpec("financial", "fail"), new MemberSpec("related", "hold")),
                List.of(new StepSpec("financial","financial",List.of(),SkipPolicy.NEVER),
                        new StepSpec("related","related",List.of(),SkipPolicy.NEVER)));
        SubAgentExecutor related = (u,g) -> {
            try (var ignored = suite.install(host("old-related"))) {
                installed.countDown();
                try { release.await(); } catch (InterruptedException ex) { interrupted.set(true); }
                return "old-result";
            } finally { cleaned.countDown(); }
        };
        SubAgentExecutor financial = (u,g) -> {
            try {
                if (!installed.await(5, TimeUnit.SECONDS)) throw new AssertionError("related never installed");
            } catch (InterruptedException ex) { throw new AssertionError(ex); }
            throw new IllegalStateException("remote outcome unknown");
        };
        BlueprintScheduler scheduler = new BlueprintScheduler(new TraceForest(), 2);
        try {
            var failed = assertThrows(BlueprintException.class, () -> scheduler.execute(spec,
                    Map.of("financial",financial,"related",related), "task"));
            assertTrue(failed.getMessage().contains("remote outcome unknown"), failed.getMessage());
            assertEquals(0, installed.getCount(), "old Loop Forest must actually be installed");
            scheduler.close();
            try (var next = suite.install(host("resumed-related"))) {
                assertNotNull(Runner.resourceMgr().getTool("fork_subtask"));
            }
            assertTrue(interrupted.get(), "close must interrupt unfinished member");
            assertEquals(0, cleaned.getCount(), "close must wait for member finally/uninstall");
            assertNull(Runner.resourceMgr().getTool("fork_subtask"));
            scheduler.close(); // idempotent
        } finally {
            release.countDown();
            assertTrue(cleaned.await(5,TimeUnit.SECONDS));
            scheduler.close();
        }
    }

    @Test @Timeout(15)
    void closeDoesNotReturnWhileMemberIgnoresInterruptAndStillOwnsResources() throws Exception {
        CountDownLatch started = new CountDownLatch(1), release = new CountDownLatch(1);
        CountDownLatch cancelled = new CountDownLatch(1), closed = new CountDownLatch(1);
        AtomicBoolean interruptedOnExit = new AtomicBoolean();
        BlueprintScheduler scheduler = new BlueprintScheduler(new TraceForest(), 1);
        BlueprintSpec spec = new BlueprintSpec("drain",BlueprintType.PARALLEL,
                List.of(new MemberSpec("member","hold")),
                List.of(new StepSpec("member","member",List.of(),SkipPolicy.NEVER)));
        Thread run = new Thread(()->scheduler.execute(spec,Map.of("member",(SubAgentExecutor)(u,g)->{
            started.countDown();
            for (;;) {
                try { release.await(); break; } catch (InterruptedException ex) { cancelled.countDown(); }
            }
            return "done";
        }),"task"));
        Thread closer = new Thread(()->{scheduler.close();interruptedOnExit.set(Thread.currentThread().isInterrupted());closed.countDown();});
        try {
            run.start(); assertTrue(started.await(5,TimeUnit.SECONDS)); closer.start();
            assertTrue(cancelled.await(5,TimeUnit.SECONDS));
            closer.interrupt();
            assertEquals(1,closed.getCount(),"must not start a replacement over a live old member");
        } finally {
            release.countDown(); run.join(5000); closer.join(5000);
        }
        assertEquals(0,closed.getCount());
        assertTrue(interruptedOnExit.get(),"owner interruption must survive cleanup");
        assertFalse(run.isAlive()); assertFalse(closer.isAlive());
    }
}
