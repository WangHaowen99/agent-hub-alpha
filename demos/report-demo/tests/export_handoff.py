"""Export W03/W04/W05 handoff projections from an actual task, read-only.

No credentials, provider request bodies or model text are exported. Original
evidence stays private; this projection is not independent acceptance.
"""
import argparse
import hashlib
import json
from pathlib import Path

import requests


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--private-root', type=Path, required=True)
    p.add_argument('--task-id', required=True)
    a = p.parse_args()
    cfg = json.loads((a.private_root / 'config.json').read_text(encoding='utf-8'))
    session = requests.Session()
    session.headers['Authorization'] = 'Bearer ' + cfg['tokens']['operator']

    def get(path):
        r = session.get(cfg['public_url'] + path, timeout=15)
        r.raise_for_status()
        return r.json()

    path = '/v1/tasks/' + a.task_id
    task = get(path)
    calls = get(path + '/calls')
    published = get('/v1/catalog')
    capabilities = {c['id']: c for c in published['catalog']['capabilities']} if published['catalog_digest'] == task['graph']['catalog_digest'] else {}
    bindings = {n['id']: {'capability_id': n['expert'], 'agent_id': capabilities[n['expert']].get('agent_id'), 'remote_binding': capabilities[n['expert']].get('remote_binding')} for n in task['graph']['nodes'] if n['expert'] in capabilities}
    call_samples = []
    for c in calls:
        observations = get('/v1/calls/' + c['call_id'] + '/observations')
        route = c.get('route') or {}
        card = route.get('card') or {}
        raw_card = card.get('rawJson', '')
        card_document = json.loads(raw_card) if raw_card else {}
        instance_versions = [{k: item.get(k) for k in ('health', 'contractVersion', 'capabilityVersion')} for item in route.get('instances', {}).get('instances', [])]
        call_samples.append({
            'identity': {k: c.get(k) for k in ('call_id', 'task_id', 'node_id', 'attempt', 'input_digest', 'agent_id', 'message_id', 'context_id', 'remote_task_id')},
            'state': c['state'], 'conflicted': c['conflicted'],
            'discovery': {'agent_id': route.get('agent_id'), 'observed_at': route.get('observed_at'),
                          'card_name': card.get('name'), 'card_version': card_document.get('version'),
                          'observed_card_digest': 'sha256:' + hashlib.sha256(raw_card.encode('utf-8')).hexdigest() if raw_card else None,
                          'instance_versions': instance_versions, 'gateway_mode': route.get('gateway_mode'),
                          'actual_execution_instance': route.get('actual_execution_instance', 'not_collected')},
            'observations': [{'source': o['source'], 'event_id': o['event_id'], 'state': o['document']['state'],
                              'remote_task_id': o['document'].get('remote_task_id'), 'received_at': o['received_at']}
                             for o in observations],
        })
    result = {'schema_version': 'report-handoff/1', 'task_id': task['task_id'], 'generation': task['generation'],
              'graph_revision': task['graph_revision'], 'graph_digest': task['graph_digest'],
              'catalog_digest': task['graph']['catalog_digest'], 'capability_bindings': bindings,
              'execution_profile': task.get('execution_profile'), 'status': task['status'],
              'model_binding_digest':task.get('model_binding_digest'),
              'selection_ref':task['graph'].get('selection'),
              'nodes': {n: {'status': s['status'], 'attempt': s['attempt'],
                            'identity': (s.get('result') or {}).get('identity'),
                            'input_digest': s.get('input_digest'),
                            'artifact_digest': (s.get('result') or {}).get('artifact_digest')}
                        for n, s in task['nodes'].items()},
              'calls': call_samples, 'read_only': True,
              'limits': ['SEND permission exists only in the first command response, not in persisted calls',
                         'Remote messageId idempotence and cancellation are not declared',
                         'Lost remote task remains UNKNOWN; no automatic resend',
                         'Same-host observation is not joint or independent acceptance']}
    after = get(path)
    if task['graph'].get('selection'):
        projection=get('/v1/workbench/tasks/'+task['task_id'])
        result['selection']=projection['selection']
    if after['generation'] != task['generation']:
        raise SystemExit('Projection changed during export; retry after task settles')
    destination = a.private_root / 'evidence' / (a.task_id + '-handoff.json')
    destination.write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding='utf-8')
    print(json.dumps({'task_id': a.task_id, 'calls': len(calls), 'generation': task['generation'], 'read_only': True}))


if __name__ == '__main__':
    main()
