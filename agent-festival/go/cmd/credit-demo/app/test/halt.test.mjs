import { build } from 'esbuild';
import { test } from 'node:test';
import assert from 'node:assert/strict';
const load = async (entry) => {
  const out = await build({ entryPoints: [entry], bundle: true, write: false, format: 'esm', platform: 'node' });
  return import('data:text/javascript;base64,' + Buffer.from(out.outputFiles[0].text).toString('base64'));
};
const { stoppedView, lastObserved, lastObservedStep, stoppedLine, inflightLine, callEvidence } = await load('src/flow/halt.ts');
const layout = await load('src/flow/layout.ts');

// R9-like snapshot: financial lost a query reply and the task stopped while the
// local black box (related) was still listed as running with one fork started.
const graph = (status) => ({
  plan: 'P9', version: 1, case: 'C006', goal: 'g', ok: true, hash: 'h', rev: 1,
  layers: [['intake'], ['financial', 'industry', 'related'], ['judge']],
  run: { id: 'R9', status, leases: 1, url: '' },
  nodes: [
    { id: 'intake', name: '材料受理', kind: 'investigate', task: 't', status: 'done',
      remote: { updated_at: '2026-11-18T07:57:18Z', committed_at: '2026-11-18T07:57:18Z', agent_started_at: '2026-11-18T07:57:14Z' } },
    { id: 'financial', name: '财务分析师', kind: 'investigate', task: 't', deps: ['intake'], status: 'unknown',
      remote: { updated_at: '2026-11-18T07:57:21Z', gateway: { last_error: { finished_at: '2026-11-18T07:57:23.1Z' }, last_exchange: { finished_at: '2026-11-18T07:57:23.1Z' } } } },
    // A sent call still out when the task stopped: the remote task may still run.
    { id: 'industry', name: '行业研究员', kind: 'investigate', task: 't', deps: ['intake'], status: 'waiting', activity: '专业 Agent 执行中，查询原任务',
      execution: { kind: 'agent_bus' },
      remote: { request_id: 'q2', task_id: 't2', agent_started_at: '2026-11-18T07:57:16Z', updated_at: '2026-11-18T07:57:19Z', gateway: { last_exchange: { finished_at: '2026-11-18T07:57:20Z' } } } },
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
    assert.equal(g.nodes[3].status, 'running', 'the served snapshot is not modified');
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

for (const status of ['waiting', 'failed', 'blocked']) {
  test(`a ${status} task draws a sent Agent Bus call as in flight, with its own last observation`, () => {
    const g = graph(status);
    const v = stoppedView(g);
    const industry = v.nodes.find((n) => n.id === 'industry');
    assert.equal(industry.status, 'inflight');
    assert.equal(g.nodes.find((n) => n.id === 'industry').status, 'waiting', 'the served snapshot is not modified');
    assert.equal(lastObservedStep(industry), '2026-11-18T07:57:20Z', 'the step time, not the task-wide one');
    const built = layout.build(v, new layout.Delays());
    const card = built.nodes.find((n) => n.id === 'industry');
    assert.equal(card.data.status, 'inflight');
    assert.equal(card.data.line.kind, 'note');
    assert.equal(card.data.line.tone, 'warn');
    assert.match(card.data.line.text, /^远端在途 · 最后观察 \d{2}:\d{2}:\d{2}$/);
    assert.match(card.data.line.title, /远端任务可能仍在执行/);
    assert.match(card.data.line.title, /最近记录：专业 Agent 执行中，查询原任务/);
    assert.ok(!built.nodes.some((n) => n.data.line?.kind === 'act'), 'no live status line in a halted task');
    assert.ok(!built.edges.some((e) => e.data.state === 'active' || e.data.state === 'lf-active'));
  });
}

test('an Agent Bus step with no recorded call is stopped, not in flight; unknown stays to be checked', () => {
  const g = graph('waiting');
  g.nodes.push({ id: 'report', name: '报告撰写人', kind: 'report', task: 't', deps: ['judge'], status: 'running', execution: { kind: 'agent_bus' } });
  const v = stoppedView(g);
  assert.equal(v.nodes.find((n) => n.id === 'report').status, 'stopped');
  assert.equal(v.nodes.find((n) => n.id === 'financial').status, 'unknown');
});

test('an in-flight call without recorded times reads only "远端在途"', () => {
  const line = inflightLine({ id: 'x', name: 'x', kind: 'investigate', task: 't', status: 'inflight', remote: { request_id: 'q' } });
  assert.equal(line.text, '远端在途');
  assert.match(line.title, /没有带时间的记录/);
});

test('a running task still draws a waiting Agent Bus call as live', () => {
  const built = layout.build(stoppedView(graph('running')), new layout.Delays());
  const card = built.nodes.find((n) => n.id === 'industry');
  assert.equal(card.data.status, 'waiting');
  assert.equal(card.data.line.kind, 'act');
  assert.equal(card.data.line.text, '专业 Agent 执行中，查询原任务');
});

// Three evidence tiers for an Agent Bus call (the owner's counterexample first):
// a waiting task, a running step, and a call record with only intent_saved_at.
const T0 = '2026-11-20T03:10:00Z', T1 = '2026-11-20T03:10:02Z', T2 = '2026-11-20T03:10:05Z';
const sendErr = { rpc_id: 'q1', method: 'SendMessage', started_at: T0, finished_at: T1, outcome: 'transport_error', sequence: 3 };
const receipt = { rpc_id: 'q1', method: 'SendMessage', started_at: T0, finished_at: T1, http_status: 200, outcome: 'task_received', sequence: 3 };
const call = (remote, status = 'running') => ({
  plan: 'P1', version: 1, case: 'C006', goal: 'g', ok: true, hash: 'h', rev: 1, layers: [['intake'], ['industry']],
  run: { id: 'R1', status: 'waiting', leases: 0, url: '' },
  nodes: [
    { id: 'intake', name: '材料受理', kind: 'investigate', task: 't', status: 'done', execution: { kind: 'agent_bus' } },
    { id: 'industry', name: '行业研究员', kind: 'investigate', task: 't', deps: ['intake'], status, execution: { kind: 'agent_bus' },
      remote: { request_id: 'q1', task_id: '', agent_started_at: '', intent_saved_at: T0, updated_at: T0, ...remote } },
  ],
});
const drawn = (g) => {
  const card = layout.build(stoppedView(g), new layout.Delays()).nodes.find((n) => n.id === 'industry');
  if (!card) return undefined;
  return { status: card.data.status, text: card.data.line?.text, tone: card.data.line?.tone, title: card.data.line?.title };
};
const tiers = [
  ['only the intent record (owner counterexample)', {}, 'intent', 'unconfirmed', /^已登记，发送待确认$/, [/没有 Gateway 发送记录/, /不能确认请求已到达专家端/]],
  ['a SendMessage that ended without a task', { gateway: { send_count: 1, query_count: 0, first_send: sendErr, last_exchange: sendErr, last_error: sendErr } },
    'send', 'unconfirmed', /^已登记，发送待确认$/, [/Gateway 记录了 1 次发送/, /未收到 HTTP 回执/, /不能确认请求已到达专家端/]],
  ['a Gateway task receipt', { gateway: { send_count: 1, query_count: 0, first_send: receipt, first_receipt: receipt, last_exchange: receipt } },
    'receipt', 'inflight', /^远端在途 · 最后观察 \d{2}:\d{2}:\d{2}$/, [/Gateway 任务回执/, /远端任务可能仍在执行/]],
  ['only a remote task id', { task_id: 'rt-1' }, 'receipt', 'inflight', /^远端在途 · 最后观察 /, [/（远端任务号）/]],
  ['only the expert start', { agent_started_at: T2 }, 'receipt', 'inflight', /^远端在途 · 最后观察 /, [/（专家已开始执行）/]],
];
for (const [name, remote, evidence, status, text, titles] of tiers) {
  for (const served of ['running', 'waiting']) {
    test(`halted task, ${served} step with ${name}: ${status}`, () => {
      const g = call(remote, served);
      assert.equal(callEvidence(g.nodes[1].remote), evidence);
      const d = drawn(g);
      assert.equal(d.status, status);
      assert.equal(d.tone, 'warn');
      assert.match(d.text, text);
      for (const t of titles) assert.match(d.title, t);
      assert.equal(g.nodes[1].status, served, 'the served snapshot is not modified');
    });
  }
}

test('an unknown result keeps the original request and shows its last observation, halted or not', () => {
  for (const runStatus of ['waiting', 'running']) {
    const g = call({ status: 'unknown', updated_at: T2, gateway: { send_count: 1, query_count: 0, first_send: sendErr, last_exchange: sendErr, last_error: sendErr } }, 'unknown');
    g.run.status = runStatus;
    g.nodes[1].activity = 'Gateway 未回执，查询原任务';
    assert.equal(stoppedView(g).nodes[1].status, 'unknown', 'Go serves unknown; the view keeps it');
    const d = drawn(g);
    assert.equal(d.status, 'unknown');
    assert.equal(d.tone, 'bad');
    assert.match(d.text, /^结果未知 · 最后观察 \d{2}:\d{2}:\d{2}$/);
    assert.match(d.title, /保留原请求（q1）/);
    assert.match(d.title, /不会自动重发/);
    assert.match(d.title, /最近记录：Gateway 未回执，查询原任务/);
  }
});

test('terminal steps are drawn as served whatever their call record proves', () => {
  for (const served of ['done', 'reused', 'failed', 'skipped']) {
    for (const [, remote] of tiers) {
      const g = call(remote, served);
      const v = stoppedView(g);
      assert.equal(v.nodes[1].status, served);
      const d = drawn(g);
      if (!d) { assert.equal(served, 'skipped', 'only a skipped step has no card'); continue; }
      assert.equal(d.status, served);
      assert.ok(!/发送待确认|远端在途|结果未知/.test(d.text ?? ''), `${served}: ${d.text}`);
    }
  }
  for (const runStatus of ['done', 'queued', 'running']) {
    const g = call({}, 'running');
    g.run.status = runStatus;
    assert.equal(stoppedView(g), g, `a ${runStatus} task is drawn as served`);
  }
});

test('the same snapshot fetched twice is drawn the same, and drawing is idempotent', () => {
  for (const [, remote] of tiers) {
    const raw = JSON.stringify(call(remote));
    const first = layout.build(stoppedView(JSON.parse(raw)), new layout.Delays());
    const again = layout.build(stoppedView(JSON.parse(raw)), new layout.Delays());
    const pick = (b) => b.nodes.map((n) => [n.id, n.data.status, n.data.line?.text, n.data.line?.title]);
    assert.deepEqual(pick(again), pick(first));
    const once = stoppedView(JSON.parse(raw));
    assert.deepEqual(stoppedView(once), once);
  }
});
