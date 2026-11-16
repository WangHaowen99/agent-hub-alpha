"""Native Bus protocol diagnostics. Intentional invalid binding => no LLM work.

Creates no W04 result; not a business expert success. Credentials stay private.
"""
import argparse
import json
from pathlib import Path
import uuid
import requests

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--private-root',type=Path,required=True);a=p.parse_args()
    cfg=json.loads((a.private_root/'config.json').read_text(encoding='utf-8'));bus=cfg['bus']
    issued=requests.post(bus['token_url'],data={'grant_type':'client_credentials','client_id':bus['client_id'],'client_secret':bus['client_secret']},timeout=10);issued.raise_for_status()
    headers={'Authorization':'Bearer '+issued.json()['access_token'],'X-User-ID':bus['subject']}
    requests_to_send=[('GetTask',{'id':'protocol-probe-'+uuid.uuid4().hex,'metadata':{'agentId':'report-extractor'}}),
        ('SendMessage',{'message':{'role':'ROLE_USER','messageId':'protocol-probe-'+uuid.uuid4().hex,'contextId':'protocol-probe-'+uuid.uuid4().hex,
          'parts':[{'kind':'text','text':json.dumps({'call_id':'intentionally-invalid-no-model-call','input_digest':'invalid','attempt':1})}]},
          'configuration':{'returnImmediately':True},'metadata':{'agentId':'report-extractor'}})]
    results=[]
    for method,params in requests_to_send:
        r=requests.post(bus['gateway_url']+'/a2a',headers=headers,json={'jsonrpc':'2.0','id':uuid.uuid4().hex,'method':method,'params':params},timeout=20)
        results.append({'method':method,'status':r.status_code,'body':r.json() if 'json' in r.headers.get('Content-Type','') else r.text[:500]})
    (a.private_root/'evidence/bus-protocol-probe.json').write_text(json.dumps(results,indent=2),encoding='utf-8')
    print(json.dumps(results))

if __name__=='__main__':main()
