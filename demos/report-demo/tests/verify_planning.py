"""Read-only check of one explicitly named real WorkSwarm dialogue and W04 task.

Exports identities, tool names, usage and checks, never raw history or prompts.
Requires WorkSwarm on PYTHONPATH. This does not perform model calls or approval.
"""
import argparse
import json
import os
from pathlib import Path
import requests


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--private-root',type=Path,required=True);p.add_argument('--session',required=True);p.add_argument('--task-id',required=True);a=p.parse_args()
    root=a.private_root.resolve();os.environ['JIUWENSWARM_DATA_DIR']=str(root/'workswarm-home')
    from jiuwenswarm.server.runtime.session.session_history import get_read_history_path
    path=get_read_history_path(a.session)
    if path.stat().st_size>16*1024*1024:raise SystemExit('Explicit conversation exceeds verifier read budget')
    rows=[json.loads(l) for l in path.read_text(encoding='utf-8').splitlines() if l.strip()]
    cfg=json.loads((root/'config.json').read_text(encoding='utf-8'));header={'Authorization':'Bearer '+cfg['tokens']['operator']}
    r=requests.get(cfg['public_url']+'/v1/tasks/'+a.task_id,headers=header,timeout=15);r.raise_for_status();task=r.json()
    def arguments(row):
        value=row['tool_call']['arguments'];return json.loads(value) if isinstance(value,str) else value
    calls=[r for r in rows if r.get('event_type')=='chat.tool_call']
    proposals=[r for r in calls if r['tool_call']['name']=='mcp_report-orchestration_propose_plan' and arguments(r).get('graph',{}).get('selection')==task['graph']['selection']]
    if len(proposals)!=1:raise SystemExit('Expected one explicit proposal matching this selected task')
    request=proposals[0]['request_id'];events=[r for r in rows if r.get('request_id')==request]
    tools=[r for r in events if r.get('event_type')=='chat.tool_call'];names=[r['tool_call']['name'] for r in tools]
    required=['mcp_report-orchestration_list_capabilities','mcp_report-orchestration_select_capabilities','mcp_report-orchestration_propose_plan']
    allowed=set(required+['mcp_report-orchestration_get_plan','mcp_report-orchestration_run_status','mcp_report-orchestration_verify_run','skill_tool','ask_user'])
    usage=[r['usage'] for r in events if r.get('event_type')=='chat.usage_summary']
    def normalized(g):
        g=json.loads(json.dumps(g))
        for n in g['nodes']:
            if not n.get('depends_on'):n.pop('depends_on',None)
            if n.get('condition') is None:n.pop('condition',None)
        return g
    checks={'catalog_selection_proposal_order':all(n in names for n in required) and [names.index(n) for n in required]==sorted(names.index(n) for n in required),
            'model_abilities_stayed_scoped':set(names)<=allowed,
            'real_model_token_usage':any(u.get('input_tokens',0)>0 and u.get('output_tokens',0)>0 for u in usage),
            'request_completed':any(r.get('event_type')=='chat.request_completed' and r.get('status')=='success' for r in events),
            'no_request_error':not any(r.get('event_type')=='chat.error' for r in events),
            'model_authored_graph_matches_authority':normalized(arguments(proposals[0])['graph'])==normalized(task['graph'])}
    result={'session_id':a.session,'request_id':request,'task_id':a.task_id,'graph_digest':task['graph_digest'],'selection_ref':task['graph']['selection'],'tool_names':names,'usage':usage,'checks':checks,'all_passed':all(checks.values()),'limits':['One explicit real dialogue; not semantic-search recall or independent business acceptance']}
    (root/'evidence'/(a.task_id+'-planning.json')).write_text(json.dumps(result,indent=2),encoding='utf-8');print(json.dumps(result))
    if not result['all_passed']:raise SystemExit(1)


if __name__=='__main__':main()
