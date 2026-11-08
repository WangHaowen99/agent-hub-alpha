# 60 · N3 范围框定（2026-11-08）：统一入口 + 端口化——判定真源先行

> 范式：xuefanfan 八段（同 36/50 号）+ xuefanfan agent-runtime-acceptance（openjiuwen 分支）
> 的分层律：**语义层（路由逻辑）与执行资源层（端口）分离——端口注入，不在入口内部造**。
> 写码纪律：SETools 精神（36 号 §头注）+ 本仓 v4 runbook。
> **判定真源冻结**：§三 X3 验收矩阵的期望列自邢超 198789653 §3+§4+47 号 §三.6 独立翻译，
> 冻结于任何 Go 实现之前。

## 一、IN

### 1. 三分流入口（改造 t2host_test.go 的 nodeEntry）

语义层升级（:542-563 的读取入口 → 执行入口）：

```go
// 三分流：同实例重试 / 新任务方法适用 / 首执需模型
// 端口注入（xuefanfan 律：执行资源不在入口内部造，调用方注入）
func (h *t2Host) nodeEntryExec(opID string, callerGrants []ledger.Grant, ports Ports) (NodeOutcome, error)
```

- **支路 A（同实例重试）**：op 已 completed 且 store 有结果 → 返回原回执；**端口零调用**
- **支路 B（新任务，方法适用）**：registry 有 valid method → `method.Execute()`；**端口 Model 零调用**
  （方法自己的执行是已发布知识的确定性复现，不算模型调用——T2K 的语义）
- **支路 C（首执/需模型）**：以上不命中 → `ports.Model.Invoke(opID, evidence)` → 经 adopt 机
  Adopt→Dispatch→store.put 全流程；**端口 Model 恰一次**

### 2. 端口接口（新类型，t2host_test.go 内）

```go
// Ports 是执行资源的注入面——调用方提供，nodeEntry 消费。
// 计数由实现方自带（T0 standinCalls 先例形态：e2e_test.go :107-160）
type Ports struct {
    Model ModelPort
    API   BusinessAPIPort  // bankAPISubmit 的接口化（可选本批）
}
type ModelPort interface {
    Invoke(opID string, evidence creditEvidence) s1Verdict
}
```

### 3. R/K 主场景改走入口（t2host_rk_test.go）

- R 主场景（:31-94）：`store.get` 直调改 `nodeEntryExec`（支路 A）
- K 主场景（:307-367）：`registry.findForNode + method.execute` 直调改 `nodeEntryExec`（支路 B）
- 断言语义不变；加端口计数断言（A=零调用 / B=零模型调用）

### 4. 阳性对照 + overlay 收编（新测试 TestX3_*）

- **阳性对照**："必须经模型"场景（支路 C），端口注入恒败实现 → 测试必须红
- **overlay 收编**：测试内破坏 nodeEntryExec（恒返 error）→ R/K 主场景必须红（**预期翻转**——
  邢超 198789653 §4 原文："修复后应更新验收预期"）

### 5. failingProbe 退役

`failingProbe.invoke`（:174-178）全仓零调用点——死代码。端口计数实现替代其角色；
退役 = 删（探针接线证明由端口计数器的阳性对照承担）。

## 二、OUT

- 不改 `adopt.Machine`（scope head=P0-B 批）
- 不改 schema（0.2.0 批）
- 不修 `resultStore.put` 非原子（已知缝隙，独立批）
- 不做 ToolPort 完整实现（本批 Ports.API 可为接口占位——bankAPISubmit 已有计数，不必改）
- 不做真 LLM（用户启动）
- 不建验证机器（105 号立法）

## 三、X3 验收矩阵（判定真源——冻结于实现前）

期望逐行引邢超 198789653 §3（overlay 实验）与 §4（验收预期更新），翻译依据留档：

