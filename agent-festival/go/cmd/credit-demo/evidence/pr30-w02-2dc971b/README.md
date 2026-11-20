# W02 在 2dc971b1 上的 R-A：插件页立即恢复

源码 `2dc971b1fa9d592bb6d61b7f61c398856cae056f`（#30 组合候选，含 #46、#33），W02 环境（compose 项目 pr30-w02，从 e1961fe8 原地升级），模型 qwen-plus（DashScope OpenAI 兼容接口）。操作者 WangHaowen99，恢复在真实 WorkSwarm 插件页提交（Playwright 驱动，`scripts/raimm2dc.py`），插件为 2dc971b1 自带版本。

## 版本与镜像

- 按 `demo.py --bus build all` 在 2dc971b1 上构建，Go 测试和 Maven 构建通过，摘要见 `logs/build-summary.txt`。升级前后的镜像 ID 见 `images-before.txt`、`images.txt`。
  - credit `sha256:02167ea62e8e…`、orchestrator `sha256:1e8e5d7e9ea1…` 为本次新构建；
  - professional、workswarm、rdc、gateway 的创建时间与上次相同（构建命中缓存，源码未变），镜像 ID 与上次记录不同，未逐层比对。
- 场景包：credit 镜像按 2dc971b1 的 Dockerfile 带入 `deploy/scenarios/`，compose 设置 `CREDIT_SCENARIO_DIR`；授信尽调场景 `credit-dd` 随程序内置，无选择策略，`propose_plan` 用法不变。
- 编排器运行镜像读回（`sdk-readback.txt`）：`agent-core-ext-orchestration` 的 pom.properties 为 `version=0.1.3-aha-drain1`；`BlueprintScheduler.class` 含 `shutdownNow` 和 `awaitTermination`（drain 补丁）；`RemoteAgentMember.class` 含 `nanoTime`（#46 单调时钟）。
- 编排器按 `compose.bus-test.yaml` 经 fault-proxy 访问 Gateway（`scripts/up_faultproxy.py`），与之前的 R-A 相同。

## 升级时的旧任务（`state-before-upgrade.txt`、`state-after-upgrade.txt`）

| 任务 | 升级前（e1961fe8） | 升级后（2dc971b1） |
|---|---|---|
| R1–R13 | blocked，`final_output` 为 RECONCILIATION_REQUIRED（e1961fe8 首次升级时写入） | 仍为 blocked，`final_output` 未变 |
| R14（e1961fe8 上的 R-A） | done | 仍为 done，`final_output` 未变 |

- 状态版本前后都是 1059：本次启动没有写入任何任务。
- R1–R13 当初完成时的报告，已在 e1961fe8 首次升级时被阻断原因覆盖，这套环境里已经读不回原报告。这与 W05 旧数据副本的现象一致，按 owner 口径归入对账异常单列，本次只记录、不处理。

## R-A（任务 R19，`ra-plugin-immediate-R19.json`）

| 时刻（UTC） | 事件 | 来源 |
|---|---|---|
| 06:03:48.008 | 第 1 次领取（epoch 1） | `logs/orchestrator-R19.log` L1 |
| 06:03:55.719 | related（Loop Forest）开始执行，55.841 建立模型调用 | L2–3 |
| 06:03:55.962 | 注入：故障代理丢弃下一次 GetTask 应答；此时 intake、industry、bureau 已完成，financial 远端在途，related 正在「拆分子目标」 | JSON `armed` |
| 06:03:56.576 | financial 的 GetTask 返回 503（注入） | JSON `transport_events` |
| 06:03:56.603 | related 的模型调用被取消（`CancellationException: model call interrupted`） | L4–5 |
| 06:03:56.608 | run R19 failed：financial 远端结果未决，原 task 保留 | L35–36 |
| 06:03:56.684 | 任务进入 waiting（financial=unknown，related 仍记为 running） | JSON `steps_at_waiting` |
| 06:03:57.223 | 在插件页提交恢复，距检测到 waiting 539 ms | JSON `waiting_to_submit_ms` |
| 06:03:57.667 | 第 2 次领取（epoch 2）；57.795 related 重新执行 | L45–46 |
| 06:04:02.465–.515 | related 三个分叉的模型调用 | L48–50 |
| 06:04:25.121 | run R19 done | L53 |

