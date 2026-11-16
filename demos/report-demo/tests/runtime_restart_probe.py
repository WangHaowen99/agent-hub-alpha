"""Real Gateway/Runtime restart with counted native A2A traffic and no resend.

All model experts, Gateway, RDC, IdP and Go/PG remain real. A loopback relay
only counts method/identity and forwards unchanged bytes. Stops only the
workspace's recorded compiler and selected service. Never restarts PG.
"""
from __future__ import annotations
import argparse
import copy
import getpass
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import json
import os
from pathlib import Path
import subprocess
import sys
import threading
import time
import uuid
from urllib.parse import urlsplit
import requests


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--private-root',type=Path,required=True);p.add_argument('--source-task',required=True)
    p.add_argument('--service',choices=['business-analyst','gateway'],default='business-analyst');p.add_argument('--base-url',default='https://api.deepseek.com');p.add_argument('--model',default='deepseek-flash');a=p.parse_args()
    root=a.private_root.resolve();config_path=root/'config.json';original=config_path.read_bytes();cfg=json.loads(original);base=cfg['public_url'];upstream=cfg['bus']['gateway_url'].rstrip('/')
    if urlsplit(upstream).hostname not in ('127.0.0.1','localhost'):raise SystemExit('Existing loopback Gateway required')
    env=os.environ.copy()
    if a.service!='gateway':
        key=env.get('DEMO_MODEL_API_KEY') or (getpass.getpass('Model API key (hidden): ') if sys.stdin.isatty() else sys.stdin.readline().rstrip('\r\n'))
        if not key:raise SystemExit('Model credential required only to restart the selected real Runtime')
        env.update(DEMO_MODEL_API_KEY=key,DEMO_MODEL_BASE_URL=a.base_url,DEMO_MODEL_NAME=a.model)
    client=requests.Session();login=client.post(base+'/auth/login',json={'user':cfg['user'],'password':cfg['user_password']},timeout=10);login.raise_for_status()
    headers={'X-CSRF-Token':login.json()['csrf'],'Origin':base}
    def api(method,path,body=None):
        r=client.request(method,base+path,json=body,headers=headers,timeout=15);r.raise_for_status();return r.json()
    pending=requests.get(base+'/v1/executions/pending',headers={'Authorization':'Bearer '+cfg['tokens']['worker']},timeout=10);pending.raise_for_status()
    if pending.json():raise SystemExit('Settle/pause existing active tasks before a real process restart')
    source=api('GET','/v1/tasks/'+a.source_task);evidence=root/'evidence';evidence.mkdir(exist_ok=True)
    counts={'sends':[],'gets':[]};lock=threading.Lock()
    class Relay(BaseHTTPRequestHandler):
        def log_message(self,*_):pass
        def do_POST(self):
            if self.path!='/a2a':self.send_error(404);return
            raw=self.rfile.read(int(self.headers.get('Content-Length','0')));rpc=json.loads(raw);method=rpc.get('method');item={'at':time.time()}
            if method=='SendMessage':
                message=rpc.get('params',{}).get('message',{});item.update(message_id=message.get('messageId'),context_id=message.get('contextId'))
                for part in message.get('parts',[]):
                    try:execution=part.get('data') or json.loads(part.get('text','null'))
                    except ValueError:continue
                    if isinstance(execution,dict):item.update({k:execution.get(k) for k in ('task_id','node_id','call_id','input_digest','attempt')})
            elif method=='GetTask':item['remote_task_id']=rpc.get('params',{}).get('id')
            with lock:
                if method=='SendMessage':counts['sends'].append(item)
                elif method=='GetTask':counts['gets'].append(item)
            try:
                response=requests.post(upstream+'/a2a',data=raw,headers={k:self.headers[k] for k in ('Authorization','X-User-ID','Content-Type') if k in self.headers},timeout=35)
                with lock:
                    item['http_status']=response.status_code
                    try:
                        value=response.json()
                        if value.get('error'):item['error']=value['error']
                    except ValueError:item['non_json']=True
                body=response.content;status=response.status_code
            except requests.RequestException as error:
                with lock:item['transport_error_type']=type(error).__name__
                body=b'{"error":{"code":"REAL_GATEWAY_UNAVAILABLE"}}';status=503
            try:
                self.send_response(status);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body)
            except (ConnectionResetError,BrokenPipeError):
                with lock:item['client_disconnected_after_forward']=True
    relay=ThreadingHTTPServer(('127.0.0.1',0),Relay);thread=threading.Thread(target=relay.serve_forever,daemon=True);thread.start()
    manager=Path(__file__).resolve().parents[1]/'deploy/services.py'
    def service(action,name):subprocess.run([sys.executable,str(manager),action,name,'--private-root',str(root)],env=env,check=True)
    def pid():return json.loads((root/'services.json').read_text(encoding='utf-8')).get(a.service,{}).get('pid')
    task_id=None;record={'restarted_service':a.service,'phases':[]};stopped=False
    def save(phase):
        record['phases'].append({'phase':phase,'at':time.time()})
        with lock:record['traffic']=copy.deepcopy(counts)
        if task_id:(evidence/(task_id+'-'+a.service+'-restart.json')).write_text(json.dumps(record,ensure_ascii=False,indent=2),encoding='utf-8')
        print(json.dumps({'task_id':task_id,'phase':phase}),flush=True)
    try:
        service('stop','compiler');temp=copy.deepcopy(cfg);temp['bus']['gateway_url']='http://127.0.0.1:'+str(relay.server_port);config_path.write_text(json.dumps(temp,indent=2),encoding='utf-8');service('start','compiler')
        graph=copy.deepcopy(source['graph']);graph['goal']='[REAL '+a.service+' PROCESS RESTART TEST; no expert output injection] '+graph['goal']
        created=api('POST','/v1/commands',{'command_id':'real-restart-'+uuid.uuid4().hex,'graph':graph});task_id=created['task_id'];path='/v1/tasks/'+task_id
        api('POST',path+'/admit',{'graph_digest':created['graph_digest']});save('admitted')
        deadline=time.monotonic()+100;call=None
        while time.monotonic()<deadline:
            calls=api('GET',path+'/calls');call=next((c for c in calls if c['node_id']=='business1' and c.get('remote_task_id') and c['state'] in ('ACCEPTED','WORKING')),None)
            if call:break
            time.sleep(.5)
        if call is None:raise RuntimeError('No actual in-flight business remote task; did not exercise restart')
        record.update(before_call=call,before_task=api('GET',path),before_pid=pid());api('POST',path+'/pause',{});service('stop','compiler');save('paused_worker_stopped_with_real_remote_task')
        service('stop',a.service);stopped=True;service('start',a.service);stopped=False;record['after_pid']=pid();save('real_service_restarted')
        api('POST',path+'/resume',{});service('start','compiler');save('worker_reconciling_original_task')
        deadline=time.monotonic()+100;seen_error=False
        while time.monotonic()<deadline:
            task=api('GET',path)
            with lock:gets=copy.deepcopy(counts['gets'])
            failures=[g for g in gets if g.get('remote_task_id')==call['remote_task_id'] and g['at']>=record['phases'][-2]['at'] and (g.get('error') or g.get('http_status',200)!=200)]
            if failures and task['nodes']['business1']['status'] in ('UNKNOWN','FAILED'):seen_error=True;break
            if task['nodes']['business1'].get('result'):break
            time.sleep(.6)
        after_calls=api('GET',path+'/calls');current=next(c for c in after_calls if c['call_id']==call['call_id'])
        with lock:sends=[copy.deepcopy(s) for s in counts['sends'] if s.get('task_id')==task_id and s.get('node_id')=='business1']
        business_result=task['nodes']['business1'].get('result')
        recovered=bool(business_result and business_result['input_digest']==call['input_digest'])
        record['observed_outcome']='RECOVERED_BY_QUERY' if recovered else 'UNKNOWN_RECORDED'
        checks={'real_process_pid_changed':record['before_pid']!=record['after_pid'],'same_admitted_graph':task['graph_digest']==created['graph_digest'],
            'same_remote_identity':all(current.get(k)==call.get(k) for k in ('call_id','message_id','context_id','remote_task_id','attempt','input_digest')),
            'business_send_requests_exactly_one':len(sends)==1,
            'observed_outcome_supported':seen_error or (a.service=='gateway' and recovered),
            'runtime_memory_loss_not_invented_success':not business_result if a.service!='gateway' else (not business_result or recovered),
            'unresolved_not_reported_complete':recovered or task['status'] not in ('DELIVERED','AWAITING_ACCEPTANCE')}
        record.update(after_task=task,after_calls=after_calls,checks=checks,all_passed=all(checks.values()));save('verified')
        if not record['all_passed']:raise RuntimeError('Restart outcome differed; inspect preserved actual task/traffic, no success assumed')
    finally:
        if task_id:
            try:api('POST','/v1/tasks/'+task_id+'/pause',{})
            except requests.RequestException:pass
        try:
            try:service('stop','compiler')
            finally:
                config_path.write_bytes(original)
                if stopped:service('start',a.service)
                service('start','compiler')
        finally:relay.shutdown();relay.server_close();thread.join(timeout=5)
    print(json.dumps({'task_id':task_id,'checks':record['checks'],'all_passed':record['all_passed'],'model_experts_real':True,'business_acceptance_asserted':False}))


if __name__=='__main__':main()