| # | 支路/场景 | 预期端口行为 | 预期结果 | 翻译依据 |
|---|---|---|---|---|
| X3-1 | A 同实例重试（completed+stored） | Model=0, API=0 | 原回执返回 | "首次执行、重试与新任务经过同一入口"（198789653 §五.5） |
| X3-2 | B 方法适用（registry valid） | Model=0 | 方法结果，新任务身份 | "有效方法适用时产生新任务自己的结果"（同上） |
| X3-3 | C 首执 | Model=1 | 判定经端口产生 | "实际模型/工具调用通过可替换端口接入"（同上） |
| X3-4 | **阳性对照**：C 场景 + 恒败端口 | 恒败被触发 | **测试红** | "确实需要模型时失败探针必触发"（198789653 §五.5 原文对照） |
| X3-5 | **overlay 收编**：破坏 nodeEntryExec | — | **R/K 主场景红** | "破坏对应入口后，相应 R/K 测试在目标处失败"（198789653 §六 R6 原文） |
| X3-6 | 授权失败（无 grants） | 全端口零调用 | 入口拒 | 既有 E5 断言语义不变 |
| X3-7 | 未完成操作（非 completed） | 全端口零调用 | 入口拒 | 既有 L39 断言语义不变 |
| X3-8 | 老的 nodeEntry 兼容 | — | 保留可用 | 既有 E5/T2R_PermissionRevoked 调用点零改动 |
| X3-9 | C 首执**全流程**（R74 B-1 清偿补行） | Model=1 | machine=dispatched + store 有判定派生回执（新任务身份） | 60号 §一.1 冻结规格原文「经 adopt 机 Adopt→Dispatch→store.put 全流程」——首版实现漏流程属未记录偏离，R74 空白子以临时探针证伪后补行 |
| X3-10 | C→完成→重试**经入口闭环**（R74 B-1 奇点推论补行） | 首次 Model=1，二次 Model=0 | 二次调用走 A 支路返回 C 存的回执 | 198789653 §五.5「首次执行、重试与新任务经过同一入口」——无 X3-9 流程则 C 路径对 A 不可达，统一入口对首执空心 |

**Γ 失败判据**（预注册）：
- X3-1/2/3 任一端口计数与预期不符 → 实现错
- X3-4 恒败端口不触发红 → 阳性对照是装饰（探针注册≠消费同病）
- X3-5 破坏入口 R/K 不红 → 预期翻转未落地（仍在绕）
- 既有 21 包任一红 → 回归

**预期翻转声明**：邢超 overlay 原始预期"破坏入口→R/K PASS（缺陷）"在本批后翻转为
"破坏入口→R/K FAIL（正确）"。这不是改测试放水——是修复后验收预期必须更新（其原文要求）。

## 四、frame-bench 碰撞（4-lens + singularity——支撑项与反向检查）

### 支撑项（骑轮子检查）

| # | 支撑 | frame-bench 证据 | N3 应用 |
|---|---|---|---|
| S1 | 判定真源形态：初态×预期终态表 | state-matrix.tsv（id\|setup\|force\|rc\|assert） | X3 矩阵即 state-matrix 形态（格#\|支路\|预期端口\|预期结果\|依据）；新发现=补行 |
| S2 | 期望先行、实现后跑表 | run-matrix.sh 逐行构造→跑→验 | X3 矩阵冻结→实现→逐行跑 |
| S3 | "新发现补行即收敛" | REVIEW-LEDGER 收敛判据 | 审查循环发现新格→补 X3 行→收敛 |
| S4 | ctags 定位先例 | gen-map.py 用 ctags 建符号索引 | grep nodeEntry/store.get/registry.find 全部调用点（ctags 形态的 Go 等价=go grep） |

### 反向检查（singularity——严格无反向才开工）

| # | 反向风险 | 检查结果 | 判 |
|---|---|---|---|
| R1 | 三分流是否有第四态（不命中任何支路的输入）？ | 支路 A 检查 completed+stored；B 检查 registry valid method；C 是 else 兜底——Go 的 if/else if/else 结构保证穷尽 | **无反向** |
| R2 | 端口计数是否可被绕过（不经端口的路径）？ | 支路 B 的 method.Execute 不是端口调用——方法执行是已注册确定性代码，不是外部资源调用（T2K 语义：模型调用≠方法执行）；端口定义恰好覆盖"外部资源"边界 | **无反向**（但 B/C 边界须在 doc 声明：method.Execute 是内部确定性执行，不计数——如果未来方法内部调模型，那是方法自身的实现细节，不在本入口的计数辖域） |
| R3 | R/K 主场景改走入口后，原直调断言语义是否漂移？ | 改动是"调用方式换入口"，断言的值不变（回执内容/方法结果内容）；端口计数是**新增**断言不替换既有断言 | **无反向** |
| R4 | X3-5 overlay 收编的破坏形态是否等价于邢超原实验？ | 邢超 overlay 用 Go -overlay 替换 nodeEntry 为恒错；X3-5 在测试内做同型破坏（函数替换）——形态等价 | **无反向** |
| R5 | 保留老 nodeEntry 是否引入"两条入口并存"新缝？ | 老 nodeEntry 是读取入口（已 completed 的结果服务），新 nodeEntryExec 是执行入口（三分流）；语义不同、名不同、调用方不同——非并存 | **无反向**（doc 须声明两入口的职责差异） |

**结论：五项反向全部排除——开工。**

## 五、验证协议（2026-11-08 执行完毕——全部通过）

