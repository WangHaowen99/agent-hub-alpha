package com.openjiuwen.demo.credit;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.openjiuwen.agents.orchestration.ActivityAwareExecutor;
import com.openjiuwen.agents.orchestration.BlueprintResult;
import com.openjiuwen.agents.orchestration.BlueprintScheduler;
import com.openjiuwen.agents.orchestration.BlueprintSpec;
import com.openjiuwen.agents.orchestration.MemberActivity;
import com.openjiuwen.agents.orchestration.MemberSpec;
import com.openjiuwen.agents.orchestration.ReActAgentMember;
import com.openjiuwen.agents.orchestration.StepOutcome;
import com.openjiuwen.agents.reactrails.loopforest.LoopForestSuite;
import com.openjiuwen.agents.reactrails.loopforest.execution.SubAgentExecutor;
import com.openjiuwen.agents.reactrails.loopforest.observability.TraceForest;
import com.openjiuwen.core.foundation.llm.Model;
import com.openjiuwen.core.foundation.llm.schema.ModelClientConfig;
import com.openjiuwen.core.foundation.llm.schema.ModelRequestConfig;
import com.openjiuwen.core.foundation.tool.mcp.McpServerConfig;
import com.openjiuwen.core.runner.Runner;
import com.openjiuwen.core.singleagent.agents.ReActAgent;
import com.openjiuwen.core.singleagent.agents.ReActAgentConfig;
import com.openjiuwen.core.singleagent.schema.AgentCard;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * Leases approved runs from credit-demo and drives them with {@link BlueprintScheduler}. Every member is
 * wrapped so a step that already has a recorded result is reused (no model call), which is what makes
 * "restart from the beginning" safe after a crash. Effects go to the Go gateway, never out from here.
 */
final class Worker implements Runnable {
    private static final Logger LOG = LoggerFactory.getLogger(Worker.class);
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final String HOST_SYSTEM = "你是关联方排查员，用 Loop Forest 方式工作：先调用 get_related_parties 拿到本案件的关联方名单，"
            + "再用 fork_subtask 为每一家关联企业分叉一个子任务（子目标里写明 company_id 和要查什么），"
            + "最后汇总各分支结论：哪些关联方有逾期或担保风险、对申请人授信的影响。只依据工具和分支返回作答。";
    private static final String BRANCH_SYSTEM = "你在执行关联方排查里的一个分支子任务，只负责这一家关联企业。"
            + "调用 query_company_bureau（参数 company_id）查它的征信摘要，给出一句话结论：有无逾期、负债与担保情况。";
    private static final String MEMBER_SYSTEM = "你是授信尽调团队的一名成员。按[角色]和任务说明完成你的部分，"
            + "需要数据时调用提供给你的工具，只依据工具返回和上游产物作答，不要编造。结尾用一两句话给出你的结论。";

    private final GoClient go;
    private final String worker;
    private final Map<String, String> env;

    Worker(GoClient go, String worker, Map<String, String> env) {
        this.go = go;
        this.worker = worker;
        this.env = env;
    }

    @Override
    public void run() {
        while (!Thread.currentThread().isInterrupted()) {
            try {
                JsonNode job = go.lease(worker);
                if (job == null) {
                    continue;
                }
                execute(job);
            } catch (RuntimeException ex) {
                LOG.warn("lease loop: {}", ex.getMessage());
                sleep(2000);
            }
        }
    }

