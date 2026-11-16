"""Verify a recorded natural model revision independently of a failed injector.

Keeps the original injection-test failure unchanged. Requires its record to
show zero injected calls, and uses current formal task/call/trace facts to
prove only the naturally occurring model/checker/revision path.
"""
import argparse
import json
from pathlib import Path


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--private-root',type=Path,required=True);p.add_argument('--task-id',required=True);a=p.parse_args()
    evidence=a.private_root.resolve()/'evidence';task_id=a.task_id
    original=json.loads((evidence/(task_id+'-revision.json')).read_text(encoding='utf-8'))
    trace=json.loads((evidence/(task_id+'-trace.json')).read_text(encoding='utf-8'))
    task=trace['task'];nodes=task['nodes'];result=lambda name:nodes[name]['result'];output=lambda name:result(name)['output']
    calls=trace['calls'];by_node={c['node_id']:c for c in calls}
    checks={
        'original_injection_test_failure_preserved':original['all_passed'] is False and not original['checks']['one_corrupted_draft_call'],
        'no_transport_fault_was_applied':original['relay']['fault_calls']=={},
        'same_admitted_graph':task['graph_digest']==original['graph_digest'],
        'natural_draft_rejected_by_program':output('check0')['outcome']=='REVISE' and bool(output('check0')['failures']),
        'check0_bound_to_original_draft':output('check0')['object_digest']==result('draft0')['artifact_digest'],
        'one_real_revision_node_attempt':nodes['revise1']['status']=='COMMITTED' and nodes['revise1']['attempt']==1,
        'two_distinct_real_writer_calls':all(n in by_node for n in ('draft0','revise1')) and by_node['draft0']['remote_task_id']!=by_node['revise1']['remote_task_id'],
        'revision_rechecked_pass':output('check1')['outcome']=='PASS' and output('check1')['object_digest']==result('revise1')['artifact_digest'],
        'selected_revised_report':output('select_report')['artifact_digest']==result('revise1')['artifact_digest'],
        'test_account_delivered_revised_report':task['status']=='DELIVERED' and any(r['artifact_digest']==result('revise1')['artifact_digest'] for r in task.get('delivery_receipts',[])),
        'all_evidence_intact':trace['verification']['evidence_intact'],
    }
    record={'task_id':task_id,'all_passed':all(checks.values()),'checks':checks,'original_check_failures':output('check0')['failures'],
        'first_draft':result('draft0')['artifact_digest'],'revised_report':result('revise1')['artifact_digest'],
        'real_remote_calls':len(calls),'extraction_reused_from':result('extract1').get('reused_from'),
        'claim':'natural model schema error, one actual writer revision, program PASS, test-account delivery; separate from the failed planned transport injection',
        'independent_business_acceptance_asserted':False}
    (evidence/(task_id+'-natural-revision-verification.json')).write_text(json.dumps(record,ensure_ascii=False,indent=2),encoding='utf-8')
    print(json.dumps(record))
    if not record['all_passed']:raise SystemExit(1)


if __name__=='__main__':main()
