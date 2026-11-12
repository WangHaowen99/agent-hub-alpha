// A12: duplicated and out-of-order snapshots must not take the view back.
import { build } from 'esbuild';
import { test } from 'node:test';
import assert from 'node:assert/strict';
const bundle = await build({ entryPoints: ['src/order.ts'], bundle: true, write: false, format: 'esm', platform: 'node' });
const { newer } = await import('data:text/javascript;base64,' + Buffer.from(bundle.outputFiles[0].text).toString('base64'));

const rank = { pending: 0, running: 1, waiting: 1, unknown: 2, done: 3, reused: 3, failed: 3, skipped: 3 };
const snap = (rev, statuses) => ({ rev, nodes: Object.entries(statuses).map(([id, status]) => ({ id, status })) });
// What the screen would show after a stream of replies, applying the same rule as the workbench.
const replay = (replies) => replies.reduce((shown, next) => (newer(shown, next) ? next : shown), undefined);

test('an older snapshot is dropped, an equal or newer one is taken', () => {
  assert.equal(newer(snap(5, {}), snap(4, {})), false);
  assert.equal(newer(snap(5, {}), snap(5, {})), true);
  assert.equal(newer(snap(5, {}), snap(6, {})), true);
  assert.equal(newer(undefined, snap(1, {})), true);
});

test('snapshots without a revision (an older service) are still shown', () => {
  assert.equal(newer({ nodes: [] }, snap(3, {})), true);
  assert.equal(newer(snap(3, {}), { nodes: [] }), true);
});

test('shuffled and duplicated replies never move a step backwards', () => {
  const run = [
    snap(10, { intake: 'running', judge: 'pending' }),
    snap(11, { intake: 'done', judge: 'pending' }),
    snap(12, { intake: 'done', judge: 'running' }),
    snap(13, { intake: 'done', judge: 'done' }),
  ];
  const replies = [run[0], run[2], run[1], run[1], run[3], run[0], run[2], run[3]];
  let shown;
  for (const reply of replies) {
    const before = shown;
    if (newer(shown, reply)) shown = reply;
    if (before) for (const n of shown.nodes) {
      const was = before.nodes.find((x) => x.id === n.id);
      assert.ok(rank[n.status] >= rank[was.status], `${n.id} went ${was.status} -> ${n.status}`);
    }
  }
  assert.deepEqual(replay(replies), run[3]);
});
