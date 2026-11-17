"""Export task/call/observation evidence, without credentials, cookies or headers."""
import argparse
import json
from pathlib import Path
import requests

def envelopes(task):
    for artifact in task.get('artifacts',[]):
        for part in artifact.get('parts',[]):
            value=part.get('data')
            if value is None and isinstance(part.get('text'),str):
                try:value=json.loads(part['text'])
                except ValueError:continue
            if isinstance(value,dict) and 'runtime_evidence' in value:yield value

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--private-root',type=Path,required=True);p.add_argument('--task-id',required=True);a=p.parse_args()
    root=a.private_root.resolve();cfg=json.loads((root/'config.json').read_text(encoding='utf-8'));base=cfg['public_url'];session=requests.Session()
    login=session.post(base+'/auth/login',json={'user':cfg['user'],'password':cfg['user_password']},timeout=10);login.raise_for_status()
    task=session.get(base+'/v1/tasks/'+a.task_id,timeout=10);task.raise_for_status();task=task.json()
    calls=session.get(base+'/v1/tasks/'+a.task_id+'/calls',timeout=10);calls.raise_for_status();calls=calls.json()
    observations={};runtime=[]
    for call in calls:
        r=session.get(base+'/v1/calls/'+call['call_id']+'/observations',timeout=15);r.raise_for_status();observations[call['call_id']]=r.json()
        for observation in r.json():
            doc=observation['document']
            if doc['state']!='COMPLETED':continue
            for envelope in envelopes(doc['payload']):
                assert envelope['call_id']==call['call_id'] and envelope['input_digest']==call['input_digest'] and envelope['attempt']==call['attempt']
                runtime.append({'call_id':call['call_id'],'node_id':call['node_id'],'input_digest':call['input_digest'],
                                'remote_task_id':call.get('remote_task_id'),'runtime_evidence':envelope['runtime_evidence']})
    verify=session.get(base+'/v1/tasks/'+a.task_id+'/verify',timeout=15);verify.raise_for_status()
    result={'task':task,'calls':calls,'observations':observations,'runtime_evidence':runtime,'verification':verify.json()}
    (root/'evidence'/(a.task_id+'-trace.json')).write_text(json.dumps(result,ensure_ascii=False,indent=2),encoding='utf-8')
    print(json.dumps({'task_id':a.task_id,'runtime_evidence_records':len(runtime),'verification':verify.json()}))

if __name__=='__main__':main()
