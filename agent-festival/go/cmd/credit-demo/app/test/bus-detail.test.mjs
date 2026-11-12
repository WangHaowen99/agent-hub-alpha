import { build } from 'esbuild';
import { test } from 'node:test';
import assert from 'node:assert/strict';
const bundle = await build({ entryPoints: ['src/flow/bus-detail.ts'], bundle: true, write: false, format: 'esm', platform: 'node' });
const { executionDetailHTML: detail, busOverviewHTML: overview, associationText, exchangeDescription } = await import('data:text/javascript;base64,' + Buffer.from(bundle.outputFiles[0].text).toString('base64'));
const node = { id: 'intake', name: '资料核验', task: '核对资料', kind: 'investigate', execution: { kind: 'agent_bus', path_mode: 'DIRECT', binding: { agent_id: 'credit-material', version: '1.0.1', card_digest: 'card', contract_digest: 'contract' } } };
test('planned binding does not claim a dispatch or an executing instance', () => {
  const html = detail(node);
  assert.match(html, /尚未派发/);
  assert.doesNotMatch(detail({ ...node, status: 'failed' }), /调用结束 · 失败/);
  assert.match(html, /派发前将再次查询 RDC/);
  assert.doesNotMatch(html, /已收到任务回执|执行已完成|当时的候选实例/);
  assert.match(html, /实际执行实例<\/dt><dd>未采集/);
});
test('historical completed call keeps missing discovery and Gateway timings explicit', () => {
  const html = detail({ ...node, status: 'done', remote: { task_id: 'original', artifact_id: 'saved', model_calls: 1 } });
  assert.match(html, /发现快照未采集/);
  assert.match(html, /Gateway 请求与回执时间未采集/);
  assert.match(html, /已有任务关联/);
  assert.doesNotMatch(html, /已收到任务回执|派发前已核对/);
});
test('HTTP 200 protocol error stays unresolved, not a successful receipt', () => {
  const error = { method: 'GetTask', http_status: 200, outcome: 'protocol_error', error_code: 'TASK_NOT_FOUND' };
  assert.match(exchangeDescription(error), /HTTP 200 · 协议错误 · TASK_NOT_FOUND/);
  const html = detail({ ...node, status: 'unknown', remote: { task_id: 'old', gateway: { last_exchange: error, last_error: error, send_count: 0, query_count: 1 } } });
  assert.match(html, /结果待核对/);
  assert.match(html, /原始派发观测未采集/);
  assert.doesNotMatch(html, /已收到任务回执|已持久保存/);
});
test('reuse labels original records and all external strings are escaped', () => {
  const html = detail({ ...node, name: '<img src=x onerror=alert(1)>', status: 'reused', output: '<script>bad()</script>', remote: { task_id: 'old', artifact_id: 'saved', reused_count: 2, reason: '<unsafe>' } });
  assert.match(html, /保留原调用记录/);
  assert.match(html, /累计复用 2 次/);
  assert.doesNotMatch(html, /<img|<script>|<unsafe>/);
  assert.match(html, /&lt;script&gt;/);
});
test('copy output excludes credentials and overview separates planned and recorded calls', () => {
  const json = associationText({ ...node, execution: { ...node.execution, token: 'private' }, remote: { task_id: 'task', capability_token: 'secret', gateway_exchanges: {} } });
  assert.doesNotMatch(json, /private|secret|capability_token|gateway_exchanges/);
  assert.equal(JSON.parse(json).task_id, 'task');
  const html = overview({ nodes: [node], bus: { planned_nodes: 1, services: 1, linked_tasks: 0, completed: 0, unresolved: 0, reused: 0, failed: 0 } });
  assert.match(html, /待用户批准 · 尚未派发/);
  assert.match(html, /data-inspect="intake"/);
});
