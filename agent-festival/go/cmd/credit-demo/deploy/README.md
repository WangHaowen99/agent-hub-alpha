# Issue #16 构建与部署入口

本目录在 W02 授信尽调原型上接入真实 Agent Bus，验证 WorkSwarm → Go → Java Graph → Gateway DIRECT → 独立 Agent Runtime。
原型来源为 [WangHaowen99/agent-hub-alpha @ 46980e8](https://github.com/WangHaowen99/agent-hub-alpha/tree/46980e84bf12e9a0fbfa860614822509bf299db5)，仅引入 `credit-demo`、`credit-orchestrator` 和必需的部署补丁，未合并整个 fork。

`--bus` 模式下，资料、分析、写作分别部署为三个 Runtime 进程；5 个普通专业节点经真实 RDC/Gateway 调用。原型的关联方 Loop Forest 和规则判断仍在 Java 宿主内。Go 保存正式结果和投影，Java Graph 仍是唯一执行推进者。

**本次完成范围是 Agent Bus 接入，业务数据仍为授信模拟材料。** W06 的年报提取、计算/引用核验和已验收报告交付不在此实现内，不能据此关闭 #16 的 A01—A15。#6 是可选增强，不参与启动依赖。

实施与证据见 [Agent Bus 执行计划](../../../../docs/issue16-agent-bus-execution-plan.md) 和 [验收记录](../../../../docs/issue16-agent-bus-results.md)。分支 `feat/issue16-agent-bus`；可回滚基线 `5051d3a`。

## 接入前的原型验证记录（2026-11-11）

| 检查 | 结果 |
| --- | --- |
| Go / Java / WorkSwarm 三个镜像构建 | 通过；`build-all.json` 保存实际镜像 ID |
| Go 单元测试、`go vet`、图卡源码构建 | 通过 |
| Go / Java 健康检查、图编译与摘要稳定性、非法依赖拒绝 | 通过 |
| MCP 能力目录、Go → Java 候选图创建 | 通过；本次生成待批准计划 `P1`，未触发执行 |
| WorkSwarm 已安装包、前端资源、启动命令与 MCP App 方法 | 禁用网络的镜像检查通过 |
| WorkSwarm Web、专家入口与模型连通 | 已启动；页面识别 `deepseek-flash`，容器内模型请求成功 |
| 真实对话 → MCP → 候选图卡片 | 通过；DeepSeek 生成 `P2 v1`，7 个节点，预检通过且保持待批准 |
| 完整业务执行与年报场景验收 | 尚未完成 |

证据见缓存目录 `evidence/build-all.json`、`core-smoke.json`、`workswarm-image-smoke.json`、`experience-smoke.json`。`core-smoke.json` 与 `workswarm-image-smoke.json` 中的零模型调用仅描述对应脚本行为；体验检查使用真实模型，不作为 #16 的调用计数验收证据。

## 构建

在仓库根目录执行（Linux x86_64，Docker/Compose、Git、Python 3、uv 0.11.7）：

```sh
python3 agent-festival/go/cmd/credit-demo/deploy/demo.py --bus build all
```

也可将 `all` 换成 `go`、`java`、`ui`，逐个构建。构建不会启动业务服务或调用模型。

- `sources.lock.json` 固定原型、WorkSwarm 和 agent-solution 提交；Go 工具链校验 SHA-256。
- Java Core 0.1.17 从 Maven Central 获取，orchestration / react-rails 0.1.3 从固定 solution 源码构建，不使用作者机器的 `kbs-m2`。
- WorkSwarm 沿用自身 `package-lock.json` 和 `uv.lock`；暂以 W02 原型的 `1142e5fb3` 为复现基线。切换 #16 引用的 `b98ec8c7d` 需独立验证，不默认视为兼容。
- 该版本的协议依赖位于 GitCode MR #391；构建只将获取引用改为该 MR，`uv.lock` 最终检出的 `4c5f3e9` 提交保持不变。使用 `uv sync --frozen` 保留上游依赖覆盖规则。
- 基础镜像使用 `python:3.11-slim`、`node:22-slim` 和 `nginx:alpine`，实际构建镜像 ID 写入证据。Java 构建与运行使用 Debian OpenJDK 21，编译目标仍为 Java 17。
- WorkSwarm 前端构建默认允许 Node 使用 3 GiB 堆；首次下载和编译依赖耗时较长，后续复用 Docker 构建缓存。
- 下载与源码缓存位于 `~/.cache/agent-hub-alpha/issue16`，可用 `DEMO_CACHE` 指定另一目录。缓存源有改动时拒绝覆盖。
- 构建继承当前 `HTTP_PROXY` / `HTTPS_PROXY`，Maven 在构建步骤内生成并删除临时代理配置；不修改 Docker 守护进程或本机其他服务。

## 配置模型并启动

可直接使用当前 shell 中的 `SWARM_API_BASE`、`SWARM_API_KEY`、`SWARM_MODEL_NAME` 和 `SWARM_ENDPOINT_PROFILE`。如果没有这组显式配置、也没有配置文件，启动脚本会复用 `DEEPSEEK_API_KEY`，并读取 `COEVOSKILLS_DEEPSEEK_BASE_URL` / `COEVOSKILLS_DEEPSEEK_MODEL`（也支持 `DEEPSEEK_BASE_URL` / `DEEPSEEK_MODEL`），自动选择 `deepseek` 协议配置。密钥只传入运行时，不写入仓库或镜像。

也可将 `secrets.env.example` 复制到仓库外，例如 `~/.config/aha-swarm/secrets.env`，填写模型地址、密钥和模型名。文件权限设为 `600`，不要提交真实配置。

```sh
# 使用文件时设置；已有环境变量时省略此行。
export AHA_SECRETS_FILE=/absolute/path/to/secrets.env
python3 agent-festival/go/cmd/credit-demo/deploy/demo.py --bus start
python3 agent-festival/go/cmd/credit-demo/deploy/demo.py --bus status
```

入口：`http://localhost:5192`。若首次出现模型配置引导，已有配置时点“跳过”。依次进入“专家 → 我的专家 → 授信编排助手”的详情页，点击“使用”。
对话示例：为客户 C006 做授信尽调，使用权限范围内的征信摘要，先给出候选图，等待我批准。
演示确认账号为 `alice`；密码由 `CREDIT_DEMO_PASSWORD` 配置。

场景包：授信尽调场景（`scenarios/credit-dd.json`）随程序内置。其它场景包由运维放入 `CREDIT_SCENARIO_DIR` 导入，镜像内为 `deploy/scenarios/`，当前只有年报场景 `annual-report-cmb-2025`。只有治理状态为 `approved` 的包会被加载；包按 RFC 8785 规范化后的 SHA-256 摘要固定，批准摘要覆盖它。发布了选择策略的场景（年报场景）先调用 `select_capabilities` 生成选择记录，再用 `propose_plan` 的 `selection_id` 提交计划；选择依据显示在确认页和 `/api/graph` 的 `selection` 中。

默认仅绑定回环地址；端口可用 `CREDIT_SWARM_PORT` / `CREDIT_PORT` 调整，允许来源随前端端口一致变化。模型密钥通过运行时配置注入，不进入镜像。Compose 中未提供配置时允许构建，完整启动前会检查各服务实际解析出的模型配置并拒绝缺失值；不要求必须有配置文件。WorkSwarm 自身会将运行配置写入受保护的数据卷文件。

## 无模型的接口验证

```sh
python3 agent-festival/go/cmd/credit-demo/deploy/demo.py start core
python3 agent-festival/go/cmd/credit-demo/deploy/smoke.py
python3 agent-festival/go/cmd/credit-demo/deploy/smoke_image.py
```

只启动 Go 与 Java，验证健康检查、真实编排模块的图编译、无效依赖拒绝及 MCP 能力目录，并通过 `propose_plan` 创建一张待批准候选图，验证 Go → Java 编译接口。不会批准业务任务或调用模型；通过结果不能替代端到端演示验收。

`smoke_image.py` 单独在禁用网络的临时容器中检查 WorkSwarm 的已安装包、前端资源与 MCP App 协议方法；不等同于界面及模型联调。

## 停止与状态

```sh
python3 agent-festival/go/cmd/credit-demo/deploy/demo.py --bus stop
```

只停止 `agent-hub-issue16` 项目，保留命名数据卷。构建记录与验证证据写在缓存目录的 `evidence/` 下。

后续由 W01/W06 提供公开年报材料、三个角色的业务逻辑、独立参考答案和核验工具，沿本次已接通的 Runtime/Bus 接口替换授信场景配置。

## Agent Bus 操作与验收

完整接入模式使用 `--bus`；省略时仍可复现原型。`--bus start core` 启动总线、数据库、Go、Java 和三个 Runtime，完整 `--bus start` 同时启动 WorkSwarm。脚本会等待三个角色完成 RDC 发现。RDC/Gateway/Runtime 均只在 Compose 网络内开放，浏览器入口继续使用 `5192`。

- 模型从现有 `SWARM_*` / DeepSeek 配置注入；三个专业 Runtime 使用同一模型配置，每节点独立上下文、MCP 挂载及只读权限。
- 总线的四个内部凭据首次生成在 `${DEMO_CACHE}/bus-secrets.json`，权限 `600`；重部署保留此文件，也可由相同名称环境变量覆盖。更换数据库密码须同步数据库，不能只删除凭据文件。
- 当前能力版本 `1.0.1`，A2A wire `1.0 JSON-RPC`，Runtime `0.1.4`，Core `0.1.17`，RDC/Gateway `0.1.3`。客户端为 JDK 21 HttpClient 窄适配器，只实现固定版本的 `SendMessage(returnImmediately)` / `GetTask`；没有另建 A2A 服务或注册中心。
- RDC 使用原生 static instance reconciliation。每个实例明确指定 `route-key: /a2a`。该快照同卡片摘要时不更新 binding，因此改路由/契约配置必须发布新的能力版本并重新生成/确认计划，不能静默替换旧计划的绑定。
- RDC 卡片响应为 `rawJson` 包装；按能力 ID 查询真实卡片和 `ONLINE` 实例。`/v2/agents` 在此快照的列表结果不用于替代业务白名单。
- 此单机 Demo 的卡片发现限定 Docker 私网 CIDR，`verify-signatures: false`；Gateway 使用上游静态 credential directory，控制服务仍使用演示用户。这不是生产身份、签名验证或多租户资格。
- Gateway 的租户环境变量须为 `GATEWAY_TEST_CREDENTIAL_TENANTID` / `...PRINCIPALID`，对应上游读取的 camelCase 属性。健康检查实际访问认证后的 A2A 接口，不能只用端口连通代替鉴权成功。

正常演示会调用已配置模型并批准模拟任务，但不包含任何银行提交节点：

```sh
python3 agent-festival/go/cmd/credit-demo/deploy/smoke_bus.py normal
python3 agent-festival/go/cmd/credit-demo/deploy/probe_bus.py
```

故障脚本只操作 `BUS_TEST_PROJECT` 指定的 Compose 项目（默认 `agent-hub-issue16`）。运行前停止其他业务测试。`host-kill`、`gateway-restart`、`runtime-restart` 注入对应进程故障。`disconnect`、`late`、`storage-failure` 需先加入测试代理 overlay，代理仍把实际请求发到真实 Gateway：

```sh
cd agent-festival/go/cmd/credit-demo/deploy
python3 - <<'PY'
import demo
demo.BUS = True
demo.compose('-f', demo.HERE/'compose.bus-test.yaml', 'up', '-d', '--no-build', '--wait', 'orchestrator', 'fault-proxy')
PY
python3 smoke_bus.py host-kill
python3 smoke_bus.py disconnect
python3 smoke_bus.py late
python3 smoke_bus.py storage-failure
python3 smoke_bus.py gateway-restart
python3 smoke_bus.py runtime-restart
python3 probe_bus.py --rdc-outage
# 恢复直接 Gateway 地址；移除本测试代理。
python3 demo.py --bus start
# 此名称只对应默认项目；使用其他 BUS_TEST_PROJECT 时替换项目名。
docker rm -f agent-hub-issue16-fault-proxy-1
```

`storage-failure` 会保存当前 `state.json` 为 `.bus-test-backup`，以目录占位使实际 atomic rename 失败，并在 `finally` 恢复原文件、重启 Go/宿主。若脚本本身被强杀，先停宿主，确认 `/data/state.json` 是测试空目录后删除该空目录，将备份改回原名，再启动 Go/宿主。不要在存在其他活动业务时运行此故障实验。

状态与恢复：

1. 宿主重启读取同一采纳图；已完成节点复用正式产物，未完成且有远端任务号的节点查询原任务。
2. 断线、响应超时或派发后无回执显示等待/未知；在运行页以 `alice` 登录后点击“核对原任务并继续”。此操作保留原身份和尝试，绝不重新 SendMessage。启用 Bus 后，旧待确认计划如没有真实能力绑定，会要求重新生成和确认。
3. Gateway 的任务归属索引及 Runtime 默认任务存储为内存实现。它们重启丢失原任务后，核对仍会等待，需人工处理。宿主恢复不等于这两个上游组件已经获得持久任务存储。
4. Go 必需状态写入失败会恢复最后提交的内存快照、冻结后续写入并返回健康检查 503；修复存储并重启后以磁盘为准核对，不能跳过正式结果提交。
5. 记录的模型计数是**在实际模型客户端调用前持久保存的尝试记录**，用于验证已完成节点恢复增量为零；不是供应商账单或所有网络重试次数。

## 全新部署、备份与回滚

### 编排器关闭补丁与恢复边界

编排器构建在 `sources.lock.json` 固定的 agent-solution 源码上应用
`patches/blueprint-scheduler-drain.patch`，并以独立 Maven 版本
`0.1.3-aha-drain1` 构建，避免与未修复的 `0.1.3` 混用。补丁修改的是
`BlueprintScheduler.close()`：由宿主取消并等待旧成员退出、释放 Loop Forest
进程级工具注册后，才允许本宿主开始下一轮；不新增调度器。

本地 Java 回归：在固定的 agent-solution 源码应用该补丁，先运行
`mvn -N -Drevision=0.1.3-aha-drain1 -Dmaven.test.skip=true install`，再运行
`mvn -f common/agent-core-ext-java/pom.xml -pl agent-core-ext-orchestration -am -Drevision=0.1.3-aha-drain1 -Dmaven.test.skip=true install`。
随后在 `agent-festival/examples/credit-orchestrator` 执行 `mvn clean test`。
`SchedulerRecoveryTest` 使用真实 SDK 调度器和工具注册表；`JudgmentErrorsTest`
使用真实 HTTP 客户端与计数模型替身，不代表真实模型或共同镜像的整链验收。

关闭等待不设强行放行时限：成员不响应取消时，宿主不得在旧资源仍存活的情况下
重新执行。此时需要运维隔离或重启进程；真实模型取消行为、共同部署恢复仍需 D2 验收。
`STORAGE_UNAVAILABLE` 也不表示“没有写入”：采纳账本可能已写入而状态快照失败。
服务冻结写入后，应保留数据并核对账本与快照，再决定恢复；不能将此错误用于模型内容修订或盲目重发。

启动时会复用现有采纳检查器和回放引擎，核对每个已知任务的 `adopt.jsonl` 与
`state.json` 中的操作序列、原决策摘要/身份/绑定和节点结果。账本先行写入、
账本丢失或损坏、旧格式不兼容、原决策结果缺失时，该任务进入
`blocked / RECONCILIATION_REQUIRED`，撤销旧令牌，普通恢复不能放行；其他正常任务继续运行。
状态改变前保存 `reconciliation-<摘要>.json` 原快照副本，副本保存失败则停止启动。
副本和账本应作为本地运维证据保留，外发前脱敏；不要删账本或将任务改为 queued 来绕过核对。
修复数据也不会自动解除原有 blocked 状态，人工修复/解除的受控流程仍待联合验收。

新判断将决策和带步骤标识的节点结果写入同一快照：响应丢失、执行端未发 StepPut 时，
StepGet 仍可复用该结果。它不提供账本与快照之间的跨文件原子性，也不从摘要恢复丢失的模型内容。
旧任务已存下完整且可核对的结果可继续；不完整或不兼容记录保留并阻断，不执行猜测式迁移。

另用干净项目和端口可重跑部署（相同镜像、独立卷；资源不足时先停当前项目）：

```sh
COMPOSE_PROJECT_NAME=agent-hub-issue16-fresh CREDIT_PORT=5291 CREDIT_SWARM_PORT=5292 python3 agent-festival/go/cmd/credit-demo/deploy/demo.py --bus start
BUS_TEST_PROJECT=agent-hub-issue16-fresh BUS_TEST_BASE=http://127.0.0.1:5291 python3 agent-festival/go/cmd/credit-demo/deploy/smoke_bus.py normal
```

启动会同时保留 `credit-data`（结果）、`rdc-data`（PostgreSQL）及 `swarm-home`（WorkSwarm）。普通 `stop` 不删除卷；备份须在本项目停止写入后分别导出这三个卷，并保留私有 `bus-secrets.json`、模型配置来源与镜像清单。恢复须匹配代码/镜像和卷版本，避免拿旧服务覆盖含新调用记录的数据。

本次实施前备份位于 `~/.cache/agent-hub-alpha/issue16/backups/20261111-184444/`，包含完整工作区、差异、原 `credit-data/` 与镜像 manifest；基线提交为 `5051d3a`。回退原型时先备份当前三个卷并停止当前项目，在单独 worktree 检出基线，用基线镜像和备份数据启动，省略 `--bus`；保留现有总线卷以便返回本分支。不能只切代码而继续改写同一个新格式结果卷。

新增组件实测稳态约 1 GiB（3 个专业 Runtime、RDC、Gateway、数据库合计，详见证据 `resources.json`）。Compose 分别限制 420/420/420/420/320/192 MiB；宿主 512 MiB。此数值不包含 WorkSwarm、Go、其他机器服务或构建峰值；应为构建和并行任务额外预留内存。

## 在页面核对 Agent Bus 与 RDC

打开运行页，展开图卡顶部的“调用清单”或点击专业节点。详情显示派发前 RDC 快照、Gateway 请求及有效回执、Agent 开始/模型尝试、正式结果。关联 ID 和摘要可展开、复制；列表可检查尚未开始的节点。确认页只显示能力绑定，用户批准后才有调用记录。

RDC 候选不等于实际执行实例，当前后者显示“未采集”；旧任务缺少的新观测也显示“未采集”。健康值带历史查询时间，HTTP 200 的协议错误不会显示成功。Gateway 观测来源是 Java 调用宿主；模型尝试数不是供应商账单。结果复用展示原调用证据，不表示重新派发。

新增页面数据验收（真实模型、模拟案件，无银行操作；生成一条执行和一份未批准计划）：

```sh
python3 agent-festival/go/cmd/credit-demo/deploy/smoke_bus_ui.py
# 前端语义与类型检查，在 app 目录安装锁定依赖后运行：
cd agent-festival/go/cmd/credit-demo/app
npm ci
npm run check
npm test
```

完整结果和体验样例见[执行结果的页面补充](../../../../docs/issue16-agent-bus-results.md#页面调用证据补充2026-11-11)。
