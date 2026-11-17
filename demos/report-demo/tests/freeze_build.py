"""Capture effective Maven models and dependency trees without rebuilding live JARs.

Run serially against the same private Maven settings used for the build. This
records the current resolution and locks; it does not certify upstream tests or
prove that a previously built artifact was produced by a different invocation.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'deploy'))
from platform_tools import go as native_go, maven as native_maven, workswarm as workswarm_source


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--private-root',type=Path,required=True)
    parser.add_argument('--maven-settings',type=Path,required=True)
    args=parser.parse_args()
    demo=Path(__file__).resolve().parents[1];repo=demo.parents[1]
    root=args.private_root.resolve();output=root/'evidence/build-resolution'
    output.mkdir(parents=True,exist_ok=True)
    settings=args.maven_settings.resolve()
    if not settings.is_file():raise SystemExit('Explicit build Maven settings missing')
    maven=str(native_maven())
    modules={
        'orchestrator':demo/'orchestrator-java/pom.xml',
        'experts':demo/'experts-java/pom.xml',
        'gateway':repo/'.local/sources/agent-solution/common/agent-bus/agent-gateway/pom.xml',
        'rdc':repo/'.local/sources/agent-solution/common/agent-bus/registry-discovery-center/pom.xml',
    }
    manifest={'schema_version':'report-demo-build-resolution/1','modules':{},'locks':{},
              'limits':['Current effective models and dependency resolution, not an independent clean rebuild',
                        'No upstream test acceptance inferred; credentials and Maven settings are not exported']}
    def run(command,log,cwd=repo):
        with log.open('wb') as stream:
            result=subprocess.run(command,cwd=cwd,stdout=stream,stderr=subprocess.STDOUT,check=False)
        if result.returncode:raise SystemExit('Build metadata command failed; inspect private log '+str(log))
    run([maven,'-version'],output/'maven-version.txt')
    for name,pom in modules.items():
        if not pom.is_file():raise SystemExit('Module POM missing: '+name)
        paths={}
        for goal,suffix in [('help:effective-pom','effective-pom.xml'),('dependency:tree','dependency-tree.txt')]:
            destination=output/(name+'-'+suffix)
            command=[maven,'-B','-q','-s',str(settings),'-f',str(pom),goal,
                     '-Doutput='+str(destination),'-DoutputFile='+str(destination)]
            run(command,output/(name+'-'+goal.split(':')[0]+'.log'))
            if not destination.is_file() or not destination.stat().st_size:
                raise SystemExit('Expected metadata output missing: '+destination.name)
            paths[suffix]={'file':destination.name,'bytes':destination.stat().st_size,
                           'sha256':hashlib.sha256(destination.read_bytes()).hexdigest()}
        manifest['modules'][name]={'pom':pom.relative_to(repo).as_posix(),'outputs':paths}
        print(json.dumps({'module':name,'resolution_captured':True}),flush=True)
    go=native_go()
    # Freeze packages actually imported by this binary. `list -m all` also
    # fetches unused requirements of the parent module, which neither builds
    # this demo nor proves anything about the running executable.
    run([str(go),'list','-deps','-json','./...'],output/'go-packages.jsonl',demo/'control-go')
    workswarm=workswarm_source()
    for name,path in {'go.mod':demo/'control-go/go.mod','go.sum':demo/'control-go/go.sum',
                      'parent-go.mod':repo/'agent-festival/go/go.mod',
                      'parent-go.sum':repo/'agent-festival/go/go.sum',
                      'workswarm-uv.lock':workswarm/'uv.lock',
                      'workswarm-package-lock.json':workswarm/'jiuwenswarm/channels/web/frontend/package-lock.json'}.items():
        if not path.is_file():raise SystemExit('Required lock missing: '+str(path))
        manifest['locks'][name]={'bytes':path.stat().st_size,'sha256':hashlib.sha256(path.read_bytes()).hexdigest()}
    (output/'manifest.json').write_text(json.dumps(manifest,indent=2),encoding='utf-8')
    print(json.dumps({'build_resolution_captured':True,'modules':list(modules),'output':str(output)}))


if __name__=='__main__':main()
