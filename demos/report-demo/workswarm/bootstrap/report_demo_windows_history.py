"""Pinned Windows file-identity compatibility for this demo's WorkSwarm process.

On the current Windows Python, stat().st_ctime_ns aliases creation time while
fstat().st_ctime_ns returns change time. WorkSwarm compares them and rejects an
unchanged history file. Use explicit birth time consistently; retain device,
inode, exact size and nanosecond mtime and all upstream replay/cursor checks.
No history or upstream source is rewritten. This is not an upstream SDK API.
"""
from __future__ import annotations

import hashlib
import os
from pathlib import Path
import sys
import tempfile
import time

SOURCE_SHA256 = '6cff70ffd9d054c147157a2f14bf00777835608c1bba173501b13450384d71e9'


def verify_source(path: Path) -> None:
    # Python source uses universal newlines; accept LF/CRLF checkouts of the
    # same audited text, while refusing any actual source change.
    if hashlib.sha256(path.read_bytes().replace(b'\r\n',b'\n')).hexdigest()!=SOURCE_SHA256:
        raise RuntimeError('WorkSwarm history replay source differs from the audited compatibility pin')


def identity_mode(path_stat, handle_stat):
    def native(s):return s.st_dev,s.st_ino,s.st_size,s.st_mtime_ns,s.st_ctime_ns
    if native(path_stat)==native(handle_stat):return 'native'
    if (hasattr(path_stat,'st_birthtime_ns') and hasattr(handle_stat,'st_birthtime_ns')
            and native(path_stat)[:4]==native(handle_stat)[:4]
            and path_stat.st_birthtime_ns==handle_stat.st_birthtime_ns
            and path_stat.st_ctime_ns==path_stat.st_birthtime_ns):
        return 'normalize_birthtime'
    raise RuntimeError('Unsupported Windows path/handle identity mismatch')


def probe_runtime():
    # Probe only an owned temporary file. A future Python with consistent
    # ctime semantics keeps the native guard rather than receiving this patch.
    with tempfile.TemporaryDirectory(prefix='report-demo-file-identity-') as directory:
        path=Path(directory)/'probe';path.write_bytes(b'before')
        for _ in range(3):
            time.sleep(.03);path.write_bytes(b'after!')
            with path.open('rb') as stream:
                a=path.stat();b=os.fstat(stream.fileno())
            if not hasattr(a,'st_birthtime_ns') or a.st_mtime_ns!=a.st_birthtime_ns:
                return identity_mode(a,b)
    raise RuntimeError('Could not establish an unambiguous Windows file-identity probe')


def install():
    if sys.platform!='win32':
        return {'active':False,'reason':'non-Windows; native identity retained'}
    from jiuwenswarm.server.runtime.session import history_replay as replay
    verify_source(Path(replay.__file__))
    existing=getattr(replay,'report_demo_windows_history_compat',None)
    if existing is not None:return existing
    original=replay._identity
    if original.__module__!=replay.__name__ or original.__name__!='_identity':
        raise RuntimeError('Unexpected preexisting history identity override')
    mode=probe_runtime()
    if mode=='native':
        return {'active':False,'reason':'native path and handle identities agree; native guard retained',
                'source_sha256':SOURCE_SHA256,'runtime_probe':mode}

    def windows_identity(stat):
        if not hasattr(stat,'st_birthtime_ns'):
            raise RuntimeError('Windows birth-time identity unavailable; refusing history restore')
        return stat.st_dev,stat.st_ino,stat.st_size,stat.st_mtime_ns,stat.st_birthtime_ns

    replay._identity=windows_identity
    replay.report_demo_windows_history_compat={'active':True,'source_sha256':SOURCE_SHA256,
        'runtime_probe':mode,
        'source_hash_encoding':'Python source with CRLF normalized to LF',
        'identity_fields':['st_dev','st_ino','st_size','st_mtime_ns','st_birthtime_ns'],
        'scope':'isolated demo Python processes; Windows creation-time normalization only'}
    return replay.report_demo_windows_history_compat


def bootstrap():
    if os.environ.get('REPORT_DEMO_WINDOWS_HISTORY_COMPAT')!='1':return
    try:
        result=install()
        print('[report-demo-history] active='+str(result['active'])+' pid='+str(os.getpid())+' source='+str(result.get('source_sha256','native')),file=sys.stderr,flush=True)
    except Exception as exc:
        # Python otherwise logs a sitecustomize exception and keeps starting.
        # This explicitly opted-in process must not silently omit its pin.
        print('[report-demo-history] startup refused: '+type(exc).__name__,file=sys.stderr,flush=True)
        raise SystemExit(78) from exc