1. ✅ X3 矩阵全绿（原八行 7 个测试函数：X3-6/X3-7 同函数两段；R74 清偿补 X3-9/X3-10 两行并入 X3-3 + 新增 tripwire X3_NoBypass——现十行 8 个测试函数，`grep -c '^func Test' x3_entry_test.go` = 8 当轮复算）
2. ✅ 既有全量 `go test ./... -count=1 -race` 21 包绿（零回归，failingProbe 退役后复跑）
3. ✅ 变异探针五连（cp 备份纪律，逐个还原后复跑全绿）：
   - **M1 删支路 A** → `TestX3_BranchA` + `TestT2R_PersistedReceiptAcrossRestart` **双红**
     （R 主场景已真走入口——翻转活体成立的第一证）
   - **M2 删支路 B** → `TestX3_BranchB` + `TestT2K_NewTaskZeroModelCalls` **双红**
   - **M3 删支路 C 的 Invoke** → `TestX3_BranchC` + `TestX3_PositiveControl` **双红**
     （端口计数不是装饰——删接线阳性对照即红）
   - **M4 删在途守卫** → `TestX3_AuthorizationAndStateGates` **红**（X3-7 面）
   - **M5 入口恒错（邢超 overlay 形态内部化）** → `TestT2R` + `TestT2K` **双红**
     —— 邢超原实验中破坏入口 R/K 仍 PASS 是缺陷；本批后预期翻转落地：
     **破坏入口 → R/K 红（正确）**。翻转不是纸面声明，是 M5 的执行记录。
4. ✅ gofmt 零输出 / `go vet ./...` 干净

**实现期两处对冻结矩阵的偏离与修正**（如实记录）：
- X3-7 首版测试用「从未采纳的 op」断言走 C——那测的是真首执（X3-3 已覆盖），
  不是矩阵冻结的「未完成操作」。已改回冻结语义（adopted+dispatched 未 completed
  → 入口拒+零端口调用），并发现实现本体会让在途 op 掉进 C 重复执行——**矩阵行
  先于实现抓到病**（判定真源先行的价值实例）。修法=A/B 之间加在途守卫（M4 证明可检出）。
- X3-5 首版用本地 `broken` wrapper（恒错函数恒错——同义反复）。R/K 真走入口后
  改为：钉住 R-path 调用形态经真实入口 + 错误面可达性断言；活体翻转证明移交 M5 变异记录。

## 五.五 R74 审查清偿（2026-11-08——空白子 1B+2M+3m 全处置）

- **B-1（C 支路缺全流程——三处声明与实态不符）**：冻结规格 §一.1 说「Invoke →
  Adopt→Dispatch→store.put 全流程」，首版实现只 Invoke 即返回（本号 §五首版验证
  漏检——X3-3 旧期望列不含流程面）。**裁决=补实现**（判定真源优先于实现现状）：
  入口签名加 `firstExecInput`（首执输入束：采纳请求+业务证据，调用方语义非执行资源，
  不违 xuefanfan 分层律）；C 支路 Invoke 恰一次后跑全流程。补 X3-9（流程面）+
  X3-10（经入口闭环面）两行。验证突变：删流程块 → TestX3_BranchC 红
  （**首败于 X3-9 面**；X3-10 面同函数位于其后、该突变下不可达——其独立可达红
  是闭环破坏形（流程保留而 complete→A 断）。内环 iter-2 m-1 改真口径）。
- **M-1（56 号外发回复数字声明失实）**：复算确认——「17 个新提交」实为 18
  （6240acb..25ab91e；17=92c9f69**^**..25ab91e 含 51 号本提交、排除 49 号存档注
  的口径——内环 iter-1 B 抓出 61 号首稿把该命令写成无 `^` 形（所写命令复算=16），
  命令→数字配对已改真）；「47111b9 + 17」任何区间对不上（47111b9..25ab91e=40）；
  「12 测试全绿」实为 11（3 顶层+8 子）。**处置=61 号更正稿**（外发待用户口令——
  内环 iter-2 M-1 曾按仓内记录误裁 56 号未发；收轮后 gitcode API 机械验证翻转：
  评论 199072885 在场=已发不可撤回，更正评论为唯一路径。发送锚
  `56-reply-xingchao-r6-sent.md` 补建，⑥ 条款补「机械验证优先」细化）。
