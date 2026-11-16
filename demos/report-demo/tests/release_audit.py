"""Inventory actual source/build hashes and scan intended source for secrets.

This is a scoped release check, not a proof of OS isolation or a replacement
for the T/Contract-C/E acceptance matrix. Findings report paths/categories only.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
from urllib.parse import unquote,urlsplit


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--private-root',type=Path,required=True);a=p.parse_args()
    demo=Path(__file__).resolve().parents[1];repo=demo.parents[1];root=a.private_root.resolve()
    cfg=json.loads((root/'config.json').read_text(encoding='utf-8'))
    secrets=set(cfg.get('tokens',{}).values())
    if cfg.get('user_password'):secrets.add(cfg['user_password'])
    for name in ('dsn','migrator_dsn'):
        password=urlsplit(cfg[name]).password
        if password:secrets.add(unquote(password))
    # Short installation passwords can occur as user directory names; do not
    # claim this scan covers short, generic credentials or every log format.
    secrets={s for s in secrets if isinstance(s,str) and len(s)>=12}
    findings=[];files=[];source_files={}
    roots=[demo,repo/'agent-festival/schema',repo/'agent-festival/go/internal/ledger']
    for path in sorted(p for source in roots for p in source.rglob('*')):
        if not path.is_file() or {'target','__pycache__','.git'}.intersection(path.parts):continue
        if path.suffix not in ('.md','.go','.java','.json','.html','.py','.xml','.yaml','.yml','.txt','.sql'):continue
        text=path.read_text(encoding='utf-8-sig');files.append(path.relative_to(repo).as_posix())
        source_files[files[-1]]={'bytes':path.stat().st_size,'sha256':hashlib.sha256(path.read_bytes()).hexdigest()}
        if re.search(r'sk-[a-zA-Z0-9]{24,}',text):findings.append({'path':files[-1],'category':'model-key-shaped-literal'})
        if any(secret in text for secret in secrets):findings.append({'path':files[-1],'category':'known-private-credential'})
    source_paths={
        'base':repo,
        'core':repo/'.local/sources/agent-core-java',
        'solution':repo/'.local/sources/agent-solution',
        'runtime':repo/'.local/sources/agent-runtime-java',
        'workswarm':Path('D:/Code/openJiuwen/jiuwenswarm'),
    }
    sources={name:subprocess.check_output(['git','-C',str(path),'rev-parse','HEAD'],text=True).strip() for name,path in source_paths.items()}
    artifacts={}
    candidates=[root/'bin/report-control.exe',demo/'orchestrator-java/target/report-orchestrator.jar',demo/'experts-java/target/report-experts.jar',
        repo/'.local/sources/agent-solution/common/agent-bus/registry-discovery-center/target/agent-rdc-0.1.3.jar',
        repo/'.local/sources/agent-solution/common/agent-bus/agent-gateway/target/agent-gateway-0.1.3.jar']
    for path in candidates:
        if not path.is_file():raise SystemExit('Required built artifact missing: '+path.name)
        artifacts[path.relative_to(repo).as_posix()]={'bytes':path.stat().st_size,'sha256':hashlib.sha256(path.read_bytes()).hexdigest()}
    result={'sources':sources,'artifacts':artifacts,'source_files':source_files,'scanned_source_files':len(files),'secret_findings':findings,
        'scoped_scan_passed':not findings,'limits':['Only intended demo source; no OS sandbox claim','Model key shape and known long private Go credentials; not exhaustive secret detection','No independent business acceptance or full gate qualification inferred']}
    (root/'evidence/source-and-artifact-audit.json').write_text(json.dumps(result,indent=2),encoding='utf-8')
    print(json.dumps(result))
    if findings:raise SystemExit(1)


if __name__=='__main__':main()
