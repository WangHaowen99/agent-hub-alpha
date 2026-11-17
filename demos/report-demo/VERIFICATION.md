# Demo 验证报告（持续更新）

核验日期：2026-11-17，北京时间。早期提交标识属于本地验证历史；发布代码按来源清单及文件/制品摘要核对，不保证每个早期提交都已推送。分支：`codex/report-demo-w03-w04`；原始实现来源基线：`8dc5f44ce4e94211375f207c7088276dc523d038`；本轮对照 main `4021491`。本文记录实际执行与尚未完成的原方案门禁，**不是完整方案验收通过声明**。逐步操作见 [LOCAL-RUN.md](LOCAL-RUN.md)，依赖固定版本见 [source-manifest.json](source-manifest.json)。

按用户本次“核心继续验证、非核心留交接”的要求，当前年报 Demo 核心链已完成实现与验证：真实对话规划、四个真实专家、持久结果、程序检查、受信测试账号交付，以及关键失败不假成功。非核心工作及其完成依据集中在 [验证报告](VERIFICATION.md)。独立人工业务验收与外部 issue 的完整联合门禁仍未完成，不因本轮核心收敛而自动通过。

## 当前双平台结果：2026-11-17

实现提交 `f657d9fa31eedb39b5c9fadcd09a928a0c5ab790`。同一套 Go/Java/Python/WorkSwarm 插件代码，分别在 Windows 和本机 WSL2 的独立 Linux 工作区构建、启动和执行真实模型任务。浏览器客户端均为 Windows Edge；Linux 验证的后端、静态前端构建和服务均由 Linux 原生进程承担，未借用 Windows exe。测试前停止 Windows 实例，并核对 Windows 客户端与 WSL 内部返回同一 Go control_session_id，排除误连旧服务。

| 项目 | Windows | Linux / WSL2 |
| --- | --- | --- |
| 环境 | Windows x64；Oracle JDK21.0.12；Python3.14.6 | Ubuntu26.04.1 x86_64；OpenJDK21.0.13；Python3.14.4 |
| 共用工具/数据库 | Go1.25.0、Maven3.9.16、Node24.22.1、PostgreSQL16.16 | 同左；独立数据库/运行目录，原生工具链 |
| 新任务 | `T-30fcc5df8694b705c1b80a1d14eaa740` | `T-10a3e5de04d794c8e6e18c59a6d7b294` |
| 新对话 | `web_1a1e984e803_96636623157b` | `web_1a1e976619f_867fa611a064` |
| 主链 | 新模型规划→插件确认→4专家/4远端task→初检PASS→批准→DELIVERED→原字节下载 | 同左 |
| 正式节点/有条件跳过 | 6 / 2；未发生修订，不把跳过节点记为正式结果 | 6 / 2；未发生修订 |
| 主链/规划只读核对 | 10 / 6 项通过 | 10 / 6 项通过 |
| PR25身份与对账核对 | 16项通过，6个正式结果有结构化身份 | 同左 |
| Go/真实PG | 47顶层、93含子用例；0失败、0跳过测试；vet通过 | 同左 |
| Java编排/专家 | 26 / 6；0 failure/error/skip | 同左 |
| 服务/真实身份检查 | 16项就绪、6项身份正反例通过 | 同左 |

Go日志另外包含selection包的`[no test files]` package事件；这是无测试包，不计为跳过的测试。Java故障窗口使用真实Go进程/PG和隔离HTTP提供方，不计为真实业务模型调用。两端真实主链和隔离故障证据分别记录。

Windows报告摘要 `sha256:88bbe30afcd47c87d28523c816ff0ca663e5b49d1692e1186e90c9c1257c33bc`；Linux报告摘要 `sha256:9332d816d604d632213a5826e15cab61b5cc9c5e61c75ab048b2d6efbb313735`。摘要最终以[脱敏样例](evidence/integration-sample.json)的`current_platform_validation`实际字段为准。两端分别保留原始下载字节、任务、调用、观察和规划工具记录；相同delivery_id重复交付返回原receipt_id，主链10项独立核对通过。

两端直接启动实际专家JAR，模型名漂移、预算漂移、缺绑定三例均在模型出口前拒绝，本地计数出口请求为0；4个跨语言预算golden向量通过。Core真实客户端测试验证所校验的max_tokens确实进入请求。该隔离出口测试不代替真实Bus上共同M1—M4验收。

实际Go重启测试验证新control_session、相同正式投影、原call/task关联、审计游标无重复和产物完整性。它不把内存Runtime重启后的远端任务恢复推定为已完成；原有故障用例和尚缺组合继续单列。

### 本轮发现并处理的问题

1. Windows先行任务`T-3b5d94d4c2bc6d19f232053df41ddb0b`的写作JSON在6000-token上限处截断，按预期FAILED，未提交半份报告。保留失败记录；默认预算改为12000，允许受限环境覆盖并进入真实模型绑定，写作输出改为紧凑格式。旧任务不改写，重新发布后用新任务验证。
2. 原浏览器测试按折叠区内不可见按钮定位报告，曾在一个已PASS任务上超时；改为用权威artifact_id定位details后展开。最终上表Windows/Linux任务均完成完整单次浏览器主链；另保留原浏览器断开后重连原任务成功的记录。
3. 全新Linux工作区的模型配置教程遮挡已安装助手；Web/Gateway缺少环境别名时也不能读取环境模型目录。安装器只关闭隔离工作区教程，启动器向三个WorkSwarm进程传同一环境模型引用，不将实际密钥写入配置或源码。
4. Gateway已知身份错误、越权、依赖不可用不再全部折叠。真实Go/PG集成验证AUTH_EXPIRED→WAITING_AUTH、CALLER_NOT_AUTHORIZED→BLOCKED、RDC_UNAVAILABLE→WAITING_REMOTE/UNKNOWN，保留分类且已有意图不盲目重发；失败自由文本不进入观察投影。
5. 对话验证器原先假设同一selection只能出现一次提案，误拒绝“先被拒再修正”的真实记录。现在必须同时匹配最终图、tool_call_id、request_id和成功回执的taskId；保留失败提案，不把它计为成功。

### 来源、可复跑与接收状态

Windows步骤见[LOCAL-RUN](LOCAL-RUN.md)，Linux见[LINUX-RUN](LINUX-RUN.md)，机器码和预算绑定见[契约说明](contracts/README.md)。实际依赖解析和平台JAR/Go哈希写入接收样例；不同平台JAR摘要不同，不声称逐字节可重现构建。四专家共用一个JAR、各自角色配置和RDC登记独立。Keycloak归档校验值已与官方发行资产匹配。

