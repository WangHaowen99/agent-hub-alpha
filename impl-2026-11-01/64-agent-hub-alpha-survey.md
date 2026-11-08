# 64 · agent-hub-alpha 调查报告（2026-11-08）

> 任务：取代 agent-festival 战场地位的新仓全量调查。来源：gitcode `xingchao_ml/agent-hub-alpha`
> + GitHub `chaosxingxc-orion/agent-hub-alpha`（最终集成端）。克隆核查 + blob 级机械比对 +
> 本地复跑。**用途：以后我方基于本仓工作。**

## 一、基本盘

- 创建 2026-11-08 20:51，公开仓，仅 `main` 分支，无 MR，3 提交
  （9e87fdd init / ba0e3b8 主导入 / 2e47d93 chmod 修复）。
- **双端结构**：GitHub 是**最终集成端**；GitCode 是团队工作区（事件触发+5 分钟轮询同步）。
  桥规则：`gitcode/mr-N` ↔ `github/pr-N` 命名空间桥持有；无自动批准/合并/强推/删支；
  分歧可见失败；评论带作者与源链接双向同步；nga-bridge 标记防回环。
- **issue #1**（新讨论入口，GitHub 正主 0 评论、GitCode 同步副本）：「R1–R6：参考组件已
  接收、系统开放项与拆仓后协作入口」。旧 NGA issue #1 历史保留，新架构讨论在本仓。

## 二、导入内容（与我方 25ab91e 的关系——blob 级验证）

**基线 = GitHub NGA `fad91b05`**（范围化接收提交，parents 含 `25ab91e`——**「保留祖先」
≠「25ab91e 快照」**，R75 M-1 改真）：我方 25ab91e 的 `agent-festival/` 子树与 hub 相比
**恰 4 处差异**（`README.md` 头部改写 + `review/2026-11-08/` 三件——均为邢方新增件，
见 §三第 3/7 项），其余路径全等（抽样三文件 blob 验证：t2host_test / minor-ledger /
58 号回执）；manifest（`docs/migration/nga-source-manifest.json`）逐文件记 blob，
来源=`fad91b05`。impl/replies 路径集合与 25ab91e 一一对应（无 59-63），**但其中两件
blob 不同**（R75 iter-1 M-2 披露）：`50-p0a-scope-prior.md` 与 `56-reply-xingchao-r6-draft.md`
在 fad91b05 各带维护者纠正注记头（50 号：「低档无出处/不可达」全局推论撤回——下带
截短引文问题；56 号草稿：「历史发送稿，数字以 199085266 更正为准」）——hub 归档的
我方过程稿已是邢方标注版。

复算命令与输出（⑬/⑳ 单仓可执行形态——每行注明执行仓）：
```
cd ~/nga-impl-wt && git rev-parse 25ab91e:agent-festival        # → 192c7a77…
cd ~/agent-hub-alpha-inspect && git rev-parse main:agent-festival  # → 16b1d15f…（DIFFERS）
cd ~/nga-impl-wt && git ls-tree -r 25ab91e --name-only agent-festival | sort > /tmp/a
cd ~/agent-hub-alpha-inspect && git ls-tree -r main --name-only agent-festival | sort > /tmp/b
diff /tmp/a /tmp/b     # → 仅 review/2026-11-08/ 三件（路径级新增）
cd ~/nga-impl-wt && git rev-parse 25ab91e:agent-festival/README.md       # → 397644db
cd ~/agent-hub-alpha-inspect && git rev-parse main:agent-festival/README.md  # → e78d843e（blob 差）
cd ~/nga-impl-wt && git rev-parse 25ab91e:agent-festival/go/internal/e2e/t2host_test.go
                       # → e69cb7dd（hub 同；minor-ledger 8d4e0b7 / 58 号回执 cf81ebd 同法全等）
```

**全树差异枚举（iter-3 B-2 改真——25ab91e → hub main，三分口径）**：

```
cd ~/nga-impl-wt && git ls-tree -r 25ab91e > /tmp/ours-full.txt
cd ~/agent-hub-alpha-inspect && git ls-tree -r main > /tmp/hub-full.txt
python3 - <<'JOIN'
ours, hub = {}, {}
for f, d in [("/tmp/ours-full.txt", ours), ("/tmp/hub-full.txt", hub)]:
    for line in open(f).read().splitlines():
        if line:
            meta, path = line.split("\t", 1); d[path] = meta.split(" ")[2]
M = sorted(p for p in ours if p in hub and ours[p] != hub[p])
D = sorted(p for p in ours if p not in hub)
A = sorted(p for p in hub if p not in ours)
print(f"M={len(M)} D={len(D)} A={len(A)}")
JOIN
# → M=11  D=151  A=10
```

