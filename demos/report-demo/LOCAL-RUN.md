# Windows 本地启动与逐步验证

Linux 原生部署见 [LINUX-RUN.md](LINUX-RUN.md)；两端使用同一套 `deploy/*.py` 和业务代码。实际验证范围见 [VERIFICATION.md](VERIFICATION.md)，最终交付材料入口见本文第14节。

这份文档对应 `demos/report-demo`，从 PowerShell 执行。应用全部在本机，云端只运行已有 PostgreSQL；无需在云端安装 Java、WorkSwarm、Redis 或 Docker 应用。**“进程已启动”“真实模型已返回”“完整业务已验收”是三个不同结果。** 当前实际通过与未通过的项目以 [验证报告](VERIFICATION.md) 为准，下面的预期结果用于逐项核对。

## 1. 先看清进程与端口

| 组件 | 本地端口 | 职责 / 存储 |
| --- | --- | --- |
| WorkSwarm Web | 5173 | 唯一聊天入口、规划助手、W02 编排台插件；MCP 图卡保留作调试 |
| WorkSwarm Python Gateway | 19000；内部另有 19001 | 浏览器 HTTP/WS 通道；与下表 Java Gateway 是不同组件 |
| WorkSwarm AgentServer / worker | 18092 | Python Core 驱动规划助手；隔离工作区下的会话、SQLite 等本地文件 |
| Go control | 8091 | 可信确认、采纳图、租约、正式结果、审计、报告交付；W04 独立 PG 库 |
| Java compiler / worker | 8195 | 先启动编译端，再按配置启动 BlueprintScheduler；不是第二个 Go 调度器 |
| Java Agent Bus Gateway | 8080 | 鉴权、发现与 A2A 路由，DIRECT 模式 |
| RDC | 8092 | 注册发现和权限数据；已有 `agent_rdc` PG 库，池上限 8 |
| Keycloak | 8180 | 本地演示身份服务；dev-file H2，无新增云端数据库 |
| 资料提取 Runtime | 8093 | `report-extractor`，真实模型，原生内存 |
| 经营分析 Runtime | 8094 | `business-analyst`，真实模型，原生内存 |
| 资产质量 Runtime | 8096 | `asset-quality-analyst`，真实模型 + 受限 Loop Forest |
| 写作 Runtime | 8097 | `report-writer`，真实模型；草稿与至多一次修订 |
| PostgreSQL | 云端 5432 | RDC 与 W04 分库；本机不占用 5432 |

所有演示 HTTP 服务仅绑定 `127.0.0.1`。Keycloak 是验证完整鉴权链所需，不是安装 PostgreSQL 的依赖。

## 2. 固定路径和工具

在仓库根目录打开 PowerShell：

```powershell
Set-Location D:\Code\ThirtyParty\agent-hub-alpha
$repo = (Get-Location).Path
$private = Join-Path $repo '.local\report-demo'
$py = Join-Path $repo '.local\runtime\workswarm-venv\Scripts\python.exe'
$go = Join-Path $repo '.local\tools\go\bin\go.exe'
$env:JAVA_HOME = 'D:\Software\Java\jdk-21.0.12'
$env:WORKSWARM_SOURCE = 'D:\Code\openJiuwen\jiuwenswarm'
$env:REPORT_DEMO_CONFIG = Join-Path $private 'config.json'
& $go version
& $py --version
& "$env:JAVA_HOME\bin\java.exe" -version
mvn -version
node --version
npm.cmd --version
git branch --show-current
```

本机已准备的版本：Go 1.25.0、Java 21.0.12、Maven 3.9.16、Node 24.22.1；WorkSwarm venv 使用锁文件安装，Python 3.14.6。工程分支为 `codex/report-demo-w03-w04`。不要以机器默认 Maven 缓存中的同名 snapshot JAR 代替固定源码构建：本次已遇到同版本、不同构造函数的旧制品。

本机现有隔离源码在 `.local/sources/{agent-core-java,agent-solution,agent-runtime-java}`；精确 SHA 见 [source-manifest.json](source-manifest.json)。WorkSwarm 使用上述现有源码目录，不覆盖其源码和用户家目录配置。

若换电脑，先从各官方仓库取得对应 SHA，安装这些工具，并通过 WorkSwarm 的 `uv.lock` 安装依赖：

```powershell
$env:UV_PROJECT_ENVIRONMENT = Join-Path $repo '.local\runtime\workswarm-venv'
$env:UV_CACHE_DIR = Join-Path $repo '.local\tools\uv-cache'
& '.local\tools\python\bin\uv.exe' sync --project $env:WORKSWARM_SOURCE --frozen --no-dev --no-install-project
```

该 venv 依赖中使用的 Python Core、Gateway、Runtime 版本由 WorkSwarm lock 决定，与 Java Core/Runtime 是不同制品。不要混用系统 Python。

## 3. 准备私有配置和固定材料

**本机已存在 `$private/config.json`，通常无需重新初始化，也不要把整个文件粘贴到聊天或文档。** 它含 W04 DSN、不同作用域的服务凭据和演示登录密码，不含模型密钥。

首次在新目录准备时，先由管理员准备独立的 W04 库和两类账号：迁移账号拥有建表权；应用账号只有该库的连接、表 DML、序列权限及后续迁移默认授权。不要用 RDC 账号替代 W04 应用账号。通过当前进程环境提供 `W04_APP_DSN`、`W04_MIGRATOR_DSN`，再执行：

```powershell
& $py demos/report-demo/deploy/prepare_local.py --private-root $private
Remove-Item Env:W04_APP_DSN -ErrorAction SilentlyContinue
Remove-Item Env:W04_MIGRATOR_DSN -ErrorAction SilentlyContinue
```

脚本生成随机服务凭据，下载固定公开 PDF，验证 SHA256、字节数、368 页，并抽取 PDF 第 14–16 页。已有配置会保留，不重置数据库账号或密码。重复执行用于核对材料，不会把 `acceptance/answers.json` 注入专家。

既有 PDF 的重提取命令：

```powershell
& $py demos/report-demo/deploy/prepare_material.py --pdf "$private/materials/cmb-2025.pdf" --output "$private/material.json"
```

预期：`source_digest_verified: true`、`pages: 3`。这证明固定材料字节与投影来源，不等于参考答案已经独立人工核验。

私有目录应仅供当前用户和 SYSTEM 访问。本机已设置 ACL；新机器可用原生 `icacls` 设置，先确认操作对象是完整的 `$private` 路径，再处理权限。不要把 `.local` 加进 Git。

## 4. 构建依赖与 Demo

下面是本次隔离构建采用的顺序；`.local/maven-settings.xml` 使用私有 `.local/m2`，将机器已有 Maven 仓库作为只读下载源，依赖回退到镜像。新机器应相应设置自己的 Maven 镜像/仓库路径。