    private void execute(JsonNode job) {
        String run = job.path("run_id").asText();
        String token = job.path("token").asText();
        LOG.info("run {} leased (epoch {})", run, job.path("epoch").asInt());
        AtomicBoolean fenced = new AtomicBoolean();
        ScheduledExecutorService beat = Executors.newSingleThreadScheduledExecutor();
        int period = Math.max(3, job.path("lease_seconds").asInt(30) / 3);
        beat.scheduleAtFixedRate(() -> {
            try {
                go.heartbeat(run, token);
            } catch (GoClient.FencedException ex) {
                fenced.set(true);
            } catch (RuntimeException ex) {
                LOG.warn("heartbeat {}: {}", run, ex.getMessage());
            }
        }, period, period, TimeUnit.SECONDS);
        List<McpServerConfig> mounts = new ArrayList<>();
        try {
            Object team = JSON.convertValue(job.path("team"), List.class);
            BlueprintSpec spec = Blueprints.compile(job.path("mode").asText("graph"), team);
            String digest = Blueprints.digest(spec);
            if (!digest.equals(job.path("spec_digest").asText())) {
                go.done(run, token, List.of(), null, "blueprint digest mismatch: approved " + job.path("spec_digest").asText()
                        + ", recompiled " + digest);
                return;
            }
            Map<String, SubAgentExecutor> bindings = new LinkedHashMap<>();
            for (MemberSpec m : spec.members()) {
                JsonNode mj = job.path("members").path(m.id());
                SubAgentExecutor inner = member(job, run, token, m.id(), mj, mounts);
                bindings.put(m.id(), new Ledgered(run, token, m.id(), inner, fenced));
            }
            BlueprintResult result;
            try (BlueprintScheduler scheduler = new BlueprintScheduler(new TraceForest(), 3)) {
                result = scheduler.execute(spec, bindings, job.path("user_input").asText());
            }
            List<String> skipped = new ArrayList<>();
            for (StepOutcome o : result.stepOutcomes()) {
                if (o.skipped()) {
                    skipped.add(o.stepId());
                }
            }
            go.done(run, token, skipped, result.finalOutputText().orElse(""), null);
            LOG.info("run {} done (skipped {})", run, skipped);
        } catch (GoClient.FencedException ex) {
            LOG.warn("run {}: lease taken over, stopping without writing ({})", run, ex.getMessage());
        } catch (Exception ex) {
            if (fenced.get() || ex.getMessage() != null && ex.getMessage().contains("fenced")) {
                LOG.warn("run {}: fenced, stopping", run);
                return;
            }
            LOG.error("run {} failed", run, ex);
            try {
                go.done(run, token, List.of(), null, String.valueOf(ex.getMessage()));
            } catch (RuntimeException ignored) {
                LOG.warn("run {}: could not report failure", run);
            }
        } finally {
            beat.shutdownNow();
            for (McpServerConfig c : mounts) {
                try {
                    Runner.resourceMgr().removeMcpServer(List.of(c.getServerId()), null, null, null, true);
                } catch (RuntimeException ex) {
                    LOG.debug("unmount {}: {}", c.getServerId(), ex.getMessage());
                }
            }
        }
    }

    private SubAgentExecutor member(JsonNode job, String run, String token, String step, JsonNode mj,
            List<McpServerConfig> mounts) {
        String kind = mj.path("kind").asText();
        switch (kind) {
            case "effect":
                return (userInput, subGoal) -> go.effect(run, token, step);
            case "blackbox": {
                String expert = mj.path("expert").asText();
                return (userInput, subGoal) -> {
                    ReActAgent host = agent("lf-host-" + step, HOST_SYSTEM, 10);
                    McpServerConfig hostTools = mount(job, run, token, step, expert, "host");
                    if (hostTools != null) {
                        mounts.add(hostTools);
                        host.getAbilityManager().add(hostTools);
                    }
                    SubAgentExecutor branches = (u, goal) -> {
                        go.branch(run, token, step, goal, "started", 0);
                        ReActAgent branch = agent("lf-branch-" + step, BRANCH_SYSTEM, 4);
                        McpServerConfig branchTools = mount(job, run, token, step, expert, "b" + System.nanoTime());
                        if (branchTools != null) {
                            mounts.add(branchTools);
                            branch.getAbilityManager().add(branchTools);
                        }
                        String out = new ReActAgentMember(branch).execute(u, "子任务：" + goal);
                        go.branch(run, token, step, goal, "finished", out == null ? 0 : out.length());
                        return out;
                    };
                    try (LoopForestSuite.Installation ignored = LoopForestSuite.builder()
                            .forkEnabled(true)
                            .executor(branches)
                            .forkDescription("为一家关联企业分叉一个排查子任务；sub_goal 写明 company_id 和要查的内容")
                            .build()
                            .install(host)) {
                        return new ReActAgentMember(host).execute(userInput, subGoal);
                    }
                };
            }
            case "judge": {
                String prompt = mj.path("judge_prompt").asText();
                return (userInput, subGoal) -> {
                    ReActAgentMember judge = new ReActAgentMember(agent("judge-" + step, prompt, 3));
                    String raw = judge.execute(userInput, subGoal);
                    try {
                        return go.judgment(run, token, step, raw);
                    } catch (GoClient.FencedException ex) {
                        throw ex;
                    } catch (RuntimeException first) {
                        String retry = judge.execute(userInput, subGoal + "\n\n上一次输出被规约校验拒绝：" + first.getMessage()
                                + "\n请严格按要求只输出一个 JSON 对象。");
                        return go.judgment(run, token, step, retry);
                    }
                };
            }
            default: {
                ReActAgent agent = agent(kind + "-" + step, MEMBER_SYSTEM, 6);
                if (mj.path("tools").size() > 0) {
                    McpServerConfig config = mount(job, run, token, step, mj.path("expert").asText(), "m");
                    if (config != null) {
                        mounts.add(config);
                        var added = agent.getAbilityManager().add(config);
                        if (!added.isAdded()) {
                            LOG.warn("tools of {} not visible: {}", step, added.getReason());
                        }
                    }
                }
                return new ReActAgentMember(agent);
            }
        }
    }

