"""验证部署用补丁：网关进程（agent_gateway.app_gateway）不导入 jiuwenswarm.common.schema.message，
mcp_app.* 不在其 ReqMethod 里，Web 端 MCP Apps 报 "unknown method"。这里在解释器启动时补上同样的三个成员。
上游修复后删除。"""
try:
    from agent_gateway.common.protocol_methods import _ensure_req_method

    _ensure_req_method("MCP_APP_LIST_TOOLS", "mcp_app.list_tools")
    _ensure_req_method("MCP_APP_READ_RESOURCE", "mcp_app.read_resource")
    _ensure_req_method("MCP_APP_CALL_TOOL", "mcp_app.call_tool")
except Exception:  # noqa: BLE001 - never block interpreter startup
    pass
