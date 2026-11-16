"""Run/inspect the shared contract through the actual Java/Bus/expert consumers.

The start action reuses an explicitly named, previously generated candidate
graph; it does not claim to perform fresh conversational planning. Approval
uses the isolated operator test account, not independent business acceptance.
"""
import argparse
import json
import time
from pathlib import Path

import requests


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--private-root', type=Path, required=True)
    p.add_argument('action', choices=['start', 'status', 'verify'])
    p.add_argument('--source-task')
    a = p.parse_args()
    root = a.private_root.resolve()
    cfg = json.loads((root / 'config.json').read_text(encoding='utf-8'))
    evidence = root / 'evidence'
    run_path = evidence / 'shared-contract-run.json'
    base = cfg['public_url']
    session = requests.Session()
    login = session.post(base + '/auth/login', json={'user': cfg['user'], 'password': cfg['user_password']}, timeout=10)
    login.raise_for_status()
    headers = {'X-CSRF-Token': login.json()['csrf'], 'Origin': base}
    if a.action == 'start':
        if not a.source_task:
            raise SystemExit('--source-task required; no synthetic planning claim')
        source = session.get(base + '/v1/tasks/' + a.source_task, timeout=10)
        source.raise_for_status()
        response = requests.post(base + '/v1/commands', headers={'Authorization': 'Bearer ' + cfg['tokens']['planner']},
                                 json={'command_id': 'shared-contract-' + str(time.time_ns()), 'graph': source.json()['graph']}, timeout=20)
        response.raise_for_status()
        task = response.json()
        admitted = session.post(base + '/v1/tasks/' + task['task_id'] + '/admit', headers=headers,
                                json={'graph_digest': task['graph_digest']}, timeout=10)
        admitted.raise_for_status()
        run = {'task_id': task['task_id'], 'source_candidate_task': a.source_task,
               'fresh_model_planning': False, 'actor': 'isolated operator test', 'identity_contract': task['identity_contract']}
        run_path.write_text(json.dumps(run, indent=2), encoding='utf-8')
        print(json.dumps(run))
        return
    run = json.loads(run_path.read_text(encoding='utf-8'))
    task_url = base + '/v1/tasks/' + run['task_id']
    response = session.get(task_url, timeout=10)
    response.raise_for_status()
    task = response.json()
    if a.action == 'status':
        print(json.dumps({'task_id': task['task_id'], 'status': task['status'],
                          'nodes': {k: v['status'] for k, v in task['nodes'].items()}}))
        return
    checks = {'shared_contract_bound': task['identity_contract'] == 'pr25/0.2.0@539d17a'}
    results = {k: n['result'] for k, n in task['nodes'].items() if n.get('result')}
    checks['formal_results_present'] = len(results) >= 6
    checks['every_formal_result_has_structured_identity'] = all(
        r['identity']['logical_execution'] == {'task_id': task['task_id'], 'graph_version': task['graph_revision'],
                                               'node_path': k, 'iteration_path': '0'}
        and r['identity']['business_request_id'] == task['task_id']
        and r['identity']['evaluation_revision'] == task['graph_revision']
        and isinstance(r['identity']['execution_generation'], int) for k, r in results.items())
    for node, result in results.items():
        url = task_url + '/nodes/' + node + '/result'
        actual = session.get(url, params={'input_digest': result['input_digest']}, timeout=10)
        actual.raise_for_status()
        checks['reconcile_same_input:' + node] = actual.json()['serve'] and actual.json()['stored'] == result
        changed = session.get(url, params={'input_digest': result['receipt']}, timeout=10)
        changed.raise_for_status()
        checks['receipt_not_input:' + node] = changed.json()['committed'] and not changed.json()['serve']
    after = session.get(task_url, timeout=10)
    after.raise_for_status()
    checks['reconciliation_read_only'] = after.json()['generation'] == task['generation']
    report = {**run, 'status': task['status'], 'checks': checks, 'all_passed': all(checks.values()),
              'limits': ['Actual Java/Bus/model consumers; fixed prior candidate graph', 'Operator test approval, not independent business acceptance']}
    (evidence / 'shared-contract-model-verification.json').write_text(json.dumps(report, indent=2), encoding='utf-8')
    print(json.dumps(report))
    if not report['all_passed']:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
