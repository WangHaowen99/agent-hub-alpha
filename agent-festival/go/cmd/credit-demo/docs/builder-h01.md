# H01：Intent GATE 只读查询参考接线

这是默认关闭的受控 Demo 接线，原登录未接 IdP，未取得共享部署或生产身份资格。
本次以 Hub #30 的 `41a36755563ebeb442f2d93050c1d3430d510e80` 为适配基线；
Builder 侧对应已合入 finance 的只读端口与身份接线。原本地实现来自 H01/H02
参考提交，本次实际验证须另行绑定当前候选源码，历史报告不代替当前验收。

## 职责与数据流

浏览器先通过原 `/ui/login` 登录。原后端 `currentUser` 核验 `cd_session`，再通过
部署登记的当前用户映射取得 subject。后端只签发本次只读请求，携独立服务通道凭据，
调用原 Intent Host 的 `/api/v1/builder-hub/capabilities` 或 `/preflight`。
原 Host 验签和请求绑定，再交给原 CapabilityService 及其当前权限读取器。
两边在输出前重新检查身份/映射或权限；签名不是执行许可。

Hub 本地入口为 `/ui/builder`、GET `/api/builder-hub/capabilities`、POST
`/api/builder-hub/preflight`。页面复用原 shell。浏览器不提交 actor、scope、目标地址、
正文、操作或证明头；前置请求的原 DTO 由后端固定生成，操作只为 `build`，profile
只为 `intent / 1.0 / gate`。其它配置不继承本次资格。

## 显式配置

所有配置只在启动进程环境注入，不放入 `Service.State`、debug、MCP、页面或提交。
以下值均由操作者自行登记；不要把真实凭据写到命令历史或文档。

| 环境变量 | 含义 |
|---|---|
| `BUILDER_HUB_ENABLED` | 未设置时不注册入口；仅 `1` 表示启用 |
| `CREDIT_DEMO_PASSWORD` | 必须显式、非空且不同于原默认 `demo` |
| `BUILDER_HUB_URL` | 原 Host 固定 origin；无路径、查询、用户名或片段 |
| `BUILDER_HUB_CHANNEL_TOKEN` | 原受信通道 Bearer，独立于下述签名密钥 |
| `BUILDER_HUB_KEY_HEX` | 独立的 32–128 字节随机密钥的十六进制表示 |
| `BUILDER_HUB_ISSUER` | 与原 Host 登记一致的签发端 |
| `BUILDER_HUB_KEY_ID` | 与原 Host 登记一致的密钥 ID |
| `BUILDER_HUB_AUDIENCE` | 与原 Host 登记一致的目标 |
| `BUILDER_HUB_SCOPE` | 与原 Host 登记一致的查询范围，例如 `intent-builder` |
| `BUILDER_HUB_USERS_JSON` | 有限原账户映射，例如 `{"alice":"alice"}`；没有匿名回退 |
| `CREDIT_ORIGINS` | 浏览器 POST 必须匹配的完整 origin 列表 |
| `CREDIT_ADDR` | 本机验证仅监听回环地址 |
| `CREDIT_PUBLIC_URL` | 原反向代理前缀；直接回环验证可设置 `/` |

配置错误拒绝启动，只输出固定配置错误。密钥还必须与 Hub cookie、原监督批准密钥分开。
HTTP 只允许 IP 字面量回环目标；跨主机要求现有经过证书验证的 HTTPS，不关闭 TLS 检查。
不改变原 Intent Host 回环绑定策略；跨主机部署需已有合规代理，不能以本步放宽绑定。

## 证明与响应边界

`X-Builder-Hub-Context` 使用 `builder-hub-context/v1` 的固定字段，ASCII 紧凑排序 JSON，
未填充 base64url 载荷加 `.` 加小写十六进制 HMAC-SHA256。
签名字节为 `builder-hub-query-context/v1\n` 的实际换行前缀加原 JSON 字节。
签发有效期 30 秒，接收上限 60 秒；请求方法、路径、原始正文 SHA256 都绑定。
证明头最大 4 KiB，正文和上游响应最大 64 KiB。GET 摘要使用空正文；POST 后端发送固定 DTO。
公开样例 [query-context-v1.json](../testdata/query-context-v1.json) 的 key 仅为合成测试材料，
不允许用于部署。Go/Python 应逐字节消费同一文件。

客户端不自动重试、不追随重定向、不转发浏览器 Cookie。只返回严格解析的有限响应；
重复键、未知字段、其它 profile、不一致前置状态及执行许可 `true` 均拒绝。
上游异常只映射为固定 502；不回显上游异常、证明、密钥或私有路径。
原 Host 返回 `blocked` / `indeterminate` 时如实保留，查询成功不表示构建前置 ready。
能力声明中的实现/接线/资格沿用原 Host，不由此页面升级。

## 复跑与边界

1. 固定两侧源码、配置摘要与隔离依赖；Go 工具链按原 sources.lock.json 的摘要验证。
2. 执行本目录 Go 测试和 Builder 的校验器/原 Host 回归，核对公共样例摘要。
3. 用 Builder `hub-integration/engineering/verify_hub_readonly.py` 的显式参数配方启动
   两个真实原宿主进程，通过原登录、两条查询、错 Origin、未登录及撤权路径验证。
4. 启动与登录后建立持久状态计数基线；查询后的 Task、Candidate、Run、批准、预算增量均应为零。
   若工具或材料不足，将相应步骤记为 BLOCKED/NOT_RUN，不以 mock 成功替代实跑。
5. 分别核对 Builder 与 Hub 差异。Hub 只允许本 H01 文件集合，`.codegraph` 不提交。

关闭 `BUILDER_HUB_ENABLED` 并重启可撤销新入口；原流程无需数据库迁移。保留失败及撤权证据。
本说明不代表实跑已完成；最终结果以双方具体提交和封存报告为准。本 PR 通过自有 fork 向 Hub 提交 Draft，依赖尚未合入的 #30；
共享部署、业务构建/闭环和生产资格仍未验证，不修改协作者分支或自动合入主线。
