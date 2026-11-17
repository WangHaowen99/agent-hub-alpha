"""Check exported real-run evidence; never starts services or approves a report.

This checks recorded bindings and outputs. It is not independent PDF/business
acceptance and does not infer that a model response is semantically correct.
"""
import argparse
import hashlib
import json
from pathlib import Path


def envelopes(task):
    for artifact in task.get('artifacts',[]):
        for part in artifact.get('parts',[]):
            value=part.get('data')
            if value is None and isinstance(part.get('text'),str):
                try:value=json.loads(part['text'])
                except ValueError:continue
            if isinstance(value,dict):yield value


def verify(trace,delivery,report,download_digest):
    task=trace['task'];checks={};calls=trace['calls']
    checks['delivered_with_intact_evidence']=task['status']=='DELIVERED' and trace['verification']['evidence_intact'] and not trace['verification']['issues']
    # Four distinct experts; a bounded revision is a fifth call to the writer,
    # not a fifth expert and not a duplicate send of the original remote task.
    checks['real_remote_tasks_for_four_experts_and_optional_revision']=len(calls) in (4,5) and len({c['agent_id'] for c in calls})==4 and len({c['remote_task_id'] for c in calls})==len(calls) and all(c['state']=='COMPLETED' and not c['conflicted'] for c in calls)
    checks['four_independent_expert_roles']={r['runtime_evidence']['role'] for r in trace['runtime_evidence']}=={'extract','business','quality','write'} and len({r['runtime_evidence']['agent_id'] for r in trace['runtime_evidence']})==4
    outputs_match=True
    for call in calls:
        result=task['nodes'][call['node_id']].get('result',{})
        observed=[]
        for entry in trace['observations'][call['call_id']]:
            observation=entry['document']
            if observation['state']!='COMPLETED':continue
            payload=observation['payload']
            if payload.get('id')!=call['remote_task_id'] or payload.get('contextId')!=call['context_id']:continue
            for value in envelopes(payload):
                if value.get('call_id')==call['call_id'] and value.get('input_digest')==call['input_digest'] and value.get('attempt')==call['attempt']:
                    observed.append(value.get('output'))
        outputs_match &= bool(observed) and result.get('valid',False) and result.get('input_digest')==call['input_digest'] and all(value==result.get('output') for value in observed)
    checks['every_remote_formal_output_matches_bound_completion']=outputs_match
    quality=[r for r in trace['runtime_evidence'] if r['runtime_evidence']['role']=='quality']
    branches=quality[0]['runtime_evidence']['branch_records'] if len(quality)==1 else []
    checks['two_completed_loop_branches_with_bound_inputs']=len(branches)==2 and all(b['completed'] and b['parent_input_digest']==quality[0]['input_digest'] and b['input_digest'] and b['output_digest'] and b['output'] for b in branches)
    delivered=delivery['delivery'];digest=delivered['artifact_digest']
    report_results=[n['result'] for n in task['nodes'].values() if n.get('result',{}).get('artifact_digest')==digest]
    checks['download_matches_formal_report']=len(report_results)==1 and report_results[0]['output']==report
    checks['download_bytes_match_artifact_digest']=download_digest==digest
    checks['same_object_program_check']=any(n.get('result',{}).get('output',{}).get('outcome')=='PASS' and n['result']['output'].get('object_digest')==digest for n in task['nodes'].values())
    checks['same_object_test_account_approval']=delivery['approval']['artifact_digest']==digest and delivered['task_id']==task['task_id'] and delivery['idempotent_delivery']
    checks['selection_preserves_program_only_qualification']=any(n.get('result',{}).get('output',{}).get('qualification')=='PROGRAM_CHECK' and n['result']['output'].get('artifact_digest')==digest and n['result']['output'].get('human_review') is None for n in task['nodes'].values())
    return {'task_id':task['task_id'],'execution_profile':task['execution_profile'],'graph_digest':task['graph_digest'],
            'checks':checks,'all_passed':all(checks.values()),
            'limits':['Recorded real model/transport evidence; not independent business acceptance',
                      'Approval and delivery performed by the isolated operator test account']}


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--private-root',type=Path,required=True);parser.add_argument('--task-id',required=True)
    args=parser.parse_args();evidence=args.private_root.resolve()/'evidence'
    def read(suffix):return json.loads((evidence/(args.task_id+suffix+'.json')).read_text(encoding='utf-8'))
    report_bytes=(evidence/(args.task_id+'-report.json')).read_bytes()
    result=verify(read('-trace'),read('-delivery'),read('-report'),'sha256:'+hashlib.sha256(report_bytes).hexdigest())
    (evidence/(args.task_id+'-main-verification.json')).write_text(json.dumps(result,indent=2),encoding='utf-8')
    print(json.dumps(result))
    if not result['all_passed']:raise SystemExit(1)


if __name__=='__main__':main()
