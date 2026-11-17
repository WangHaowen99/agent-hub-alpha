"""Model ability boundary for the isolated report-demo WorkSwarm workspace."""
import logging

from openjiuwen.core.single_agent.rail import AgentRail
from openjiuwen.core.foundation.tool.mcp.base import mcp_model_tool_name
from openjiuwen.core.foundation.tool import McpServerConfig
from jiuwenswarm.runtime.tool_allowlist import install_tool_allowlist

TOOLS = frozenset({
    mcp_model_tool_name('report-orchestration',name)
    for name in ('list_capabilities','select_capabilities','propose_plan','get_plan','run_status','verify_run')
} | {'skill_tool','ask_user'})


class ReportPlannerPolicyRail(AgentRail):
    """Keep planner model abilities to six scoped MCP tools, skills and clarification."""
    priority: int = 1000

    def init(self,agent):
        manager=agent.ability_manager
        original_list=manager.list
        existing=getattr(manager,'jiuwenswarm_tool_allowlist',None)
        if existing is None:install_tool_allowlist(manager,TOOLS)
        elif existing!=TOOLS:raise RuntimeError('Unexpected preexisting planner ability policy')
        # Core's host-facing list also contains MCP bootstrap configurations,
        # which have server_name rather than name. Preserve only our connector
        # there; native list_tool_info and execute still enforce exact tool IDs.
        def host_abilities():
            return [item for item in original_list()
                    if (isinstance(item,McpServerConfig) and item.server_name=='report-orchestration')
                    or (not isinstance(item,McpServerConfig) and getattr(item,'name',None) in TOOLS)]
        manager.list=host_abilities
        # The upstream boundary filters discovery and rejects execution,
        # including tools registered by other rails after this init.
        manager.report_demo_policy_active=True
        logging.getLogger(__name__).info('[report-demo-policy] installed exact model ability boundary; allowed=%s',','.join(sorted(TOOLS)))