```powershell
mvn -s .local/maven-settings.xml -f .local/sources/agent-core-java/pom.xml -pl core -am install -DskipTests -q
mvn -s .local/maven-settings.xml -f .local/sources/agent-solution/pom.xml -Pall -pl :agent-core-ext-orchestration,:agent-client-sdk-for-jvm,:agent-gateway,:registry-discovery-center -am install -DskipTests -q
mvn -s .local/maven-settings.xml -f .local/sources/agent-runtime-java/pom.xml -pl service/agent-service-app,service/agent-service-adapters/agent-service-adapters-agentcore -am install -DskipTests -q
& $go -C demos/report-demo/control-go build -o "$private/bin/report-control.exe" .
mvn -s .local/maven-settings.xml -f demos/report-demo/orchestrator-java/pom.xml package -DskipTests -q
mvn -s .local/maven-settings.xml -f demos/report-demo/experts-java/pom.xml package -DskipTests -q
npm.cmd --prefix "$env:WORKSWARM_SOURCE/jiuwenswarm/channels/web/frontend" ci
npm.cmd --prefix "$env:WORKSWARM_SOURCE/jiuwenswarm/channels/web/frontend" run build
Push-Location demos/report-demo/workswarm/workbench-plugin/app
npm.cmd ci --no-audit --no-fund
npm.cmd run check
npm.cmd run build
Pop-Location
```

每条命令都应退出码 0，再继续下一条。`-DskipTests` 只用于构建依赖，不是测试通过证据；Demo 测试在第 8 节单独执行。修改 Go 后先停止自己的 control 再覆盖 EXE。Java JAR 修改后也需重启对应 JVM。

WorkSwarm 插件的编译产物只留本地 `frontend/dist`，不会提交。`configure.py` 将该资源与 W02 插件装到隔离工作区的 `domain/application_plugins/credit-workbench`，无需修改 WorkSwarm 源码。更新已有运行实例时，先停止本工作区的 compiler/四专家再打包，避免 Windows JAR 占用；用 Go 临时 EXE 构建后停 control 再替换。重新提供模型环境变量，不从配置或日志找密钥。

## 5. 启动身份、编译与控制服务

先确保 `.local/tools/keycloak-26.8.2` 为官方发行包。本机已完成下载和归档校验；不需要另装外部 IdP 数据库。

```powershell
& $py demos/report-demo/deploy/services.py start keycloak
& $py demos/report-demo/deploy/configure_identity.py --private-root $private
```

`configure_identity.py` 通过真实 Keycloak 管理 API 创建隔离 `report-demo` realm、worker/admin/introspection 客户端和角色，验证真实令牌 introspection。重复运行保持服务主体和 client secret，避免已有 RDC 用户失效。

预期：`real_token_introspection_passed: true`。令牌应同时面向应用 audience 与 introspection 客户端。新版 Keycloak 要求校验客户端在 audience 中；本脚本用两个 audience mapper，不关闭验证。

RDC 首次生成配置需要已有 PG 密码，仅在当前 PowerShell 设置 `RDC_DB_PASSWORD`，通过 `RDC_DB_URL`、`RDC_DB_USER` 指向自己的实例（默认URL为本机PG）：

```powershell
$env:RDC_DB_URL = 'jdbc:postgresql://<PG_HOST>:5432/agent_rdc'
& $py demos/report-demo/deploy/services.py configure
Remove-Item Env:RDC_DB_PASSWORD -ErrorAction SilentlyContinue
& $py demos/report-demo/deploy/services.py start rdc gateway
& "$private/bin/report-control.exe" -migrate
& $py demos/report-demo/deploy/services.py start compiler control
Invoke-RestMethod http://127.0.0.1:8091/healthz
Invoke-RestMethod http://127.0.0.1:8195/healthz
```

初次 RDC 会用自己的 Flyway 初始化 `agent_rdc`。W04 的 `-migrate` 只在 `agent_hub_demo` 中执行幂等迁移。预期两个 health 返回 `status=ok`，Go health 还会实际 Ping PG。此时 `worker_enabled=false`，先不要让未完成的环境消费任务。

若以前手工启动过本工作区的 Keycloak，可先用 `Get-NetTCPConnection -LocalPort 8180 -State Listen` 找实际监听 PID，再用 `services.py adopt-keycloak --pid <核对过的PID>` 纳入管理；脚本会校验命令行属于本工作区的固定 Keycloak 目录及实际端口。当前本机的既有实例已纳入管理。启动器使用文件锁保护 PID 记录，并检查端口确实由新进程或其子进程监听。

## 6. 环境变量模型配置、四专家和 WorkSwarm

两种方式选其一。**不要在命令行参数中写实际模型密钥，不要 `setx`，不要把真实密钥写进脚本、`.env` 或文档。**

方式 A：隐藏输入启动器。本机已实测此种方式可启动模型服务：

```powershell
& $py demos/report-demo/deploy/launch_models.py --base-url '<你的兼容 API 基地址>' --model '<可用模型名称>' --private-root $private report-extractor business-analyst asset-quality-analyst report-writer workswarm-agentserver
```

看到 `Model API key (hidden):` 后输入密钥。启动器只向子进程传 `DEMO_MODEL_API_KEY`、`DEMO_MODEL_BASE_URL`、`DEMO_MODEL_NAME`。WorkSwarm AgentServer 会映射成其原生模型环境变量。它还保存模型/源码配置的**哈希**，不保存密钥、API 地址和模型名称值。

方式 B：在当前 PowerShell 以隐藏输入设置环境变量后启动：

```powershell
$modelSecret = Read-Host '模型 API Key' -AsSecureString
$env:DEMO_MODEL_API_KEY = [System.Net.NetworkCredential]::new('', $modelSecret).Password
$env:DEMO_MODEL_BASE_URL = Read-Host '兼容 API 基地址'
$env:DEMO_MODEL_NAME = Read-Host '可用模型名称'
& $py demos/report-demo/deploy/bind_profile.py --private-root $private
& $py demos/report-demo/deploy/services.py start report-extractor business-analyst asset-quality-analyst report-writer workswarm-agentserver
```

首次绑定 profile，或更换模型/提示词/源码后，重启 control、compiler 和所有专家，再采纳新图。旧图不得悄悄切换执行配置。停止后重新启动模型服务，需要重新提供环境变量；不能从历史日志恢复密钥。

`configure_identity.py` 同时生成并保持私有 `config.bus.environment_id`，发布 `bus_binding={environment_id,tenant,issuer,audience,caller_subject}`；`bind_profile.py` 检查并发布同一逻辑身份。已有工作区升级时先重新运行第5节的身份装配，再绑定当前 profile、重启相关服务。身份装配复用原有客户端；不要手工把当前环境补进旧任务记录。

