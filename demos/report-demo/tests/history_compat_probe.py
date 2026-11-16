"""Real Windows metadata/replay checks; never rewrites an existing conversation."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
from types import SimpleNamespace


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--report',type=Path,required=True)
    parser.add_argument('--workspace',type=Path,required=True);parser.add_argument('--session',action='append',default=[])
    args=parser.parse_args();repo=Path(__file__).resolve().parents[3]
    bootstrap=repo/'demos/report-demo/workswarm/bootstrap'
    source=Path(os.environ.get('WORKSWARM_SOURCE','D:/Code/openJiuwen/jiuwenswarm'))
    sys.path[:0]=[str(bootstrap),str(source)]
    os.environ['JIUWENSWARM_DATA_DIR']=str(args.workspace.resolve())
    if sys.platform!='win32':raise SystemExit('This probe requires the actual Windows metadata boundary')
    import report_demo_windows_history as compat
    from jiuwenswarm.server.runtime.session import history_replay as replay
    from jiuwenswarm.server.runtime.session.session_history import get_read_history_path
    checks={};sessions=[]
    with tempfile.TemporaryDirectory(prefix='report-demo-history-') as directory:
        path=Path(directory)/'history.jsonl'
        raw=b'{"role":"user","request_id":"one","content":"first"}\n'
        path.write_bytes(raw)
        # The file must really have been written after its creation clock
        # tick; an immediate first write can make both timestamps identical.
        time.sleep(.03)
        path.write_bytes(raw)
        with path.open('rb') as stream:
            a=path.stat();b=os.fstat(stream.fileno())
        checks['reproduced_native_ctime_mismatch']=replay._identity(a)!=replay._identity(b) and a.st_birthtime_ns==b.st_birthtime_ns and (a.st_dev,a.st_ino,a.st_size,a.st_mtime_ns)==(b.st_dev,b.st_ino,b.st_size,b.st_mtime_ns)
        def read():return replay._read_prefix(path,0,replay._identity(path.stat()),0,replay.is_context_record,None,replay.HistoryReplayLimits())
        try:read()
        except replay.HistoryReplayError as exc:checks['unchanged_history_rejected_before_compat']=exc.code=='HISTORY_RESTORE_CHANGED'
        else:checks['unchanged_history_rejected_before_compat']=False
        result=compat.install();checks['compat_active']=result['active']
        native=SimpleNamespace(st_dev=1,st_ino=2,st_size=3,st_mtime_ns=4,st_ctime_ns=5,st_birthtime_ns=0)
        checks['future_consistent_native_identity_not_patched']=compat.identity_mode(native,native)=='native'
        unsupported=SimpleNamespace(**{**vars(native),'st_ino':99})
        try:compat.identity_mode(native,unsupported)
        except RuntimeError:checks['unrelated_identity_mismatch_refused']=True
        else:checks['unrelated_identity_mismatch_refused']=False
        with path.open('rb') as stream:checks['path_and_handle_match_after_compat']=replay._identity(path.stat())==replay._identity(os.fstat(stream.fileno()))
        checks['unchanged_history_restores_after_compat']=len(read().records)==1
        loader=replay.json.loads
        for mode in ('append',):
            path.write_bytes(raw);done=False;before=path.stat()
            def mutate(value,*pos,**kw):
                nonlocal done
                item=loader(value,*pos,**kw)
                if not done:
                    done=True
                    with path.open('ab') as out:out.write(raw)
                return item
            replay.json.loads=mutate
            try:
                try:read()
                except replay.HistoryReplayError as exc:checks[mode+'_still_rejected']=exc.code=='HISTORY_RESTORE_CHANGED'
                else:checks[mode+'_still_rejected']=False
            finally:replay.json.loads=loader
        path.write_bytes(raw);before=path.stat();identity=replay._identity(before)
        replacement=path.with_suffix('.replacement');replacement.write_bytes(raw)
        os.utime(replacement,ns=(before.st_atime_ns,before.st_mtime_ns));os.replace(replacement,path)
        try:replay._read_prefix(path,0,identity,0,replay.is_context_record,None,replay.HistoryReplayLimits())
        except replay.HistoryReplayError as exc:checks['replacement_between_stat_and_open_rejected']=exc.code=='HISTORY_RESTORE_CHANGED'
        else:checks['replacement_between_stat_and_open_rejected']=False
        for mode in ('same-length-rewrite','truncate'):
            path.write_bytes(raw);batch=read();stamp=path.stat()
            if mode=='truncate':path.write_bytes(b'')
            else:
                path.write_bytes(raw.replace(b'first',b'other'));os.utime(path,ns=(stamp.st_atime_ns,stamp.st_mtime_ns+1000000))
            try:replay._check_cursor(batch.cursor,path,0,replay._identity(path.stat()))
            except replay.HistoryReplayError as exc:checks[mode+'_stale_cursor_rejected']=exc.code=='HISTORY_RESTORE_CHANGED'
            else:checks[mode+'_stale_cursor_rejected']=False
        fake=Path(directory)/'different-source.py';fake.write_text('different upstream source',encoding='utf-8')
        try:compat.verify_source(fake)
        except RuntimeError:checks['different_source_pin_refused']=True
        else:checks['different_source_pin_refused']=False
        fake_source=Path(directory)/'unsupported-upstream'
        package=fake_source
        for part in ('jiuwenswarm','server','runtime','session'):
            package=package/part;package.mkdir(parents=True,exist_ok=True);(package/'__init__.py').write_text('',encoding='utf-8')
        (package/'history_replay.py').write_text('def _identity(value): return ()\n',encoding='utf-8')
        refuse_env=os.environ.copy();refuse_env.update(PYTHONPATH=str(bootstrap)+os.pathsep+str(fake_source),REPORT_DEMO_WINDOWS_HISTORY_COMPAT='1')
        refused=subprocess.run([sys.executable,'-c',"print('UNSAFE_STARTUP_CONTINUED')"],env=refuse_env,capture_output=True,text=True,timeout=20)
        checks['fresh_process_refuses_unsupported_source']=refused.returncode!=0 and 'UNSAFE_STARTUP_CONTINUED' not in refused.stdout and '[report-demo-history] startup refused' in refused.stderr
    for sid in args.session:
        path=get_read_history_path(sid);before=hashlib.sha256(path.read_bytes()).hexdigest()
        batch=replay._read_prefix(path,0,replay._identity(path.stat()),0,replay.is_context_record,None,replay.HistoryReplayLimits())
        unchanged=before==hashlib.sha256(path.read_bytes()).hexdigest()
        sessions.append({'session_id':sid,'bytes':path.stat().st_size,'semantic_records':len(batch.records),'history_sha256':before,'history_unchanged':unchanged})
        checks['existing_'+sid+'_read_without_rewrite']=unchanged and bool(batch.records)
    env=os.environ.copy();env.update(PYTHONPATH=str(bootstrap)+os.pathsep+str(source),REPORT_DEMO_WINDOWS_HISTORY_COMPAT='1')
    child=subprocess.run([sys.executable,'-c',"import json; from jiuwenswarm.server.runtime.session import history_replay as r; print(json.dumps(r.report_demo_windows_history_compat))"],env=env,capture_output=True,text=True,check=True,timeout=20)
    checks['fresh_child_inherits_compat']=json.loads(child.stdout)['active']
    report={'checks':checks,'all_passed':all(checks.values()),'compat':result,'sessions':sessions,
            'limits':['Real replay reader and file metadata; UI/model conversation recovery is a separate gate','Only temporary fixtures mutated; existing conversation bytes unchanged']}
    args.report.write_text(json.dumps(report,indent=2),encoding='utf-8');print(json.dumps(report))
    if not report['all_passed']:raise SystemExit(1)


if __name__=='__main__':main()