W02/W05接收人已确认；W01 ae17ee3已有人工核对，核对独立性由负责人裁定。W06/W07完整业务签认、另一协作者部署、实际执行实例独立证明、GLM推理模型适配、完整M1—M4/人工UNKNOWN恢复的联合验收仍未完成。PR36的独立Root/批准/状态服务未接入；业务检查输入、完整报告摘要和版本组合需共同确认。不得把本机双平台通过写成上述外部门禁关闭。

当前保留Windows服务供本机访问；WSL验证完成后正常停止其工作区服务，源码、工具、配置与持久数据保留，可按Linux运行文档重启。API密钥仍只通过获准进程环境使用。

## 历史受控集成结果：2026-11-17（首次Windows组合）

本节及以下是此前实际验证历史；当前双平台结果与接收人状态以以上一节为准，不用旧的待认领/待核对文字覆盖后续进展。

当前基线为 W02 Application Plugin + W03原生Java + 本Demo W04 Go/PG + W05 Bus/RDC/Runtime；窄适配PR30插件与PR33纯选择组件，未整包合入旧授信控制宿主，未使用第二份年报正式状态。PR34跨任务远端快路径后置。下面早期日期段落是验证历史，涉及“插件待接入”等旧结论已被本节替代。

- **真实新规划**：会话 `web_1a1e77dba72_64d4468a982f`、请求 `req_mwk49oxs_34`，实际工具顺序 `skill_tool → list_capabilities → select_capabilities → propose_plan → get_plan`，usage为88391输入/3646输出。模型生成的8节点图与Go权威内容相符；不是复用旧图。选择记录保存排除/未知、配置排序、成员及数据组合；高分母公司与未知候选均零派发。
- **真实业务**：新任务 `T-2085fb8b43601839c4fb31f0ffea613f`，图摘要 `sha256:530e1a7a04468b044ee9047770dd3926d38a6b3dd67545ee3709d8f99984f0cb`。四位独立真实模型专家共5次远端调用（写作初稿+一次修订），8个正式节点，初检REVISE、复检PASS，最终DELIVERED。资产质量含两个完成的受限Loop分支。
- **真实插件用户链**：W02编排台批准图、拒绝错误密码、批准PASS报告、交付、稳定命令重复交付取原回执、浏览器下载正式原字节均通过。报告摘要 `sha256:ef2cb021cc1440e246203b846d47ad5ea456a19f7d7dc06e3c515bffd23b44bd`，下载5285字节；回执 `delivery-receipt-f59b8fa524e0244932d8c26b625be004`。规划模型无批准/派发工具。
- **真实恢复**：提取正式提交、两个后继远端任务运行时停止compiler；租约过期后重启原生Java，保留原任务、call/remote task/attempt，已完成提取新增调用为0。本次已知task由worker自动查询恢复，不能据此声称真实缺task可重发；显式resume的控件生命周期另有实际构建页面/受控RPC测试。
- **实际模型绑定**：专家进程内校验ENV，并用同一不可变参数创建Core客户端；Go绑定Task/输入摘要并核对远端完成证据。直接启动真实专家JAR、保持旧profile仅换ENV模型B以及缺绑定两例均拒绝，本地模型出口收到0请求。另有Core实际客户端请求模型A的本地出口正例；真实业务专家使用授权外部模型。未把隔离出口替身算作真实业务模型调用。
- **回归**：Go真实PG全量47顶层/93含子用例，0fail/skip，build/vet通过；Java编排22项、专家4项，0failure/error/skip；W02真实RPC客户端3项（有效乱序与超时丢弃分开）、实际构建页面浏览器3项通过。后续仅对同义清理/MCP入口说明做了构建、vet和相关准入定向回归。
- **可核对证据**：[integration-sample.json](evidence/integration-sample.json) 为白名单脱敏接收包，包含选择、图、真实c/task/attempt、一个指标原文到下游及报告的同值同引文链、恢复及验证结果。规划6项、主链10项、共享身份/只读对账20项均通过；原始会话、模型返回、私有配置和完整日志留在.local，不上传。备份/2包含选择记录，旧/1快照兼容读取/恢复测试通过。

**未完成**：另一协作者异机复跑与W02/W05实际接收确认、W01八指标独立人工核验/W06业务签认、共同发布资格、真实Builder导出链。完整HUMAN人工处置表单仍在同一Go可信备用页，插件尚未搬入；通用语义检索、多租户/HA、高并发、PR34远端跨任务快路径不作为本轮通过项。当前仅说明本机候选组合已运行，不能自签项目D1/D2联合验收。

## 1. 已验证的真实主链

### 2026-11-17：W03/W04 发送许可、版本绑定与真实服务验证

本轮与issue #16、PR #30/#31/#32对齐的是调用/版本/故障边界；未整包合入PR #30，Application Plugin和共同部署仍待接收方联调。agent-festival原共享文件未修改。

- Go/真实PG全量42顶层、含子用例85 pass，0 fail/skip；go vet通过。Java22项、0 failure/error/skip，其中10项为真实Go进程+PG+Java适配器的隔离HTTP窗口；页面实际源脚本Node测试5项通过。
- SEND仅首次意图提交成功后授予；并发8个请求只有一个SEND，既有task返回QUERY，无task返回UNKNOWN。真实PG COMMIT应答切断后，重复请求保持UNKNOWN、一条原意图；Java重复使用旧GRANTED许可也不增加SendMessage。另在真实Go进程中完成发送意图提交后立即强制终止，重启接管仍为REMOTE_UNKNOWN，原c/message/context不变、零发送/零查询（没有远端task id），对应S06当前提供方能力边界。
- 能力remote_binding固定三类版本和可选原始AgentCard字节摘要；Go/Java按采纳绑定校验，未匹配不写意图。2/3/4等非1.0.0配置仅在受控测试验证，真实部署仍为1.0.0。目录/profile变化拒绝旧计划采纳与漂移执行；Go强制终止重启后旧profile不被默认替换；慢RDC期间重启接管后旧执行者返回LEASE_EXPIRED、零意图/零发送。
- 新Go/Java编排服务、四个真实Runtime专家与注册心跳已启动，真实Keycloak/RDC/Gateway继续使用。新任务`T-6522725dda2fbee2fa541892a06ae976`已DELIVERED、generation=141；四个不同专家/远端task全部COMPLETED，质量Loop两分支；6个正式节点及2个有依据跳过，check0 PASS。
- 本轮使用历史候选拓扑，显式换用当前获准目录，新建任务并由受控测试账号重新确认；fresh_model_planning=false。没有把这次运行当作新一轮WorkSwarm规划或独立人工业务验收。
- 采纳图摘要`sha256:472b54557f383f2a89a64e82d68ba41664bd10c1b4965238cab12812e204e9f3`，执行profile `sha256:4af4969ea503b2a586077077d81b5a527ec67ab920ab3fea90f04e241aa2b2cf`。报告`sha256:901cf898712913e11edb135c7cbebc98d21300c2024ff80dd3d3cbc0d7b25cc6`；交付回执`delivery-receipt-08fc5a564e728d23b89d8e78b699923e`，同交付身份重试返回原回执。
- 主链10项与身份/对账16项核对通过。4条SEND决策、每条调用记录1条SendMessage HTTP应答观察；这不是独立抓包的物理调用exactly-once证明。交接导出含实际o/a/c、能力绑定、RDC版本、观测、结果关联，不含凭据/模型正文。
- 部署前后旧任务`T-7853c55c66499997c4597c83e3868949`投影完全一致，旧报告仍可下载且摘要为`sha256:dd34513a337f953fc78d07051986175793fc44d2a9a49fe48db6b5ee26edb617`。真实HTTP确认页已包含共用generation防倒退脚本；未做本轮浏览器视觉/真实断网验收。

