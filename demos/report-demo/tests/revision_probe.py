"""Real expert revision flow with one explicitly recorded transport fault.

The loopback proxy forwards to the already authorized native Gateway, then
corrupts only the target task's first draft revenue unit. Original and injected
response bytes are retained. The real checker must detect it; the real writer
must revise it. No reference answers, mock expert output, or weakened checker.
Only this workspace's compiler/worker is restarted; its config is restored.
"""
from __future__ import annotations
import argparse
import copy
import hashlib
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import json
from pathlib import Path
import subprocess
import sys
import threading
import time
import uuid
from urllib.parse import urlsplit
import requests


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--private-root',type=Path,required=True);p.add_argument('--source-task',required=True);a=p.parse_args()
    root=a.private_root.resolve();config_path=root/'config.json';original_config=config_path.read_bytes();cfg=json.loads(original_config)
    base=cfg['public_url'];upstream=cfg['bus']['gateway_url'].rstrip('/')
    if urlsplit(upstream).hostname not in ('127.0.0.1','localhost'):raise SystemExit('Fault relay is restricted to the existing loopback Gateway')
    session=requests.Session();login=session.post(base+'/auth/login',json={'user':cfg['user'],'password':cfg['user_password']},timeout=10);login.raise_for_status()
    headers={'X-CSRF-Token':login.json()['csrf'],'Origin':base}
    def api(method,path,body=None):
        r=session.request(method,base+path,json=body,headers=headers,timeout=15)
        if not r.ok:raise RuntimeError('Control rejected '+path+' HTTP '+str(r.status_code))
        return r.json()
    pending=requests.get(base+'/v1/executions/pending',headers={'Authorization':'Bearer '+cfg['tokens']['worker']},timeout=10);pending.raise_for_status()
    if pending.json():raise SystemExit('Settle/pause existing active test tasks before this explicit worker fault run')
    source=api('GET','/v1/tasks/'+a.source_task)
    evidence=root/'evidence';evidence.mkdir(exist_ok=True)
    state={'task_id':None,'call_targets':{},'fault_calls':{},'forwarded_send_requests':0,'forwarded_get_requests':0};lock=threading.Lock()
    class Relay(BaseHTTPRequestHandler):
        def log_message(self,*_):pass
        def do_POST(self):
            if self.path!='/a2a':self.send_error(404);return
            raw=self.rfile.read(int(self.headers.get('Content-Length','0')))
            rpc=json.loads(raw)
            with lock:
                if rpc.get('method')=='SendMessage':
                    state['forwarded_send_requests']+=1
                    for part in rpc.get('params',{}).get('message',{}).get('parts',[]):
                        try:execution=part.get('data') or json.loads(part.get('text','null'))
                        except ValueError:continue
                        if isinstance(execution,dict) and execution.get('call_id'):
                            state['call_targets'][execution['call_id']]={'task_id':execution.get('task_id'),'node_id':execution.get('node_id')}
                elif rpc.get('method')=='GetTask':state['forwarded_get_requests']+=1
            outgoing={key:self.headers[key] for key in ('Authorization','X-User-ID','Content-Type') if key in self.headers}
            try:
                response=requests.post(upstream+'/a2a',data=raw,headers=outgoing,timeout=35)
                body=response.content
                if response.status_code==200:
                    value=json.loads(body);remote=value.get('result',{}).get('task',value.get('result',{}))
                    for artifact in remote.get('artifacts',[]):
                        for part in artifact.get('parts',[]):
                            envelope=part.get('data');was_text=False
                            if envelope is None and isinstance(part.get('text'),str):
                                try:envelope=json.loads(part['text']);was_text=True
                                except ValueError:continue
                            if not isinstance(envelope,dict):continue
                            target=state['call_targets'].get(envelope.get('call_id'),{})
                            if target.get('task_id')!=state['task_id'] or target.get('node_id')!='draft0':continue
                            if not isinstance(envelope.get('output'),dict):continue
                            indicators=envelope['output'].get('indicators',[])
                            revenue=next((x for x in indicators if x.get('id')=='revenue'),None)
                            if revenue is None:continue
                            call_id=envelope['call_id'];original_unit=revenue.get('unit')
                            if original_unit!='人民币百万元':raise RuntimeError('Original real model draft already has an unexpected revenue unit; do not mask its failure')
                            with lock:
                                if call_id not in state['fault_calls']:
                                    (evidence/(state['task_id']+'-original-draft-transport.json')).write_bytes(response.content)
                                    state['fault_calls'][call_id]={'original_unit':original_unit,'injected_unit':'人民币亿元','original_response_sha256':hashlib.sha256(response.content).hexdigest(),'responses_changed':0}
                                state['fault_calls'][call_id]['responses_changed']+=1
                            revenue['unit']='人民币亿元'
                            envelope.setdefault('runtime_evidence',{})['transport_fault_injection']={'fixture':'revision-probe/1','field':'output.indicators[revenue].unit','original_unit':original_unit,'injected_unit':'人民币亿元','not_model_authored':True}
                            if was_text:part['text']=json.dumps(envelope,ensure_ascii=False,separators=(',',':'))
                            else:part['data']=envelope
                            body=json.dumps(value,ensure_ascii=False,separators=(',',':')).encode('utf-8')
                            (evidence/(state['task_id']+'-injected-draft-transport.json')).write_bytes(body)
                self.send_response(response.status_code);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body)
            except Exception as error:
                with lock:state['relay_error_type']=type(error).__name__
                body=b'{"error":{"code":"ISOLATED_RELAY_FAILED"}}'
                self.send_response(503);self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body)
    relay=ThreadingHTTPServer(('127.0.0.1',0),Relay);thread=threading.Thread(target=relay.serve_forever,daemon=True);thread.start()
    manager=Path(__file__).resolve().parents[1]/'deploy/services.py'
    def service(action):subprocess.run([sys.executable,str(manager),action,'compiler','--private-root',str(root)],check=True)
    completed=False;task_id=None;record={}
    try:
        service('stop')
        temporary=copy.deepcopy(cfg);temporary['bus']['gateway_url']='http://127.0.0.1:'+str(relay.server_port)
        config_path.write_text(json.dumps(temporary,indent=2),encoding='utf-8');service('start')
        graph=copy.deepcopy(source['graph']);graph['goal']='[ISOLATED TRANSPORT FAULT / REAL MODEL REVISION; draft0 revenue unit corrupted by a loopback relay; not a claimed model error] '+graph['goal']
        created=api('POST','/v1/commands',{'command_id':'revision-fault-'+uuid.uuid4().hex,'graph':graph});task_id=created['task_id'];state['task_id']=task_id
        record={'task_id':task_id,'source_task':a.source_task,'graph_digest':created['graph_digest'],'runtime_spec_digest':created['runtime_spec_digest'],'fault':'explicit response corruption; original real model response retained; not a model-authored error','phases':[]}
        def save(phase):
            record['phases'].append({'phase':phase,'at':time.time()})
            with lock:record['relay']=copy.deepcopy(state)
            (evidence/(task_id+'-revision.json')).write_text(json.dumps(record,ensure_ascii=False,indent=2),encoding='utf-8');print(json.dumps({'task_id':task_id,'phase':phase}),flush=True)
        api('POST','/v1/tasks/'+task_id+'/admit',{'graph_digest':created['graph_digest']});save('admitted')
        deadline=time.monotonic()+420;last=None
        while time.monotonic()<deadline:
            task=api('GET','/v1/tasks/'+task_id)
            summary=(task['status'],tuple((k,v['status']) for k,v in sorted(task['nodes'].items())))
            if summary!=last:save('state:'+task['status']+':'+','.join(k+'='+v['status'] for k,v in task['nodes'].items()));last=summary
            if task['status'] in ('AWAITING_ACCEPTANCE','DELIVERED','WAITING_INPUT','WAITING_AUTH','NEEDS_REVIEW','FAILED'):break
            time.sleep(1)
        nodes=task['nodes'];out=lambda n:nodes.get(n,{}).get('result',{}).get('output',{})
        checks={'one_corrupted_draft_call':len(state['fault_calls'])==1,'program_detected_revise':out('check0').get('outcome')=='REVISE',
            'real_revision_committed':nodes.get('revise1',{}).get('status')=='COMMITTED','revision_recheck_pass':out('check1').get('outcome')=='PASS',
            'computation_completed':task['status']=='AWAITING_ACCEPTANCE','same_admitted_graph':task['graph_digest']==created['graph_digest'],
            'bounded_single_revision':nodes.get('revise1',{}).get('attempt')==1}
        record.update(task=task,checks=checks,all_passed=all(checks.values()));save('verified')
        if not record['all_passed']:raise RuntimeError('Real revision flow did not pass; preserved task/evidence, no business success claimed')
        completed=True
    finally:
        if task_id and not completed:
            try:api('POST','/v1/tasks/'+task_id+'/pause',{})
            except Exception as error:
                print(json.dumps({'task_id':task_id,'pause_cleanup_error':type(error).__name__}),flush=True)
        try:
            try:service('stop')
            finally:
                config_path.write_bytes(original_config)
                service('start')
        finally:
            relay.shutdown();relay.server_close();thread.join(timeout=5)
    print(json.dumps({'task_id':task_id,'all_passed':record['all_passed'],'checks':record['checks'],'business_acceptance_asserted':False}))


if __name__=='__main__':main()