- **M（两侧在场、blob 差）＝11**：`.github/` 四件（GITCODE-MIRROR.md d642fc98→ecbc688b、
  sync_gitcode.py 4699a8f5→70fe426d、test_sync_gitcode.py 4789ef7e→1012b53d、
  sync-gitcode.yml 65b5a66f→81e29c04——**hub 改写了我方在场的镜像桥四件**）+
  `.gitignore` e7d60574→a86b6af6 ＋根 `README.md` cc69c3ea→51971ad6 ＋
  `achitecture design/README.md` 6a522cf0→335e94d3 ＋`agent-festival/README.md`
  397644db→e78d843e ＋`design/nga-overall-design.md` 1c4a21ca→c38a7cff ＋
  impl 两件（50/56 号维护者注记版 c22013ac→baef0a09 / d6175b78→ef1e34f4）。
  其余同在场路径 blob 全等（含 agent-festival 子树——「恰 4 处」即 README(M)+review×3(A)）。
- **D（仅 25ab91e，未迁）＝151**：CLAUDE.md 1＋achitecture design 旧 07-15 共 9＋
  experiments 84＋proposal 6＋survey 13＋tools 21＋wiki 17（模型研究线+V0 原文历史留 NGA）。
- **A（仅 hub，新增）＝10**：`.gitattributes`＋collaboration_bridge.py+其测试＋
  07/08 两件新提案文档＋review/2026-11-08/ 三件＋docs/migration/ 两件。
- **manifest（fad91b05）与 hub main 恰 2 件 blob 不符**（arch README dec1e97b→335e94d3、
  design 5abc3b9e→c38a7cff——导入提交 ba0e3b8 的导入期调整）：「基线=fad91b05」
  对该两件为路径级真、blob 级需此注记。复算（于 hub 克隆）：
  ```
  cd ~/agent-hub-alpha-inspect && python3 -c "
  import json, subprocess
  m = json.load(open('docs/migration/nga-source-manifest.json'))
  bad = [(f['path'], f['blob'][:8], subprocess.run(['git','rev-parse','main:'+f['path']],capture_output=True,text=True).stdout.strip()[:8]) for f in m['files'] if subprocess.run(['git','rev-parse','main:'+f['path']],capture_output=True,text=True).stdout.strip() != f['blob']]
  print(len(bad), bad)"
  # → 2 [('achitecture design/README.md', 'dec1e97b', '335e94d3'), ('design/nga-overall-design.md', '5abc3b9e', 'c38a7cff')]
  ```

导入根：`achitecture design/`（05/06 原件+07/08 新写）、`design/`（英文重写）、
`agent-festival/`、`impl-2026-10-31/`、`impl-2026-11-01/`（**00-58，无 59-63**）、
`replies/`（原件）。

**未导入**（留 NGA 仓）：`experiments/ survey/ wiki/ tools/ proposal/`（模型研究线）+
旧架构文档 07-15（V0 原文历史在 NGA；本地 architecture-history 目录被 ignore）。

## 三、新增内容（非我方产出的）

行数复算（于 hub 克隆）：`cd ~/agent-hub-alpha-inspect && wc -l "achitecture design/07-nga-architecture-proposal.md" "achitecture design/08-r1-r6-acceptance-and-open-items.md"` → 335/78。

| # | 件 | 内容要点 |
|---|---|---|
| 1 | `achitecture design/07-nga-architecture-proposal.md`（335 行，见上行 wc） | **唯一活提案**（原地更新、不再编号迭代；新件用英文）：首发交付=Java 三件套 `nga-sdk`（纯 Java 核心）/`nga-openjiuwen`（适配器）/`nga-spring-boot-starter` + 授信参考应用 + conformance 夹具。**M01-M15 验收门**（生命周期/幂等/恢复/重放/扩展）。四增量序列：契约基线→垂直切片→生命周期/扩展→企业扩展。openJiuwen 是工程基座；**Go agent-festival 定位=组件契约研究参考，不替代 Java 主基座** |
| 2 | `achitecture design/08-r1-r6-acceptance-and-open-items.md`（78 行） | **当前权威验收状态**（详见 §四） |
| 3 | `agent-festival/review/2026-11-08/` | 可复跑探针三件：`run_probes.py`（go -overlay 注入临时测试包，**非零=已知系统缺口**）+ `system_contract_probes.go.txt`（7 探针：3 PASS/4 FAIL）+ README（结果表）。探针复现反例 2/3/4+新反例 5 FAIL；下带/epoch/反例 1 PASS |
| 4 | `design/nga-overall-design.md` | 英文重写版三项目纲领：①模型研究（NGA 仓）②S1+S2 框架（本仓）③企业工作台。P1-P11 原则 |
| 5 | `docs/migration/` | 迁移边界+manifest；**后续交付规则：从新仓 main 开分支；旧分支后续提交只导入架构路径改动并记原提交来源；不要把旧 NGA main 整合进来** |
| 6 | `.github/` | 跨平台协作桥。**6 件中 4 件为我方在场件的改写**（GITCODE-MIRROR.md/sync_gitcode.py/其测试/sync 工作流——blob 差见 §二 M 清单），2 件新增（collaboration_bridge.py+其测试） |
| 7 | `agent-festival/README.md` | 头部加「2026-11-08 接收范围：研究参考组件」声明（正文仍是设计仓旧文案——「20 包」实测为 21 包，陈旧） |
| 8 | 根 `README.md` | 定位/导航/设计约束（一体三态四面/管理面治理/S1+S2/框架生效/幂等≠正确/复用开放/业务边界）。**改写自 25ab91e 存量**（cc69c3ea→51971ad6），非纯新增 |