最终证据：`w03-w04-final-go-tests.jsonl`、`java-w03-w04-process-final.log`（最终22项）、`java-w03-w04-final-pipes.log`（前轮21项）、`w03-w04-ui-final.log`、`w03-w04-live-release.json`、`w03-w04-live-send-check.json`、上述任务的trace/report/delivery/main-verification/handoff JSON。证据在本地私有目录，未上传原始模型正文。

首轮schema数量断言12→13、测试夹具固定节点名/旧owner错误码、Windows进程日志句柄清理问题已修正；原始失败分别留在`w03-w04-next-go-tests.jsonl`、`java-w03-w04-next-full.log`、`java-w03-w04-final.log`，最终完整串行回归通过。节点名现在按能力定位；进程输出用管道采集并显式等待退出，已确认无孤立测试Go进程。页面测试为可控DOM/网络替身，不能替代W02的A12共同浏览器验收。

### Demo 适配与共享规约的边界

`agent-festival` 中本PR的95个变化文件直接来自PR #25，内容与固定来源版本一致。Demo专用的身份字段映射、生成资源、查重/对账策略及其测试位于 `control-go`，生成器位于 `deploy/embed_identity_schema.py`；共享模块不承担本场景特有的 task/revision 相等关系。

2026-11-16适配归属调整时的串行完整Go/PG回归为37顶层、含子用例80 pass，0 fail/skip，其中包括身份和结果规则的9项测试；不把测试归属调整计为新增业务能力。证据为 `demo-owned-adapters-serial-tests.jsonl`。共享模块Linux全量21包、313顶层/342项通过，1项参考T1模型实验未启用而skip。

首轮并发回归中的迁移死锁（40P01）及Windows临时日志清理错误分别保留在 `demo-owned-adapters-full-tests.jsonl`、`java-demo-owned-adapters-tests.log`；不删除或改写成通过。共享PG的Go与Java跨进程回归采用串行执行，Java串行16项通过、0 failure/error/skip（`java-demo-owned-adapters-serial-tests.log`）。不宣称并发测试设施或多实例迁移已验收。

现有正式报告下载、主链10项只读核对、身份/对账16项核对仍通过；生成schema正文和对外协议不变，本轮不重复执行模型。

### 公开报告准入与当前实施基线

HTTP `POST /v1/commands` 与 MCP `propose_plan` 共用 `checkCandidate`：先校验材料、基础图与报告交付结构，再请求 Java 编译和保存候选。每份初稿/修订稿有同分支检查，一个无条件 selector 消费全部检查；缺少路径时返回 POLICY_REJECTED。纯计算控制夹具仍可使用基础 ValidateGraph，外部无绕过开关，报告实际交付的检查/批准门禁保留。

- `report-admission-tests.log`：两个顶层、含子用例八项通过；包含缺 selector、遗漏初检/复检、缺修订检查、分支不匹配等反例，以及正常初稿和有限修订正例。HTTP/MCP 两入口拒绝不完整候选时，编译器计数为0；合法候选调用编译器一次。
- `report-admission-technical-smoke.json`：实际运行的 Go/MCP/PG/Java compiler 接通；两入口拒绝缺检查/选择的候选。合法五节点技术候选为 `T-e704da8c75793265f596bf9696391e4a`，相同命令复用、可信确认与 PG 回读通过，任务保持暂停、全部 attempt=0。明确 `real_model_planning=false`，本次不触发真实专家计算。
- `java-report-admission-tests.log` 与 Surefire XML：本轮16项通过，0 failure/error/skip，六个远端窗口使用当前 Go 制品。`go vet ./...` 通过。
- `report-admission-full-tests.jsonl`：本轮完整Go/PG回归34顶层、含子用例71 pass，0 fail/skip，216.667秒。不是仅列出测试名称或把缺数据库时的skip计为通过。
- 当前控制实例已部署新准入代码。既有真实任务 `T-7853c55c66499997c4597c83e3868949` 只读下载和10项主链核对仍通过；本次没有重跑模型、重新批准或交付。

设计03已明确同scope事务门和JSONB物理映射，当前协议文档同步相同决定。LOCAL-RUN列出PG/快照/Java跨进程用例的前置条件、失败与skip含义；产物任务级授权和Java独立输入核验列为HANDOFF后续工作，未假称本轮已经实现。

### 共享契约与结果读面（PR #25）

使用 PR #25 `539d17a47eb01c178e789fc8fcd68c17d791baa3` 的 schema、结构化身份与配套样例，代码位于 `agent-festival` 原模块。Demo 导入共享 ledger 类型，身份映射及查重/对账策略在 Demo 内实现；PG 正式提交、结果复用及只读 HTTP 对账是实际消费者。共享资源从原 schema 生成；不额外部署文件结果服务。

