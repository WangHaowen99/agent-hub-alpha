# N3 交接文件：统一入口 + 端口化（t2Host nodeEntry 垄断化）

> 给即将接手的智能体。你对本战场不熟——本文件自包含，先读背景再动手。
> 仓库：`~/nga-impl-wt`（git worktree，detached HEAD `25ab91e`，分支锚 `feat/agent-festival-r57`）。
> 所有代码在 `agent-festival/go/` 下。用 Go 1.24。

---

## 一、这个战场是什么（30 秒版）

我们在建一个智能体框架 NGA 的**参考宿主**（Go 测试代码形态，不是生产服务）。邢超（企业方架构师）
在 gitcode `xingchao_ml/nga` 的 issue #1 做验收。他用 overlay 破坏实验证明：我们的 R/K（结果复用/
知识复用）主场景测试**绕过了统一入口** `nodeEntry`，直接调用 `store.get` 和 `registry.findForNode`
——"探针注册≠探针消费"。修复=N3：把 nodeEntry 升级为**唯一执行入口**（三分流+端口化）。

## 二、N3 要做什么（精确规格）

### 目标

`nodeEntry` 成为 R/K 的**必经路径**，带可替换端口（模型/工具/业务 API 的注入点），使"复用"与
"新判断"的行为差异**可被端口计数测量**。

### 三分流规格（X3 验收矩阵的三个支路）

```
nodeEntry(opID, callerGrants, ports) → NodeOutcome
  │
  ├─ 支路 A（同实例重试）: op 已 completed 且有 stored result
  │    → 返回原回执，端口零调用（复用）
  │
  ├─ 支路 B（新任务，方法适用）: registry 有 valid method for this node
  │    → method.execute()，端口零模型调用（知识复用——方法自己的执行不算模型调用）
  │
  └─ 支路 C（首执/需模型）: 以上都不命中
       → ports.Model.Invoke(...)，端口恰好一次调用
```

### 端口接口（最小形态）

```go
type Ports struct {
    Model ModelPort   // 模型席位（首执用）
    Tool  ToolPort    // 工具端口（可选，本批可不实现）
    API   BusinessAPI // 业务 API（现有 bankAPISubmit 的接口化）
}

type ModelPort interface {
    // Invoke 做一次模型调用并返回判定。实现方计数。
    Invoke(opID string, evidence creditEvidence) s1Verdict
}

// 端口的计数由实现方自带（就像 T0 的 standinCalls——见下方先例）
```

### 验收（X3 实验矩阵——先冻结期望再写实现，判定真源先行纪律）

| 支路 | 预期端口行为 | 断言 |
|---|---|---|
| A 同实例重试 | Model 调用=0, API 调用=0 | 返回原回执 |
| B 方法适用 | Model 调用=0 | 方法结果带新任务身份 |
| C 首执 | Model 调用=1 | 判定经端口产生 |
| **阳性对照** | **故意给一个"必须经模型"的场景，端口注入失败实现→测试必须红** | "真需要模型时探针必触发" |
| **overlay 收编** | 破坏 nodeEntry（改为恒错）→ R/K 主场景测试必须红 | 邢超方法的内部化 |

### 预期翻转声明（预注册——防"修到测试变绿为止"）

修复后，邢超原 overlay 实验的预期**翻转**：原来"破坏入口→R/K 测试 PASS（缺陷）"，
修复后"破坏入口→R/K 测试 FAIL（正确行为）"。这不是改测试放水，是**修复后验收预期必须更新**
（邢超 198789653 §4 原文要求："新版本修复后应更新验收预期，不能把永远复现旧问题当作新的目标"）。

## 三、现有代码结构（你要改的文件）

### `internal/e2e/t2host_test.go`（核心宿主——主要改动面）

- **`t2Host` struct**（:418）: 有 `machine *adopt.Machine`、`store *resultStore`、
  `registry *knowledgeRegistry`、`probes map[string]*failingProbe`
- **`nodeEntry`**（:542-563）: 当前只做"授权检查+completed 状态检查+store 读"——是读取入口
  不是执行入口。**你要把它升级为三分流执行入口**
- **`adoptAndDispatch`**（:567-587）: 首执流程（Adopt→Dispatch→store.put）——支路 C 的底层
- **`resultStore`**（:84-93）: `put` 是非原子 `os.WriteFile`（已知缝隙，不在本批修）
- **`failingProbe`**（:164-190）: 零调用探针——**`invoke` 方法全仓零调用点**（死代码），
  端口化后它的角色由端口的计数实现替代

### `internal/e2e/t2host_rk_test.go`（R/K 主场景——你要改的测试）

