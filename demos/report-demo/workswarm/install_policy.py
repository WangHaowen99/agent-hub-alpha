"""Install a versioned policy using WorkSwarm's supported RailManager API.

Only the explicit isolated demo workspace is affected. No user global model
settings, credentials, upstream source, or old policy packages are removed.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil


def install(workspace:Path):
    os.environ['JIUWENSWARM_DATA_DIR']=str(workspace.resolve())
    from jiuwenswarm.common.utils import get_agent_workspace_dir
    from jiuwenswarm.agents.harness.common.plugins.rail_manager import get_rail_manager
    source=Path(__file__).parent/'report_planner_policy/rail.py'
    digest=hashlib.sha256(source.read_bytes()).hexdigest();name='report_planner_policy_'+digest[:12]
    package=get_agent_workspace_dir()/'generated-policy-source'/name;package.mkdir(parents=True,exist_ok=True)
    shutil.copyfile(source,package/'rail.py')
    manager=get_rail_manager();extensions=manager.get_extensions()
    for extension in extensions:
        if extension['name'].startswith('report_planner_policy_') and extension['name']!=name:manager.toggle_extension(extension['name'],False)
    if not any(e['name']==name for e in extensions):manager.import_extension(str(package))
    manager.toggle_extension(name,True)
    return {'policy':name,'sha256':digest,'enabled':True,'scope':'isolated demo workspace; model ability policy, not OS process isolation','restart_agentserver_required':True}


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--workspace',type=Path,required=True);a=p.parse_args();print(json.dumps(install(a.workspace)))