- 实际任务 `T-7853c55c66499997c4597c83e3868949`，DELIVERED；图来自之前真实规划任务，`fresh_model_planning=false`，不是本轮新规划。Java、四个 Runtime/Core 专家及 Gateway/RDC 本轮真实执行，两个质量 Loop 分支完成，check0 PASS，测试账号批准和同交付身份重试通过。
- 正式报告 `sha256:dd34513a337f953fc78d07051986175793fc44d2a9a49fe48db6b5ee26edb617`，回执 `delivery-receipt-9f8f051a4a30c4980fc47ee8a2b8dee9`。主链 10 项核对与共享契约 16 项核对通过，所有 6 个正式节点有结构化身份，对账不改变投影版本。模型/提示词执行绑定仍为 `d7576071…`。
- Go/PG/HTTP 夹具验证：未提交但已有 call 不获得新执行许可；输入相同返回原结果，receipt 不能替代输入摘要；同 o 的第二行被 PG 唯一约束拒绝；撤回后 committed=true、serve=false；未认证及无权角色不能读对账结果；备份中伪造身份即使重算快照摘要也被拒绝。
- 共享模块 Linux 全量：21 包通过、313 顶层 / 含子用例 342 pass，1 项真实 T1 模型实验未启用而 skip；该实验不属于本 Demo 四专家调用。Windows 全量另存失败记录，涉及 syscall.Umask、Unix 文件权限、路径分隔符与 python3 启动入口，不把 Windows 全量写成通过。Windows 下实际使用的 ledger 类型与 Demo 适配另有通过证据。
- PR #25 接入版本（cd44c55）的全套真实 PG 回归：32 顶层 / 含子用例 63 pass，0 fail/skip；Java 16 项本轮重新执行，0 failure/error/skip，六个远端窗口使用当前 Go 制品。共享资源导出与 DTO 漂移检查通过；最新补充的对账角色拒绝单独复跑。
- 当前 PG 快照包含 48 份内容，摘要 `sha256:ad42a984918e5be2cd53af60744791e8fa6b8ce524be4a36a364891732911f30`；新旧任务、结构化 identity 与规范化结果一致，离线回放验证通过，数据库/Runtime/模型/发布调用均为 0。未覆盖或清空既有隔离恢复库。
- 证据：`shared-contract-consumer.json`、`shared-contract-model-verification.json`、新任务 `-trace/-report/-delivery/-main-verification.json`、`shared-module-linux-tests.jsonl`、`demo-shared-contract-final-tests.jsonl`、`java-shared-contract-tests.log`。原模型输出留在私有目录，不将测试账号交付当独立人工业务核验。

共享契约限定 o/a/e、输入绑定与结果读面的语义；正式事务保证来自 PG，不能据此宣称 PR #25 全部参考宿主、实验或其它提案已通过。旧任务报告下载与 10 项只读核对仍通过，未重写其正式身份和结果。

### 本轮新增证据与版本边界

首次交付代码为 `0aa6bc1`。下述真实修订/进程断点使用执行绑定 `1434fc69…`；后续修复 Java 失败收敛及 Go 远端结果资格后，执行绑定更新为 `sha256:d7576071143ad82280f84dbb36f797ae782288723137d386d35d1dd72f309fd0`。**该新绑定已用新一轮真实聊天候选完成四专家、程序检查和测试账号交付**；旧故障任务仍只证明其当时制品，不混用版本。

| 最新主链 | 2026-11-16 实际结果 |
| --- | --- |
| WorkSwarm 会话 | `web_1a1e31c96ac_1346e73c0615`；模型读取目录，前两次非法候选被拒，第三次修正后预检成功 |
| 任务 | `T-202e02a56e2275a0aa62837e51d79969`，`g1`，`DELIVERED`，generation `138` |
| 图摘要 | `sha256:09418a01b522346da4f01cd5a9f15aa46dde038f2fd46a552561329f17c619a4` |
| 正式事实 | 4 个不同真实 remote task、6 个正式节点、2 个按 PASS 条件授权跳过、78 条观察、138 条审计 |
| 报告摘要 | `sha256:cd15595fb45035a4c8414df8fbd0277134bfe46be6c5abd2cdf3ba8db3312145`，check0 PASS |
| 交付回执 | `delivery-receipt-5d53463fcb8e29ebca5d469bea8a85ae`；同 delivery_id 重试取回相同回执，JSON 下载与正式报告一致 |
| Loop / 结果资格 | 两个质量分支真实完成且绑定实际输入；四个远端正式正文逐一等于同 c/h/a、remote task/context 下观察到的 COMPLETED 输出 |
| 独立只读证据复核 | `verify_main_flow.py` 10 项通过（包含下载原始字节 SHA-256）；不启动模型、不批准、不交付，不代表独立人工业务核验 |

- **自然模型错误与一次真实修订**：`T-1a79d785df1a3baa61923b4a037783d6` 首稿在 section 中加入 schema 不允许的 `limitations`，check0 返回 REVISE；真实 writer 的 `revise1` 一次修订，check1 PASS，选择并由测试账号交付修订稿 `393c7ad6…e864a5b`。四次真实远端调用，提取结果来自主任务的合法复用。原计划的传输注入因测试器未匹配响应信封而未发生，原注入测试失败记录保留；独立的 `-natural-revision-verification.json` 仅核对自然修订事实，11 项通过，不倒改失败为通过。
- **明确传输故障与真实 writer 修订**：修复测试器信封关联后，`T-32a9c6ff87dc37b2c736c91642a069bb` 将真实 draft0 回包的 revenue 单位从“人民币百万元”改为“人民币亿元”，原回包和修改后回包分别保留。程序报 enum/unit mismatch，真实 writer 修订一次，复检 PASS，7 项通过。这个错误明确由本机测试代理注入，不冒充模型自然错误；未降低 checker。
- **真实 Runtime 重启**：`T-42a4c84a0df2e96605f6159c4a481e14` 在真实 business remote task 运行中停止 worker 和 business Runtime，重启后查询原 task 返回 `-32001 Task not found`。原 c/message/context/remote task/a/h 不变，计数代理记录 business SendMessage 恰好 1 次，UNKNOWN 可见，未生成假结果；7 项通过。任务已暂停留证。
- **真实 Gateway 重启**：`T-8eb8038bda76f21cb733509f01759943` 重启 Gateway 后返回 `-32055 ROUTE_OWNERSHIP_UNKNOWN`；保留原调用身份、仅查询不重发，7 项通过。它证明内存 owner 丢失时诚实停在未知，不宣称自动恢复。
- **真实 PG COMMIT 应答丢失**：在 PG 服务端发出 CommandComplete(COMMIT) 后、pgx 收到前切断本测试连接。独立连接确认正式结果与必要审计已提交，原请求重试复用原回执。另测真实 Go HTTP handler 完成提交后断开响应连接，重试仍复用回执；没有模拟成“提交前失败”。
- **慢 RDC 屏障**：实际 Go/PG 与 Java 适配器、本地隔离 RDC 提供方；发现请求被 CountDownLatch 阻塞时，同任务心跳与独立 quality 正式提交完成。用户暂停后释放旧发现快照，派发被拒，提供方调用数 0。
- **新代码门禁**：失败节点使任务明确 FAILED，settled 的 Java 调度器即使抛异常仍请求 Go 核对完成，Go 释放许可并停止自动轮询；用户明确继续才能重试。已有远端调用的 C03 必须匹配唯一已观察到的 COMPLETED 输出；未完成、冲突、候选正文替换均拒绝。真实 PG/Java 回归已通过。