- **M-2（§六.3「防线常驻」为假——绕行突变存活）**：空白子把 R 主场景回退直调
  store.get（邢超 overlay 病形态）跑全量=绿——M5 变异记录是一次性的，不是常驻防线。
  **处置=补常驻 tripwire**（TestX3_NoBypassInRKScenarios：R/K 主场景函数体内
  禁含 .store.get(/.registry.findForNode(/.execute( 直调——**直改形态**绕行即红，
  验证突变已跑）+ §六.3 改真。
  **内环 iter-1 M-1 追清**：该 tripwire 是源码子串**文字钉**，开放集绕过形态
  实测在案——①helper 抽取（直调移出函数体，体内只留 helper 调用）全绿；
  ②首匹配锚定欺骗（真函数前插含函数名的注释行，strings.Index 锚到注释）全绿。
  attack-corpus §二「形态钉的开放集」条款适用——文字钉在测试场景源码面无更强
  行为观测面可换（常驻行为面只能证明「调了入口」，证明不了「没绕开入口」——
  绕开本体是测试源形态问题）。**如实标注形态受限**（代码注释+§六.3），不撤
  ——对「新代码不小心又直调」的事故形态它有效（直改突变双红）。
- **m-1/m-2/m-3（外环三轮，全修）**：文件头 DESIGN 段与 TestT2R doc 的幽灵适配器
  叙述改真（外环 m-1）；completed-无存档从在途守卫分出独立面+诊断词改真+X3-6b
  永久探针（外环 m-2）；RETIRED 注释方向词 above→below（外环 m-3）。
- **内环 iter-2 追清（1M+3m）**：M-1=61 号「已发不可撤回」前提与仓内记录冲突
  （56 号状态行仍「待发」、无发送锚）→ 按仓内记录为准改写 61 号前置段（默认路径=
  更正并入 56 号后发，发送与否用户裁）；m-1=§五.五 红面口径改真（同函数首败不可
  压缩成多面红）；m-2=firstExecInput.evidence 腿无观测面 → countingModel 记录
  证据标量投影+X3-3 断言（吞证据突变已验红）；m-3=m-3 标签撞号 → 本段双侧加
  「外环/iter」限定消歧。
- **内环 iter-3 追清（2M+2m）**：M-1=证据投影漏 veto 载体腿（overdue slice 无观测，
  外科吞 slice 突变全绿）→ 补 lastOverdueLen+夹具证据带窗外短逾期（len=1 可判别）
  +外科吞突变验红；M-2=corpus §二「文字钉禁用」行与 runbook R74 ⑤ 相反规则 →
  corpus 行加 R74 ⑤ 例外指针（guard-fixture 面仍禁用/场景源码面允许带标注）；
  m-1/m-2=61 号条款指针错挂（「iter-2 条款②」「火力面条款①」无可解析锚/错真源）
  → 改挂 runbook §四 R74 ⑥/①。
- **内环 iter-4 收敛轮（0B/0M/4m，同 diff 收尾卫生全处置）**：m-1=len 投影元素级
  盲区（同 len 异内容吞换实测全绿）→ KNOWN BOUNDARY 注释+⑩ 条款；m-2=⑤ 辖域词
  单侧 → 双侧同步；m-3=61 号第二处指针缺 → 补（runbook §四 R74 ①）；m-4=记账链
  iter-3/4 断链 → 本段续写闭合。**内环 0B/0M 收敛，跳出。**

## 六、先验攻击面

1. **支路 B 的 method 边界**：`knowledgeMethod.Execute` 是 interface 方法——未来实现可能内部调模型。本批 doc 声明"方法执行是确定性复现"；如果方法实现引入模型调用，那是方法的边界不是入口的
2. **端口注入时机**：Ports 由调用方构造——如果测试忘了注入 Model（nil），支路 C 会 panic。入口须做 nil 守卫（nil ports.Model → 返回 error 而非 panic）
3. **X3-8 兼容面**：老 nodeEntry 保留，但如果新代码不小心又直调 store/registry，绕行复现。防线=**双面**（R74 M-2 改真：原「X3-5 收编常驻」声明为假——M5 变异记录是一次性证明，绕行突变当时存活全绿）：①常驻 tripwire `TestX3_NoBypassInRKScenarios`（R/K 主场景函数体禁含直调符号——**直改形态**即红，验证突变已跑；**形态受限声明**：源码子串文字钉的开放集绕过——helper 抽取/首匹配锚定欺骗两形态实测可绕（内环 iter-1 M-1），文字钉在场景源码面无更强行为观测面可换，如实标注不撤——对事故形态（不小心直改回直调）有效）；②入口侧五连变异记录（M1-M5 归档 §五）
4. **firstExecInput.req 与入口 opID 的一致性**（R74 内环 iter-1 m-3，文档化通道）：
   Adopt 用 req 身份、Dispatch 用入口 opID——错配时靠 Dispatch 失败**间接**拦截
   （fail-loud ✓，但诊断词报 dispatch 状态而非身份错配，且失败路径在机器留下一个
   永不 dispatch 的孤儿 adopted 事件）。需调用方自伤才可达，fixture 层趋零——
   文档化，不修。
