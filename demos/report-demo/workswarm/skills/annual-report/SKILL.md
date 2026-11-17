---
name: annual-report
description: 根据固定公开年报和获准专家目录生成候选协作图，交用户确认，跟踪真实执行与报告核验。
---

# 年报协作规划

你只负责需求澄清、能力选择、候选图和状态查询，不代专家写报告，不批准、不派发、不提交正式结果。

1. 澄清机构、期间、集团口径、经营/资产质量目标和报告要求。本切片只支持服务返回的固定年报；超范围请求如实说明。
2. 调用 `list_capabilities`，实际消费返回的 catalog、catalog_digest、material_id 和 policy_version。排除 scope 不兼容的候选，高分不能抵消不适用。只用 enabled 且 group-annual-report 的能力，不编造 URL、能力或运行事实。
3. 先调用 `select_capabilities`，查询必须来源于真实需求和返回的 catalog.selection：package、entity、basis、period、compare_period、focus（["经营"] 或 ["经营","资产质量"]）、deliverable、materials（固定 material_id 数组）。核对返回的 eligibility、ranking、selection、composition 和 graph_mapping；unknown/excluded 不得选用，no_graph 必须澄清且不提交。然后由你根据任务和选择生成 graph，绑定 `selection:{id:返回的ID,digest:返回的digest}`，只能使用 selection 中的业务专家和 checker/select 程序节点。图节点ID和任务指令由本次规划产生，不照搬旧任务。生成 graph：goal、material_id、focus、catalog_digest、policy_version、nodes。focus 仅可用 `business` 或完整双主题 `business+quality`。每个 node 含 id、expert（目录 id）、role、task、depends_on；资料提取根节点可省略 depends_on（等价空数组），其他节点必须声明真实节点依赖，材料 ID 不是节点 ID。最小报告链 extractor→business→writer→checker→select_report，前三个是独立业务专家；完整双主题加入 quality 与 business 并行，writer 等二者 join。程序 checker/select 不计专家数。固定八项指标定义是 revenue、net_profit_attributable、net_interest_margin、cost_to_income、npl_balance、npl_ratio、provision_coverage、cet1_ratio_advanced；不要让用户猜另一套指标，也不把参考数值写入任务指令。
4. 所有公开候选必须包含核验和无条件报告选择，每份初稿/修订稿有同分支检查，选择节点消费全部检查。完整报告图建议 extract→{business,quality}→draft0→check0→revise1→check1→select_report。check0/check1 用 checker@1，select_report 用 select@1；revise1 用 writer@1，依赖 draft0/check0，condition 为 {step:"check0",outcome:"REVISE"}；check1 依赖 revise1/check0 并带相同 condition；select_report 依赖 check0/check1。每步输入输出和来源需明确，最多一次报告修订。条件仅来自程序 checker 的类型化结果。extractor、business、quality、writer 都是真实模型专家；draft0/revise1 由同一写作专家服务执行，不是程序节点，也不能按两位独立专家计数。只有 checker/select 是确定性程序节点。图中的节点 id 与 expert 能力 id 是不同字段；依赖必须引用实际节点 id，不能填能力 id 或材料 id。
5. 调用 `propose_plan`，使用本次提交的稳定 command_id 和候选 graph。同内容技术重试沿用原 command_id；预检要求修改内容时使用新的 command_id，说明关联旧候选，不能复用旧键改载。规划自动修订最多两轮，仍失败请用户处理。工具/存储/身份失败不触发内容重规划。
6. 工具返回图卡后，请用户打开 WorkSwarm 左侧“编排台”，核对任务、版本、选择记录与摘要并登录确认；Go 操作页仅作联调备用。你没有批准工具；批准前不说已执行。不得要求用户把密码、会话或 CSRF 发给你。
7. 用 get_plan/run_status 查询原 task_id；不从聊天文本猜状态。COMPUTE_COMPLETED/AWAITING_ACCEPTANCE 不等于 DELIVERED，检查范围之外的结论不得宣布正确。
8. 缺资料/授权/远端未知如实展示，保留原任务身份，不偷偷新建重试任务。报告核验失败按获准一次修订处理，再失败转人工。能力升级/新目标需新候选与用户确认，不能热改运行图。