    private McpServerConfig mount(JsonNode job, String run, String token, String step, String expert, String tag) {
        Map<String, String> headers = new LinkedHashMap<>();
        headers.put("X-Run", run);
        headers.put("X-Step", step);
        headers.put("X-Token", token);
        McpServerConfig config = McpServerConfig.builder()
                .serverId("credit-data@" + run + "-" + step + "-" + job.path("epoch").asInt() + "-" + tag)
                .serverName("credit-data-" + step)
                .serverPath(job.path("member_mcp_url").asText() + "/" + expert)
                .clientType("streamable-http")
                .params(new LinkedHashMap<>())
                .build();
        config.setAuthHeaders(headers);
        config.setCallTimeoutSeconds(30.0);
        var results = Runner.resourceMgr().addMcpServer(config, "credit-orchestrator", null);
        if (results.stream().anyMatch(r -> r.isError())) {
            LOG.warn("member MCP for {} unavailable", step);
            return null;
        }
        return config;
    }

    private ReActAgent agent(String name, String systemPrompt, int maxIterations) {
        ReActAgent agent = new ReActAgent(AgentCard.builder().name(name).build());
        ReActAgentConfig config = ReActAgentConfig.builder().build();
        config.configureMaxIterations(maxIterations);
        config.configureContextEngine(200, 60, false);
        config.configurePromptTemplate(List.of(Map.of("role", "system", "content", systemPrompt)));
        agent.configure(config);
        ModelClientConfig client = ModelClientConfig.builder()
                .clientId(name + "-" + System.nanoTime())
                .clientProvider("OpenAI")
                .apiKey(first("MODEL_API_KEY", "SWARM_API_KEY"))
                .apiBase(first("MODEL_API_BASE", "SWARM_API_BASE"))
                .verifySsl(true)
                .timeout(120.0)
                .build();
        ModelRequestConfig request = ModelRequestConfig.builder()
                .modelName(first("MODEL_NAME", "SWARM_MODEL_NAME"))
                .temperature(0.1)
                .topP(0.95)
                .maxTokens(2000)
                .build();
        agent.setLlm(new Model(client, request));
        return agent;
    }

    private String first(String... keys) {
        for (String k : keys) {
            String v = env.get(k);
            if (v != null && !v.isBlank()) {
                return v;
            }
        }
        return keys[0].endsWith("NAME") ? "qwen-plus" : "";
    }

    private static void sleep(long ms) {
        try {
            Thread.sleep(ms);
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
        }
    }

    /** Ledger-first member: reuse the recorded step result, otherwise run and record it. */
    private final class Ledgered implements ActivityAwareExecutor {
        private final String run;
        private final String token;
        private final String step;
        private final SubAgentExecutor inner;
        private final AtomicBoolean fenced;

        Ledgered(String run, String token, String step, SubAgentExecutor inner, AtomicBoolean fenced) {
            this.run = run;
            this.token = token;
            this.step = step;
            this.inner = inner;
            this.fenced = fenced;
        }

        @Override
        public String execute(String userInput, String subGoal, MemberActivity activity) {
            if (fenced.get()) {
                throw new GoClient.FencedException("lease lost before step " + step);
            }
            JsonNode rec = go.stepGet(run, token, step);
            if (rec.path("recorded").asBoolean()) {
                LOG.info("run {} step {}: reused recorded result", run, step);
                return rec.path("output").asText();
            }
            LOG.info("run {} step {}: executing", run, step);
            String out = inner instanceof ActivityAwareExecutor aware
                    ? aware.execute(userInput, subGoal, activity) : inner.execute(userInput, subGoal);
            // The scheduler joins upstream outputs without step ids; tag ours so a downstream
            // judgment can cite which step its evidence came from.
            String tagged = "[step:" + step + "]\n" + (out == null ? "" : out);
            go.stepPut(run, token, step, tagged);
            return tagged;
        }
    }
}
