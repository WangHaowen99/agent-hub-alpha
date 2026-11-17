# Linux 原生启动与逐步验证

与 [Windows 运行文档](LOCAL-RUN.md) 共享服务、端口、契约、部署 Python 脚本和业务代码。使用原生 Linux JVM、Go、Python；无需通过 Wine、Windows exe 或一套新的控制服务运行。Windows 原会话兼容层只在 Windows 启用。

## 1. 工具和目录

使用 JDK21、Go1.25.0、Maven3.9.16、Node24.22.1、Python3.14 与 uv；WorkSwarm Python 依赖按 `uv.lock` 安装。确切已测发行版/补丁版本见验证报告；不把 WSL 验证推广成所有 Linux 发行版已验收。

在 Linux 文件系统建立独立 checkout，不共用 Windows 的 target、venv、node_modules、PID 文件、Keycloak 数据或 Maven 缓存。下面从本仓库根目录执行：

```bash
umask 077
export repo="$PWD"
export private="$repo/.local/report-demo"
export JAVA_HOME=/usr/lib/jvm/java-21-openjdk-amd64  # 按实际安装路径设置
export WORKSWARM_SOURCE="$repo/.local/sources/jiuwenswarm"
export REPORT_DEMO_CONFIG="$private/config.json"
export UV_PROJECT_ENVIRONMENT="$repo/.local/runtime/workswarm-venv"
export py="$UV_PROJECT_ENVIRONMENT/bin/python"
export KEYCLOAK_HOME="$repo/.local/tools/keycloak-26.8.2"
mkdir -p "$private" .local/sources .local/tools
java -version
go version
mvn -version
node --version
uv --version
```

Java 从 JAVA_HOME 或 PATH 查找；Go 从 GO_BINARY、仓内 `.local/tools/go/bin/go` 或 PATH 查找；WORKSWARM_SOURCE 必须指向固定源码。可选 SOLUTION_SOURCE 覆盖默认 `.local/sources/agent-solution`。不要继承 PATH 中来自 `/mnt/c`、`/mnt/d` 的 Windows Maven/npm。

按 [source-manifest.json](source-manifest.json) 取得 Core、Runtime、Solution、WorkSwarm 精确源码 SHA，分别放入 `.local/sources/{agent-core-java,agent-runtime-java,agent-solution,jiuwenswarm}`。如果从本地 bundle 或镜像取得，仍逐一检查 `git rev-parse HEAD`。同名 snapshot JAR 不能替代固定源码构建。

```bash
uv sync --project "$WORKSWARM_SOURCE" --frozen --no-dev --no-install-project
"$py" -c 'import psutil,requests,mcp; print("dependencies available")'
```

网络问题按实际环境配置代理。WSL NAT 下 Windows 的 `127.0.0.1` 代理不自动等于 Linux localhost；先验证连通性。代理配置仅进入所需进程，不修改锁文件或关闭 TLS 校验来绕过下载问题。

