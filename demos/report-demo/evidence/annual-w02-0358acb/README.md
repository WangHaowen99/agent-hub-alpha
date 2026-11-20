# W02 年报 D1 插件接收记录（0358acb，W02 环境）

被测：yougq/agent-hub-alpha `0358acbad0ce1b0ce73772f31eda845057ba4dc5`（WorkSwarm `1f168664`、agent-solution `eb48c482`），按 LINUX-RUN 原生部署。操作者 WangHaowen99，插件页由 Playwright 驱动（`scripts/w02_receive.py`），第二个插件客户端用同一插件 RPC 做重复提交。

## 部署差异

| 项 | LINUX-RUN | W02 环境 |
|---|---|---|
| 平台 | WSL2 Ubuntu 26.04/24.04 | WSL Ubuntu 22.04 |
| 端口 | Gateway 8080、WorkSwarm 网关 19000 | 8088、19100（本机端口被其他环境占用；启动参数、健康检查、Gateway 地址配置一并改） |
| PostgreSQL | 16.16 | 16.15 本机容器 |
| 工具链 | JDK 21.0.12、Python 3.14.6 | Temurin 21.0.13、Python 3.14.4；Maven 经镜像下载，未改 pom 和锁文件 |
| 模型 | deepseek flash | DashScope 兼容接口 `deepseek-v4.1-flash`（该端点无 v3.x），绑定摘要 `sha256:230d3e47…`；最早用 qwen-plus 时规划只出 6 个节点、写作专家把页码输出成字符串，见下 |
| WorkSwarm 首启 | 装配后直接启动 | 按 yougq 在 fork PR #1 的复跑步骤，先建空的 workswarm-home/config |

## 平台 live_smoke

`platform-live-smoke.json`、`platform-smoke-*.png`

- qwen-plus（T-3f848f8e）：四位专家完成，但规划只有 6 个节点、没有修订节点；写作专家把 `printed_page` 输出为字符串，检查判 REVISE，选择节点停在 WAITING_INPUT，live_smoke 报「did not settle」。
- deepseek-v4.1-flash 第 1 次（T-a391df83）：资产质量专家输出的 JSON 有重复键 `section_id`，被严格解析拒收，远端只允许 1 次，任务 FAILED。
- 第 2 次（T-6fe980b52406393ddd4d1f2cc35ccc65）：规划 8 个节点，4 次远端调用完成，check0 PASS，DELIVERED，下载件与正式摘要一致。批准、交付由 live_smoke 用演示账号自动完成。

## 四项接收（`w02-receive.json`，任务 T-af0c870c7329e59587fea9d0d0cc1cea）

| 项 | 结论 | 依据 |
|---|---|---|
| ① 同任务 generation/rev | 接受 | 插件收到该任务 117 个 Graph 应答，rev 1→203 按到达顺序不回退，最终等于 W04 generation 203 |
| ② BLOCKED 原因与恢复入口 | 未触发 BLOCKED；有发现 | 旧任务 T-3f848f8e 绑定的是 qwen 时的模型摘要（823b2fd0…，当前 230d3e47…）。插件显示恢复控件，两个客户端各恢复一次都返回 QUEUED，执行权只有 1 个；没有因绑定不同而阻断（剩下的选择节点是本地节点，不调模型）。之后任务一直是 RUNNING、选择节点 INPUT_REQUIRED，编排器原地等待，插件显示「执行中 5/6」（`shots/w02-02-old-after.png`）；经同一接口请求暂停后 40 秒仍为 RUNNING。插件的受阻横幅是固定文案，未实测原因展示 |
| ③ 采纳与恢复回执、去重 | 部分接受，2 处差异 | 同摘要重复采纳返回同一 graph_digest、不重复执行（仍 4 次远端调用）；同一 command_id 重复交付返回同一回执（epoch 1）。差异：重复批准报告生成第二个 approval_id；换 command_id 再交付生成第二张交付回执（epoch 2），其 recorded_at 比 epoch 1 早约 90 ms。插件的 command_id 按页面生成，两个标签页各交付一次就会出现两张回执 |
| ④ 报告摘要与 HUMAN 跳转 | 摘要接受；跳转未覆盖 | 插件下载件 sha256:b9b68faf… 与投影摘要、artifact_id 一致，/verify evidence_intact=true（6 个正式节点、4 次远端调用）。两次任务检查都直接 PASS，没有产生 HUMAN，跳转没测到 |

## 未覆盖

- HUMAN 跳转可信页与核验后继续：留到 D2，按 yougq `00d3af7` 在同一真实 HUMAN 任务上做。
- 真实 BLOCKED 的原因展示。
