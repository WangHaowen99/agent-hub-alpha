"""Read-only verification of original conversation recovery and process restart.

Reads only the explicitly named demo conversation; exports identifiers, checks
and hashes, never message content, credentials, cookies or full process args.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import psutil
import sys
import requests


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--private-root',type=Path,required=True)
    parser.add_argument('--session',required=True);parser.add_argument('--task-id',required=True)
    args=parser.parse_args();root=args.private_root.resolve();evidence=root/'evidence'
    trace=json.loads((evidence/(args.task_id+'-trace.json')).read_text(encoding='utf-8'))
    sys.path.insert(0,str(Path(os.environ.get('WORKSWARM_SOURCE','D:/Code/openJiuwen/jiuwenswarm'))))
    os.environ['JIUWENSWARM_DATA_DIR']=str(root/'workswarm-home')
    from jiuwenswarm.server.runtime.session.session_history import get_read_history_path
    path=get_read_history_path(args.session);raw=path.read_bytes()
    if len(raw)>16*1024*1024:raise SystemExit('Explicit recovery evidence file exceeds this verifier budget')
    rows=[json.loads(line) for line in raw.splitlines() if line.strip()];checks={};baselines=[]
    for filename in ('windows-history-compat.json','windows-history-compat-qualified.json'):
        original=json.loads((evidence/filename).read_text(encoding='utf-8'))
        item=next(r for r in original['sessions'] if r['session_id']==args.session)
        intact=len(raw)>=item['bytes'] and hashlib.sha256(raw[:item['bytes']]).hexdigest()==item['history_sha256']
        checks[filename+'_original_prefix_unchanged']=intact;baselines.append({'file':filename,'prefix_bytes':item['bytes'],'sha256':item['history_sha256']})
    probes=[('这是原会话重启恢复验证',{'mcp_report-orchestration_run_status','mcp_report-orchestration_verify_run'}),
            ('第二次实际进程重启后的同会话恢复检查',{'mcp_report-orchestration_run_status','mcp_report-orchestration_verify_run'}),
            ('最终启动器验收',{'mcp_report-orchestration_verify_run'})]
    runs=[]
    for prefix,expected in probes:
        requests_found=[r for r in rows if r.get('role')=='user' and isinstance(r.get('content'),str) and r['content'].startswith(prefix)]
        if not requests_found:raise SystemExit('Missing explicit recovery request: '+prefix)
        rid=requests_found[-1]['request_id'];events=[r for r in rows if r.get('request_id')==rid]
        calls=[r['tool_call'] for r in events if r.get('event_type')=='chat.tool_call'];names=[c['name'] for c in calls]
        checks[rid+'_only_requested_read_tools']=set(names)==expected and len(names)==len(expected)
        checks[rid+'_same_task_arguments']=all((json.loads(c['arguments']) if isinstance(c['arguments'],str) else c['arguments'])=={'task_id':args.task_id} for c in calls)
        checks[rid+'_completed_without_error']=any(r.get('event_type')=='chat.request_completed' and r.get('status')=='success' for r in events) and not any(r.get('event_type')=='chat.error' for r in events)
        usage=[r.get('usage',{}) for r in events if r.get('event_type')=='chat.usage_summary']
        checks[rid+'_real_model_usage']=any(u.get('input_tokens',0)>0 and u.get('output_tokens',0)>0 for u in usage)
        results=[r for r in events if r.get('event_type')=='chat.tool_result' and r.get('tool_name')=='mcp_report-orchestration_verify_run']
        verified=[]
        for r in results:
            value=r.get('raw_output',{}).get('result')
            if isinstance(value,str):value=json.loads(value)
            if isinstance(value,dict):verified.append(value)
        checks[rid+'_fresh_tool_verified_formal_state']=any(v.get('task_id')==args.task_id and v.get('status')==trace['task']['status'] and v.get('generation')==trace['task']['generation'] and v.get('evidence_intact') and v.get('issues')==[] for v in verified)
        runs.append({'request_id':rid,'tool_names':names,'completed':checks[rid+'_completed_without_error'],'usage':usage})
    checks['historical_failure_retained']=any(r.get('event_type')=='chat.error' and 'HISTORY_RESTORE_CHANGED' in str(r.get('error',''))+str(r.get('content','')) for r in rows)
    before=json.loads((evidence/'history-runtime-before-final-restart.json').read_text(encoding='utf-8'))
    state=json.loads((root/'services.json').read_text(encoding='utf-8'));record=state['workswarm-agentserver'];parent=psutil.Process(record['pid'])
    checks['new_manager_instance_alive']=parent.pid!=before['manager_pid'] and abs(parent.create_time()-record['created'])<.01
    old_alive=[]
    for r in before['processes']:
        try:
            if abs(psutil.Process(r['pid']).create_time()-r['created'])<.01:old_alive.append(r['pid'])
        except psutil.Error:pass
    checks['original_managed_process_tree_stopped']=not old_alive
    current=[]
    for process in [parent,*parent.children(recursive=True)]:
        try:
            command=' '.join(process.cmdline());role='worker' if 'jiuwenswarm.server.worker' in command else 'agentserver' if 'jiuwenswarm.server.app_agentserver' in command else None
            if role:current.append({'pid':process.pid,'created':process.create_time(),'role':role})
        except psutil.Error:pass
    marker=False
    for log in (root/'workswarm-home/run/worker-logs').glob('*.stderr.log'):
        with log.open(encoding='utf-8',errors='replace') as stream:line=stream.readline()
        for process in current:
            if process['role']=='worker' and ('pid='+str(process['pid'])+' ') in line and 'source=6cff70ffd9d054c147157a2f14bf00777835608c1bba173501b13450384d71e9' in line and 'active=True' in line:marker=True
    checks['current_worker_loaded_pinned_compat']=marker
    cfg=json.loads((root/'config.json').read_text(encoding='utf-8'));client=requests.Session();base=cfg['public_url']
    login=client.post(base+'/auth/login',json={'user':cfg['user'],'password':cfg['user_password']},timeout=10);login.raise_for_status()
    def get(path):
        response=client.get(base+path,timeout=10);response.raise_for_status();return response.json()
    task=get('/v1/tasks/'+args.task_id);calls=get('/v1/tasks/'+args.task_id+'/calls');verify=get('/v1/tasks/'+args.task_id+'/verify')
    checks['go_task_graph_generation_unchanged']=all(task[k]==trace['task'][k] for k in ('task_id','graph_revision','graph_digest','generation','status'))
    checks['remote_call_identities_unchanged']=sorted(calls,key=lambda c:c['call_id'])==sorted(trace['calls'],key=lambda c:c['call_id'])
    checks['formal_evidence_still_intact']=verify['evidence_intact'] and not verify['issues'] and verify['audit_events']==trace['verification']['audit_events']
    result={'session_id':args.session,'task_id':args.task_id,'checks':checks,'all_passed':all(checks.values()),'history_prefixes':baselines,'model_runs':runs,'before_processes':before,'current_processes':current,
            'limits':['Explicit original conversation and recorded Windows environment only','Not independent business acceptance or all IPC/stream-interruption recovery paths']}
    (evidence/'workswarm-history-recovery-verification.json').write_text(json.dumps(result,indent=2),encoding='utf-8');print(json.dumps(result))
    if not result['all_passed']:raise SystemExit(1)


if __name__=='__main__':main()