任务在候选阶段冻结 Bus 绑定，用户采纳前和节点执行/发送意图前由 Go 检查；Java 在发现、首次发送和恢复查询前再次核对配置。缺绑定的历史任务仍可只读查看，执行返回 `BUS_BINDING_CHANGED`，需要生成新候选并确认。更换租户、逻辑环境、issuer、audience 或调用主体需要新候选；同环境调整 Gateway/RDC 地址、加计数代理或轮换凭据不改变绑定。token_url 必须属于明确的 issuer。环境标识用于一致性检查，不能替代 Gateway/IdP 的真实鉴权。

装配规划助手：

```powershell
$env:PYTHONPATH = $env:WORKSWARM_SOURCE
& $py demos/report-demo/workswarm/configure.py --config "$private/config.json" --workspace "$private/workswarm-home"
& $py demos/report-demo/deploy/services.py start workswarm-gateway workswarm-web
& $py demos/report-demo/deploy/configure_bus.py --private-root $private
& $py demos/report-demo/deploy/services.py start registry-heartbeat
```

装配脚本使用公共 workspace、助手和 MCP API；实际 MCP 握手后才标记连接。它删除生成模板 `.env` 中的模型示例赋值，防止上游 `override=True` 用示例覆盖进程环境；不会把实际密钥写进去。若之前已经启动 AgentServer，装配后用 `launch_models.py --restart ... workswarm-agentserver` 重启它。

原生四 Runtime 会被注册到 RDC，并通过 RDC 管理 API 给真实 worker 主体授权。`registry-heartbeat` 每 8 秒续注册；停止它后，发现结果会在可见窗口后消失，这是 liveness 行为，不是 W04 结果丢失。

## 7. 先验证连通，再验证业务

遇到 `HUMAN` 时，编排台显示“前往人工处理”，打开当前任务的 Go 可信页。该页需要独立登录；核对报告、检查摘要和依据后选择接受、修订或拒绝。处理后回到编排台刷新。人工接受保留原始 HUMAN 记录，后续批准、交付仍走现有服务端检查；不要直接改成程序 PASS。

```powershell
& $py demos/report-demo/deploy/services.py status
& $py demos/report-demo/deploy/check_identity.py --private-root $private
& $py demos/report-demo/tests/validate_services.py --private-root $private
```

预期：四张 Runtime 卡片名称正确、RDC 四个候选非空、无身份访问专家被拒绝、模型以外的控制/编译/身份/Web 都就绪。结果保存在 `.local/report-demo/evidence/service-readiness.json`。`all_passed=true` 只表示该脚本的检查通过。

打开 <http://127.0.0.1:5173/>。模型菜单应是实际配置的模型。先发：“这是一条本地模型连通性测试，请不要调用任何工具，只回复：模型连接成功。” 本次已得到真实回复和 token 用量。这个步骤不证明年报专家已经执行。

然后进入“专家 → 我的专家 → 年报协作规划助手 → 使用”。发送：

> 请使用年报协作技能，基于固定的招商银行2025年报快照，生成本集团2025对比2024的经营与资产质量简报计划。覆盖八项指标，由独立经营、资产质量专家分析后汇总，带来源与程序检查，最多一次定向修订。请先读取能力目录，排除不适用口径的能力，提交候选图给我确认。

检查真实工具轨迹：先有 `list_capabilities`，再有 `select_capabilities`（eligibility/ranking/composition/记录摘要），最后 `propose_plan`；返回真实 `T-...`、图摘要和 Java 编译摘要。不能只看模型文字画了一张图。候选必须停在 `AWAITING_APPROVAL`，模型不能批准。

点击 WorkSwarm 左侧“编排台”，选本次 `T-...`，展开选择记录、查看图与摘要，然后点击批准。在本机查看私有 `config.json` 的演示密码，只输入插件密码框，不发给模型。插件将密码交 Go 换取用户会话和 CSRF，再提交当前图摘要；WebSocket 自报用户名不会被当成批准身份。备用 Go 页面仅用于联调。

启用 Java worker：修改私有 `config.json` 的 `worker_enabled` 为 `true`，保存后执行：

```powershell
& $py demos/report-demo/deploy/services.py stop compiler
& $py demos/report-demo/deploy/services.py start compiler
```

预期真实顺序：资料提取 → 经营/资产质量并行 → 草稿 → checker → 必要时一次修订和复检 → 选择已检查报告。看节点下的正式 `result`、调用记录中的 RDC 快照/远端 task/真实产物，而非只看 `COMPLETED` 文案。

`AWAITING_ACCEPTANCE` 表示计算结束。用户在编排台展开报告，核对对象摘要与检查结果（初稿和修订稿分别展示），仅 PASS 版本可点击“批准此报告”，再“交付此报告”，最后“下载正式 JSON”。下载保持正式产物原始字节，并在浏览器复核 SHA-256。交付由独立 delivery 租约域处理，不能依赖早已结束的图执行租约。`HUMAN`、`UNKNOWN`、检查失败和失效产物必须如实展示，不视为 PASS。

### 7.1 遇到 HUMAN 如何处理

程序检查返回 HUMAN 时进入 `WAITING_INPUT`。本轮插件展示报告与状态，但尚未搬入完整人工审阅表单；这时打开备用 Go 可信页 `/ui/tasks/<原task_id>`，使用同一测试账号。可信页的“待人工核验的检查”显示原程序结果、待审阅项、检查产物 ID 和报告摘要。审阅者逐项查看原文与报告后填写实际依据，再选“人工核验接受”“要求定向修订”或“拒绝该报告”。随后点“继续”，由 Java 原图继续推进。

人工判断是独立的不可覆盖事实：记录操作人、理由、检查 ID、同一报告摘要和回执。程序产物中的 HUMAN 原样保留；UI、选择记录和审计仍能看见判断来自人工。相同请求重试复用回执，改判断或改对象会冲突；数字/结构程序失败不能通过这个入口豁免。人工接受不等于最终交付批准，仍需完成后的报告验收与交付操作。

本仓的人工处置回归使用独立测试账号和控制协议夹具，不代表某个人已经签认真实年报结论，也不替代 W01 独立人工核验。

## 8. 自动验证与故障回归

先按实际前置条件区分用例，不能把退出码 0 与大量 skip 当作完整回归通过：

