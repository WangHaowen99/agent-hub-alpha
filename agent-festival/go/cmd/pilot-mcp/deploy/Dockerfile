# WorkSwarm（JiuwenSwarm）验证用镜像：从 develop 源码构建，不改源码。
# 构建上下文 swarm-src：git archive 导出的 jiuwenswarm 源码。
ARG NODE_IMAGE=docker.1ms.run/library/node:22-slim
ARG PYTHON_IMAGE=python:3.11-slim

FROM ${NODE_IMAGE} AS web
ARG NPM_REGISTRY=https://registry.npmmirror.com
COPY --from=swarm-src . /src
WORKDIR /src/jiuwenswarm/channels/web/frontend
RUN npm ci --registry=${NPM_REGISTRY} --no-audit --no-fund --loglevel=error && npm run build

FROM ${PYTHON_IMAGE}
ARG PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple
ENV PIP_INDEX_URL=${PIP_INDEX_URL} PIP_DISABLE_PIP_VERSION_CHECK=1 PYTHONUNBUFFERED=1
RUN sed -i 's#deb.debian.org#mirrors.tuna.tsinghua.edu.cn#g' /etc/apt/sources.list.d/debian.sources \
 && apt-get update && apt-get install -y --no-install-recommends git ca-certificates \
 && rm -rf /var/lib/apt/lists/*
COPY --from=swarm-src . /opt/src/jiuwenswarm
COPY --from=web /src/jiuwenswarm/channels/web/frontend/dist /opt/src/jiuwenswarm/jiuwenswarm/channels/web/frontend/dist
RUN pip install --no-cache-dir /opt/src/jiuwenswarm && rm -rf /opt/src/jiuwenswarm
COPY entrypoint.sh /usr/local/bin/aha-entrypoint.sh
COPY configure.py /usr/local/bin/aha-configure.py
RUN chmod +x /usr/local/bin/aha-entrypoint.sh
ENV FRONTEND_HOST=0.0.0.0
ENTRYPOINT ["/usr/local/bin/aha-entrypoint.sh"]
