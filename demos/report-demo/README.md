# 公开年报多智能体协作 Demo

WorkSwarm 对话先消费目录和持久选择记录，再由真实规划模型生成候选图；W02 Application Plugin 作为编排、确认、报告及交付入口；Go/PG 控制图采纳、正式结果与交付；Java 原生 BlueprintScheduler 经 Gateway/RDC 调用四个真实 Runtime/Core 专家。Core/Runtime 使用原生内存，PG保存正式业务状态。

当前实现直接使用 agent-festival 原模块中 PR #25 的 schema/结构化身份；字段映射、schema资源打包和查重/对账策略属于 Demo 适配。实际验证范围与未完成项以验证报告为准；本地测试账号交付不代表独立人工业务验收。

- [环境准备、逐步启动与验证](LOCAL-RUN.md)
- [实际验证报告与未覆盖范围](VERIFICATION.md)
- [接口、共享身份与跨语言契约](contracts/README.md)
- [来源、贡献与依赖固定版本](source-manifest.json)

运行配置、凭据、原始模型返回和年报文件留在被忽略的 .local/report-demo，不随代码提交。模型密钥通过环境变量或隐藏输入启动器提供。源码中的 prompts 和 SKILL.md 是运行资源，须随代码保留。

本轮按 issue #16 的受控集成方向，窄适配 PR #30 的 W02 插件和 PR #33 的纯选择组件；年报任务只有本 Demo 一份 Go/PG 权威，不启动旧授信控制宿主。PR #34 跨任务远端快路径后置。本机已跑通新对话、四位真实专家、一次报告修订、同任务编排进程重启、插件批准/交付/原字节下载；脱敏接收样例见 [integration-sample.json](evidence/integration-sample.json)。这不代表外部协作者或业务负责人已签收。