| 用例 | 必需条件 | 缺失时的实际行为 |
| --- | --- | --- |
| 严格 JSON/JCS、基础/报告图准入、HTTP/MCP 拒绝、内容寻址、DTO/golden vectors、身份映射 | Go 工具及仓库文件；HTTP/MCP 测试自建本机监听 | 不需要数据库、模型或已启动的 Java |
| Go `TestPG*` | `REPORT_DEMO_CONFIG` 指向有效私有配置；其中 `dsn` 为应用账号、`migrator_dsn` 为可迁移账号，PG可达且两个账号权限正确；目录/材料等配置字段完整 | 未设置配置会 skip；配置已设置但文件无效或数据库不可达会失败 |
| Go 离线回放防篡改 | `REPORT_TEST_SNAPSHOT` 指向先前生成的真实快照 | 未设置快照路径会 skip；用例本身不连接数据库或调用模型 |
| Go COMMIT 应答切断 | 上述 PG 条件，且测试连接本来就是无 TLS | TLS配置下该用例 skip，不会为测试关闭 TLS |
| Java canonical 跨语言测试 | `REPORT_CANONICAL_BINARY` 指向实际 Go EXE | 缺 EXE 时相关用例 skip |
| Java 十个远端窗口 | 私有配置、PG、实际 Go EXE、已运行的 Java compiler 和源任务；材料按 `REPORT_TEST_MATERIAL` 提供 | 缺必要环境时相关用例 skip；使用隔离提供方，不需模型调用 |

不连接 PG 的局部检查可以单独运行：

```powershell
& $go -C demos/report-demo/control-go test ./... -run 'TestStrictJSON|TestAdmissionRejects|TestReport|TestImmutableContent|TestExportedContracts|TestCanonicalGolden|TestSharedIdentityKeeps' -count=1 -v
```

完整 Go 回归需要下面两个环境变量均准备好。数据库连接只放私有配置中，不把 DSN 或密码填进本文、提交代码或复制进终端输出。

```powershell
$env:REPORT_DEMO_CONFIG = "$private/config.json"
$env:REPORT_TEST_SNAPSHOT = "$private/evidence/restore-source.json" # 先完成 8.1；未准备时离线快照用例会 skip
& $go -C demos/report-demo/control-go test ./... -count=1 -json > "$private/evidence/go-tests.jsonl"
$env:REPORT_TEST_MATERIAL = "$private/material.json"
$env:REPORT_CANONICAL_BINARY = "$private/bin/report-control.exe"
mvn -s .local/maven-settings.xml -f demos/report-demo/orchestrator-java/pom.xml test -q
& $py demos/report-demo/tests/canonical_probe.py --binary $env:REPORT_CANONICAL_BINARY --report "$private/evidence/canonical-python.json"
& $py demos/report-demo/tests/p0_probe.py --config "$private/config.json" --report "$private/evidence/p0-technical.json"
& $py demos/report-demo/tests/identity_probe.py --private-root $private
```

Go 测试使用独立 `test-...` scope，实际连接 PG；会构造控制协议夹具和故障，不能计为真实业务专家结果。未设置 PG 配置时测试可能 skip，不能只用退出码 0 宣称数据库测试通过。

共用同一 PG 实例时，Go/PG全量与Java跨进程测试按上面的顺序串行执行；两者都会执行迁移和构造隔离环境。并发DDL/测试环境清理不作为已验证保证，失败应保留日志后区分业务断言与测试设施问题。

JSON日志中应核对 `Action=fail` 和 `Action=skip`，当前完整 Demo 口径为二者均为0；TLS等环境差异导致的预期skip须单列。Java查看 Surefire XML 的 tests/failures/errors/skipped，不能只看构建成功。

这些回归覆盖同 command 的并发去重、同节点正式结果唯一、改载冲突、过期租约不能续命、迟到/冲突观察、强制审计失败时业务回滚、延迟验收独立交付、跨任务提取复用、文件篡改和撤销传播。Java 测试覆盖真实固定材料上的 W01 12 个正反例、协议 Part 解析、HTTP 409 分类与 schema。具体数量和最后一次运行结果见实际报告。

另有原程序 HUMAN 不改写、人工审阅对象绑定/不可覆盖、审阅后独立交付及检查撤销回归。`identity_probe.py` 通过真实 IdP/RDC 验证四专家合法正例、伪造主体拒绝、伪造令牌拒绝、未授权目标拒绝，不用“一律拒绝”充当鉴权通过。

`p0_probe.py` 使用预制的技术图验证 MCP/编译/确认/PG，最后会暂停该夹具，避免真实 worker 消费。它明确 `real_model_planning=false`；不能替代第 7 节的真实对话。

Java `RemoteWindowIntegrationTest` 额外需要有效 `REPORT_DEMO_CONFIG`、Go EXE、已运行的本工程 Java compiler，以及已经创建的源任务。默认读取本次主任务，也可通过 `REPORT_FAULT_SOURCE_TASK` 指定自己的模型候选任务。它启动独立本机 Go 进程和独立 `fault-...` PG scope，通过隔离 HTTP 提供方注入断点；进程和临时配置在结束时清理，不停止主演示服务，不调用模型。这些测试不是实际 Gateway/Runtime 重启或独立业务核验；对应真实身份/主链另见第 6–7 节。

缺必要环境时相关 Java 集成用例会 skip，不能只看 Maven 返回 0。协议与 10 组 golden vectors 见 [contracts/README.md](contracts/README.md)。Java 通过实际 JSON 解析/序列化后核对 Go 的权威 JCS，不另造 Java JCS；原生 runtimeSpecDigest 单独保留。

真实任务上的恢复验证，应先记下 task_id、每个 node 的 input_digest/attempt、call_id/message_id/context_id/remote task_id 和正式 receipt，再执行故障。不要通过新建任务掩盖恢复失败：

1. **暂停**：可信页点暂停，已接收远端任务可继续产生观察；不再准入新节点。恢复后沿用原任务。
2. **Java worker 重启**：停止 compiler，等待旧许可到期，再启动；已有正式结果返回 REUSE，已有远端 task 只 GetTask，不重新 SendMessage。
3. **Go 重启**：只重启 control；PG 正式事实保留，新 control session 不能沿用旧会话许可。
4. **结果不明**：保留 UNKNOWN 和原 call 身份，禁止自动换 message_id 重发有副作用操作。
5. **撤销**：使用受信 operator 入口撤销测试产物，检查后代和复用任务变成 NEEDS_REVIEW，原交付下载被拒绝。不要撤销唯一的正式演示样本后仍期待它可下载。

上述真实进程故障是否已执行，必须在验证报告中逐项记录；本节给出步骤不代表全部已测。

### 8.1 备份、离线回放与隔离恢复

以下使用已构建的 Go EXE。先创建证据目录，再备份当前 scope；备份含表记录和实际不可变内容，不含模型配置/密钥和登录会话。