## 四、08 号验收状态——对我方有直接冲击的新事实

**接收定性**：研究参考组件+可复现实验+显式保留缺口的宿主原型。不解释为生产 SDK
完成/openJiuwen 集成完成/Jev 已验证/F-02 闭环通过。**历史过程稿强结论以本表范围化判断
为准**。

**独立复核实录**（Ubuntu 24.04 / Go 1.24.13；Windows autocrlf 会坏 TSV 字节须
`core.autocrlf=false` 导出——环境坑备案）：Go 全套+Python 八组夹具通过；P0-A=
3 顶层+8 子测试（不得当独立样本量）；反例 1 PASS；**反例 2/3/4 FAIL**（与我方
声明一致）。

**三个必须纠正的推论（打我方三处）**：
1. **下带规则有原始出处**——负责人原文全文含「而在客户资产小于100万的时候，授予1万
   美金的信用贷」；后续评论截短致我方 Y 实验「grant-10000 原文不可达」发现**被证伪**
   （材料截短≠业务未定义；「独立翻译共享截短材料，独立人数再多也不提供完整性证明」）。
   代码本就有下带（新对照 PASS）。
2. **X-S2 只证伪弱实现**——参数臂删了 `Current(ι,σ)` 与对象绑定，是弱实现反例非完整
   公式反例。工程收紧被接受（受信状态读绑定、epoch 保留为并发条件）；验证须补真实操作
   绑定/同键改载荷/重试/独立作用域/并发/重启。
3. **八格闭合需限定**——日历边界歧义须写成 `Y(d | approved assumptions)`；覆盖不足格
   只测来源名 helper 非统一入口验证；补证/等待多控制动作不能翻译者自行删减；Γ 事后补记
   保留「回顾性验收」标签。

**新反例 5（我方未见）**：删除必需 TimeWindow 配置→测试态 verify 跳过窗口检查放行
近期逾期授信——「缺失运行时配置验证提供方的组合反例」。进入运行时前必须明确谁拒绝
损坏/缺失的发布规约。

**下轮交付边界（邢超钦定四条）**：
1. **N3 统一入口**（重试复用原结果/新任务知识消费自己身份/模型探针必须真实被调用）
   ——我方本地刚完成（e6b7bab+f671972），hub 快照还是 N3 前状态
2. **生效批**＝我方 P0-B + 扩展：对象动作/参考时间绑定（原 N6 件提前）、固定检查者
   版本、发布必需检查集、检查实际执行记录、适用性/保鲜、保守重验、权威有效修订、
   条件提交与恢复
3. 结果与审计（不可静默覆盖/候选-已采纳-已生效-失效区分/真实拒绝终态留存/未知 API
   结果按能力等待-查询-取消-补偿）
4. 四面责任（管理面批准+降级/控制面准入/运行面真实事实/用户面如实呈现；S2 运行时
   不改生产图）

另：check/control 分族被接受为组件设计选择（须说明故障策略）；q/b/w/s/v/p 六字段
还须保留 L 的适用性/保鲜语义或明确不可变引用来源。

## 五、对我方战场的直接影响

1. **N3 需交新仓**——08 号「下轮交付边界 #1」就是 N3，我方已做完但 hub 快照停在
   25ab91e；后续把 e6b7bab..f52ef5f 的架构路径改动按迁移规则导入新仓分支
2. **50 号 §七 Y 双源新发现之一作废**——grant-10000 不可达是材料截短产物；50 号
   修正+下轮回复须自纠（含 Y 实验方法论限定：材料完整性独立于翻译者人数）
3. **X-S2 定性收窄**——56 号 §〇 第 4 件的表述需口径随动（弱实现反例，非完整公式推翻）
4. **新反例 5 入批**——运行时配置验证提供方，并入生效批设计
5. **生效批范围扩宽**——N6（动作/参考时间绑定）从「P0-B 批后」提前入批；检查者版本
   /保守重验/适用性-保鲜为新增件
6. **工作流变化**——新讨论在 agent-hub-alpha issue #1；提交从新仓 main 开分支；
   61 号数字更正邢超已阅（08 号引用 199085266）
7. GitHub 是最终集成端——推送方式注意（github.com TLS 间歇干扰，gh api 可用的既有经验）

## 六、调查完备性声明

已核查：仓元信息/三提交全文件清单/导入根与排除根/全树差异枚举（M=11/D=151/A=10，见 §二 join 命令与输出）+逐件 blob 对账+manifest 交叉核对/
07+08+review 三件全文/design diff/.github 桥规则/GitCode+GitHub 双端 issue 状态/
本地复跑（21 包全绿）。已知未查项：GitHub 侧 PR 列表（桥未运转应同空）；java-consumer
构建（08 号声明本轮未重建）。
