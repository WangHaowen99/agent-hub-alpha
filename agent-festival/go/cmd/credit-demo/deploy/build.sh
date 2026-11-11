#!/bin/bash
# 在构建机上构建授信编排演示的两个镜像：
#   credit-demo/credit:dev        Go 服务（agent-festival 内核 + 规约预检 + 账本 + 网关 + 页面）
#   credit-demo/orchestrator:dev  Java 执行器（agent-solution 编排模块 + openJiuwen ReActAgent）
# Java 依赖的 com.openjiuwen 构件取自共享 Maven 卷 kbs-m2（kb-suite 的 DA 构建已装好），只读取、不安装。
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
SVC="$(cd "$HERE/.." && pwd)"                   # go/cmd/credit-demo
FEST="$(cd "$SVC/../../.." && pwd)"             # agent-festival
OUT="$HERE/credit"
what="${1:-all}"

if [[ "$what" == all || "$what" == go ]]; then
  docker run --rm -v "$FEST":/src -v credit-gocache:/root/.cache -v credit-gomod:/go/pkg/mod -w /src/go/cmd/credit-demo \
    -e CGO_ENABLED=0 -e GOPROXY="${GOPROXY:-https://goproxy.cn,direct}" golang:1.25 \
    sh -c 'go vet ./... && go build -trimpath -ldflags="-s -w" -o deploy/credit/credit-demo .'
  mkdir -p "$OUT/schema"
  cp "$FEST"/schema/*.schema.json "$OUT/schema/"
  cp "$FEST"/schema/examples/judgment/valid/judgment-spec-credit.json "$OUT/"
  cat > "$OUT/Dockerfile" <<'DOCKER'
FROM alpine:latest
COPY credit-demo /opt/aha/run/credit/credit-demo
COPY judgment-spec-credit.json /opt/aha/run/credit/judgment-spec-credit.json
COPY schema /opt/aha/schema
WORKDIR /opt/aha/run/credit
ENV CREDIT_DATA=/data CREDIT_ADDR=:8091
ENTRYPOINT ["/opt/aha/run/credit/credit-demo"]
DOCKER
  docker build -q -t credit-demo/credit:dev "$OUT"
fi

if [[ "$what" == all || "$what" == java ]]; then
  J="$FEST/examples/credit-orchestrator"
  mkdir -p "$OUT/mvn" && cat > "$OUT/mvn/settings.xml" <<'XML'
<settings><mirrors><mirror><id>huaweicloud</id><mirrorOf>central</mirrorOf>
<url>https://repo.huaweicloud.com/repository/maven/</url></mirror></mirrors></settings>
XML
  docker run --rm -v "$J":/b alpine rm -rf /b/target
  docker run --rm -m 3g -e MAVEN_OPTS=-Xmx1536m -v "$J":/build -v "$OUT/mvn":/mvn -v kbs-m2:/root/.m2 -w /build \
    maven:3.9-eclipse-temurin-17 mvn -B -q -s /mvn/settings.xml -DskipTests package 2>&1 | tail -40 || true
  test -f "$J/target/credit-orchestrator.jar" || { echo "java build failed" >&2; exit 1; }
  mkdir -p "$OUT/java" && cp "$J/target/credit-orchestrator.jar" "$OUT/java/"
  cat > "$OUT/java/Dockerfile" <<'DOCKER'
FROM eclipse-temurin:17-jre
WORKDIR /app
COPY credit-orchestrator.jar /app/credit-orchestrator.jar
EXPOSE 8195
ENTRYPOINT ["java", "-XX:MaxRAMPercentage=70", "-jar", "/app/credit-orchestrator.jar"]
DOCKER
  docker build -q -t credit-demo/orchestrator:dev "$OUT/java"
fi
echo built