```powershell
New-Item -ItemType Directory -Force "$private/evidence" | Out-Null
& "$private/bin/report-control.exe" -config "$private/config.json" -backup "$private/evidence/state-backup.json"
& "$private/bin/report-control.exe" -config "$private/intentionally-missing.json" -replay "$private/evidence/state-backup.json"
```

第二条故意不给有效配置。预期 `VERIFIED_OFFLINE`，模型/Runtime/发布/数据库调用均为 0；它核验原记录和投影，不重跑业务，也不补发批准或报告。

完整恢复验证需要**不同且为空**的 `*_restore_test` 数据库和不同内容目录。已有本次测试库 `report_demo_restore_test` 已含恢复结果，不能再次当空库覆盖。首次准备新环境时，可用下列窄维护命令在现有 PG 容器中创建固定测试库（系统 Python 需已有 paramiko，SSH 密码隐藏输入，不落文件）。命令不删库、不重装或重启 PG：

```powershell
python demos/report-demo/deploy/postgres_admin.py create-restore-db --private-root $private --host <PG_HOST>
& $py demos/report-demo/tests/restore_probe.py --private-root $private --binary "$private/bin/report-control.exe"
```

首次脚本会从私有 migrator DSN 在内存中派生隔离目标，备份后恢复并再次读回，比较全部 scope 表、内容字节、材料/目录和任务投影。预期 `all_passed=true`，不启动恢复库上的应用或发布器。若首次中途失败，先核对隔离目标，禁止删数据后盲目重跑。

本次已成功恢复的结果可直接只读复验：

```powershell
& $py demos/report-demo/tests/restore_probe.py --private-root $private --binary "$private/bin/report-control.exe" --verify-only
$env:REPORT_TEST_SNAPSHOT = "$private/evidence/restore-source.json"
```

它保留原 `restore-source.json`，不再次导入行、不覆盖正式库。离线包包含报告正文和审计，不随 Git 提交。

### 8.2 验证 Go 真实重启后身份和游标保持

对已经计算结束的测试任务执行；会短暂停止本工程记录的 control，不停止 Java/Runtime/PG：

```powershell
& $py demos/report-demo/tests/control_restart_probe.py --private-root $private --task-id T-d2c511867ab929cbfe371cf5057de6b6
```

预期新 `control_session_id`，任务投影/远端调用身份/持久事件相同，数值游标递增且续读无重复，内容完整。该脚本为清晰比较只接受事件少于 200 条的已结束任务；不是任意活动任务的故障注入器。结果见 `evidence/control-restart.json`。

### 8.3 PG 槽位耗尽与心跳恢复

先诊断，不按端口可连就认为数据库能接受新连接：

```powershell
python demos/report-demo/deploy/postgres_admin.py diagnose --private-root $private --host <PG_HOST>
```

仅当确实是本 Demo 遗留 idle 连接时，先停本工程 `registry-heartbeat rdc control`，确认没有本机 PG 连接，再运行 `recover-idle`；随后启动 RDC/control/heartbeat 并重跑就绪检查。维护脚本拒绝在本工程 RDC/control 仍活着时清理，只针对指定 Demo 库和角色的 idle 后端；不终止活动事务，不变更公网安全配置。本次已经给 Demo 角色设置服务端 TCP keepalive 60/10/3 秒，连接池仍为 RDC 8、Go 4，PG max_connections 30。

注册心跳的当前事实写入 `heartbeat-status.json`，失败会退避恢复，404 时重注册实际 Runtime 卡片；不能只看进程仍活着。

### 8.4 真实修订与实际进程重启

以下是有意注入故障的联合验证，使用真实模型，会重启本工程记录的 worker/指定本地服务。先确保没有其它活动任务；脚本拒绝在 pending 非空时开始，不重启 PG，也不以新任务冒充原任务恢复。

```powershell
& $py demos/report-demo/tests/revision_probe.py --private-root $private --source-task T-d2c511867ab929cbfe371cf5057de6b6
& $py demos/report-demo/tests/runtime_restart_probe.py --private-root $private --source-task T-d2c511867ab929cbfe371cf5057de6b6 --service business-analyst
& $py demos/report-demo/tests/runtime_restart_probe.py --private-root $private --source-task T-d2c511867ab929cbfe371cf5057de6b6 --service gateway
```

修订脚本仅修改目标 draft0 的回包单位，保留真实原回包与注入回包，检查 REVISE→真实 writer 修订一次→复检 PASS；不将注入错误写成模型自然错误。Runtime 重启需要在隐藏提示中输入模型密钥，只传子进程环境；Gateway 重启不需要模型密钥。观察代理只记录方法、身份和错误，转发字节不改动。脚本结束恢复原 Gateway 配置并启动原 worker；未知任务暂停留证，不盲目补发。

这几份已验证的历史任务绑定旧执行版本；源代码/执行配置改变后应使用新版本真实规划候选作为 source-task。原任务和报告仍可只读核验，不能热改旧绑定。

### 8.5 规划助手工具边界

安装助手的 configure.py 会通过 WorkSwarm 原生 RailManager 安装版本化策略。已安装的工作区可单独更新策略，不改用户模型配置：

```powershell
$env:PYTHONPATH = 'D:/Code/openJiuwen/jiuwenswarm'
& $py demos/report-demo/workswarm/install_policy.py --workspace "$private/workswarm-home"
& $py demos/report-demo/tests/planner_policy_probe.py --report "$private/evidence/planner-ability-policy.json"
```

随后用模型环境/隐藏输入启动器重启本工程 AgentServer。策略只保留五个 report-orchestration MCP 工具、skill_tool 和 ask_user，隐藏并拒绝文件、shell、其它 MCP 与后注册的无权工具；UI 的“完全访问权限”不撤销这层能力限制。MCP 服务配置须保留给宿主完成连接，模型可见工具和执行仍按精确名称过滤。原生边界不等于进程或用户权限隔离，不能把此测试当作完整沙箱证明。

首次启动时等配置加载完成再使用。如果模型列表初次请求在 worker 尚未就绪时超时，待 worker 就绪后刷新页面；不要因此重新录入或覆盖已有密钥。

### 8.6 只读复核本次完整模型链

本机最新主任务为 `T-202e02a56e2275a0aa62837e51d79969`，执行绑定 `d7576071…`，测试账号已交付。下面只导出和检查已有事实，不再次调用模型或交付：

```powershell
& $py demos/report-demo/tests/export_trace.py --private-root $private --task-id T-202e02a56e2275a0aa62837e51d79969
& $py demos/report-demo/tests/task_probe.py --private-root $private --task-id T-202e02a56e2275a0aa62837e51d79969 --action download
& $py demos/report-demo/tests/verify_main_flow.py --private-root $private --task-id T-202e02a56e2275a0aa62837e51d79969
```