Keycloak 使用官方 [26.8.2 zip](https://github.com/keycloak/keycloak/releases/download/26.8.2/keycloak-26.8.2.zip)，SHA-256 为 `321ffee62ec28fab18fe885d8cd62cc814edce72a35cd062dc81b504c1a406b3`。解压到 KEYCLOAK_HOME，给 `bin/*.sh` 执行权限；Linux 启动器调用 kc.sh。发行包不是运行数据备份，不能覆盖已有实例的数据目录。

## 2. PostgreSQL 与私有配置

当前实际验证的 PostgreSQL 为 **16.16**，不声明其他主版本均已验证。RDC、W04 分库。单独一台机器可以继续连接已有云端 PG，也可以使用自己的 PG16 容器；应用不要求 Docker。

在全新 PG 上由数据库管理员创建：

| 数据库/账号 | 权限 |
| --- | --- |
| W04 库（如 agent_hub_demo）/ aha_control_migrator | 该库所有者，执行 `-migrate` 创建/升级表、索引、约束 |
| W04 / aha_control_app | CONNECT、public schema USAGE、所有应用表 SELECT/INSERT/UPDATE/DELETE、序列 USAGE/SELECT；无建表、建角色或集群管理权限 |
| RDC 库（如 agent_rdc）/ agent_rdc | 该库所有者，RDC 启动时执行自己的 Flyway；不使用 W04 应用账号 |

管理员用 psql 的 `\password <role>` 等隐藏交互方式设置实际密码，不把密码拼进公开 SQL。账号已存在时不重复创建、不自动重置密码。创建数据库后，在 W04 库设置：

```sql
GRANT CONNECT ON DATABASE agent_hub_demo TO aha_control_app;
GRANT USAGE ON SCHEMA public TO aha_control_app;
ALTER DEFAULT PRIVILEGES FOR ROLE aha_control_migrator IN SCHEMA public
  GRANT SELECT,INSERT,UPDATE,DELETE ON TABLES TO aha_control_app;
ALTER DEFAULT PRIVILEGES FOR ROLE aha_control_migrator IN SCHEMA public
  GRANT USAGE,SELECT ON SEQUENCES TO aha_control_app;
```

若迁移前未设置默认权限，对迁移完成后的已有表/序列补同样授权。数据库名自定义时对应替换；WSL 本机验证使用独立测试库，避免与 Windows 同时消费任务或覆盖 RDC 心跳。

首次准备：

```bash
read -rsp 'W04 application DSN: ' W04_APP_DSN; printf '\n'; export W04_APP_DSN
read -rsp 'W04 migrator DSN: ' W04_MIGRATOR_DSN; printf '\n'; export W04_MIGRATOR_DSN
"$py" demos/report-demo/deploy/prepare_local.py --private-root "$private"
unset W04_APP_DSN W04_MIGRATOR_DSN
```

此脚本准备固定材料和私有配置，不创建数据库/账号。已有配置保持不变；不要复制 Windows 中的盘符路径到 Linux。材料可以复用相同原件，但必须经已有脚本核对 SHA/页数和重建材料投影。

W04 的 config.json、角色配置、IdP 客户端 secret、服务 token 均只保存在私有目录，权限限制为当前用户；**模型密钥只进入进程环境**。不要提交 `.local`。

## 3. 构建

Maven 使用独立 Linux 仓库；如需要镜像，在私有 settings 中配置，别复制 Windows 的盘符或只读 file 仓库地址。

```bash
printf '<settings><localRepository>%s/.local/m2</localRepository></settings>\n' "$repo" > .local/maven-settings.xml
mvn -s .local/maven-settings.xml -f .local/sources/agent-core-java/pom.xml -pl core -am install -DskipTests -q
mvn -s .local/maven-settings.xml -f .local/sources/agent-solution/pom.xml -Pall -pl :agent-core-ext-orchestration,:agent-client-sdk-for-jvm,:agent-gateway,:registry-discovery-center -am install -DskipTests -q
mvn -s .local/maven-settings.xml -f .local/sources/agent-runtime-java/pom.xml -pl service/agent-service-app,service/agent-service-adapters/agent-service-adapters-agentcore -am install -DskipTests -q
mkdir -p "$private/bin"
go -C demos/report-demo/control-go build -o "$private/bin/report-control" .
mvn -s .local/maven-settings.xml -f demos/report-demo/orchestrator-java/pom.xml package -DskipTests -q
mvn -s .local/maven-settings.xml -f demos/report-demo/experts-java/pom.xml package -DskipTests -q
npm --prefix "$WORKSWARM_SOURCE/jiuwenswarm/channels/web/frontend" ci
npm --prefix "$WORKSWARM_SOURCE/jiuwenswarm/channels/web/frontend" run build
npm --prefix demos/report-demo/workswarm/workbench-plugin/app ci --no-audit --no-fund
npm --prefix demos/report-demo/workswarm/workbench-plugin/app run check
npm --prefix demos/report-demo/workswarm/workbench-plugin/app run build
```

串行执行，失败即停止。构建成功不等于测试通过；不要复制 Windows 二进制作为 Linux 结果。

## 4. 启动、绑定和注册

```bash
"$py" demos/report-demo/deploy/services.py start keycloak --private-root "$private"
"$py" demos/report-demo/deploy/configure_identity.py --private-root "$private"
export RDC_DB_URL='jdbc:postgresql://<PG_HOST>:5432/agent_rdc'
export RDC_DB_USER=agent_rdc
read -rsp 'RDC DB password: ' RDC_DB_PASSWORD; printf '\n'; export RDC_DB_PASSWORD
"$py" demos/report-demo/deploy/services.py configure --private-root "$private"
unset RDC_DB_PASSWORD
"$py" demos/report-demo/deploy/services.py start rdc gateway --private-root "$private"
"$private/bin/report-control" -migrate
```

实际模型名和地址从获准环境设置；密钥隐藏输入。专家默认 token 上限12000、普通迭代4、质量迭代8。可选环境变量 `DEMO_MODEL_MAX_TOKENS`、`DEMO_MODEL_NORMAL_ITERATIONS`、`DEMO_MODEL_QUALITY_ITERATIONS` 在绑定和所有专家启动进程中必须一致；范围见 contracts。推理模型预算先实测，不隐式改旧任务绑定。

```bash
export DEMO_MODEL_BASE_URL='<OpenAI-compatible base URL>'
export DEMO_MODEL_NAME='<available model>'
# WorkSwarm端点方言：默认deepseek；其他兼容服务按实际设置openai等受支持值。
# export DEMO_MODEL_ENDPOINT_PROFILE=openai
read -rsp 'Model API key: ' DEMO_MODEL_API_KEY; printf '\n'; export DEMO_MODEL_API_KEY
"$py" demos/report-demo/deploy/bind_profile.py --private-root "$private"
"$py" demos/report-demo/deploy/services.py start control compiler report-extractor business-analyst asset-quality-analyst report-writer --private-root "$private"
"$py" demos/report-demo/deploy/configure_bus.py --private-root "$private"
"$py" demos/report-demo/deploy/services.py start registry-heartbeat --private-root "$private"
export PYTHONPATH="$WORKSWARM_SOURCE"
"$py" demos/report-demo/workswarm/configure.py --config "$private/config.json" --workspace "$private/workswarm-home"
"$py" demos/report-demo/deploy/services.py start workswarm-agentserver workswarm-gateway workswarm-web --private-root "$private"
unset DEMO_MODEL_API_KEY
```

WorkSwarm的AgentServer、Gateway和Web都在进程环境中解析同一模型引用，配置文件仍保留`${API_KEY}`等引用，不写入真实模型密钥。隔离工作区的首次配置教程由安装器关闭；这不等于模型验证通过，仍须运行真实新对话。

首次 worker 默认关闭。先检查依赖和权限，再显式开启；重启 compiler 后才会消费已批准任务：

```bash
"$py" demos/report-demo/tests/validate_services.py --private-root "$private"
"$py" demos/report-demo/tests/identity_probe.py --private-root "$private"
"$py" demos/report-demo/deploy/services.py worker-enable --private-root "$private"
"$py" demos/report-demo/deploy/services.py stop compiler --private-root "$private"
"$py" demos/report-demo/deploy/services.py start compiler --private-root "$private"
```

所有服务默认仅绑定 Linux 127.0.0.1，端口与 Windows 文档相同。WSL 场景从 Windows 浏览器访问转发后的 5173，须先确认未命中仍在运行的 Windows 实例；用所选环境的 Go control_session_id、任务列表和真实服务 PID 核对。不要只修改一个端口而漏改 issuer、注册地址和回调。

## 5. 从可启动到可服务

手工从 `/chat/new` 选择年报协作规划助手，按 Windows 文档第13节完成新规划、编排台确认、四专家、检查、批准交付和下载。未批准前远端调用必须为0，正式下载摘要与当前报告绑定一致。

可用浏览器自动化复跑同一路径（会真实调用模型并批准/交付测试任务）：

```bash
# 如本机执行浏览器测试，先按 Playwright 官方要求安装浏览器及系统依赖。
"$py" demos/report-demo/tests/live_smoke.py --private-root "$private"
```

也可以由 Windows Playwright/Edge 测试客户端访问 WSL 服务；这种情况下服务仍须由 Linux 原生进程承担。测试客户端平台和服务平台分开记录。

Go/PG 集成回归需要真实 DSN：

```bash
export REPORT_DEMO_CONFIG="$private/config.json"
export REPORT_CANONICAL_BINARY="$private/bin/report-control"
export REPORT_FAULT_SOURCE_TASK='<本环境新任务T-ID>'
# GO 测试读取的 DSN 变量和执行方式见 LOCAL-RUN 第8节；不要使用无数据库导致的 skip 冒充通过。
mvn -s .local/maven-settings.xml -f demos/report-demo/orchestrator-java/pom.xml test
mvn -s .local/maven-settings.xml -f demos/report-demo/experts-java/pom.xml test
"$py" demos/report-demo/tests/model_process_probe.py --private-root "$private" --java "$JAVA_HOME/bin/java"
"$py" demos/report-demo/tests/control_restart_probe.py --private-root "$private" --task-id '<已交付T-ID>'
```

所有故障仅作用于明确的本环境测试实例。运行过期/无权限负例、RDC 不可用或人工恢复前先说明预期和场景；本机 WSL 通过不代替另一协作者独立接收或 W06/W07 业务签收。

## 6. 停止和恢复

```bash
"$py" demos/report-demo/deploy/services.py status --private-root "$private"
"$py" demos/report-demo/deploy/services.py stop --private-root "$private"
```

无服务名时只处理此私有目录记录且创建时间匹配的 PID；不批量 kill 系统 Java/Python。重新启动模型服务需重新提供进程环境，更新源码/模型/预算需重新绑定和新候选。不删除 PG 正式表或 artifacts 来“修复”失败。

W04 PG 和其 artifacts 内容必须配套备份；RDC 数据库、Keycloak data、WorkSwarm home/config/history 分别保存。重建 IdP 会改变主体/秘密，重建 RDC 会丢注册和权限，内存 Runtime 重启可能丢远端 task。已有 W04 备份/离线恢复入口见 Windows 文档第8.1节；整套环境灾难恢复不由单项 W04 恢复测试证明。