此前核心回归：Go **30 顶层、含子用例 61 pass**，Java **16 测试通过**，均无失败和跳过；两套本轮均重新执行，Java 的跨进程测试使用最新 Go 二进制。规划工具白名单经原生能力边界拒绝测试通过，MCP bootstrap 配置兼容修复已安装，真实助手已成功加载技能、读目录和提交候选。拒绝测试使用实际原生 allowlist 和包含真实 McpServerConfig 的管理器夹具；真实正例来自上述会话。它是本隔离工作区的模型工具边界，不是 OS 进程沙箱。

本次还发现规划模型将 writer 错说成程序节点。实际 Runtime 证据明确 writer 是第四个真实模型专家；已补强技能中的角色说明，不能把模型口头描述当作服务运行事实。只有 checker/select 为确定性程序节点。

WorkSwarm 0.2.8 中的年报规划助手实际读取获准目录，排除母公司口径能力，生成八节点候选；Go 预检和 Java 原生 Blueprint 编译后，受信测试账号确认同一图摘要。Java 经真实 Gateway/RDC 调用四个独立 Runtime/Core 模型专家，Go 将结果条件提交到独立 PostgreSQL 库。报告程序检查 PASS 后，测试账号完成同对象批准、交付、同 delivery_id 重试和 JSON 下载。

下表保留上一轮 `1434fc69…` 主链，便于复核旧故障任务的来源。

| 项目 | 上一轮事实 |
| --- | --- |
| 主任务 | `T-d2c511867ab929cbfe371cf5057de6b6`，`g1`，`DELIVERED`，generation `138` |
| 真规划会话 | `web_1a1dfc3f3c8_b8631dd80f56`；command `cmb-2025-vs-2024-group-brief-010` |
| 重启后只读查询会话 | `web_1a1e1f75903_8e80facb0a37`，再次真实调用 `run_status` / `verify_run` |
| 模型 | `deepseek-flash`；模型设置与凭据不随本文或代码提交 |
| 图摘要 | `sha256:1b6938f193920eea445f3ff460c64329e45cf46715d8a94d74d84471bce1020e` |
| 执行绑定 | `sha256:1434fc69dbcae7c867a0d9207b9bb5d7dc1e47334038c8d2f696a14bbe831f80` |
| 正式节点 | 提取、经营分析、资产质量分析、草稿、程序检查、报告选择，共 6 个 |
| 授权跳过 | 首稿 PASS，定向修订和复检 2 个节点按已确认条件跳过；不冒充实际修订分支已测 |
| 远端调用 | 四个 Runtime、四个真实远端 task；80 条观察；138 条必要审计事件 |
| 报告产物 | `sha256:32329c14d7356af8d8ec0a0f76f9d3cfd447bfed7051706cf3be2d78897d33a7` |
| 交付回执 | `delivery-receipt-cc2df4ea109d8b6b9d3342920bb193aa`，2026-11-16 09:13:39 北京时间 |
| 完整性检查 | `evidence_intact=true`，`issues=[]`，只读核对记录事实和内容摘要 |

程序检查范围是固定材料中的结构、八项指标、来源行、单位、期间、算术与有界数值论断，不是任意自然语言的完整语义证明。`select_report.qualification=PROGRAM_CHECK`、`human_review=null`。批准主体为 `demo` 测试账号，delivery_id 带 `operator-test-` 前缀；这不代表用户本人或独立业务人员已经验收年报结论。W01 独立人工核验仍待本人完成，AI 不代签。

### Loop Forest 实际运行

资产质量专家通过原生 ForkTool 派生了两个只读模型分支，分别核对资本充足率口径和不良贷款相关数值/变动。两分支均真实完成，包含原指标值、引用行和局限，未再出现旧版“缺少上游输入”的空上下文核对。

每条分支记录含目标、父节点输入摘要、实际业务输入摘要、输出摘要、原始返回及完成标记；父摘要与该真实远端 call 的 input_digest 相同。业务输入摘要不包含另行记录的分支目标，不能称其为完整最终模型提示词摘要。分支是分析依据，不是第三方独立人工复核；模型仍如实说明仅凭上游 citation 无法独立证明 PDF 图像中的表头/页码。

### 本轮隔离效果与下载修复

- 新增 [验证报告](VERIFICATION.md)：默认不注册测试 API；真实用户确认正例成功，四类服务 token、缺 CSRF、三类提供方无权请求被拒。目标 origin 固定为本机显式端口，不接受模型地址或重定向。
- 提供方先计数生效再断开响应，首次 UNKNOWN；同 b 仅 GET 找回 APPLIED，POST=1、效果=1、GET=1。真实 100ms HTTP 超时单独验证得到相同计数。
- 真实 PG 意图/派发必要审计 INSERT 失败均零发送；已生效后的观察审计失败，恢复后只 GET 核对，效果仍为 1。暂停与新 epoch 后的迟到回执留下，不释放新 owner 的 ACTIVE 许可；不宣称已发生效果被取消。
- DENIED 明确业务拒绝可复用；401/404/超时为 UNKNOWN；同 b 不同 receipt_id 或互斥终态为 CONFLICT，原回执保留。网络意图和观察仍在已有 15 表快照中，离线回放不增加提供方请求。
- 新交付准入核对当前 policy_version。原检查/批准不能授权换策略后的新动作，但当前受权读者仍可下载已交付的原报告。
- 下载原先经通用 JSON 编码器追加换行，语义相同仍可能改变字节摘要。现直接写不可变原始字节；运行实例只读重新下载主任务报告，实际 SHA-256 与 `cd15595f…312145` 完全一致，没有再次批准或交付。主链核对从 9 项加强为 10 项，旧的仅 JSON 等价检查不再算精确字节证明。
- 当前运行实例 GET/POST 测试出口均为 404，正常私有配置没有 test_effect；16 项只读服务就绪仍通过。网络用例用独立 test-* scope 的控制夹具，不冒充真实专家或独立业务验收。

