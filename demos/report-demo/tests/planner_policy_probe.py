"""Test the actual native allowlist boundary, including late tool injection."""
import argparse
import asyncio
import importlib.util
import json
from pathlib import Path
from types import SimpleNamespace


async def verify():
    source=Path(__file__).resolve().parents[1]/'workswarm/report_planner_policy/rail.py'
    spec=importlib.util.spec_from_file_location('report_demo_policy_under_test',source);module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
    from openjiuwen.core.foundation.tool import McpServerConfig
    class Manager:
        def __init__(self):self.cards=[SimpleNamespace(name=n) for n in [*module.TOOLS,'read_file','bash','mcp_other-server_deliver']];self.executions=[]
        def list(self):return self.cards
        async def list_tool_info(self,*_,**__):return [c for c in self.cards if not isinstance(c,McpServerConfig)]
        async def execute(self,ctx,call,session,*_,**__):self.executions.append(call.name);return 'allowed'
    manager=Manager()
    approved=McpServerConfig(server_name='report-orchestration',server_path='http://127.0.0.1:8091/mcp')
    other=McpServerConfig(server_name='other-server',server_path='http://127.0.0.1:1/mcp')
    manager.cards.extend([approved,other])
    module.ReportPlannerPolicyRail().init(SimpleNamespace(ability_manager=manager))
    assert {c.name for c in manager.list() if not isinstance(c,McpServerConfig)}==module.TOOLS
    assert approved in manager.list() and other not in manager.list()
    assert {c.name for c in await manager.list_tool_info()}==module.TOOLS
    manager.cards.append(SimpleNamespace(name='late_registered_shell'))
    denied=[]
    for name in ('read_file','bash','mcp_other-server_deliver','late_registered_shell'):
        try:await manager.execute(None,SimpleNamespace(name=name),None)
        except ValueError:denied.append(name)
        else:raise AssertionError('Forbidden model ability executed: '+name)
    assert await manager.execute(None,SimpleNamespace(name='mcp_report-orchestration_list_capabilities'),None)=='allowed'
    assert manager.executions==['mcp_report-orchestration_list_capabilities']
    return {'all_passed':True,'approved_mcp_bootstrap_preserved':True,'unapproved_mcp_bootstrap_hidden':True,'allowed_tools':sorted(module.TOOLS),'denied_execution':denied,'late_tool_hidden_from_discovery':all(getattr(c,'name',None)!='late_registered_shell' for c in manager.list()),
            'native_boundary_used':True,'limits':'uses an ability manager fixture with the actual native boundary; live planner startup/tool execution is a separate gate'}


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--report',type=Path,required=True);a=p.parse_args();result=asyncio.run(verify());a.report.write_text(json.dumps(result,indent=2),encoding='utf-8');print(json.dumps(result))
