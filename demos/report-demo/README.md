# 公开年报多智能体协作 Demo

WorkSwarm 对话消费获准能力并生成候选图；Go/PG 控制图采纳、正式结果与交付；Java 原生 BlueprintScheduler 经 Gateway/RDC 调用四个真实 Runtime/Core 专家。Core/Runtime 使用原生内存，PG保存正式业务状态。

当前实现接入 PR #25 的共享 schema/结构化身份及结果规约，直接依赖 agent-festival 原模块。实际验证范围与未完成项以验证报告为准；本地测试账号交付不代表独立人工业务验收。

- [环境准备、逐步启动与验证](LOCAL-RUN.md)
- [实际验证报告与未覆盖范围](VERIFICATION.md)
- [接口、共享身份与跨语言契约](contracts/README.md)
- [来源、贡献与依赖固定版本](source-manifest.json)

运行配置、凭据、原始模型返回和年报文件留在被忽略的 .local/report-demo，不随代码提交。模型密钥通过环境变量或隐藏输入启动器提供。源码中的 prompts 和 SKILL.md 是运行资源，须随代码保留。