预期 `all_passed=true`、10 项为 true：四个独立真实专家/remote task、两条完成的 Loop 分支、同 c/h/a 的完成回包与正式正文一致、同报告摘要的程序检查/测试账号批准、重复交付身份、下载正文及原始字节 SHA-256 一致。独立人工核验仍待完成。对自己的新任务，应先完成第 7 节操作并导出 `-trace/-delivery/-report.json`，再替换 task-id。

### 8.7 留存实际依赖解析与制品来源

这一步不打包、不覆盖正在运行的 JAR，串行记录当前四个 Maven 模块的 effective POM 和 dependency tree、Go 实际导入包、WorkSwarm/Go 依赖锁摘要，以及实际二进制与 Demo 源文件摘要：

```powershell
& $py demos/report-demo/tests/freeze_build.py --private-root $private --maven-settings .local/maven-settings.xml
& $py demos/report-demo/tests/release_audit.py --private-root $private
```

输出位于 `evidence/build-resolution` 和 `evidence/source-and-artifact-audit.json`，保留在私有目录。依赖解析使用与构建相同的私有 Maven settings，不导出 settings/凭据；当前解析记录不能替代独立 clean rebuild。限定凭据扫描也不等于覆盖所有日志或所有秘密类型。

### 8.8 隔离网络效果故障

测试会自建随机端口的隔离 HTTP 提供方与独立 PG scope，无需修改日常配置或启动银行模拟服务。正常控制实例仍不注册测试效果 API。

```powershell
& $go -C demos/report-demo/control-go test ./... -run 'TestPGIsolatedEffect|TestIsolatedEffectConfig|TestPGDeliveryPolicyChange' -count=1 -v
```

逐窗口步骤、计数和范围见 [验证报告](VERIFICATION.md)。有界网络计数器效果不是银行实测，也不替代真实专家链。

### 8.9 Windows 原会话恢复

本机原长会话恢复失败已定位到 stat/fstat 的 ctime 字段来源差异。启动器对本 Demo 的 Windows AgentServer/worker 自动启用经过来源校验和运行时探测的局部兼容层；既有历史不改写，原恢复拒绝与游标检查保留。

探测命令、源码 pin、反例和真实原会话重启步骤见 [验证报告](VERIFICATION.md)。不要再通过删历史或重新录入模型来处理这个已定位的问题。

Windows 兼容层仅适配固定上游源码，保留原历史校验；未知源码或其它元数据差异时拒绝启动，不可通过删除历史绕过。可执行不调用模型的探测：

```powershell
& $py demos/report-demo/tests/history_compat_probe.py --workspace "$private/workswarm-home" --report "$private/evidence/windows-history-compat-local.json"
```

需要检查既有会话时追加 `--session <自己的会话ID>`。跨重启验证应先留存原任务/历史基线，再在无在途请求时用第6节隐藏输入启动器重启 AgentServer，在原会话查询原 task_id；核对新请求的实际工具返回，不能把旧页面文本当恢复证据。范围与已记录案例见 VERIFICATION。

### 8.10 正式提交后的矛盾回执

先按第 8 节设置 `REPORT_DEMO_CONFIG`。下面使用独立 PG 测试 scope 与控制协议夹具，不调用模型、不改已交付主任务，也不需要重启服务：

```powershell
& $go -C demos/report-demo/control-go test ./... -run 'TestPGLateRemoteConflict|TestPGCompletedRemoteCallIgnoresLateNonterminalOrder' -count=1 -v
```

预期两个顶层测试通过：不同完成正文/互斥终态都会使源结果及跨任务复用后代失效，旧/新交付和已鉴权下载被拒，原文件不变；必要审计失败时整笔事务回滚；重复事件去重；晚到非终态不误撤回完成事实。测试报告见 VERIFICATION 的“最后核心修复”。其它非核心后续工作见 [验证报告](VERIFICATION.md)。

### 8.11 共享契约（PR #25）与真实消费者

PR #25 原有类型和 schema 保留在 `agent-festival` 原目录，Demo 的 Go replace 仍指向该模块；Demo 专用适配、生成资源与测试位于本应用内，不需要额外 PR25 目录或服务。构建后执行原有 `-migrate`，会应用 `002_pr25_identity.sql`；它保留旧任务和结果。

```powershell
& $py demos/report-demo/deploy/embed_identity_schema.py
& $go -C demos/report-demo/control-go test ./... -run 'TestGraphIdentity|TestEmbeddedIdentity|TestReconciliation' -count=1
& $go -C demos/report-demo/control-go test ./... -run 'TestPGSharedIdentity|TestSharedIdentity|TestExportedContracts' -count=1 -v
```

共享模块完整测试使用 Linux；上游部分测试依赖 Unix 权限/Umask，不以 Windows 下跳过这些断言声称全量通过。实际结果见 VERIFICATION。

用已生成的候选图做一次真实专家集成复跑（不是新一轮模型规划）：

```powershell
& $py demos/report-demo/tests/shared_contract_probe.py --private-root $private start --source-task T-202e02a56e2275a0aa62837e51d79969
& $py demos/report-demo/tests/shared_contract_probe.py --private-root $private status
```

新环境须将 source-task 替换为自己按第 7 节生成的任务。等待程序检查 PASS 后，按第 7 节对新 task_id 完成测试账号交付，再执行：

```powershell
& $py demos/report-demo/tests/shared_contract_probe.py --private-root $private verify
```

预期 16 项通过：每个正式结果具有实际 T/g/节点路径/e，输入相同可读取原结果、把回执当输入时不允许复用，对账不改变任务版本。本机记录任务为 `T-7853c55c66499997c4597c83e3868949`，DELIVERED。真实主链的 10 项核对仍按 8.6 的导出/下载/verify_main_flow 操作，替换为该任务。

## 9. 常见问题

