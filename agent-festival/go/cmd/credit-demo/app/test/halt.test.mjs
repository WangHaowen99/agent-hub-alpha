import { build } from 'esbuild';
import { test } from 'node:test';
import assert from 'node:assert/strict';
const load = async (entry) => {
  const out = await build({ entryPoints: [entry], bundle: true, write: false, format: 'esm', platform: 'node' });
  return import('data:text/javascript;base64,' + Buffer.from(out.outputFiles[0].text).toString('base64'));
};
const { stoppedView, lastObserved, stoppedLine } = await load('src/flow/halt.ts');
const layout = await load('src/flow/layout.ts');

// R9-like snapshot: financial lost a query reply and the task stopped while the
// local black box (related) was still listed as running with one fork started.
const graph = (status) => ({
  plan: 'P9', version: 1, case: 'C006', goal: 'g', ok: true, hash: 'h', rev: 1,
  layers: [['intake'], ['financial', 'related'], ['judge']],
  run: { id: 'R9', status, leases: 1, url: '' },
  nodes: [
    { id: 'intake', name: '材料受理', kind: 'investigate', task: 't', status: 'done',
      remote: { updated_at: '2026-11-18T07:57:18Z', committed_at: '2026-11-18T07:57:18Z', agent_started_at: '2026-11-18T07:57:14Z' } },
    { id: 'financial', name: '财务分析师', kind: 'investigate', task: 't', deps: ['intake'], status: 'unknown',
      remote: { updated_at: '2026-11-18T07:57:21Z', gateway: { last_error: { finished_at: '2026-11-18T07:57:23.1Z' }, last_exchange: { finished_at: '2026-11-18T07:57:23.1Z' } } } },
    { id: 'related', name: '关联方排查', kind: 'blackbox', task: 't', deps: ['intake'], status: 'running', activity: '读关联方名单 · C006',
      branches: [{ goal: 'a', status: 'started', title: '王氏汽配', activity: '查征信摘要' }, { goal: 'b', status: 'finished', title: '顺达物流', finding: '无逾期' }] },
    { id: 'judge', name: '风险判断', kind: 'judge', task: 't', deps: ['financial', 'related'], status: 'pending' },
  ],
});

for (const status of ['waiting', 'failed', 'blocked']) {
  test(`a ${status} task draws its still-running step and started forks as stopped`, () => {
    const g = graph(status);
    const v = stoppedView(g);
    const related = v.nodes.find((n) => n.id === 'related');
    assert.equal(related.status, 'stopped');
    assert.deepEqual(related.branches.map((b) => b.status), ['stopped', 'finished']);
    assert.equal(v.nodes[0], g.nodes[0], 'untouched steps are kept as served');
    assert.equal(g.nodes[2].status, 'running', 'the served snapshot is not modified');
  });
}

test('queued, running and done tasks are drawn as served', () => {
  for (const status of ['queued', 'running', 'done']) {
    const g = graph(status);
    assert.equal(stoppedView(g), g);
  }
  const plan = { ...graph('running'), run: undefined };
  assert.equal(stoppedView(plan), plan);
});

test('the observation time is the latest recorded remote time, never invented', () => {
  assert.equal(lastObserved(graph('waiting')), '2026-11-18T07:57:23.1Z');
  const local = { ...graph('waiting'), nodes: graph('waiting').nodes.filter((n) => !n.remote) };
  assert.equal(lastObserved(local), undefined);
  assert.equal(stoppedLine(local).text, '已停止');
  const line = stoppedLine(graph('waiting'), '读关联方名单 · C006');
  assert.match(line.text, /^已停止 · 最后观察 \d{2}:\d{2}:\d{2}$/);
  assert.match(line.title, /停止前记录：读关联方名单 · C006/);
  assert.match(line.title, /最近一次有记录的远端调用时间/);
});

test('the drawn graph has no spinner state, live line or moving edge for a halted task', () => {
  const built = layout.build(stoppedView(graph('waiting')), new layout.Delays());
  const host = built.nodes.find((n) => n.id === 'related/host');
  assert.equal(host.data.status, 'stopped');
  assert.equal(host.data.line.kind, 'note');
  assert.match(host.data.line.text, /^已停止 · 最后观察 /);
  assert.notEqual(built.nodes.find((n) => n.id === 'related').data.n.status, 'running');
  assert.deepEqual(built.nodes.filter((n) => n.type === 'branch').map((n) => n.data.b.status), ['stopped', 'finished']);
  const states = built.edges.map((e) => e.data.state);
  assert.ok(!states.includes('active') && !states.includes('lf-active'), states.join());
  assert.equal(built.edges.find((e) => e.id === 'related/e0').data.state, 'lf-off');
});

test('a running task still draws its running step as live', () => {
  const built = layout.build(stoppedView(graph('running')), new layout.Delays());
  const host = built.nodes.find((n) => n.id === 'related/host');
  assert.equal(host.data.status, 'running');
  assert.equal(host.data.line.kind, 'act');
  assert.equal(host.data.line.text, '读关联方名单 · C006');
  assert.ok(built.edges.some((e) => e.data.state === 'active'));
  assert.equal(built.edges.find((e) => e.id === 'related/e0').data.state, 'lf-active');
});
