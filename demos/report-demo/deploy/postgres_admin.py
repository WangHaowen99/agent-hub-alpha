"""Narrow maintenance for the already installed demo PostgreSQL container.

Uses hidden SSH input, never writes the password, never drops a database or
restarts PostgreSQL. Recovery requires this workspace's RDC/control stopped.
Run with a Python environment containing paramiko.
"""
from __future__ import annotations
import argparse
import getpass
import json
from pathlib import Path
import shlex
import ctypes
from ctypes import wintypes
import paramiko

ACTIVITY="SELECT json_build_object('max_connections',current_setting('max_connections'),'connections',(SELECT json_agg(x) FROM (SELECT datname,usename,application_name,client_addr,state,count(*) AS count,min(backend_start) AS oldest FROM pg_stat_activity GROUP BY datname,usename,application_name,client_addr,state) x))"

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('action',choices=['diagnose','recover-idle','create-restore-db'])
    p.add_argument('--private-root',type=Path,required=True)
    p.add_argument('--host',required=True)
    a=p.parse_args();root=a.private_root.resolve()
    if a.action=='recover-idle':
        state=json.loads((root/'services.json').read_text(encoding='utf-8'))
        kernel=ctypes.WinDLL('kernel32',use_last_error=True)
        kernel.OpenProcess.argtypes=[wintypes.DWORD,wintypes.BOOL,wintypes.DWORD]
        kernel.OpenProcess.restype=wintypes.HANDLE
        kernel.GetExitCodeProcess.argtypes=[wintypes.HANDLE,ctypes.POINTER(wintypes.DWORD)]
        kernel.GetExitCodeProcess.restype=wintypes.BOOL
        kernel.CloseHandle.argtypes=[wintypes.HANDLE]
        kernel.CloseHandle.restype=wintypes.BOOL
        for name in ('rdc','control'):
            row=state.get(name,{})
            if row.get('pid'):
                handle=kernel.OpenProcess(0x1000,False,int(row['pid']))
                if handle:
                    code=wintypes.DWORD()
                    try:
                        if not kernel.GetExitCodeProcess(handle,ctypes.byref(code)):
                            raise ctypes.WinError(ctypes.get_last_error())
                    finally:kernel.CloseHandle(handle)
                    if code.value==259:raise SystemExit('Stop this workspace '+name+' before idle-connection maintenance')
    password=getpass.getpass('SSH password (hidden): ')
    ssh=paramiko.SSHClient();known=root/'ssh-known-hosts'
    if known.exists():ssh.load_host_keys(str(known))
    else:ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(a.host,username='root',password=password,timeout=15,allow_agent=False,look_for_keys=False)
    if not known.exists():ssh.save_host_keys(str(known))
    def sql(query):
        cmd='docker exec agent-hub-demo-postgres psql -U postgres -d postgres -X -v ON_ERROR_STOP=1 -A -t -c '+shlex.quote(query)
        _,out,err=ssh.exec_command(cmd,timeout=20);data=out.read().decode();message=err.read().decode();code=out.channel.recv_exit_status()
        if code:raise RuntimeError('PostgreSQL maintenance failed: '+message[:500])
        return data.strip()
    result={'action':a.action,'before':json.loads(sql(ACTIVITY))}
    if a.action=='recover-idle':
        result['terminated_idle_backends']=json.loads(sql("SELECT COALESCE(json_agg(x),'[]'::json) FROM (SELECT pid,datname,usename,pg_terminate_backend(pid) AS terminated FROM pg_stat_activity WHERE datname IN ('agent_rdc','agent_hub_demo') AND usename IN ('agent_rdc','aha_control_app','aha_control_migrator') AND state='idle' AND backend_start<clock_timestamp()-interval '30 seconds') x"))
        for role in ('agent_rdc','aha_control_app','aha_control_migrator'):
            sql('ALTER ROLE '+role+' SET tcp_keepalives_idle=60; ALTER ROLE '+role+' SET tcp_keepalives_interval=10; ALTER ROLE '+role+' SET tcp_keepalives_count=3;')
        result['role_keepalive_seconds']={'idle':60,'interval':10,'count':3}
    elif a.action=='create-restore-db':
        exists=sql("SELECT count(*) FROM pg_database WHERE datname='report_demo_restore_test'")
        if exists=='0':sql('CREATE DATABASE report_demo_restore_test OWNER aha_control_migrator')
        result['restore_database']='report_demo_restore_test';result['created']=exists=='0'
    result['after']=json.loads(sql(ACTIVITY));ssh.close()
    path=root/'evidence'/('postgres-'+a.action+'.json');path.write_text(json.dumps(result,indent=2),encoding='utf-8')
    print(json.dumps(result))

if __name__=='__main__':main()
