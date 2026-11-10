# pilot-mcp：WorkSwarm 用户面最小验证

对应 [Issue #4](https://github.com/chaosxingxc-orion/agent-hub-alpha/issues/4) 第 1 点“WorkSwarm 用户面”的验证。不改 WorkSwarm 源码，用一个 Go 桩服务回答五个问题（V1–V5）。只用于验证，不是生产组件，不向主线提交。

**状态**：设计＝验证用桩；实现＝本目录；验证＝下表实测；集成＝无（未接 Java Runtime）。

## 组成

| 部分 | 内容 |
| --- | --- |
| Go 桩服务 | MCP（streamable HTTP）工具：提交申请、查询、列表、申请取消、查看调用方身份。复用 `internal/adopt` 做业务请求的采纳、派发、终态；命令按 `(scope, issuer, command_id)` 去重，同号不同内容判冲突 |
| 测试业务 API | 进程内模拟，可注入：正常、拒绝、超时、丢回执、重复回执加迟到回执 |
| MCP App 控制台 | `ui://pilot/console`（`app/`，打包为单文件）。状态只从 Go 读取；命令号在用户点击时生成；有“重发同一命令”“同号改资产重发” |
| 部署 | `deploy/`：WorkSwarm develop 源码构建镜像、Go 桩镜像、连接器和“授信试用”专家的自动注册；`compose.enterprise.yaml` 为企业版双令牌模拟 |
| 端到端脚本 | `deploy/e2e/`：playwright-core 连接常驻 Chrome（CDP），驱动 WorkSwarm Web |

## 运行

```bash
cd deploy
SWARM_REPO=/path/to/jiuwenswarm ./build.sh      # 构建 WorkSwarm 镜像，写入 .env 的 SWARM_COMMIT
./build-pilot.sh                                 # 构建 Go 桩镜像
mkdir -p ~/.config/aha-swarm && $EDITOR ~/.config/aha-swarm/secrets.env   # SWARM_API_BASE / SWARM_API_KEY / SWARM_MODEL_NAME
docker compose up -d                             # Web: 127.0.0.1:5182，MCP App 沙箱：127.0.0.1:19000，桩服务调试：127.0.0.1:5183/debug/state
```

浏览器必须能直接访问 19000（见下文 U2）。单元测试：`go test ./...`（Go 1.25）。

## 结果

环境：WorkSwarm develop `5778e1aec`（2026-11-10），模型 qwen-plus，Web 单 Agent 会话。

| 编号 | 问题 | 结果 | 证据 |
| --- | --- | --- | --- |
| V1 | Go 能否拿到可信的终端用户身份 | **不通过（开源版默认配置）** | 连接器不配请求头时，Go 只收到 `Mcp-Session-Id`、`User-Agent` 等通用头：模型调用与用户在 App 里点击无法区分，也区分不出用户。配置静态头后所有调用都是同一个身份，只能证明调用方服务。企业版“按用户动态取 MCP 令牌”未测 |
| V2 | 命令身份由谁分配、重试时是否稳定 | **App 分配：通过；模型分配：不通过** | App 内点击生成命令号：重发返回原回执，同号改资产判冲突，账本只有一组 adopted → dispatched → completed。模型自行生成命令号，并把来源自报为 `app`；被要求“直接再提交一次”时换了新命令号，Go 只能视为新意图，同一客户产生第二笔业务请求 |
| V3 | 哪种批准能绑定金额和对象并被 Go 核验 | **不通过** | ① MCP Apps 确认只覆盖 App 发起的调用，“始终允许”按 App 实例存在前端内存，新实例要重新批准，Go 收不到任何批准凭据；② “默认权限”且开启 A4P 时，模型发起的提交没有任何审批就执行；③ A4P 可授权动作是固定白名单（`bash`、文件读写、终端、`acp_chat` 等），自定义 MCP 工具不在内，签发时用户 ID 固定为 `jiuwenswarm-local-user`，令牌在 WorkSwarm 内部校验 |
| V4 | 状态页是否只投影 Go 的权威记录 | **通过（App 内）** | 控制台状态全部来自 Go；模型复述出错（把 10,000 USD 说成“10,000 元”）时控制台显示正确；WorkSwarm 与桩服务各重启后，新控制台仍显示全部记录 |
| V5 | 等待与晚到、重复回执 | **通过（展示层）** | 超时显示“结果未知”，查询后转终态，不重新提交；重复与迟到回执记为“重复 / 迟到被忽略”，状态不倒退。缺口：参考实现的 adopt 状态机没有 `unresolved → pending` 边（#4 §6.2 要求），只能同时显示“结果未知”和“业务方处理中”。状态更新靠 App 每 3 秒经宿主代理调用工具轮询，没有服务端推送 |

截图在 [`evidence/`](evidence)：`v2-console-conflict`、`v2-model-resubmit`（V2），`v3-default-permission`（V3），`v4-v5-ledger-unknown`、`v5-after-query`（V1、V4、V5）。

## V1 补测：企业版双令牌（模拟）

WorkSwarm 企业版能把本轮用户令牌转成 MCP 请求的 `Authorization`，并附上实例自己的 `X-Actor-Token`（`mcp_dynamic_token_patch`）。没有企业版环境，按仓内端到端测试的做法在本机模拟：

- `compose.enterprise.yaml` 叠加在 `compose.yaml` 上，用 `./enterprise-up.sh` 启动，`./ent.sh` 代替 `docker compose`；测试凭据只放在 `~/.config/aha-swarm/kc.env`。
- Keycloak 签发用户令牌（`./kc-token.sh alice`，代替 Manager 登录）和实例令牌。
- 企业版网关所需的 Redis、MySQL，以及代替 Manager 改写 `/gateway-api` 的 nginx。
- `patches/aha_sim_manager_credentials.py` 代替 Manager 下发实例凭据，并补上下面 E1、E2 两处缺口。
- Go 桩设置 `PILOT_OIDC_ISSUER` 后，用 Keycloak 公钥验签两种令牌；设置 `PILOT_REQUIRE_USER` 后，没有已验证用户令牌的提交一律拒绝。
- 浏览器以 `?user_id=…&debug_context=1` 进入，`openjiuwen_federation` 写入用户令牌（`e2e/a/ent-login.mjs`）。

| 编号 | 场景 | 结果 |
| --- | --- | --- |
| K1 | alice 让模型提交 | ✅ Go 验签得到 `user:alice`，经 `workswarm-agent` 实例 |
| K2 | 换 bob 提交，同一实例 | ✅ `user:bob`，不串号 |
| K3 | 在 MCP App 控制台里点击提交 | ❌ HTTP 传输（企业版默认）下 MCP App 无法加载（E3）；改用 WebSocket 后能加载，但 App 和模型的调用都不带用户令牌，只有模板静态头 |
| K4 | 浏览器里放伪造令牌（改成 admin） | ✅ Go 拒绝（签名不符）。WorkSwarm 不校验，原样转发 |
| K5 | 不带用户令牌 | ✅ Go 拒绝。WorkSwarm 不拦截，照常发出模板静态头 |

令牌里没有 `aud`：Go 只能验证“谁签的、是谁”，无法确认令牌是发给自己的。转发的是用户原始登录令牌。

## WorkSwarm 侧发现（待向上游报告）

| 编号 | 现象 | 部署中的临时处理 |
| --- | --- | --- |
| U1 | 网关拆为独立 `agent_gateway` 进程后不导入 `jiuwenswarm.common.schema.message`，`mcp_app.*` 不在其 `ReqMethod` 中，Web 端 MCP App 报 `unknown method: mcp_app.read_resource` | `deploy/patches/` 在解释器启动时补注册这三个方法 |
| U2 | MCP App 沙箱页地址取自 WebChannel 绑定的 socket，绑 0.0.0.0 时写成 `http://127.0.0.1:19000`；浏览器不在服务器本机时无法加载 | 绑 0.0.0.0 并映射 19000，浏览器经端口转发访问 |
| U3 | 工具标 `_meta.ui.visibility: ["app"]` 后，模型仍能直接调用（只做了“App 不能调用仅模型工具”这一方向） | 无；用 `PILOT_SUBMIT_APP_ONLY=1` 复现 |
| E1 | `probe_mcp_live_connection`（启动预热、MCP App 取连接）建连前不做动态 token 标记；连接进进程级缓存后，之后注册时虽已标记，仍复用这条只带静态头的连接，整个进程期间双令牌静默失效 | `AHA_SIM_MARK_ON_PROBE`：探活建连前先标记（候选修法）；关掉即复现 |
| E2 | Web 专家只能挂 `mcp/state.json` 里已连接的连接器，而 `record_to_mcp_entry` 转条目时丢掉 `dynamic_token_url`；`config.yaml` 里的双令牌条目在专家卡片上显示未连接、不能使用 | `AHA_SIM_KEEP_TOKEN_URL`：转条目时保留该字段 |
| E3 | 企业版前端默认 HTTP 传输，`webHttpClient` 没有 `mcp_app.*` 路由，MCP App 一律“无法加载应用：请求失败”；WebSocket 传输又不带用户令牌 | 无 |
| E4 | 企业模式 `config.get` 返回 500：`config_set_handlers` 没有 `config_get_handler` | 无（前端回落默认配置） |
| E5 | WorkSwarm Web 静态服务的 `/api` 代理读完整个响应再转发，流式回复被截住、15 秒后前端中断 | nginx 把 `/gateway-api` 直接转给 WebChannel（19000） |

## 对 #4 的含义

1. 用户面可以复用 WorkSwarm 的 MCP App 承载权威状态投影（V4、V5 通过）。
2. 业务意图不能经由模型可直接调用的 MCP 工具产生：命令号必须在可信入口分配（V2），而 WorkSwarm 当前既不向 MCP 服务传递终端用户身份（V1），也不隐藏仅 App 可见的工具（U3）。
3. WorkSwarm 现有的工具确认和 A4P 都不能充当业务批准（V3）；批准需要 Go 可独立核验的凭据，承载方式仍待设计。
4. 企业版双令牌能让 Go 验证“哪个用户、经哪个实例”（K1、K2），V1 在企业版可改判为有条件通过；条件是上游修好 E1–E3。它只回答“是谁”，回答不了“是用户点的还是模型调的”：同一轮里两者带同一个用户令牌，而 App 发起的调用目前带不上令牌（K3）。
