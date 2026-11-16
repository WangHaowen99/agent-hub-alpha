"""Real worker-stop/pause/recovery test derived from an actual model-authored graph.

Does not regenerate the graph after failure. Only this workspace's recorded
compiler/worker process is stopped. Actual expert Runtimes continue in memory.
"""
from __future__ import annotations
import argparse
import json
from pathlib import Path
import subprocess
import sys
import time
import uuid
import requests

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--private-root',type=Path,required=True);p.add_argument('--source-task',required=True);a=p.parse_args()
    root=a.private_root.resolve();cfg=json.loads((root/'config.json').read_text(encoding='utf-8'));base=cfg['public_url'];session=requests.Session()
    login=session.post(base+'/auth/login',json={'user':cfg['user'],'password':cfg['user_password']},timeout=10);login.raise_for_status()
    headers={'X-CSRF-Token':login.json()['csrf'],'Origin':base}
    def api(method,path,body=None):
        r=session.request(method,base+path,json=body,headers=headers,timeout=15)
        if not r.ok:raise RuntimeError(str(r.status_code)+' '+r.text[:600])
        return r.json()
    source=api('GET','/v1/tasks/'+a.source_task)
    graph=source['graph'];graph['goal']='[REAL RECOVERY TEST; business content unchanged] '+graph['goal']
    created=api('POST','/v1/commands',{'command_id':'recovery-'+uuid.uuid4().hex,'graph':graph})
    task_id=created['task_id'];url='/v1/tasks/'+task_id
    record={'task_id':task_id,'source_task':a.source_task,'graph_digest':created['graph_digest'],'runtime_spec_digest':created['runtime_spec_digest'],'checks':{},'phases':[]}
    evidence=root/'evidence'/(task_id+'-recovery.json')
    def phase(name,**extra):
        record['phases'].append({'name':name,'at':time.time(),**extra});evidence.write_text(json.dumps(record,indent=2),encoding='utf-8');print(json.dumps(record['phases'][-1]),flush=True)
    api('POST',url+'/admit',{'graph_digest':created['graph_digest']});phase('admitted')
    deadline=time.monotonic()+100
    interrupted=[]
    while time.monotonic()<deadline:
        task=api('GET',url);calls=api('GET',url+'/calls')
        interrupted=[c for c in calls if c.get('remote_task_id') and c['state'] in ('ACCEPTED','WORKING')]
        if interrupted:break
        time.sleep(.6)
    if not interrupted:raise RuntimeError('No real in-flight remote task; failure injection not exercised')
    record['before_stop']={'task':task,'calls':calls};record['checks']['cross_task_extraction_reuse']=task['nodes']['extract1'].get('result',{}).get('reused_from',{}).get('task_id')==a.source_task
    api('POST',url+'/pause',{});phase('paused_with_in_flight_tasks',call_ids=[c['call_id'] for c in interrupted])
    time.sleep(2)
    paused=api('GET',url);paused_calls=api('GET',url+'/calls')
    record['checks']['pause_blocks_new_remote_admission']=set(c['call_id'] for c in paused_calls)==set(c['call_id'] for c in calls)
    manager=Path(__file__).resolve().parents[1]/'deploy/services.py'
    subprocess.run([sys.executable,str(manager),'stop','compiler','--private-root',str(root)],check=True)
    phase('worker_stopped')
    api('POST',url+'/resume',{})
    subprocess.run([sys.executable,str(manager),'start','compiler','--private-root',str(root)],check=True)
    phase('new_worker_started_waiting_for_permit')
    deadline=time.monotonic()+240
    while time.monotonic()<deadline:
        task=api('GET',url)
        if task['status'] in ('AWAITING_ACCEPTANCE','DELIVERED','WAITING_INPUT','NEEDS_REVIEW'):break
        time.sleep(1)
    after_calls=api('GET',url+'/calls');record['after_recovery']={'task':task,'calls':after_calls}
    original={c['call_id']:c for c in interrupted};current={c['call_id']:c for c in after_calls}
    record['checks']['same_admitted_graph']=task['graph_digest']==record['graph_digest'] and task['runtime_spec_digest']==record['runtime_spec_digest']
    record['checks']['same_interrupted_remote_identities']=all(c['call_id'] in current and all(current[c['call_id']].get(k)==c.get(k) for k in ('remote_task_id','message_id','context_id','attempt','input_digest')) for c in interrupted)
    record['checks']['no_duplicate_send_attempts']=len({(c['node_id'],c['attempt']) for c in after_calls})==len(after_calls) and all(sum(1 for c in after_calls if c['node_id']==old['node_id'])==1 for old in interrupted)
    record['checks']['formal_completion']=task['status']=='AWAITING_ACCEPTANCE'
    record['all_passed']=all(record['checks'].values());phase('recovery_checked',checks=record['checks'],all_passed=record['all_passed'])
    if not record['all_passed']:raise SystemExit(1)

if __name__=='__main__':main()
