# 跨服务协议交接

当前 Task/Binding 还冻结非秘密的 `bus_binding={environment_id,tenant,issuer,audience,caller_subject}`。它进入节点输入摘要；Go 在采纳、解析/提交、发送意图前检查当前环境，Java 在发现、发送、恢复查询前检查实际 Bus 配置，失配返回 `BUS_BINDING_CHANGED` 并阻止外呼。发布的执行 profile 必须带显式环境绑定；历史缺字段只读兼容，不回填当前环境后继续执行。Gateway/RDC 地址、令牌、client secret 不属于逻辑身份，更换同环境代理和轮换凭据不改变既有远端任务身份。token_url 必须属于绑定 issuer；身份真实性仍由现有 Gateway/IdP 校验。

工作台投影仅对当前有效、尚未处理的 HUMAN 检查返回 `human_review={url,checks:[{node,check_artifact,object_digest}]}`。url 由 Go 的 public_url 与任务 ID 构造，不含登录凭据；插件打开可信页，用户重新登录并走原有会话/CSRF 和对象绑定审核。审核后投影不再展示待处理提示；原始检查产物不变。

这是当前实现的应用协议 `report-demo/1`。这里的 Cxx 是设计中的 **Contract-Cxx**，不替代项目 issue 的其它同名编号。接入 [PR #25](https://github.com/chaosxingxc-orion/agent-hub-alpha/pull/25) `539d17a` 的结构化身份/schema；共享定义在主模块原目录，应用 schema 由实际 DTO 和共享身份资源导出。能力口径、前驱资格、租约、检查/批准和事务不变量仍由 Go 领域规则执行，不以“schema 合法”推断“业务获准”。

| 身份 / 读面 | 实际映射 |
| --- | --- |
| o | `ledger.LogicalExecution=(task_id,graph_version,node_path,iteration_path)`；本场景节点只执行一次，`iteration_path="0"`，有限报告修订是另一节点；节点内 Loop 轨迹不冒充正式图节点 |
| a / e | `operation_id` 由 o 与 attempt 稳定派生；`execution_generation` 来自 PG 租约 epoch；Task.generation 只表示投影版本 |
| 输入 / 回执 | `input_digest` 绑定材料、版本、指令和实际前驱；独立 `material_digest` 校验文件。对账比较输入摘要，不比较 receipt；相同输入与内容重交只取原回执 |
| 正式持久化 | 结构化 identity 与结果、必要审计同事务；PG 同 o 唯一索引。attempt 改变不创建第二份 R(o)；复用提取的新任务拥有自己的 o，attempt 0 表示无新专家执行 |
| 只读对账 | `GET /v1/tasks/{T}/nodes/{o}/result?input_digest=...`，worker/operator/用户身份；返回 committed/serve/stored、状态和只读标记。无结果不能推导可重执行；失效结果 committed=true、serve=false |
| 旧任务 | 保留原绑定与原结果。新任务固定 `identity_contract=pr25/0.2.0@539d17a`；旧记录无此字段时不补写猜测的历史身份 |

共享类型来自 `agent-festival/go/internal/ledger`；当前 Demo 的字段绑定、查重/对账适配位于 `control-go/identity.go`、`control-go/result_policy.go`，PG 是唯一正式结果存储，Demo 不维护文件结果库或第二份内存结果缓存。参考宿主的文件实现不提供本应用的事务保证。共享身份的嵌入资源由 `demos/report-demo/deploy/embed_identity_schema.py` 生成，不依赖服务启动 CWD；导出 DTO 不另行手写一套身份结构约束。`business_request_id=task_id`、`evaluation_revision=graph_version` 是本 Demo 的映射，由 Demo 校验，不作为共享组件的通用规定。

当前物理映射将已采纳图和节点投影保存在 `tasks.document`，尝试与正式结果分别在 `node_attempts`、`committed_results`；规范化结果和任务投影同事务更新。唯一正式结果受数据库主键及 o 四元组唯一索引约束。

同 scope 的权威变更使用 scope/authority 短事务门，保证提交、跨任务复用、撤回和交付的一致性；它不替代唯一索引。普通读取、Java 并行计算及模型/RDC网络调用不持有该门。当前不承诺高并发，锁粒度优化见验证报告未完成项。

公开 `POST /v1/commands` 与 MCP `propose_plan` 共用报告准入：每份初稿/修订稿必须有同分支检查，一个无条件 selector 消费全部检查，否则在调用编译器前返回 POLICY_REJECTED。内部纯计算用例使用基础 ValidateGraph，外部没有绕过开关。结构检查不替代实际报告检查与用户批准。

## 1. 生成与跨语言验证

```powershell
& "$private/bin/report-control.exe" -export-contracts demos/report-demo/contracts
$env:REPORT_CANONICAL_BINARY = "$private/bin/report-control.exe"
& $py demos/report-demo/tests/canonical_probe.py --binary $env:REPORT_CANONICAL_BINARY --report "$private/evidence/canonical-python.json"
mvn -s .local/maven-settings.xml -f demos/report-demo/orchestrator-java/pom.xml test -q
```

`TestExportedContractsMatchOwningTypes` 检查导出与已提交 schema 是否漂移。`canonical-vectors.json` 包含 Unicode、UTF-16 键序、指数小数、安全整数上界、空/缺省/null 区别、数组顺序、大 epoch 字符串与行分隔符/转义文字。Go 使用主仓 RFC 8785/reference library；Python 复用主仓冻结 oracle 作独立测试；Java 用实际 `Json.read/write` 边界序列化后交给生产 Go JCS 入口核对同一语义摘要。**Java 原生 runtimeSpecDigest 不是该 JCS 摘要**，不可混用。

三个验证入口均不读取私有数据库配置。Java 缺实际 Go EXE 时用例会 skip，不能将这种情况记为跨语言通过。禁止重复键、非有限数、无效 UTF-8 或孤立 Unicode surrogate；Go 在替换字符之前使用已有 reference parser 拒绝非法 Unicode。正文金额及大计数的类型仍需遵守各 schema/领域规则。

## 2. Contract-C 映射到真实接口

路径中的 `{T}` 为 task_id，`{o}` 为 node_id，`{c}` 为 call_id。API 服务地址来自私有部署配置，不来自模型自报 URL。HTTP JSON 请求经严格解析；错误响应形如 `{"schema_version":"report-demo/1","error":{"code":"INPUT_CONFLICT","message":"..."}}`。

| 契约 | 实际接口与主体 | 结构/判定 |
| --- | --- | --- |
| C01 意图与图采纳 | planner MCP `list_capabilities/propose_plan` 或 `POST /v1/commands`；用户 `POST /v1/tasks/{T}/admit` | [propose-input](propose-input.schema.json)、[graph](graph.schema.json)；稳定 command_id、获准目录与材料、报告核验/选择路径、图摘要和 Java 编译摘要；用户确认体为 `{graph_digest}`，Cookie/CSRF/Origin |
| C02 节点入场 | worker `POST /v1/tasks/{T}/claim`、`heartbeat`、`nodes/{o}/resolve` | [lease](lease.schema.json)、[binding](binding.schema.json)；claim `{owner,domain:"graph"}`；heartbeat `{owner,epoch}`；resolve `{owner,epoch,input_digest?}`；返回 GRANTED/REUSE/WAIT 或类型化错误 |
| C03 正式提交 | worker 先 `POST /v1/artifacts` 发布候选，再 `POST /v1/tasks/{T}/nodes/{o}/commit` | [commit-request](commit-request.schema.json)、[result](result.schema.json)；正文实际 bytes/digest、a/h/e、当前 owner/session/期限与前驱资格联合校验，必要审计同事务 |
| C04 检查与批准 | checker 仍走统一 C03；用户 `artifacts/{id}/approve`、HUMAN 独立审阅 | 正式 checker 对同一报告摘要；[human-review](human-review.schema.json) 是输出事实，用户不能自填 principal/receipt；人工不能豁免程序数值/结构失败 |
| C05 受保护请求 | worker `nodes/{o}/calls` 记录纯计算远端发送意图；用户 `artifacts/{id}/deliver` 发布报告 | [remote-call](remote-call.schema.json)、[call-decision](call-decision.schema.json)；发送前持久 c/message/context；deliver 请求 `{delivery_id}`，绑定同报告摘要/renderer、合格检查、当前用户批准和独立 delivery 租约域 |
| C06 可信观察/核对 | worker `POST /v1/calls/{c}/observations`，查询同 c | [observation](observation.schema.json)；认证主体/source + event_id 幂等、context/remote task 绑定；旧 epoch 失权不丢可信迟到事实；观察不等于正式结果 |
| C07 查询/干预 | MCP `get_plan/run_status/verify_run`；`GET /v1/tasks/{T}`、`events?after=N`、`calls`；受信用户 pause/resume | [task](task.schema.json)；generation/epoch/cursor 字符串、事件按数据库数值序号排序；继续先核对原任务/许可/远端身份，不盲重发 |

C05 主流程发布器提供已检查本地 JSON 报告的可读取交付事实；它不包含银行付款。显式启用的 [验证报告](../VERIFICATION.md) 用真实 PG/HTTP 验证稳定 b、派发许可、迟到观察和网络窗口；默认不注册，也不会把计数器效果写成主任务 DELIVERED。

新增 [effect-intent](effect-intent.schema.json) 与 [effect-observation](effect-observation.schema.json)，加上发送决策共 13 份实际 DTO schema。测试效果 C05 由可信用户确认，C06 由固定提供方的受信 HTTP 客户端记录；旧 b 恢复只查询，记录观察不要求过期租约。完整 E/旁路覆盖仍以实际验收证据为准。

## 3. 身份、错误与等待语义

planner 只有 6 个 MCP 工具（目录、选择、提案、计划、运行状态、核对），没有采纳/提交/交付工具。worker 与每个 expert 使用独立服务身份；用户确认使用服务端 Cookie 会话、CSRF 和 Origin。MCP App 的单任务短期只读凭证仅在 `_meta`，不能批准或派发。服务进程同 OS 用户运行仍不是进程级秘密隔离；这个边界单独审计。

| code / 结果 | 处理 |
| --- | --- |
| GRANTED | 获得当前节点执行许可；还不是正式成功 |
| REUSE | 返回原有效正式结果/回执；不调用模型 |
| WAIT / WAITING_REMOTE | 原身份查询；不换 message_id |
| INPUT_REQUIRED / WAITING_INPUT | 等待资料；用户继续后查询原 remote task，不能热改 g/h |
| AUTH_REQUIRED / WAITING_AUTH | 修复授权后明确继续；重新核对，不推定已授权 |
| INPUT_CONFLICT | 同身份改载/关联冲突，不当成 FENCED 或自动内容重规划 |
| FENCED / LEASE_EXPIRED | 停止新控制提交；可信观察另走 C06 |
| POLICY_REJECTED / BINDING_CHANGED / INVALIDATED | 展示实际原因，禁止默认放行或复用旧批准 |
| STORAGE_UNAVAILABLE / UNKNOWN | 不推断是否已提交或外部已执行；保留原 command/call/delivery 身份查询对账 |

HTTP 409 只是状态码，Java 根据 code 分类，不能把全部 409 映射成失租。Remote INPUT_REQUIRED/AUTH_REQUIRED 与 Go checker HUMAN 是不同事实；HUMAN 必须保留程序原产物和独立人工审阅回执。

真实 Gateway 的已知机器码按白名单保留，支持 flat `code`、`error.code` 和 JSON-RPC `error.data.code`。未知正文/数值 JSON-RPC 错误保持 REMOTE_UNKNOWN（401/403 无已知码则 AUTH_REQUIRED）。不将远端自由文本作为控制指令或向投影泄露；失败的 TRANSPORT 观察仅保存方法、HTTP 状态与分类码。

| Gateway 分类码 | 节点状态 / 规则 |
| --- | --- |
| AUTH_MISSING / AUTH_INVALID / AUTH_EXPIRED | WAITING_AUTH；修复后仍核对原 call，不重新发送已有意图 |
| CALLER_NOT_AUTHORIZED / SCOPE_VIOLATION / TENANT_SCOPE_VIOLATION / AUTH_DISABLED | BLOCKED；普通恢复不能绕过 |
| RDC_UNAVAILABLE / IAM_UNAVAILABLE / AUDIT_UNAVAILABLE | UNKNOWN；保持原调用关联，不把暂时不可用当业务失败或自动新派发 |

专家模型预算参与 `report-model/1` 摘要：`DEMO_MODEL_MAX_TOKENS` 默认 12000（1024—32768），`DEMO_MODEL_NORMAL_ITERATIONS` 默认4、`DEMO_MODEL_QUALITY_ITERATIONS` 默认8（均1—32）。Python 发布与 Java 实际客户端读取同一设置；模型、端点、任一预算变化必须重新发布、重启并使用新候选。历史 6000-token 绑定仍可校验，不向旧任务补当前默认；仅改环境预算会在模型调用前拒绝。此预算是专家设置，WorkSwarm 主规划配置单独核对。

## 4. 摘要边界

- admittedGraph/binding：Go 对约定完整对象做 JCS → SHA-256，包括图/节点、材料/投影、能力、策略、上游正式结果与执行绑定；对象不同不能因哈希算法相同而互换。
- runtimeSpecDigest：Java 对原生 Blueprint 投影及条件/数据绑定的确定性编码做 SHA-256；编译期与执行前重编译比对。原生摘要与 Go JCS 身份分别保留。
- artifact ID：实际文件 UTF-8 JSON **字节** SHA-256，不重新排版后伪称同产物；内容持久化后再进入 PG 正式提交。
- Loop 分支：父节点 h、业务输入实际编码摘要、分支目标、输出摘要分别记录；不是独立人工核验。

完整正常/拒绝/等待接口样例、E 系列逐窗口请求及各条交接签收仍持续补充。生成 schema 与本页映射只证明协议交接已有具体结构，不代表所有 C/E 用例通过。


## 5. W03/W04 与 W05 的调用交接

`POST .../nodes/{o}/calls` 响应为原调用字段加 `action`，见 call-decision schema。只有首次意图事务成功返回 `SEND`；已有远端 task 返回 `QUERY`；无 task 或关联冲突返回 `UNKNOWN`。Java 按本次响应决定发送，不能用之前 resolve 的 GRANTED 推导可以再次发送。action 不写入 remote_calls，读调用记录不会恢复一次丢失的发送许可。COMMIT 或 HTTP 应答丢失时先查询原 c；UNKNOWN 不重发。本方案不声明远端 messageId 幂等或网络 exactly-once。

能力卡可带 `remote_binding={card_version,contract_version,capability_version,card_digest?}`，随 catalog/binding 摘要固定。RDC 发现位于 Java 锁外，Java按期望版本检查；Go 在发送意图短事务内重新检查当前目录、执行 profile、租约与发现快照。card_digest 为 RDC `card.rawJson` 的**原始 UTF-8 字节** SHA-256，不是 JCS 摘要。未收集实际执行实例时明确输出 not_collected。这个快照来自受信 worker，并不证明对恶意 worker 的独立远端证明。

旧目录缺 remote_binding 时，Java仅保留原1.0.0兼容路径，不向历史记录补字段/重算摘要。当前发布目录已经显式绑定版本；目录或执行profile变化后，旧待确认计划拒绝采纳，已采纳任务拒绝漂移执行，需要新候选/确认。当前不提供旧二进制和模型的多版本自动恢复；历史结果仍可按原权限读取。

部署能力逐项声明：支持原 remote_task_id 查询、正式结果复用；不声明取消和远端 messageId 幂等；Gateway/Runtime丢失task状态保持UNKNOWN。`tests/export_handoff.py`从实际任务只读生成身份、绑定、观察与结果关联投影；不导出凭据或模型正文，不替代跨组验收。

## 当前受控集成边界

| 接口 | 生产 / 校验生效 | 消费 |
| --- | --- | --- |
| `list_capabilities` → `select_capabilities` / `POST /v1/selections` | PR33纯规则窄适配；W04校验固定主体、期间、材料、真实Bus绑定，在PG保存选择记录及摘要 | WorkSwarm规划模型生成自己的图；不是Go预置完整图 |
| `graph.selection` 与采纳 | W04核对记录摘要、目录、材料、重点、选定成员；图结构校验数据边；选择引用进入图摘要，采纳事务再核对 | W03原生编译/推进 |
| W02 `plugin.credit_workbench.*` | 复用Application Plugin/local RPC；读面用后端scoped operator；写命令由Go认证密码签发用户Cookie/CSRF并核对对象 | Go/PG唯一年报状态；不启动PR30授信Go第二写者 |
| `GET /v1/workbench/{plans,tasks/<T>}` | Go只读投影，decimal string `rev=Task.generation`；状态/调用/选择/报告；读期间变化返回SNAPSHOT_CHANGED再轮询 | W02 iframe；有效乱序用BigInt比较，超时回执在RPC层丢弃 |
| `model_binding_digest` | Python发布无密钥摘要；专家进程自己校验实际ENV并用同一不可变设置建模型客户端，Go要求正式远端结果携带匹配的实际绑定证据 | Task/Binding/input_digest/Runtime证据；旧任务缺绑定不从当前默认补齐 |
| `BLOCKED` | 版本/实际模型绑定失配；Go释放图租约，普通resume拒绝 | 插件显示原因，保留观察和原任务身份 |

实际模型摘要按UTF-8字节长度前缀编码 provider/base/model/temperature/token及迭代预算，规范化base末尾斜杠，不含密钥。跨语言向量：[model-binding-vectors.json](model-binding-vectors.json)；实际DTO：[selection-query](selection-query.schema.json)、[selection-record](selection-record.schema.json)。

同任务恢复保留原o/a/c和远端task；PR34跨任务远端快路径后置。已有提取复用仍只代表本Demo原覆盖范围。完整人工审阅继续使用同一Go权威的备用可信页，不把插件缺表单冒充业务签收完成。