| 现象 | 先检查 |
| --- | --- |
| 页面长期“加载配置中” / WS 错误 | 5173 只是静态页；检查 19000 Gateway、18092 AgentServer、worker 日志是否就绪，再刷新页面 |
| 显示 `your-model-name` 或请求示例域名 | 生成的模板 `.env` 覆盖了模型环境；运行助手装配并重新启动 AgentServer，或核对用户已设置的实际模型 |
| Keycloak introspection active=false | 检查 token audience 包含 introspection 客户端；不要关闭校验 |
| RDC `audience does not match` | 应用 audience 与校验客户端 audience 用独立 mapper；重新签 token，核对 identity 配置 |
| RDC 管理接口 400 | 必须有 `X-Tenant-ID: demo`，写操作还须真实 admin 身份、Idempotency-Key、适用时 expectedRevision |
| RDC 候选为空 | 四 Runtime 卡片是否可达、注册成功、heartbeat 是否仍运行；不是把服务 URL 手写进规划图 |
| Gateway 401/403 | 核对 Keycloak 身份、RDC user/role/agent 许可；不要改成全允许 |
| Compiler NoSuchMethodError | 使用固定源码重建到私有 Maven 仓库；不要把代码改成迎合机器上的旧 JAR |
| Address already in use | 用 `Get-NetTCPConnection` 查 owning PID，再确认命令行。只停止本 Demo 记录的进程；Java/Python launcher 可能还有子进程 |
| `BINDING_CHANGED` | 模型、源码、目录或策略改变；重新生成绑定并采纳新图，不热改旧任务 |
| 文件证据丢失 / 被改 | 从同一备份恢复并校验摘要；不能重新生成不同字节后假装是原产物 |
| PG 或网络 503 | 保留原 command/node/call/delivery 身份，恢复后查询事实；503 不等于“事务肯定没提交” |
| 旧图卡 Failed to fetch / 授权过期 | 在对话中对原 task_id 调用 run_status 获取新只读图卡；不需要新建业务计划 |
| 常驻后 WorkSwarm“正在启动”或 HISTORY_RESTORE_CHANGED | 先区分 IPC 断开和历史校验失败。本机 stat/fstat 误报已有局部兼容修复，运行 history_compat_probe 并核对当前进程启动标记。其它原因仍按原拒绝处理，不删历史、不跳过检查；详见本文件第8.9节 |

日志位置：`.local/report-demo/logs`、隔离 WorkSwarm workspace 的 `run/worker-logs` 和 `agent/.logs`。不要把日志原样提交到 Git；不要输出环境变量全集或私有配置。

## 10. 停止与再次启动

```powershell
& $py demos/report-demo/deploy/services.py stop registry-heartbeat workswarm-web workswarm-gateway workswarm-agentserver compiler control report-extractor business-analyst asset-quality-analyst report-writer gateway rdc
& $py demos/report-demo/deploy/services.py stop keycloak
```

进程管理器核对 PID 创建时间，只停止自己记录的进程和子进程；不删除 PG、材料、结果、会话或 IdP 数据。历史手动启动且尚未登记的进程需先核对命令行，再单独停止，不能按 `java.exe` / `python.exe` 全杀。

再次启动复用私有配置和数据库，按“Keycloak → RDC/Gateway → compiler/control → 模型服务 → WorkSwarm → 注册/heartbeat”执行。模型凭据重新从环境变量或隐藏输入提供。不要重新安装云端 PG，也不要删除本地证据来消除失败记录。


### 8.12 W03/W04 发送与版本绑定回归

```powershell
node --test demos/report-demo/tests/projection.test.cjs
& $go -C demos/report-demo/control-go test ./... -run 'TestPGSend|TestPGConcurrentSend|TestPGReleaseDrift|TestDiscovery' -count=1 -v
& $py demos/report-demo/tests/export_handoff.py --private-root $private --task-id <实际任务ID>
```

Go专项仍需要上面的真实PG配置。Java完整测试额外覆盖重复GRANTED许可、受理应答丢失、意图落库未发送即强杀Go后保持UNKNOWN/零重发、发现期间强杀Go并接管、换执行profile后强杀重启；使用实际Go进程和PG、隔离HTTP提供方，不能记为真实Agent Bus故障签收。页面测试执行实际HTML中的脚本与共用projection.js，使用可控DOM/网络替身；不等于浏览器视觉或断网验收。

更新本轮版本时，先暂停/收敛活动任务，再更新制品；设置已有模型环境变量，运行bind_profile.py，并重启control/compiler/四个专家。configure_bus.py从cfg.catalog_path读取remote_binding版本用于注册。新目录/执行profile会使旧任务执行返回BINDING_CHANGED；不要修改旧任务或旧批准来绕过，应生成新候选并重新确认。既有已交付报告只读可核对，不因此重新调用模型。

calls创建响应新增action，与Java须成套升级；缺失或未知action时新Java停在UNKNOWN，不按GRANTED盲发。手工重试也不能从持久调用中恢复SEND。交接文件写入本地evidence，需独立脱敏审核后才对外分享。


若明确需要将历史候选拓扑用于当前目录，须创建新任务并重新确认：

```powershell
& $py demos/report-demo/tests/shared_contract_probe.py --private-root $private start --source-task <历史任务ID> --rebind-current-catalog
```

此参数只显式更新新候选的目录摘要，不修改源任务、不自动替换策略版本；策略变化时仍拒绝，要求审阅新图。它不是新一轮对话规划，验证报告须注明 fixed candidate / fresh_model_planning=false。

## 13. 本轮受控集成的复跑核对

1. 按第4—6节构建并启动当前源码；更换源码/模型后重新执行 `bind_profile.py`，重启 control/compiler/四专家，重新装配助手及插件，并重启 WorkSwarm AgentServer/Gateway。四专家本身会读 ENV 校验实际模型绑定，不能只保留旧 profile 换模型启动。
2. 按第7节从“专家 → 我的专家 → 年报协作规划助手 → 详情 → 使用”新开对话。当前 WorkSwarm 构建的卡片悬浮“使用”按钮可能被头像挡住，详情页的“使用”入口已实测。不复用历史任务冒充新规划。
3. 新候选应带 `graph.selection={id,digest}`；选择记录在同一 PG 的 `selection_records`，母公司高分仍排除，未知不通过。当前只开放“经营”和“经营+资产质量”，不是通用知识平台或语义召回。未经用户确认调用数必须为0。
4. 从编排台批准，观察四专家和实际 Bus task。可在前驱已提交、后继实际运行时停止本工作区 compiler，等待租约到期后重启；原任务应查询原 remote task，已完成节点新增调用为0，不重新规划。需要显式暂停时，输入演示密码点“暂停新节点”；等待状态可从原任务恢复。BLOCKED 必须核对绑定原因，不能普通恢复。
5. 完成后按第7节从插件批准/交付/下载。写作至多修订一次，可能有4或5次远端调用，但始终是4位独立专家。程序 PASS、测试账号交付和独立业务验收分别记录。

对某个真实新对话做只读核对（替换为自己的会话/任务ID）：

```powershell
$env:PYTHONPATH = $env:WORKSWARM_SOURCE
& $py demos/report-demo/tests/verify_planning.py --private-root $private --session '<WorkSwarm会话ID>' --task-id '<T-ID>'
& $py demos/report-demo/tests/export_trace.py --private-root $private --task-id '<T-ID>'
& $py demos/report-demo/tests/export_handoff.py --private-root $private --task-id '<T-ID>'
& $py demos/report-demo/tests/model_process_probe.py --private-root $private --java "$env:JAVA_HOME/bin/java.exe"
Push-Location demos/report-demo/workswarm/workbench-plugin/app
npm.cmd test
$env:BROWSER_EXECUTABLE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
& $py test/workbench-browser.py
Pop-Location
```

