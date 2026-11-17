"""Launch the real expert JAR directly, bypassing services.py's fingerprint.

No external model call or real model credential. A local outlet counts requests.
The positive actual Core model-client path lives in ModelBindingTest; this probe
proves the executable rejects ENV drift/missing bindings before any outlet use.
"""
import argparse
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import threading


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--private-root',type=Path,required=True)
    p.add_argument('--java',type=Path,required=True)
    args=p.parse_args();repo=Path(__file__).resolve().parents[3];root=args.private_root.resolve()
    calls=[]
    class Outlet(BaseHTTPRequestHandler):
        def do_POST(self):
            calls.append(self.path);self.send_response(500);self.end_headers()
        def log_message(self,*_):pass
    server=ThreadingHTTPServer(('127.0.0.1',0),Outlet);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
    spec=importlib.util.spec_from_file_location('binding',repo/'demos/report-demo/deploy/bind_profile.py');module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
    previous={k:os.environ.get(k) for k in ('DEMO_MODEL_BASE_URL','DEMO_MODEL_NAME')}
    os.environ.update(DEMO_MODEL_BASE_URL=f'http://127.0.0.1:{server.server_port}/v1',DEMO_MODEL_NAME='model-a')
    expected=module.model_binding_digest()
    results=[]
    try:
        for mode in ('env-model-b-with-old-profile','missing-binding'):
            cfg=json.loads((root/'experts/report-extractor.json').read_text(encoding='utf-8'))
            original_profile=cfg['execution_profile']
            if mode=='missing-binding':cfg.pop('model_binding_digest',None)
            else:cfg['model_binding_digest']=expected
            path=root/'evidence'/('model-process-'+mode+'-config.json');path.write_text(json.dumps(cfg),encoding='utf-8')
            env=os.environ.copy();env.update(REPORT_EXPERT_CONFIG=str(path),REPORT_EXPERT_PORT='0',REPORT_AGENT_ID='model-outlet-fixture',REPORT_AGENT_PUBLIC_URL='http://127.0.0.1:1/a2a',DEMO_MODEL_API_KEY='test-fixture-only',DEMO_MODEL_NAME='model-b')
            result=subprocess.run([str(args.java),'-Xms32m','-Xmx256m','-jar',str(repo/'demos/report-demo/experts-java/target/report-experts.jar')],env=env,capture_output=True,text=True,encoding='utf-8',errors='replace',timeout=50)
            output=result.stdout+result.stderr
            (root/'evidence'/('model-process-'+mode+'.log')).write_text(output,encoding='utf-8')
            code='MODEL_BINDING_UNAVAILABLE' if mode=='missing-binding' else 'MODEL_BINDING_CHANGED'
            row={'mode':mode,'direct_jar_start':True,'execution_profile_unchanged':cfg['execution_profile']==original_profile,'exit_nonzero':result.returncode!=0,'expected_rejection_seen':code in output,'model_outlet_requests':len(calls)}
            results.append(row)
            assert row['exit_nonzero'] and row['expected_rejection_seen'] and not calls,row
    finally:
        server.shutdown();server.server_close();thread.join()
        for key,value in previous.items():
            if value is None:os.environ.pop(key,None)
            else:os.environ[key]=value
    report={'all_passed':True,'cases':results,'limits':['Direct executable startup rejection; positive real Core outlet and real remote experts are verified separately']}
    (root/'evidence/model-process-binding.json').write_text(json.dumps(report,indent=2),encoding='utf-8');print(json.dumps(report))


if __name__=='__main__':main()