完整 Go 回归证据：`go-network-effects-tests.jsonl`，28 顶层/含子用例 57 pass，无失败和 skip。随后将“已有 b 对账”与“新准入的内容文件校验”分开：文件暂缺不阻止查询既有远端事实，新动作仍要求完整内容；最终网络范围复跑 `go-effects-reconciliation-tests.jsonl` 的 10 顶层/18 项全部通过。这是两次不同范围的真实执行，不将 57+18 写成 75 个独立用例。最终 Go 制品已更新到本地运行实例，主任务只读下载/10 项核对再次通过。

上述网络效果一轮未改 Java，16 项结果当时沿用相同源码测试；不把那一轮说成重新执行。新增 schema 共 12 份。JSONB 的键顺序规范化曾使“原始 JSON 排版相同”的测试误报，现按类型化回执身份核对，同时确实测试不同 receipt_id 必须转 CONFLICT；报告文件下载另用精确字节比较。

### 最后核心修复：正式提交后的矛盾远端回执

真实 PG 测试先复现了缺口：已提交的提取结果被另一任务复用并生成报告后，晚到的矛盾回执虽然保存，源任务仍可验收，克隆报告仍有交付资格。修复前失败记录保留为 `late-conflict-before-fix.log`。

现在，可信观察、call 冲突、原结果及跨任务后代有效性撤回、NEEDS_REVIEW 投影和必要审计在同一 PG 事务中完成。复用、提交与交付通过同 scope 的短数据库事务门互斥，避免撤回期间新增复用；没有在门内调用模型或网络。原 R、输入摘要、回执和文件字节保留。只有互斥终态、不同远端关联或不同的已绑定完成正文构成矛盾；晚到的 ACCEPTED/WORKING/UNKNOWN/TRANSPORT 不再把已完成事实误判为冲突。

- `late-remote-conflict-different-output.json` 与 `late-remote-conflict-contradictory-terminal.json`：真实 PG/Go 控制夹具，分别验证不同完成正文和矛盾终态；源任务与复用任务后代全部失效，原文件不变，旧/新 delivery_id 和重新批准均拒绝，实际已鉴权下载返回 403。重复事件不重复建处置记录。
- 实际必要 INVALIDATED 审计 INSERT 故障使整笔观察、call 状态与全部任务有效性变更回滚；故障解除后同事件可重试成功。上述夹具不调用真实专家或外部提供方，不冒充新的模型业务运行。
- `go-final-core-tests.jsonl`：全套 30 顶层 / 含子用例 61 pass，0 fail/skip，227.112 秒；`java-final-core-tests.log` 与 Surefire XML：16 项、0 failure/error/skip，包含使用最新 Go 制品的六个隔离远端窗口。
- 最新二进制已替换本机控制服务；服务就绪 16 项通过。对原主任务重新导出真实持久事实并只读下载，10 项核对仍通过，状态 DELIVERED、generation 138、报告摘要与原交付回执不变。没有重跑模型、重新批准或交付。

此前网络效果一轮的“Java 未重跑”描述仅对应当时执行；以上是最后核心修复后的新一轮实际回归。短 scope 门偏向 Demo 正确性，并非高并发性能结论；生产粒度优化留在交接文档。

## 2. 实际故障、恢复与修复

| 验证 | 实际结果与边界 |
| --- | --- |
| Java worker 停止/重启 | `T-9ac5306977eda133b739cbcf86197a6a` 使用真实模型候选的测试副本；在实际远端运行中暂停并停止本工程 worker，原 g/图摘要、call/message/context/remote task/attempt/h 均保留；完成后 6 项断言全通过。该任务使用较早执行绑定，不用它证明最新 Loop 输入修复 |
| Go 真实进程重启 | 主任务在 control session 改变后，正式任务投影、远端调用身份和持久事件相同，文件完整，游标续读无重复；7 项断言全通过 |
| 事件跨位数排序 | 实测发现 `sequence::text` 的输出别名使 ORDER BY 按文本排序，`999` 排到 `1033` 后；现按 `audit_outbox.sequence` 数值排序。PG 回归使用不同位数序号并核对续读 |
| 补资料/重新授权后的继续 | 原等待类型分别保留；明确继续后重新排队核对。只要已有持久 send intent，就返回 WAIT 查询原 call，不授予新发送 attempt。真实 PG 两类回归通过；尚非真实外部补资料 UI 全链验收 |
| PG 连接槽耗尽 | 实际观测到公网链路切换后的闲置遗留连接，健康检查 503。只停止本工程 RDC/control，在无本地 PG 连接时清理指定 Demo 角色的 idle 后端；没有重启 PG、修改 max_connections 或修改用户搁置的公网安全配置。给 Demo 角色设置 60/10/3 秒 TCP keepalive 后恢复 |
| 注册心跳 | 短暂失败记录退避和状态，不再因一次 PG/RDC 错误退出；实例确实 404 时重新注册本机实际卡片。就绪使用新鲜 heartbeat 状态，不误要求客户端拥有 RDC 监听端口 |
| 备份/隔离恢复 | 真实独立 `report_demo_restore_test` 库、不同本地内容目录；15 张表、22 份内容、9 个任务恢复后逐项相等；未启动恢复库上的 worker/publisher，也未恢复登录会话 |
| 离线回放 | 故意指定不存在的配置文件仍可核验快照；数据库、Runtime、模型和发布调用为 0。记录路由改指 HTTP spy 后请求数仍为 0；篡改内容、缺内容和已采纳图被修改均被拒绝 |
| 隔离 HTTP 远端窗口 | 实际 Go 进程 + PG + Java BoundNodeExecutor/RemoteExpertExecutor：提供方受理后丢响应只 SendMessage 1 次；已有 task 后查询丢失仍保持关联；INPUT_REQUIRED/AUTH_REQUIRED 继续均只 GetTask，call/message/context/remote task/attempt/h 不变。提供方是明确的本地夹具，没有模型调用，不当成真实 Runtime 重启 |
| 前驱失败且调度仍继续 | 当前 TeamBlueprints 默认会跳过全部前驱无结论的后继。测试显式注入 skip=NEVER 的坏策略，仍使用原生 BlueprintScheduler；8 个包装器实际进入，但仅提取请求到达隔离提供方，下游调用 0、attempt 0、无正式结果。修复后完成申请返回 FAILED 并释放许可，真实 worker 循环也核对退出；生产已采纳图未修改 |

备份摘要：`sha256:c879310a4acafac5199db764747fb160a0aa754ba9b26d671d938ea6168b2fb2`。恢复后的事实相等不代表重新执行专家或重新签发业务批准。

WorkSwarm 常驻一夜后曾出现 supervisor/worker IPC 断开。重启后原长会话触发 `HISTORY_RESTORE_CHANGED` 的具体原因，本轮已定位为本机 Windows Python 的 stat/fstat ctime 来源不一致。已用来源 pin 与运行时探测保护的局部兼容层修复；原会话真实恢复证明见下节。常驻 IPC 断开本身的所有诱因、强杀中的流式续接仍未穷尽，不能因本次历史恢复成功就宣称全部已解决。

