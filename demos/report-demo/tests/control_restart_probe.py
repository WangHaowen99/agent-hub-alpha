"""Restart only the recorded Go service and verify durable read/cursor identity."""
import argparse
import json
from pathlib import Path
import subprocess
import sys
import requests


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--private-root',type=Path,required=True);p.add_argument('--task-id',required=True);p.add_argument('--install-binary',type=Path);a=p.parse_args()
    root=a.private_root.resolve();cfg=json.loads((root/'config.json').read_text(encoding='utf-8'));base=cfg['public_url'];client=requests.Session()
    client.headers['Authorization']='Bearer '+cfg['tokens']['operator']
    def get(path):
        r=client.get(base+path,timeout=15);r.raise_for_status();return r.json()
    path='/v1/tasks/'+a.task_id
    before={'health':get('/healthz'),'task':get(path),'calls':get(path+'/calls'),'events':get(path+'/events')}
    if before['task']['status'] not in ('DELIVERED','AWAITING_ACCEPTANCE'):raise SystemExit('Use a settled test task for exact projection comparison')
    if len(before['events'])>=200:raise SystemExit('Use a settled test task with fewer than 200 events for this probe')
    before['events'].sort(key=lambda row:int(row['sequence']))
    cursor=int(before['events'][-1]['sequence']) if before['events'] else 0
    tail=get(path+'/events?after='+str(cursor))
    if tail:raise SystemExit('Task has more than one event page; choose a cursor by paging first')
    manager=Path(__file__).resolve().parents[1]/'deploy/services.py'
    subprocess.run([sys.executable,str(manager),'stop','control','--private-root',str(root)],check=True)
    if a.install_binary:
        import shutil
        shutil.copyfile(a.install_binary.resolve(),root/'bin/report-control.exe')
    subprocess.run([sys.executable,str(manager),'start','control','--private-root',str(root)],check=True)
    after={'health':get('/healthz'),'task':get(path),'calls':get(path+'/calls'),'events':get(path+'/events')}
    checks={
        'new_control_session':before['health']['control_session_id']!=after['health']['control_session_id'],
        'same_task_projection':before['task']==after['task'],
        'same_remote_call_identities':before['calls']==after['calls'],
        'same_persistent_events':before['events']==after['events'],
        'events_in_numeric_sequence_order':after['events']==sorted(after['events'],key=lambda row:int(row['sequence'])),
        'cursor_resumes_without_duplicates':get(path+'/events?after='+str(cursor))==[],
        'artifact_integrity':get(path+'/verify')['evidence_intact'],
    }
    result={'task_id':a.task_id,'checks':checks,'all_passed':all(checks.values()),'cursor':str(cursor),
        'before_session':before['health']['control_session_id'],'after_session':after['health']['control_session_id']}
    (root/'evidence'/'control-restart.json').write_text(json.dumps(result,indent=2),encoding='utf-8');print(json.dumps(result))
    if not result['all_passed']:raise SystemExit(1)


if __name__=='__main__':main()