`model_process_probe` 用真实专家JAR和本地模型出口替身，验证直接启动时“旧profile+ENV模型B”“缺绑定”均在出口前拒绝，不能拿它代替真实业务模型测试。前三条只读命令不做批准/交付；`verify_main_flow.py` 还需要 `task_probe.py --action download` 和已记录的测试账号交付证据。公开接收样例不含凭据、完整模型返回和思考链；异机复跑和业务签收仍由实际接收人完成。

新增备份使用 `report-demo-backup/2`，包含选择记录；旧 `/1` 快照仍可读取/恢复，不为旧任务补写模型或选择绑定。旧任务可以读历史，新执行不能偷偷采用当前默认模型。

## 14. 接收配置与最终材料入口

W03/W04 提供方为 yougq；W02 WangHaowen99、W05 xierongqiag2015 已确认年报接收，W06 wanshoulu、W07 tonyxu2019 负责各自工作包和集成组织。以下是部署材料索引，不代替独立接收或业务签收。

| 材料 | 入口 / 内容 |
| --- | --- |
| 代码、依赖、来源差异 | source-manifest.json；最终采用源码 SHA 与平台制品哈希分别记录，同名0.1.3 JAR不代表同一源码 |
| Windows / Linux 构建启停 | 本文与 LINUX-RUN.md；同一 services.py，Linux使用 java/kc.sh/report-control，Windows使用 java.exe/kc.bat/report-control.exe |
| 配置与权限 | W04 config.json、角色 experts/*.json、identity.json 和 rdc/gateway.properties 仅在私有根；字段和权限见下表 |
| 持久化、恢复 | 第8.1节和Linux文档第6节；W04 PG与artifacts配套，RDC、IdP、WorkSwarm分别保存，不声明整环境灾备已测 |
| 控制/结果契约 | contracts/README.md、对应schema、错误分类与SEND/QUERY/UNKNOWN规则 |
| 跨模块贯通样例 | evidence/integration-sample.json；原始记录留私有环境；实际结果与未覆盖项见VERIFICATION.md |

| 配置字段/变量 | 来源与用途 |
| --- | --- |
| config.json: dsn / migrator_dsn | 管理员提供的W04应用/迁移账号；仅访问独立W04库，不能借用RDC账号 |
| RDC_DB_URL / RDC_DB_USER / RDC_DB_PASSWORD | 管理员提供的RDC库；实际已测PG16.16，首次Flyway初始化。没有额外手工插入业务表步骤 |
| JAVA_HOME / WORKSWARM_SOURCE | 当前平台原生JDK与固定WorkSwarm源码；可选GO_BINARY、KEYCLOAK_HOME、SOLUTION_SOURCE覆盖工具/源码路径 |
| REPORT_DEMO_CONFIG / --private-root | 当前运行环境私有根；其artifacts/material/catalog路径必须属于当前OS，不共用Windows/WSL的PID和构建目录 |
| config.json: tokens / user / user_password | prepare_local生成的独立planner/worker/operator/expert凭据和测试用户，用户动作另验Cookie/CSRF/Origin |
| identity.json / config.bus | configure_identity通过真实IdP生成客户端主体/秘密，RDC登记前必须可introspect；不将x-user-id当作独立身份凭证 |
| config.bus.gateway_url / rdc_url | Java真实调用及发现地址；允许Gateway前加独立传输计数代理，配置后重启compiler，代理须保持请求语义和关联标识 |
| DEMO_MODEL_API_KEY / BASE_URL / NAME | 只从启动进程环境读取；实际变量全名为DEMO_MODEL_API_KEY、DEMO_MODEL_BASE_URL、DEMO_MODEL_NAME |
| DEMO_MODEL_MAX_TOKENS / NORMAL_ITERATIONS / QUALITY_ITERATIONS | 全名分别为DEMO_MODEL_MAX_TOKENS、DEMO_MODEL_NORMAL_ITERATIONS、DEMO_MODEL_QUALITY_ITERATIONS；默认12000/4/8；进入专家实际模型绑定，改变需重新发布 |

Gateway `gateway.authorization.enabled=true`，RDC `openjiuwen.authorization.enabled=true`。Bus调用主体是worker的真实服务账号：Java携带Bearer与X-User-ID，Gateway传递原身份，专家introspection检查active/client_id/issuer/audience/subject。专家之间不另经Bus互调；资产质量Loop分支是该节点内的Core调用。

configure_bus将主体绑定report-demo-executor角色，并给该角色授予四个目标Agent的访问；它不是逐方法授权编辑器。四专家共用report-experts.jar，各自角色配置/AgentCard/端口和实例登记独立；构建摘要给一个公共JAR哈希加四角色对应表，不捏造四个不同JAR。RDC/Gateway和Keycloak来源校验见source-manifest；归档摘要与实际运行制品摘要分别记录。

WorkSwarm主规划使用其固定版本配置/模型设置，不继承专家12000/4/8预算。当前Demo未设置主规划max_tokens覆盖；部署方核对其实际模型配置和react迭代预算。GLM等推理模型换端点/模型/预算后产生不同model_binding_digest是预期；兼容性须实测，不把DeepSeek成功当作GLM已过。

WorkSwarm端点方言可通过DEMO_MODEL_ENDPOINT_PROFILE指定，默认deepseek，其他兼容服务选择其支持的openai等方言。启动器为三个WorkSwarm进程都提供环境模型引用，避免全新工作区的前端模型目录为空；隔离工作区安装时关闭首次配置教程，不修改用户全局配置或持久化模型密钥。

新增真实浏览器复跑入口（实际调用模型并执行测试用户确认、批准和交付）：

```powershell
& $py demos/report-demo/tests/live_smoke.py --private-root $private --browser-executable '<本机Chromium/Edge路径>'
```

若浏览器在报告完成后断开，可显式传`--resume-task <同一次smoke的T-ID>`，只重连本私有根已有记录的原任务，不新建图或重新调用专家。验证报告保留断开和重连记录，不把它说成全程无中断。新任务规划允许模型修正被拒绝提案；verify_planning按成功工具回执的taskId、tool_call_id和最终图关联，拒绝把同selection的失败提案计成成功。

完整HUMAN表单暂由同一Go可信备用页承接；PR36的独立批准/任务存储不接入正式链。W06检查结论作为业务事实接入前，须统一招行材料、完整报告摘要与检查投影摘要、错误结构和实际框架依赖，不能把外部具名字符串当可信用户批准。W07最终材料直接引用已有版本文件；外部接收和业务签收结果由本人/接收人回填。