### 原会话恢复的最终证据

详见 [验证报告](VERIFICATION.md)。原会话为 `web_1a1dfc3f3c8_b8631dd80f56`，仍查询原任务 `T-d2c511867ab929cbfe371cf5057de6b6`。

- `windows-history-compat-qualified.json`：16 项通过。修复前稳定文件真实报错；修复后原生读取器恢复；读取期间追加、同大小/mtime 的路径替换、同长度改写、截断仍拒绝。未知源码的新进程确实启动失败；原生一致的环境不被改写。两个既有历史文件的只读探测前后摘要不变。
- `workswarm-history-recovery-verification.json`：24 项通过。三个重启后请求分别为 `req_mwiyls5z_635`、`req_mwiyszqo_817`、`req_mwiyyt12_994`；前两次确实调用 run_status/verify_run，末次只调用 verify_run；均有新模型 usage、实际工具返回及 success 完成记录，没有新错误或 propose_plan。
- 最终启动器末轮真实进程树已更换：管理 PID `32692→38352`，实际 worker `20996→12204`；旧进程创建时间核对后确认退出，当前 worker 日志显示正确来源 pin 的兼容层。不是把进程观察超时当作重启证明。
- 首次修复前 1,100,059 字节历史前缀及末轮前 1,371,239 字节前缀的 SHA-256 均未变。09:23 的原错误仍保留；只是追加真实请求/回复，没有删错、截断或迁移后假装恢复。
- 原 Go `T/g`、graph_digest、generation `138`、4 个 remote call 完整身份及必要审计数量不变，正式证据仍完整。没有重新执行专家、重新批准或交付。报告与回执仍为 `32329c14…797d33a7` / `delivery-receipt-cc2df4ea…`。

最终兼容层使用上游文本 SHA-256 `6cff70ff…84d71e9`（仅 CRLF→LF 规范化）及当前运行时字段行为探测。最初实现、来源校验加固、最终探测保护之间有版本变化；不把三次请求说成同一个兼容文件字节版本跑了三遍。末次是在最终启动器上实际恢复并重新调用工具，16 项探测同样对应最终版本。仅本机已记录的 Windows 故障路径通过；不代表全部会话迁移、并发写与 IPC 断流场景已验收。

## 3. 自动验证

| 验证类别 | 结果 | 覆盖边界 |
| --- | --- | --- |
| Go 全套，真实 PG 与真实快照 | 42 个顶层测试、含子用例 85 个 pass，0 fail、0 skip | 严格 JSON/JCS/Unicode、10 组 golden vectors、DTO schema 漂移、准入/输入、并发正式提交、过期租约、迟到/冲突观察、必要审计回滚、独立交付租约、HUMAN 审阅、复用/撤销、游标、等待继续、离线回放、真实 COMMIT/HTTP 应答切断、失败收敛、远端输出资格；新增提交后矛盾的跨任务撤回、下载拒绝及事务回滚 |
| Java 原有与跨语言测试 | 12 个，0 failure/error/skip | 原生图编译、实际 JSON 序列化的跨语言摘要、协议解析/状态分类、固定材料 checker 正反例、schema 与人工审阅选择 |
| Java 隔离远端窗口测试 | 10 个通过，0 failure/error/skip；与上行合计 22 个 | 真实 Go/PG/Java adapter；本地 HTTP 故障提供方，含显式坏调度策略、实际 worker 失败收敛与慢 RDC/暂停屏障；不代表真实 Runtime/IdP 故障恢复 |
| 最新真实主链证据核对 | 10 项通过 | 新绑定、四专家、两 Loop 分支、观察与正式正文一致、同对象检查/批准/下载及下载原始字节摘要、同交付身份；只读核对已有证据 |
| 构建依赖冻结 | 4 模块 effective POM 与 dependency tree、Go 实际导入包和 6 份依赖锁已记录 | `freeze_build.py` 串行执行；是当前依赖解析与锁文件证据，不替代独立 clean rebuild 或上游测试 |
| Python 跨语言样例 | 10 组全部通过 | 既有 Python 冻结 oracle 与实际 Go JCS 相符，大计数字符串不丢精度；不另造生产 canonicalizer |
| 服务就绪 | 16 项通过 | 本机各端口/卡片/目录/编译/未鉴权访问拒绝；不等于模型业务通过 |
| IdP/Bus 身份 | 6 项通过 | 真实合法主体、假令牌/假主体/无权目标拒绝，包含 Runtime 正例；非全量渗透测试 |
| 隔离备份恢复脚本 | 6 项通过 | 表、内容、材料/目录、投影相等、离线零外呼、不启动服务 |
| Windows 历史兼容探测 | 16 项通过 | 实际元数据/原读取器、真实变更反例、来源拒绝与子进程继承；不是模型端到端证明 |
| 原长会话重启恢复 | 24 项证据核对通过 | 三个真实模型请求，末次最终版本、同会话/原 T/g、历史前缀未改、进程树更换；不代表所有 IPC/断流路径 |

来源 fresh build 的 `-DskipTests` 仅证明制品构建，不能计为上游全部测试通过。没有运行 race 检查或宣称上游所有仓完整测试通过。P0 技术预制图明确 `real_model_planning=false`，不算真规划证据。

## 4. 原方案 T01—T29 覆盖审计

“局部”表示已有相应实现/证据，但未满足该行原定全部联合条件；下面没有把未执行的断点降格成通过。

