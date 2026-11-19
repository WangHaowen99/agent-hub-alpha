# H02：已有产物与原 RDC 的只读交接

本入口消费操作者提供的真实 Builder 导出/接收材料，核对已有部署声明和原 RDC 发现结果。它不构建 wheel、不部署、不注册 Agent，也不派发业务请求。页面经原登录访问，响应前再次检查当前用户资格。

## 来源与配置

Builder 侧的格式与工具沿用已合入的 [H02 交接说明](https://gitcode.com/Euphoria_Yan/agent-store-builder-dev-wo-fork/blob/finance/community/finance/builder/docs/hub-integration/guides/hub-bus-handoff.md) 及 [publication-reference-v2 合同](https://gitcode.com/Euphoria_Yan/agent-store-builder-dev-wo-fork/blob/finance/community/finance/builder/docs/hub-integration/contracts/publication-reference-v2.md)。原 SDK、AgentCard、Bus 和公共协议不在本 PR 修改。

启用配置：

| 环境变量 | 用途 |
| --- | --- |
| `CREDIT_DEMO_PASSWORD` | 操作者另行设置的非默认演示密码 |
| `BUILDER_PUBLICATION_ENABLED=1` | 显式启用；未设置时无新增产物路由 |
| `BUILDER_PUBLICATION_CONFIG` | 本机只读 JSON 配置文件路径，不含 RDC token |
| `BUILDER_PUBLICATION_RDC_TOKEN` | 通过进程环境注入的 RDC 通道凭据，不写页面或持久状态 |

配置字段为 `registry_url`、`tenant`、`allowed_users`、`publications` 和可选的 `retained_publications`。RDC 只允许明确的回环 HTTP 地址；这是本机受控参考的限制，不是生产网络配置建议。主发布列表最多 8 项，保留列表最多 32 项，允许用户最多 64 项。

每份发布材料的 v2 字段区分：

- 稳定逻辑能力 `logical_id` 与原生 `native_agent_id`；
- 业务 `capability_version`、部署声明 `deployment_version` 与实际 `instance_id`；
- 完整 `artifact_sha256`，以及原 manifest、wheel、接收和闭包摘要；
- 原卡/契约、协议版本、来源 revision 与原交接投影；
- 治理状态和 `execution_authorized=false`。

完整摘要不截短为 Bus 业务版本。v1、未知字段、错绑或显式冲突被拒绝。保留旧发布材料只用于保持历史关联，不会迁移或恢复旧远端任务；本入口不更改已采纳任务的绑定。

`testdata/publication-reference-v2.json` 是公开合成解析夹具，**不是已发布产物或部署凭据**。真实配置须来自原 Builder 的实际构建、独立接收与发布交接；不能把夹具声明当作治理准入或专业质量通过。

## 读取与判定边界

`GET /api/builder/publications/{logical_id}` 仅做一次有限的原 RDC 查询，不接受正文、查询参数或浏览器提供的身份上下文头。只有配置且当前获准的原登录用户可读；不存在项返回 404。远端失败、缺实例或不匹配结果保持不可用/indeterminate，不注册、重派或切换目标。

`/ui/builder` 在 H01 启用且当前用户也具备产物读取资格时列出主发布列表。完整摘要和来源只表示本次声明/发现的关联；RDC 不认证远端安装字节。本 PR 不声称当前有真实产物部署、知识驱动图、专业业务验收、恢复能力或生产资格。

## 接收与后续

这是从本地 H01/H02 参考接线移植到上游 #30 候选的增量。真实双包部署/注册和旧 Task 续查须在当前共同环境另验；原 Bus 的兼容补丁尚未作为本 PR 提交或接收，不能暗示依赖已经解决。

知识选择与图消费将对齐 #31～#34 独立交付；H04/H05 不在本批挂载。撤销开关后重启即可移除入口，既有任务、批准和预算均保留。
