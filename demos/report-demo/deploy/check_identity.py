"""Print only non-secret identity diagnostics; never print access tokens or client secrets."""
from __future__ import annotations
import argparse
import json
from pathlib import Path
import requests

def main():
    p=argparse.ArgumentParser();p.add_argument('--private-root',type=Path,required=True);a=p.parse_args()
    identity=json.loads((a.private_root/'identity.json').read_text(encoding='utf-8'))
    target=identity['clients']['report-runtime-introspect']
    for name in ['report-worker','report-admin']:
        client=identity['clients'][name]
        issued=requests.post(identity['issuer']+'/protocol/openid-connect/token',data={'grant_type':'client_credentials','client_id':name,'client_secret':client['client_secret']},timeout=10)
        issued.raise_for_status()
        r=requests.post(identity['issuer']+'/protocol/openid-connect/token/introspect',data={'token':issued.json()['access_token'],'client_id':target['client_id'],'client_secret':target['client_secret']},timeout=10)
        r.raise_for_status();v=r.json()
        print(json.dumps({'client':name,'active':v.get('active'),'aud':v.get('aud'),'iss':v.get('iss'),'realm_roles':v.get('realm_access'),'token_fields':list(v.keys())}))

if __name__=='__main__':main()
