"""Read-only check of one explicitly named real WorkSwarm dialogue and W04 task.

Exports identities, tool names, usage and checks, never raw history or prompts.
Requires WorkSwarm on PYTHONPATH. This does not perform model calls or approval.
"""
import argparse
import json
import os
from pathlib import Path
import requests


def arguments(row):
    value=row['tool_call']['arguments'];return json.loads(value) if isinstance(value,str) else value


def normalized(graph):
    graph=json.loads(json.dumps(graph))
    for node in graph.get('nodes',[]):
        if not node.get('depends_on'):node.pop('depends_on',None)
        if node.get('condition') is None:node.pop('condition',None)
    return graph


def accepted_proposals(rows, task):
    """A matching selection alone cannot distinguish a rejected proposal from its correction."""
    result=[]
    for call in rows:
        if call.get('event_type')!='chat.tool_call' or call['tool_call']['name']!='mcp_report-orchestration_propose_plan':continue
        if normalized(arguments(call).get('graph',{}))!=normalized(task['graph']):continue
        call_id=call['tool_call'].get('tool_call_id')
        if not call_id:continue
        for row in rows:
            if row.get('event_type')!='chat.tool_result' or row.get('request_id')!=call.get('request_id') or row.get('tool_call_id')!=call_id:continue
            if row.get('is_error') or row.get('success') is False:continue
            raw=row.get('raw_output',{})
            value=raw.get('result') if isinstance(raw,dict) else None
            if value is None:value=row.get('result')
            try:value=json.loads(value) if isinstance(value,str) else value
            except ValueError:continue
            if isinstance(value,dict) and value.get('task',{}).get('task_id')==task['task_id']:
                result.append(call);break
    return result


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--private-root',type=Path,required=True);p.add_argument('--session',required=True);p.add_argument('--task-id',required=True);a=p.parse_args()
    root=a.private_root.resolve();os.environ['JIUWENSWARM_DATA_DIR']=str(root/'workswarm-home')
    from jiuwenswarm.server.runtime.session.session_history import get_read_history_path
    path=get_read_history_path(a.session)
    if path.stat().st_size>16*1024*1024:raise SystemExit('Explicit conversation exceeds verifier read budget')
    rows=[json.loads(l) for l in path.read_text(encoding='utf-8').splitlines() if l.strip()]
    cfg=json.loads((root/'config.json').read_text(encoding='utf-8'));header={'Authorization':'Bearer '+cfg['tokens']['operator']}
    r=requests.get(cfg['public_url']+'/v1/tasks/'+a.task_id,headers=header,timeout=15);r.raise_for_status();task=r.json()
    calls=[r for r in rows if r.get('event_type')=='chat.tool_call']
    proposals=accepted_proposals(rows,task)
    if len(proposals)!=1:raise SystemExit('Expected one exact graph proposal with its successful task-bound tool receipt')
    request=proposals[0]['request_id'];events=[r for r in rows if r.get('request_id')==request]
    tools=[r for r in events if r.get('event_type')=='chat.tool_call'];names=[r['tool_call']['name'] for r in tools]
    required=['mcp_report-orchestration_list_capabilities','mcp_report-orchestration_select_capabilities','mcp_report-orchestration_propose_plan']
    allowed=set(required+['mcp_report-orchestration_get_plan','mcp_report-orchestration_run_status','mcp_report-orchestration_verify_run','skill_tool','ask_user'])
    usage=[r['usage'] for r in events if r.get('event_type')=='chat.usage_summary']
    checks={'catalog_selection_proposal_order':all(n in names for n in required) and [names.index(n) for n in required]==sorted(names.index(n) for n in required),
            'model_abilities_stayed_scoped':set(names)<=allowed,
            'real_model_token_usage':any(u.get('input_tokens',0)>0 and u.get('output_tokens',0)>0 for u in usage),
            'request_completed':any(r.get('event_type')=='chat.request_completed' and r.get('status')=='success' for r in events),
            'no_request_error':not any(r.get('event_type')=='chat.error' for r in events),
            'model_authored_graph_matches_authority':normalized(arguments(proposals[0])['graph'])==normalized(task['graph'])}
    result={'session_id':a.session,'request_id':request,'task_id':a.task_id,'graph_digest':task['graph_digest'],'selection_ref':task['graph']['selection'],'proposal_attempts':names.count('mcp_report-orchestration_propose_plan'),'tool_names':names,'usage':usage,'checks':checks,'all_passed':all(checks.values()),'limits':['One explicit real dialogue and task-bound successful proposal; rejected graph attempts are retained, not counted as successful plans','Not semantic-search recall or independent business acceptance']}
    (root/'evidence'/(a.task_id+'-planning.json')).write_text(json.dumps(result,indent=2),encoding='utf-8');print(json.dumps(result))
    if not result['all_passed']:raise SystemExit(1)


if __name__=='__main__':main()
