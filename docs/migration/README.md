# NGA → Agent Hub Alpha，2026-11-08

项目负责人要求 NGA 聚焦 Jev 类模型实验，其余架构工作迁入本仓，并在 GitCode 建立同名协作仓。

## 迁移边界与来源

源集成提交：[fad91b05b70805a7f4b3910c6f355dc79c700d95](https://github.com/chaosxingxc-orion/nga/commit/fad91b05b70805a7f4b3910c6f355dc79c700d95)。它保留了原 MR #4 `25ab91e`、MR #2 `ee58bfb` 的提交祖先，并附维护者范围化接收及独立诊断。

迁移 `achitecture design/`、`design/`、`agent-festival/`、两个 `impl-*` 目录及 `replies/`，原路径不变。逐文件 Git blob 来源见 [清单](nga-source-manifest.json)。本仓采用精确内容快照导入，不伪造原作者历史；原提交和评审历史保留在 NGA。迁移后文档链接、当前入口和同步配置的调整另由本仓提交记录。

NGA 保留 `survey/`、`proposal/`、`experiments/`、`tools/`、`wiki/`、实验 `configs/`、`docker/`、`tests/` 及实验 onboarding：这些是 Jev/KEV/BEV 等模型族的研究和执行依赖，不是被遗忘的框架代码。两仓各自保留最小协作设施，不能让拆仓破坏模型实验或跨平台同步。

## 历史评审与当前状态

- [原 GitCode Issue #1](https://gitcode.com/xingchao_ml/nga/issues/1)：架构协商与 R1—R6 全部历史。
- [原 GitHub Issue #1](https://github.com/chaosxingxc-orion/nga/issues/1)：带来源归属的副本。
- [原 GitCode MR #4](https://gitcode.com/xingchao_ml/nga/merge_requests/4) / [GitHub PR #5](https://github.com/chaosxingxc-orion/nga/pull/5)：本轮基线及累计成果。
- [原 GitCode MR #2](https://gitcode.com/xingchao_ml/nga/merge_requests/2) / [GitHub PR #2](https://github.com/chaosxingxc-orion/nga/pull/2)：已包含于后续成果，不应再独立 squash 合入。
- [当前接收范围](../../achitecture%20design/08-r1-r6-acceptance-and-open-items.md)：所有历史完成声明应结合这里的明确范围阅读。

旧对象保留作为取证链，编号不在新仓冒充原编号。请从新仓 `main` 创建分支；不要将旧 NGA 的整条 main 合并到本仓，这会重新引入模型实验。确需迁移后续旧分支提交时，只导入架构路径的改动，并记录原提交来源。

同步为两组独立关系：GitHub NGA ↔ GitCode NGA；GitHub Agent Hub Alpha ↔ GitCode Agent Hub Alpha。不同项目间不互相镜像代码。
