"""Exercise the trusted Go user path with the isolated test account.

This operator test is separate from the WorkSwarm planner and does not assert
independent human business acceptance. No cookies/passwords/CSRF are exported.
"""
from __future__ import annotations
import argparse
import json
from pathlib import Path
import requests

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--private-root',type=Path,required=True)
    parser.add_argument('--task-id',required=True)
    parser.add_argument('--action',choices=['status','admit','pause','resume','deliver','download'],default='status')
    args=parser.parse_args()
    root=args.private_root.resolve();cfg=json.loads((root/'config.json').read_text(encoding='utf-8'))
    session=requests.Session();base=cfg['public_url'];task_url=base+'/v1/tasks/'+args.task_id
    login=session.post(base+'/auth/login',json={'user':cfg['user'],'password':cfg['user_password']},timeout=10);login.raise_for_status()
    headers={'X-CSRF-Token':login.json()['csrf'],'Origin':base}
    task=session.get(task_url,timeout=10);task.raise_for_status();task=task.json()
    if args.action in ('admit','pause','resume'):
        body={'graph_digest':task['graph_digest']} if args.action=='admit' else {}
        r=session.post(task_url+'/'+args.action,json=body,headers=headers,timeout=10)
        if not r.ok:raise RuntimeError(str(r.status_code)+' '+r.text[:600])
        print(json.dumps({'action':args.action,'receipt':r.json(),'actor':'isolated operator test; not planner'}))
    elif args.action in ('deliver','download'):
        checked={n['result']['output']['object_digest'] for n in task['nodes'].values() if n.get('result',{}).get('output',{}).get('outcome')=='PASS'}
        reports=[n['result'] for n in task['nodes'].values() if n.get('result',{}).get('valid') and n['result']['output'].get('indicators') and n['result']['artifact_digest'] in checked]
        if not reports:raise SystemExit('No current PASS-checked formal report; cannot approve/deliver')
        report=reports[-1];url=task_url+'/artifacts/'+report['artifact_id']
        if args.action=='download':
            download=session.get(url+'/download',timeout=10);download.raise_for_status()
            (root/'evidence'/(args.task_id+'-report.json')).write_bytes(download.content)
            print(json.dumps({'task_id':args.task_id,'downloaded':True,'approval_or_delivery_performed':False}))
            return
        approval=session.post(url+'/approve',headers=headers,json={},timeout=10);approval.raise_for_status()
        body={'delivery_id':'operator-test-'+args.task_id+'-'+report['artifact_id']}
        delivery=session.post(url+'/deliver',headers=headers,json=body,timeout=10);delivery.raise_for_status()
        duplicate=session.post(url+'/deliver',headers=headers,json=body,timeout=10);duplicate.raise_for_status()
        assert delivery.json()['receipt_id']==duplicate.json()['receipt_id'],'delivery retry changed receipt'
        download=session.get(url+'/download',timeout=10);download.raise_for_status()
        (root/'evidence'/(args.task_id+'-report.json')).write_bytes(download.content)
        receipt={'approval':approval.json(),'delivery':delivery.json(),'idempotent_delivery':True,'actor':'isolated operator test; not independent human business acceptance'}
        (root/'evidence'/(args.task_id+'-delivery.json')).write_text(json.dumps(receipt,indent=2),encoding='utf-8')
        print(json.dumps(receipt))
    current=session.get(task_url,timeout=10);current.raise_for_status();current=current.json()
    (root/'evidence'/(args.task_id+'-task.json')).write_text(json.dumps(current,ensure_ascii=False,indent=2),encoding='utf-8')
    print(json.dumps({'task_id':args.task_id,'status':current['status'],'nodes':{k:{'status':v['status'],'attempt':v['attempt'],'reason':v.get('reason'),'formal':bool(v.get('result')),'check':v.get('result',{}).get('output',{}).get('outcome')} for k,v in current['nodes'].items()}}))

if __name__=='__main__':main()
