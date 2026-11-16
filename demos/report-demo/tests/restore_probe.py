"""Back up and verify an explicit, empty isolated PG restore target.

Never creates/drops databases, overwrites existing rows, or starts services.
The restore DSN is derived in memory from the private migrator connection;
model credentials are not read. Recheck the same snapshot with --verify-only.
"""
from __future__ import annotations
import argparse
import json
import os
from pathlib import Path
import subprocess
from urllib.parse import urlsplit, urlunsplit


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--private-root',type=Path,required=True)
    p.add_argument('--binary',type=Path,required=True)
    p.add_argument('--database',default='report_demo_restore_test')
    p.add_argument('--verify-only',action='store_true')
    a=p.parse_args();root=a.private_root.resolve();exe=str(a.binary.resolve())
    cfg=json.loads((root/'config.json').read_text(encoding='utf-8'))
    if not a.database.endswith('_restore_test') or not a.database.replace('_','').isalnum():
        raise SystemExit('An explicitly named *_restore_test database is required')
    uri=urlsplit(cfg['migrator_dsn'])
    if uri.scheme not in ('postgres','postgresql'):raise SystemExit('URI migrator DSN required')
    target=urlunsplit(uri._replace(path='/'+a.database))
    env=dict(os.environ,DEMO_RESTORE_DSN=target)
    evidence=root/'evidence';evidence.mkdir(exist_ok=True)
    snapshot=evidence/'restore-source.json';readback=evidence/'restore-readback.json'
    content=root/'restore-test'/'content';restorecfg=root/'restore-test'/'config.json'
    restorecfg.parent.mkdir(parents=True,exist_ok=True)
    def run(*args):
        result=subprocess.run([exe,*map(str,args)],env=env,capture_output=True,text=True,timeout=180)
        if result.returncode:
            # Never echo command/config/DSN or unreviewed database exception text.
            raise RuntimeError('Snapshot CLI failed; exit='+str(result.returncode)+'; reconcile the isolated target before retrying')
        return json.loads(result.stdout)
    if not a.verify_only:
        run('-config',root/'config.json','-backup',snapshot)
        offline=run('-config',root/'intentionally-missing.json','-replay',snapshot)
        restored=run('-config',root/'config.json','-restore',snapshot,'-restore-content-root',content)
    else:
        offline=run('-replay',snapshot)
        restored={'status':'VERIFY_EXISTING_ISOLATED_TEST_DB'}
    private=dict(cfg,dsn=target,migrator_dsn=target,artifact_root=str(content))
    restorecfg.write_text(json.dumps(private),encoding='utf-8')
    run('-config',restorecfg,'-backup',readback)
    restored_offline=run('-replay',readback)
    before=json.loads(snapshot.read_text(encoding='utf-8'))['payload']
    after=json.loads(readback.read_text(encoding='utf-8'))['payload']
    checks={
        'all_scoped_tables_equal':before['tables']==after['tables'],
        'immutable_contents_equal':before['contents']==after['contents'],
        'catalog_and_material_equal':before['catalog']==after['catalog'] and before['material']==after['material'],
        'task_projections_equal':offline['task_projections']==restored_offline['task_projections'],
        'offline_zero_external_calls':all(restored_offline[k]==0 for k in ('model_calls','runtime_calls','publication_calls','database_calls')),
        'services_not_started':restored.get('services_started') is False or a.verify_only,
    }
    result={'restore':restored,'checks':checks,'all_passed':all(checks.values()),
        'tables':len(after['tables']),'artifacts':len(after['contents']),
        'tasks':len(restored_offline['task_projections']),
        'source_backup_digest':offline['digest'],'business_acceptance_asserted':False}
    (evidence/'restore-verification.json').write_text(json.dumps(result,indent=2),encoding='utf-8')
    print(json.dumps(result))
    if not result['all_passed']:raise SystemExit(1)


if __name__=='__main__':main()
