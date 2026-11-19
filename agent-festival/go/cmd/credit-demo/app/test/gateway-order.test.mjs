// Exercise the real RPC client and revision gate with a controlled transport.
// Unlike the 25s deployment experiment, the first case keeps BOTH RPCs alive.
import { build } from 'esbuild';
import { test } from 'node:test';
import assert from 'node:assert/strict';
const bundle = await build({ stdin: { contents: 'export { Gateway } from "./src/workbench/gateway"; export { newer } from "./src/order";', resolveDir: process.cwd() }, bundle: true, write: false, format: 'esm', platform: 'node' });
const { Gateway, newer } = await import('data:text/javascript;base64,' + Buffer.from(bundle.outputFiles[0].text).toString('base64'));

function transport(t) {
  const sent = [], timers = new Map();
  let socket, seq = 0;
  class Socket {
    static OPEN = 1;
    readyState = 1;
    constructor() { socket = this; queueMicrotask(() => this.onopen()); }
    send(data) { sent.push(JSON.parse(data)); }
  }
  for (const [key, value] of Object.entries({
    WebSocket: Socket, location: { protocol: 'http:', host: 'fixture' },
    localStorage: { getItem: () => null },
    window: { setTimeout: (fn) => { timers.set(++seq, fn); return seq; } },
    clearTimeout: (id) => timers.delete(id),
  })) {
    const descriptor = Object.getOwnPropertyDescriptor(globalThis, key);
    Object.defineProperty(globalThis, key, { value, configurable: true, writable: true });
    t.after(() => descriptor ? Object.defineProperty(globalThis, key, descriptor) : delete globalThis[key]);
  }
  return { sent, timers, reply: (id, rev) => socket.onmessage({ data: JSON.stringify({ type: 'res', id, ok: true, payload: { rev, nodes: [] } }) }) };
}

test('two live RPCs resolve out of order; revision gate rejects the older graph', async (t) => {
  const wire = transport(t), gw = new Gateway();
  let shown, deliveries = 0;
  const accept = (g) => { deliveries++; if (newer(shown, g)) shown = g; };
  const older = gw.call('plugin.credit_workbench.graph').then(accept);
  const latest = gw.call('plugin.credit_workbench.graph').then(accept);
  await new Promise(setImmediate);
  assert.equal(wire.timers.size, 2); // neither request has expired
  wire.reply(wire.sent[1].id, 296);
  await latest;
  assert.equal(wire.timers.size, 1); // older request still active
  wire.reply(wire.sent[0].id, 247);
  await older;
  assert.equal(deliveries, 2); // both reached the revision gate
  assert.equal(shown.rev, 296);
});

test('expired RPC response is discarded before it can reach the revision gate', async (t) => {
  const wire = transport(t), gw = new Gateway();
  let deliveries = 0;
  const pending = gw.call('plugin.credit_workbench.graph').then(() => deliveries++);
  const rejected = assert.rejects(pending, { code: 'TIMEOUT' });
  await new Promise(setImmediate);
  wire.timers.values().next().value();
  await rejected;
  wire.reply(wire.sent[0].id, 247);
  assert.equal(deliveries, 0);
});