- **同一编排器进程**：恢复前、waiting 时、完成后三次记录的容器启动时间、重启次数 0、宿主 PID、PID 1 启动 tick 都相同（`identity_*`）。
- **旧成员先退出**：模型调用取消（56.603）早于 run failed（56.608），新领取在 57.667。
- **线程转储**：编排器运行镜像里没有 jcmd/jstack，没有取到（`logs/thread-dump-attempt.txt`）。
- **无冲突**：日志里没有 `fork_subtask`、`registration`、`invalid Gateway observation` 或 `time_order`；第 2 次执行时三个分叉正常发出模型调用。
- **身份不变**：intake、industry、bureau、financial 的 request_id、remote_task_id 前后相同（`kept_request_and_task`）。
- **ΔM=0**：已完成的 intake、industry、bureau 的 request、产物、model_events（各 2 条）前后相同（`completed_unchanged`）。
- **发送次数**：故障代理原始事件 31 条，5 个 request 各 1 次 SendMessage；GetTask 200 ×25，唯一的 503 是注入。
- **Gateway 记录**（`gateway-R19.json`，取自 Go 保存的 Java 观察）：每个 request 各 1 条 `SendMessage/task_received/200`；financial 另有 1 条 `GetTask/http_error/503`；没有结束时间早于开始时间的记录。
- **一次恢复即续跑**：只从插件恢复了 1 次，任务 done。

## 过程中的两处偏差

1. **R15–R17 没有注入成功**（`logs/setup-miss-R15-R17.out`）：升级时按 `demo.py --bus start` 重建了编排器，没带 `compose.bus-test.yaml`，Gateway 流量绕过了 fault-proxy，故障一次都没触发，三个任务都正常完成。补上叠加文件重建编排器后再跑。
2. **R18 未进入 R-A 条件**（`ra-attempt-R18-not-armed.json`）：上一步遗留在 fault-proxy 里的「丢弃一次」在 R18 的第一次 GetTask 上生效，intake 变为 unknown，任务在 related 开始前就进入 waiting。脚本判定条件不满足，转入第 2 次尝试（R19）。R18 之后用作「结果未知」的显示样本，最后经接口恢复，任务 done，请求身份不变，5 个 request 各发送 1 次（`cleanup-resume-R18-R20.json`）。

## #59 插件在真实环境的显示（`w02-59-real-R20.json`）

把 #59 的插件构建（`2c022ea2`）临时换进这套环境，截完后已还原为 2dc971b1 自带版本。

- 做法：两个以上 Agent Bus 调用同时在途时注入「丢弃一次 GetTask」，任务为 R20。
- 结果：丢弃的是 industry 的查询（06:07:50.962）。之后 Java 继续查询 financial、bureau，直到两者完成（06:07:55.199），run 才在 06:07:55.247 失败。任务停住时：
  - industry：结果未知 · 最后观察 14:07:50；
  - related：已停止 · 最后观察 14:07:55，不再转圈；
  - financial、bureau 已完成。

  旧任务 R18 的 intake 同样显示「结果未知 · 最后观察 14:03:46」。
- 刷新前后每个节点的显示文字一致（`display_same_after_reload`）。
- 查看和刷新期间，故障代理新增事件 0 条，R20 和 R18 在 Go 中的记录前后完全相同。
- **没有复现出的两档**：
  - 「远端在途」：按上面的时序，同批的远端调用会先跑完，任务才停住，所以用丢弃查询的办法得不到「停住时仍有远端在途」的状态；
  - 「已登记，发送待确认」：故障代理只能作用于 GetTask，造不出「只有派发意图」或「已发送但没有回执」的记录。

  这两档只有单元测试和浏览器测试覆盖，见 #59。
- R20 最后经接口恢复，任务 done，5 个 request 各发送 1 次。

## 截图

- `e2dc-2-1-waiting-submitted.png`：R19 进入 waiting 后在插件页提交恢复。插件是 2dc971b1 自带版本，related 还在转圈，#59 修的就是这个显示。
- `e2dc-2-2-after-resume.png`、`e2dc-2-3-final.png`：恢复后与完成时。
- `e2dc-59-1-halted.png`、`e2dc-59-2-halted-reloaded.png`：#59 插件下的 R20，刷新前后各一张。
- `e2dc-59-3-old-unknown-R18.png`：#59 插件下的 R18。

## 未覆盖

- 成员不响应中断时 `close()` 会一直等待并阻塞宿主，这个边界没有测，归 W03/SDK。
- 线程转储（镜像无工具）。

## #59 三档投影：替身截图（插件 `2c022ea`）

真实环境复现不了「远端在途」和「已登记，发送待确认」两档（原因见上文），这两档由替身数据截图，并由 #59 的单元测试和浏览器测试覆盖。

| 文件 | 内容 |
|---|---|
| `shots/59-fixture/three-tiers.png` | 三档同屏 |
| `shots/59-fixture/tier1-unconfirmed.png` | 仅有派发意图：已登记，发送待确认 |
| `shots/59-fixture/tier2-inflight.png` | 有对端回执：远端在途 · 最后观察 |
| `shots/59-fixture/tier3-unknown.png` | 结果未知 · 最后观察，保留原请求 |
