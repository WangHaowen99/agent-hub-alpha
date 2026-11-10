# Go 桩服务：静态二进制 + adopt 用到的 schema。schema 按 ../../schema 的相对路径查找。
FROM scratch
COPY pilot-mcp /opt/aha/run/pilot/pilot-mcp
COPY schema /opt/aha/schema
WORKDIR /opt/aha/run/pilot
ENV PILOT_DATA=/data PILOT_ADDR=:8090
ENTRYPOINT ["/opt/aha/run/pilot/pilot-mcp"]
