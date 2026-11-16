# PR #30（7dfd061）W02 真实插件路径验收记录

- 组合头：`7dfd061dc679beb84731377b15680ac0cbb4e43c`（源码未改动）
- 构建：`demo.py --bus build all`，仓库内 Dockerfile/compose 原样；DEMO_CACHE 放在仓库外的 ext4 目录
- 运行：compose 项目 `pr30-w02`，WorkSwarm 前端 127.0.0.1:5292，Go 调试 127.0.0.1:5291，均只听回环；环境保留运行
- 浏览器：部署主机上的 Python Playwright 1.58 / Chromium 无头，1440×1000，所有操作经 WorkSwarm 前端 → 编排台插件页（iframe）→ 插件 RPC，不直开 Go 旧 UI

## 1. D1-环境

| 项 | 结果 |
|---|---|
| 源码 | agent-solution `bf702bc95064d3491cf8b374f54d271363d84230`，WorkSwarm `1142e5fb3984a2b1bc6352669a464976e204b2dc`（与 sources.lock.json 一致） |
| 工具链 | Go 1.25.0 tarball 取自 golang.google.cn，SHA-256 与 lock 一致；uv 0.11.7；Maven 直连中央仓库，未用私有 m2 |
| 镜像 | 见 `d1-env.json`（credit/orchestrator/professional bus-v1、workswarm 1142e5fb3-local、rdc/gateway bf702bc、postgres 按 lock 的 digest 拉取） |
| 插件发现 | `swarm-plugin-discovery.log`（WorkSwarm 启动日志摘录）：ExtensionManager 发现 `/root/.jiuwenswarm/domain/application_plugins/credit-workbench` 并加载 `CreditWorkbench`，extensions loaded: 1 |
| 前端加载 | `/api/application-plugins` 返回编排台清单（`d1-plugin-manifest.json`），entry `/api/application-plugins/credit-workbench/assets/index.html` 200 |
| 四个 RPC | `d1-rpc-probe.json`：plans 正常返回；graph/approve/resume 由插件自身参数校验返回 BAD_REQUEST；对照的未知方法返回 METHOD_NOT_FOUND → 四个方法均已注册 |

部署差异（非代码）：部署主机的 Docker 默认地址池已用完，预建了带 compose 标签的 `pr30-w02_default`（192.168.64.0/20，落在 rdc.yml 的 target-cidrs 内）；DEMO_CACHE 不能放在 Windows 挂载盘（DrvFs）上，否则凭据文件无法设为 600。

## 2. D1-入口（P1 / R1，`d1-entry-P1.json`，截图 shots/e1-*）

- 对话：专家 → 授信编排助手 → 「为客户 C006 做授信尽调……等待我批准」，真实模型（qwen-plus）生成 P1 v1，7 节点，回复引导到编排台。
- 编排台：选中 P1 v1，图 7 节点；Bus 总览「Agent Bus DIRECT · RDC 能力绑定 · 3 类服务 · 5 个远端节点 · 待用户批准 · 尚未派发」；节点详情展示 RDC 绑定（credit-analysis 1.0.1）与 Gateway「尚未派发」。
- 错密码：提示「密码不对」，Rev 1 → 1，run 数 0 → 0。
- 正确密码：生成 R1，`approved_via = {channel: workswarm-plugin}`；约 65 秒完成，「验证通过 8/8」，5 个远端节点均有 request/task/artifact 关联。

## 3. waiting 恢复（P2 / R2，`resume-R2.json`，截图 shots/r2-*）

- 叠加 compose.bus-test.yaml 的 fault-proxy，设置 `drop-query-once`；P2 经 MCP propose（测试用 TEAM），在编排台批准。
- intake 的首次 GetTask 被丢 → intake unknown，R2 waiting；列表「等待核对」，图头「等待核对远端结果」，出现「核对原任务并继续」。
- 错密码恢复：「账号或密码不对。请先查看原任务最新状态，勿连续重试。」Rev 108 → 108，状态仍 waiting，传输事件增量 0，密码框已清空。
- 正确密码、同一时刻提交两次：只发出 1 次 resume RPC；Rev 108 → 128，转 running。
- 之后再次经插件 RPC 恢复：`REJECTED`「任务不存在或已不在等待状态，请刷新查看原任务」。
- 结果：R2 done、验证通过 8/8；5 个远端节点每个 request_id 恰好 1 次 SendMessage；intake 沿用原 request_id/remote_task_id，恢复后对原 task 发 GetTask（200）。

## 4. A12（插件路径）

| 用例 | 结果 |
|---|---|
| Run A（P3/R3）刷新 | 刷新后仍选中 P3 v1，无倒退 |
| Run A WorkSwarm Gateway 重启（docker restart swarm） | 插件页被宿主收起约 19 秒（脚本时钟 41.9–60.8 s 无 iframe），之后自动回来、选中 P3 v1 恢复、run 继续并完成 8/8 |
| Run B（P4/R4）迟到应答 | WS 层扣住一条带 run 的 graph 应答（rev 247）25 秒；期间前端 20 秒超时（横幅「请求超时」）后继续轮询，显示了 6 个更新快照（最高 rev 296）；旧应答随后送达，无任何节点或进度倒退 |
| Run B 断网 6 秒（CDP offline） | 未断开已建立的 WebSocket，界面无变化 → 实际未覆盖 |
| Run B Go 停 / 启 | 停机发生在 R4 完成之后：横幅「编排服务暂时连不上」、图保留；Go 恢复后重新打开，横幅消失、选中 P4 v1 保持。运行中停 Go 本轮未覆盖 |

全程 regressions = []（`a12-runA.json`、`a12-runB.json`，逐样本日志 `a12-run*-log.json`）。

## 发现的问题

1. 恢复成功提示看不到（低）：ResumeControl 只在 `run.status === "waiting"` 时渲染，接受恢复后下一次轮询状态变为 queued/running，组件随即卸载，「恢复请求已接受……这不表示任务已完成」只闪一下（2.5 s 时已不在页面）。复现：让 run 进入 waiting → 编排台输入正确密码点「核对原任务并继续」→ 黄框约 1 秒内消失。
2. waiting 时图只画出已开始的节点（R2 只显示「材料受理」），沿用 W02「开始才出现」的设计，用户在等待态看不到其余计划。属设计取舍，记录备查。

## 未覆盖

- 浏览器真实断网（需断开已建 WS，如停 front 或网络层切断）。
- 运行中停 Go 后的租约接管。
- 「late-query-once」等 Bus 侧故障（W05 范围）。

## 公开版说明

- 完整的 WorkSwarm 运行日志含模型服务地址和模型调用内容，未公开，仅保留插件发现摘录 `swarm-plugin-discovery.log`。
- 未公开运行配置、演示密码与总线凭据；已核对公开文件中不含这些值。`go-state-redacted.json` 为脱敏后的 Go 状态。
- 恢复用例的第一次尝试因测试脚本自身报错中断，正式记录以 `resume-R2.json` 为准。
- `scripts/` 是实际使用脚本的副本，仅把本机路径和密码来源改成环境变量：`CREDIT_DEMO_PASSWORD`、`CREDIT_DEMO_DEPLOY`（指向 `agent-festival/go/cmd/credit-demo/deploy`）、`EVIDENCE_DIR`、`SWARM_BASE`、`CREDIT_BASE`。
