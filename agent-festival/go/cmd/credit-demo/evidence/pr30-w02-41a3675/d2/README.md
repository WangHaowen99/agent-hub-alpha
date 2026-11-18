# W02 组合 D2 记录（41a3675，W02 环境）

环境与 D1 相同，见上级目录 `env-41a3675.json`。所有恢复都在真实 WorkSwarm 插件页里提交（Playwright 驱动，操作者 WangHaowen99）。

- R-A、R-C 的场景定义直接复用 W05 的 `w05_rd_extra.py`（W05 fork `e084318`，sha256 前 16 位 `01a64b37cf7880bf`），未改动，只把 `RESUME_MODE` 设为 `external`，由插件页完成恢复。
- R-B 和 R-C 的插件路径步骤，以及 O-A、O-B，由 `scripts/d2.py` 实现。
- O-A、O-B 在插件页的 WebSocket 层扣住一次 Graph 应答，不改产品代码。
- 每个 JSON 里的 `transport_events` 是故障代理的原始事件，按本任务的 request/task 编号筛选，不是脚本算出的计数。

| 文件 | 编号 | 任务 |
|---|---|---|
| `d2-oa.json` | O-A | R6 |
| `d2-ob.json` | O-B | R7 |
| `d2-ra-report.json` | R-A（report 未知） | R8 |
| `d2-ra-two.json`、`d2-ra-two-second-resume.json` | R-A（两个节点完成后 financial 未知） | R9 |
| `d2-rb.json` | R-B（插件恢复回复丢失） | R10 |
| `d2-rc.json` | R-C（插件恢复时保存失败） | R11 |
| `d2-tc.json` | R-D 的插件部分：两个客户端同时恢复 | R12 |
| `../ua-41a3675-transport-events.json` | U-A 补原始传输事件 | R5 |

## 结果

- **O-A**：扣住 R6 的一次 Graph 应答（rev 392）6 秒，期间切走再切回同一任务，新轮询先送达 rev 394、399、405；旧应答在发出后 6.0 秒送达（未超过 20 秒），界面前后一致，没有倒退。
  - 注意：插件轮询是串行的，切换后旧循环被 `stop` 作废，旧应答在进入 `newer()` 版本比较前就被丢弃。真实插件路径上构造不出「两个都有效的应答进入版本比较」，版本比较只有补丁里的单元测试覆盖。
- **O-B**：扣住 R7 的一次 Graph 应答（rev 507）25 秒。客户端 20 秒超时后继续轮询（21.2 s 起新请求先后送达 rev 529～534）；旧应答 25.0 秒到达时网关已没有等待项，被直接丢弃，界面不变。
- **R-A（report 未知）**：intake、financial、industry、bureau、related、judge 都完成后丢 report 的一次查询应答，任务 waiting；从插件恢复后 report 查询原 task 完成，任务 done。已完成节点的 request、产物、model_events 不变，5 个请求各 1 次 SendMessage。
- **R-A（两个节点完成后 financial 未知）**：intake、industry、bureau 完成，financial 查询应答丢失，此时 related 已开始执行；任务 waiting。
  - 第 1 次插件恢复（约 12 秒后）：related 重新执行并完成，judge 完成；**没有**复现 W05 R11 的 `fork_subtask` 重复注册。但 report 的 `/exec/call/update` 被 Go 以 `400 invalid Gateway observation` 拒收，任务再次 waiting（report 状态 waiting，查询原任务）。
  - 第 2 次插件恢复：report 查询原 task 完成，任务 done；report 的 model_events 仍是同一条，5 个请求各 1 次 SendMessage。
  - waiting 时界面上 related 仍显示执行中（`shots/d2-ra-two-waiting.png`），即 owner 指出的投影问题。
- **R-B**：任务 waiting 后在插件提交恢复，扣掉回复。Go 2 秒内已转 running（已落盘）；页面只发出 1 次恢复请求、没有自动重试；20 秒后提示「请求超时。请先查看原任务最新状态，勿连续重试。」，不宣称失败。重新打开后显示真实进度；再提交一次恢复被拒（REJECTED「任务不存在或已不在等待状态」）。领取次数只加 1，request/task 不变，5 个请求各 1 次 SendMessage，任务 done。
- **R-C**：任务 waiting 时把状态文件换成目录，再从插件恢复：提示「状态保存失败，未接受恢复；请联系维护人员。」，`/healthz` 503，任务保持 waiting。修复存储并重启 credit 后仍是 waiting；再从插件恢复被接受，任务 done，request/task 不变，5 个请求各 1 次 SendMessage。
- **R-D（两个插件客户端同时恢复）**：同一浏览器两个标签页各开插件，几乎同时提交恢复。一个提示「恢复请求已接受」，另一个提示「任务不存在或已不在等待状态，请刷新查看原任务」；领取次数只加 1，request/task 不变，5 个请求各 1 次 SendMessage，任务 done。

## 没有覆盖

- 首条 SendMessage 回执丢失：W02 环境的故障代理没有这个模式，见 W05 记录。
- R-D 的运行中停 Go、Runtime、Gateway：W05 主验。
- M-A、U-B：依赖 #31。
