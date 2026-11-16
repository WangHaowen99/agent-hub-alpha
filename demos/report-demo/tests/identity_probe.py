"""Real IdP/RDC authorization positive and negative paths; no auth bypass."""
import argparse
import json
from pathlib import Path
import uuid
import requests

def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--private-root',type=Path,required=True);args=parser.parse_args()
    root=args.private_root.resolve();cfg=json.loads((root/'config.json').read_text(encoding='utf-8'));bus=cfg['bus']
    issued=requests.post(bus['token_url'],data={'grant_type':'client_credentials','client_id':bus['client_id'],'client_secret':bus['client_secret']},timeout=10);issued.raise_for_status()
    token=issued.json()['access_token']
    headers={'Authorization':'Bearer '+token,'X-Origin-Authorization':'Bearer '+token,'X-User-ID':bus['subject'],'X-Tenant-ID':'demo'}
    agents=['report-extractor','business-analyst','asset-quality-analyst','report-writer'];checks=[]
    def decision(name,changed,target,expected_http,allowed=None):
        r=requests.post(bus['rdc_url']+'/v2/authorization/invocations:check',headers={**headers,**changed},
                        json={'requestId':uuid.uuid4().hex,'callerAgentId':None,'targetAgentIds':target,'operation':'SendMessage'},timeout=12)
        body=r.json();passed=r.status_code==expected_http and (allowed is None or body.get('allAllowed') is allowed)
        checks.append({'check':name,'http_status':r.status_code,'all_allowed':body.get('allAllowed'),'error_code':body.get('code'),'passed':passed})
    decision('real_worker_four_grants',{},agents,200,True)
    decision('forged_user_subject',{'X-User-ID':'forged-'+uuid.uuid4().hex},agents,401)
    decision('invalid_origin_token',{'X-Origin-Authorization':'Bearer deliberately-invalid'},agents,401)
    decision('ungranted_target',{},['parent-only-analyst'],200,False)
    for name,claimed in [('runtime_valid_identity',bus['subject']),('runtime_forged_subject','forged-'+uuid.uuid4().hex)]:
        r=requests.post('http://127.0.0.1:8093/a2a',headers={'Authorization':'Bearer '+token,'X-User-ID':claimed},
                        json={'jsonrpc':'2.0','id':uuid.uuid4().hex,'method':'GetTask','params':{'id':'nonexistent-probe-'+uuid.uuid4().hex}},timeout=12)
        if name=='runtime_valid_identity':passed=r.status_code==200 and r.json().get('error',{}).get('code')==-32001
        else:passed=r.status_code==403
        checks.append({'check':name,'http_status':r.status_code,'passed':passed})
    result={'all_passed':all(row['passed'] for row in checks),'checks':checks,'real_identity':True,'scope':'RDC authorization decisions; no expert execution or business acceptance asserted'}
    (root/'evidence/identity-positive-negative.json').write_text(json.dumps(result,indent=2),encoding='utf-8');print(json.dumps(result))
    if not result['all_passed']:raise SystemExit(1)

if __name__=='__main__':main()