| 项 | 当前证据 | 未完成部分 |
| --- | --- | --- |
| T01 真图与数据链 | 四真实专家主任务、实际前驱产物消费、程序检查/交付 | 独立人工指标核验 |
| T02 非法图 | Go 准入反例 | 远端提供方计数为零的完整集成对照 |
| T03 命令幂等 | PG 同命令并发唯一、改载冲突 | HTTP 断线/重试联合证据 |
| T04 节点唯一提交 | PG 并发同 h 原回执、改 h 冲突 | 新旧 worker 联合模型增量计数 |
| T05 前驱失败 | 实际 Go/PG、原生 scheduler 注入坏 skip 策略；8 包装器进入而下游 HTTP 提供方调用 0 | 明确是隔离故障提供方，非真实业务模型失败复跑 |
| T06 worker 重启 | 已执行真实在途进程恢复 | 最新绑定再复跑与完整失败窗口 |
| T07 旧 epoch/迟到 | PG 到期与可信观察回归；提交后矛盾终态/正文撤回结果资格 | 全路径真实迟到结果对账 |
| T08 发送关联窗口 | 隔离 HTTP 受理后断响应、同 call 重试无第二次发送；有 task 但查询丢失保留关联 | 发送前/关联保存失败其余断点与实际进程故障 |
| T09 提交应答丢失 | 真实 PG COMMIT 应答切断与真实 Go HTTP 提交响应切断，原请求取回原回执 | 独立复跑与完整交接样例 |
| T10 PG/审计失败 | 实际必要审计失败回滚、真实 PG 不可用/恢复、COMMIT 应答连接切断 | Go 故障其余窗口与独立复跑 |
| T11 Gateway/Runtime 重启 | 两类真实进程重启均已执行；task/owner 内存丢失时 UNKNOWN，真实转发计数无盲重发 | 独立复跑；不宣称有自动恢复能力 |
| T12 暂停继续 | 真实在途暂停、禁止新增派发、同任务继续 | 对各业务等待状态的全部集成 |
| T13 版本绑定 | h/catalog/profile 绑定及交付对象检查 | 规则/能力/报告逐版本替换矩阵 |
| T14 报告 checker | 固定材料反例、自然模型 schema 错误后真实修订/复检/测试交付、显式单位故障后真实修订 | 独立人工指标核验与复跑 |
| T15 慢 RDC | 明确同步屏障、同任务心跳/独立提交完成、暂停后旧快照派发数 0 | 其它目录/权限变化竞态；当前证明的是暂停边界 |
| T16 身份与秘密 | Go 角色、Cookie/CSRF、原生 Bus 正反例 | WorkSwarm 普通内置 OS 工具与同用户私有配置隔离审计、全面脱敏扫描；MCP 无批准工具不等于 OS 沙箱 |
| T17 事件与刷新 | Go 重启相同事件/结果、数值游标与续读；重复矛盾事件去重，晚到非终态不误判完成事实 | HTTP 重复/乱序观察全矩阵 |
| T18 跨任务复用 | 真实恢复任务新 R 引用提取、PG 配置/字节负例 | 源材料逐版本集成对照 |
| T19 撤销失效 | PG 正式结果/后代/克隆失效与交付拒绝；可信晚到矛盾自动触发同事务撤回及审计失败回滚 | 实际纠正任务/知识命中完整链 |
| T20 隔离回放 | 真实快照、HTTP spy 零调用、篡改与缺证据拒绝 | 独立协作者复跑 |
| T21 主入口/确认 | 真 WorkSwarm、MCP Apps、同对象受信确认、重查图卡；原长会话 Windows 误报已定位修复，末轮最终启动器实际重启后同会话查询通过 | 其它 IPC/流式中断路径与独立协作者复跑 |
| T22 真规划 | 真实目录消费、排除错误口径、模型候选 | 独立协作者复跑 |
| T23 到期/统一节点 | PG 时钟、无接管也拒绝到期自续/旧提交 | 全联合调度矩阵 |
| T24 报告交付 | 真实测试账号批准/同 b/下载、PG 独立 delivery 域；晚到矛盾后旧/新 b、批准及实际 HTTP 下载拒绝 | E 系列全量联合验收 |
| T25 外部效果网络窗口 | 已有显式启用、默认关闭的真实 Go/PG/HTTP 隔离出口；受理丢响应/实际超时/迟到/三类审计失败/稳定 b/拒绝及冲突回执/提供方权限计数 | 无真实银行；所有 Loop/Workflow/远端旁路及独立复跑未穷尽，见 LOCAL-RUN 第8.8节 |
| T26 错误/业务等待 | 类型化错误、两等待类型 PG 回归、真实 Java/Go 经 HTTP 继续只查询同 call | 真 Runtime 补资料/授权恢复、完整技术失败矩阵 |
| T27 备份与内容 | 真实隔离恢复、文件篡改拒绝、必要审计回滚；网络意图/观察纳入原快照，离线回放零外呼；本地文件暂缺仍可对账旧 b | 发布后 PG 失败孤立候选/清理保护完整对照 |
| T28 来源/打包 | 固定 SHA、fresh build、schema 内嵌、真实 MCP/Part、5 份制品摘要、4 模块 effective POM/依赖树、Go 包与依赖锁、Demo 源码逐文件摘要 | 最终来源归属复审及独立 clean rebuild；当前解析不追认旧构建时刻的完整环境 |
| T29 可替换场景 | 专家装饰器无 credit 回调 | Builder 导出部署注册样例、替换场景不改控制状态机的实测 |

Contract-C01—C07 已补真实接口映射、12 份从 DTO 导出的机器 schema 与 10 组跨语言 golden vectors，见 [协议交接](contracts/README.md)。各条完整请求/拒绝/等待样例、E01—E12 逐条证据与 A/S/AR/ALK 交接包尚未补齐；不因 T 局部通过而推定这些编号通过。原完整目标保留追溯；按用户最新收敛要求，非核心剩余项移交 [验证报告](VERIFICATION.md)。

## 5. 原始证据位置

私有目录 `.local/report-demo/evidence` 保留：主任务 `-task/-trace/-report/-delivery.json`；`control-restart.json`；真实 worker 恢复任务 `-recovery.json`；`restore-source.json`、`restore-readback.json`、`restore-verification.json`；`go-tests.jsonl`；`service-readiness.json`、身份探测报告；`postgres-recover-idle.json`、`postgres-create-restore-db.json`；`workswarm-model-and-report.jpg`。

Java 结果位于 `orchestrator-java/target/surefire-reports`。隔离窗口证据为 `fault-accepted-lost-reply.json`、`fault-accepted-remote-task-lost.json`、`fault-input_required-resume.json`、`fault-auth_required-resume.json`、`fault-predecessor-failed-scheduler-continued.json`。`source-and-artifact-audit.json` 记录实际源码 HEAD、5 个制品摘要与本 Demo 源码的限定凭据扫描；该扫描不等于 OS 隔离或全面泄密证明。这些目录不提交凭据或未经检查的原始日志；本文只写必要标识与核验边界。

此前核心核对另存 `final-core-release-check.json`：33 份 Python 语法解析、47 处本地 Markdown 链接无缺失，Go/Java 实际计数、主任务状态及 10 项核对均通过。已部署控制 EXE 与跨进程回归使用的二进制逐字节一致，SHA-256 为 `c4eed367389fb8100077b3779a0cf1e8a9d81acbb8ffa90e5bb0cbe3225c1292`。独立复跑和完整外部签收仍是交接项，不能把本文日期当作签收时间。
