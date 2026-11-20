# W02 在 e1961fe8 上的 R-A：插件页立即恢复

源码 `e1961fe8dcfc1c859b515854f793f24948d844fe`，W02 环境（compose 项目 pr30-w02），模型 qwen-plus（DashScope OpenAI 兼容接口）。操作者 WangHaowen99，恢复在真实 WorkSwarm 插件页提交（Playwright 驱动，`scripts/raimm.py`）。

## 版本与镜像

- 按 `demo.py --bus build all` 在 e1961fe8 上构建，摘要见 `logs/build-summary.txt`，镜像 ID 见 `images.txt`。
- 编排器镜像 `sha256:099f844293e2…` 内 `credit-orchestrator.jar` 的读回（`sdk-readback.txt`）：`agent-core-ext-orchestration` 的 pom.properties 为 `version=0.1.3-aha-drain1`；`BlueprintScheduler.class` 含 `shutdownNow` 和 `awaitTermination`（drain 补丁）。
- workswarm 镜像本次重建得到新 ID（源码仍为 1142e5fb3）；rdc、gateway 沿用已有 bf702bc 标签镜像。

## R-A（任务 R14，`ra-plugin-immediate-R14.json`）

| 时刻（UTC） | 事件 | 来源 |
|---|---|---|
| 00:15:44.167 | 第 1 次领取（epoch 1） | `logs/orchestrator-R14.log` L1 |
| 00:15:54.238 | related（Loop Forest）开始执行；54.323 发出模型调用 | L2–3 |
| 00:15:54.442 | 注入：故障代理丢弃下一次 GetTask 应答；此时 intake、industry、bureau 已完成，financial 远端在途，related 正在「拆分子目标」 | JSON `armed` |
| 00:15:55.196 | related 的模型调用被取消（`CancellationException: model call interrupted`） | L4–5 |
| 00:15:55.203 | run R14 failed：financial 远端结果未决，原 task 保留 | L35–36 |
| 00:15:55.240 | 任务进入 waiting（financial=unknown，related 仍显示 running） | JSON `steps_at_waiting` |
| 00:15:56.002 | 在插件页提交恢复，距检测到 waiting 762 ms | JSON `waiting_to_submit_ms` |
| 00:15:56.255 | 第 2 次领取（epoch 2）；56.349 related 重新执行 | L45–46 |
| 00:16:00.19–.23 | related 三个分叉的模型调用 | L48–50 |
| 00:16:23.530 | run R14 done | L53 |

- **同一编排器进程**：恢复前、waiting 时、完成后三次记录的容器启动时间、重启次数 0、宿主 PID、PID 1 启动 tick 都相同（`identity_*`）。
- **旧成员先退出**：模型调用取消（55.196）早于 run failed（55.203），新领取在 56.255。
- **线程转储**：编排器运行镜像里没有 jcmd/jstack，没有取到（`logs/thread-dump-attempt.txt`）。
- **无冲突**：日志里没有 `fork_subtask`、`registration`、`invalid Gateway observation` 或 `time_order`；第 2 次执行时三个分叉正常发出模型调用。
- **身份不变**：intake、industry、bureau、financial 的 request_id、remote_task_id 前后相同。
- **ΔM=0**：已完成的 intake、industry、bureau 的 request、产物、model_events（各 2 条）前后相同。
- **发送次数**：故障代理原始事件 36 条，5 个 request 各 1 次 SendMessage；GetTask 200 ×30，唯一的 503 是注入。
- **一次恢复即续跑**：只从插件恢复了 1 次，任务 done。

## 升级时的旧任务

- 升级前 13 个任务（R1–R13）全部 done（`state-before-upgrade.txt`）。
- e1961fe8 启动对账后全部变为 blocked，`final_output` 都是「RECONCILIATION_REQUIRED: 采纳账本与快照操作序列不一致；保留原账本与快照，需人工核对，不能重新生成决策。」（`state-after-upgrade.txt`、`old-tasks-blocked.json`）。
- 插件里这些任务显示「任务受阻」，没有恢复表单，横幅为通用维护提示（`shots/e196-old-task-blocked.png`）；具体原因没有投影到插件。

## 截图

- `e196-1-1-waiting-submitted.png`：waiting 时提交恢复（这套环境的插件仍是 e1961fe 版本，related 还在转圈；#59 修复的就是这个显示）。
- `e196-1-2-after-resume.png`、`e196-1-3-final.png`：恢复后与完成时。

## 未覆盖

- 成员不响应中断时 `close()` 会一直等待并阻塞宿主，这个边界没有测，归 W03/SDK。
- 线程转储（镜像无工具）。