R 主场景（`TestT2R_PersistedReceiptAcrossRestart` :31-94）**直接** `store.get("op-t2r")`（:66）。
K 主场景（`TestT2K_NewTaskZeroModelCalls` :307-367）**直接** `registry.findForNode` + `method.execute`
（:333-342）。两者都不经 `nodeEntry`。**你要把主场景改走 `nodeEntry`**（保留断言语义不变）。

### 先例（本仓已验证的形态——照抄）

- **T0 的计数器**（`e2e_test.go` :107-160）: `bankAPISubmit` 是业务 API 桩，`standinCalls` struct
  记 s1Judge/s2Query/bankAPI 各自计数——**这是端口计数的正确范式**，X3 直接复用这个形态。
- **`grantsCoverRead`**（`t2host_test.go` :504-535）: 授权内容匹配——nodeEntry 已有，保留。

## 四、你不需要做的事（边界）

- **不改 `adopt.Machine`**（生产件——scope head 是下一批 P0-B 的事）
- **不改 schema**（0.2.0 批的事）
- **不修 `resultStore.put` 非原子**（已知缝隙，已登记，独立批）
- **不做 true LLM 实验**（用户本人启动，刚性纪律）
- **不建验证机器**（审查用零机器流程——见下方 runbook）

## 五、开发纪律（这个战场的规矩——违反会被审查循环抓）

1. **判定真源先行**：X3 验收矩阵的期望**先冻结在范围档里**（照 50 号 §三的格式：每格附
   翻译依据），然后才写实现。期望不从实现反推——这是本战役最重的病（W1）的制度化防线。
2. **写码前看先例**：grep 一个已过审查的文件看形态（`e2e_test.go` 的计数器/`t2host_test.go`
   的授权检查都是先例）。注释风格：解释"为什么"不解释"什么"。行宽 ≤120。
3. **写完自检**：`gofmt -l .` 零输出、`go vet ./...` 干净、`go test ./... -count=1 -race` 21 包绿。
4. **变异还原用 cp 备份**：做突变实验时先 `cp file /tmp/backup`，还原用 `cp /tmp/backup file`。
   **永远不要用 `git checkout --` 还原**——工作树有未提交改动时这会洗掉你的工作（本战役两次事故）。
5. **COMPOSED 面**：如果你的改动涉及判定+验证两侧，加组合断言（`verify(s1Judge(e)).ok`）。
6. **审查循环**：N3 完成后要过 review-loop（空白子 4-lens+singularity，直到外层连续 2 轮
   0 BLOCKER/0 MAJOR）。流程正本：`agent-festival/docs/review-loop-runbook.md`（v4，零机器形态）。

## 六、验证协议（做完后必须全过）

```bash
cd ~/nga-impl-wt/agent-festival/go

# 1. 基本卫生
gofmt -l .                          # 零输出
go vet ./...                        # 零输出

# 2. 全量回归（不能破坏任何既有测试）
go test ./... -count=1 -race        # 21 包全绿

# 3. X3 新测试全绿
go test ./internal/e2e/ -run 'TestX3' -count=1 -v

# 4. 阳性对照：端口注入失败实现→"必须经模型"场景红
go test ./internal/e2e/ -run 'TestX3.*Positive' -count=1 -v

# 5. overlay 收编：破坏 nodeEntry→R/K 主场景红（预期翻转验证）
#    （在测试内做，跑完还原——cp 备份）
```

## 七、背景阅读（按需，不强制全读）

| 文件 | 内容 | 何时读 |
|---|---|---|
| `docs/review-loop-runbook.md` | 审查循环流程 v4 | 提交前必读 |
| `docs/attack-corpus.md` §二 | 审查火力面（判据×用例映射） | 审查循环时 |
| `docs/minor-ledger.md` | 114 条方法论实证台账（很长，grep 按需） | 遇到具体判据时 |
| `~/nga-impl-wt/impl-2026-11-01/50-p0a-scope-prior.md` | 判定真源冻结的样例（§三） | 写范围档时照抄格式 |
| `~/nga-impl-wt/impl-2026-11-01/53-collision-merged-51-52.md` §3.4 | N3 件的完整 A/G/V 规格 | 设计时 |

## 八、提交纪律

- 本地提交（`git add` + `git commit`）可以自主做
- **推送到 MR 须用户口令**（逐次授权，惯例推断不是授权）
- 提交 message 格式：`feat(e2e): N3 统一入口+端口化——三分流+X3 验收矩阵`

---

*本文件由操作员（yaojun）2026-11-08 编写。上游上下文：邢超 198789653 §5 overlay 实验 + 47 号回复
§三 + 53 号碰撞合账 §3.4 N3 件 + 54 号奇点博弈 §3.4 红队 #4（端口计数强制）。*
