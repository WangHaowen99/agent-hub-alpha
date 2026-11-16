"""Check shared vectors through Python serialization and the real Go JCS owner.

Uses the main repository's frozen Python oracle as an independent test oracle;
it is not another production canonicalizer or a replacement for the Go library.
"""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--binary',type=Path,required=True);p.add_argument('--report',type=Path,required=True);a=p.parse_args()
    demo=Path(__file__).resolve().parents[1];repo=demo.parents[1]
    sys.path.insert(0,str(repo/'agent-festival/schema'))
    from rfc8785_min import canonicalize
    suite=json.loads((demo/'contracts/canonical-vectors.json').read_text(encoding='utf-8'))
    checks=[]
    for v in suite['vectors']:
        value=json.loads(v['input']);encoded=json.dumps(value,ensure_ascii=False,separators=(',',':'))
        independent=canonicalize(value)
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/'input.json';path.write_text(encoded,encoding='utf-8')
            result=subprocess.run([str(a.binary.resolve()),'-config','intentionally-missing-config.json','-canonical-json',str(path)],capture_output=True,text=True,encoding='utf-8',timeout=10)
            if result.returncode:raise RuntimeError('Go rejected vector '+v['id'])
            actual=json.loads(result.stdout)
        expected='sha256:'+hashlib.sha256(independent).hexdigest()
        passed=independent.decode()==v['canonical']==actual['canonical'] and expected==v['digest']==actual['digest'] and actual['database_calls']==0
        checks.append({'id':v['id'],'passed':passed})
    report={'profile':suite['profile'],'checks':checks,'all_passed':all(c['passed'] for c in checks),'oracle':'existing frozen Python reference; Go owns production JCS'}
    a.report.parent.mkdir(parents=True,exist_ok=True);a.report.write_text(json.dumps(report,indent=2),encoding='utf-8');print(json.dumps(report))
    if not report['all_passed']:raise SystemExit(1)


if __name__=='__main__':main()
