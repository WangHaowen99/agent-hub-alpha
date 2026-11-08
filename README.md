# Agent Hub Alpha

[GitHub](https://github.com/chaosxingxc-orion/agent-hub-alpha) · [GitCode](https://gitcode.com/xingchao_ml/agent-hub-alpha) · [同步状态](https://github.com/chaosxingxc-orion/agent-hub-alpha/actions/workflows/sync-gitcode.yml)

面向企业核心业务的智能体架构研究与参考实现。承接 NGA 的 System 1 + System 2 框架、企业工作台、语义规约治理、决策与执行幂等、回放和审计设计；Jev 类模型的任务、数据、训练与评测继续在 [NGA 模型研究仓](https://github.com/chaosxingxc-orion/nga)开展。

**当前交付是架构提案和研究参考组件，不是已经通过生产验收的 SDK。** openJiuwen 仍是工程基座，Java SDK + openJiuwen 适配器 + Spring Boot Starter 是首个框架交付方向。`agent-festival/` 的 Go 实现用于组件契约、跨语言消费及宿主机制研究，不替代 Java 主基座。

## 从这里开始

1. [07 — 当前架构与最小 Java SDK 提案](achitecture%20design/07-nga-architecture-proposal.md)：设计、接口与控制边界。
2. [R1—R6 当前接收范围与开放项](achitecture%20design/08-r1-r6-acceptance-and-open-items.md)：已对齐、已验证、仍有异议与待实现的状态。
3. [V0 输入和架构索引](achitecture%20design/README.md)：保留专家原始设计，平滑演进。
4. [参考组件](agent-festival/README.md)与[独立复核入口](agent-festival/review/2026-11-08/README.md)：代码、实验及可复跑反例。
5. [迁移与协作说明](docs/migration/README.md)：来源、历史讨论、双仓边界及后续提交方法。

## 设计约束

- **一体三态四面**：智能体进化贯穿开发、运行、运营三态，以及用户、运行、管理、控制四面；这是协同和职责结构，不要求按维度新建服务。
- **管理面治理**：业务图、语义规约、授权、必需检查、无配置/未覆盖/故障处置经授权发布。生产任务绑定发布版本。
- **S1 + S2**：S1 在明确输入和候选范围内判断；S2 在节点契约内调查、补证、执行和评价。S2 可辅助开发态规划，不能自行重写运行中的生产业务图。
- **框架负责生效**：受信状态、当前有效性、必要检查和授权共同决定准入；框架维护身份、持久结果、等待恢复和审计。
- **幂等不等于正确**：一致但错误的判断仍须被检查、隔离或交由已发布路径处理。检查器也有作用域和错误边界，不宣称通用绝对正确。
- **复用与开放扩展**：优先使用 openJiuwen 与行业组件。原语、检查器、知识方法和存储可扩展，但不能绕过发布契约或改写历史。
- **业务系统边界**：通过业务 API 请求执行，内部额度、事务及风控归业务系统维护；等待、取消、重试和补偿遵循 API 实际能力。

## 当前工程状态

2026-11-08 接收原 NGA GitCode MR #4（包含 MR #2）的参考成果。确定性授信替身的“近期短期逾期”反例已修复，Go 原套件与 Python 夹具入口独立复核通过。

**仍未关闭**：R/K 统一入口、当前有效修订保护、基于生效事实的结果投影、拒绝持久审计，以及检查进入真实采纳路径。Jev 真实模型能力、Java/openJiuwen 集成和企业生产可用性分别验收。详细证据与争议以当前状态表为准，不从历史过程稿的完成勾选继承。

同一节点执行实例重试应复用原结果；新任务可消费已验证知识方法，但必须产生自己的身份和结果。零模型调用不等于零业务 API 调用，也不豁免授权和当前有效性检查。

## 仓库组织与协作

| 路径 | 内容 |
| --- | --- |
| `achitecture design/` | 当前主提案、验收状态和 V0 原文；保留原目录拼写以维持引用 |
| `design/` | NGA 整体研究计划及架构定位，模型部分链接 NGA |
| `agent-festival/` | 组件、schema、Go 研究宿主、Java consumer spike、复核材料 |
| `impl-2026-10-31/`、`impl-2026-11-01/`、`replies/` | 历史推演与回复来源，供追溯，不是独立当前方案 |
| `.github/` | GitHub/GitCode 安全同步与讨论桥接 |

两边团队均可在本平台提交分支、请求和评论；GitHub 是最终集成端，GitCode 通过事件触发与定期轮询协同。自动同步不批准、不合并、不强推，也不删除团队分支。GitCode 侧是周期性最终一致，不能承诺实时或五分钟 SLA。

模型实验、训练服务器及数据管线请在 NGA 提交；框架和工作台改动在本仓提交。历史 Issue/MR 保留原地址，新的架构讨论在本仓继续。
