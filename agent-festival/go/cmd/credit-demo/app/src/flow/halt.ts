// A task that is waiting for a check, has failed or is blocked no longer
// advances, but the store can still list a step as running: a local step
// (the Loop Forest black box) whose thread was not stopped with the task, or
// a fork it had started. Drawn as running it would look like work in
// progress, so the view shows such steps as stopped, with the latest time
// this snapshot records.
//
// An Agent Bus step of a halted task is labelled by what its call record
// proves (callEvidence), never by the mere existence of the record:
//   "unconfirmed" 已登记，发送待确认 — the call was registered, nothing shows
//                 the expert side has it;
//   "inflight"    远端在途 — the expert side has the request and may still run;
//   "unknown"     结果未知 — served by Go as is; the original request is kept.
// None of them is animated. Display only: the service's state is not changed
// and drawing sends nothing.
import { exchangeDescription, type RemoteExecution } from "./bus-detail";
import type { Graph, GraphNode } from "../types";

const HALTED = new Set(["waiting", "failed", "blocked"]);
export const halted = (g: Graph) => !!g.run && HALTED.has(g.run.status);

// What one Agent Bus call record proves. Only fields projected by remoteView
// (graph_bus.go) and gatewayView (bus_observation.go) are read:
//   "receipt" the expert side has the request:
//             gateway.first_receipt — a SendMessage the Java host observed
//               returning a task (outcome task_received; Go accepts it only
//               with HTTP 200, a remote task id and the call's context);
//             task_id — the remote task id Go stored from such a reply;
//             agent_started_at — the expert runtime fetched this call's
//               context from Go (AgentContext), i.e. it started on it.
//   "send"    gateway.first_send but no receipt: a SendMessage was observed and
//             ended without a task (HTTP, protocol or transport error, or an
//             invalid reply). Delivery to the expert is not proven.
//   "intent"  neither: only the record Go saved before dispatch
//             (intent_saved_at).
export type CallEvidence = "receipt" | "send" | "intent";
export function callEvidence(r: RemoteExecution): CallEvidence {
  if (r.gateway?.first_receipt || r.task_id || r.agent_started_at) return "receipt";
  return r.gateway?.first_send ? "send" : "intent";
}

// The latest time the snapshot records for this task. Only Agent Bus calls
// carry timestamps (call updates, commits, Gateway exchanges, discovery);
// local steps have none of their own.
export function lastObserved(g: Graph): string | undefined {
  let best: string | undefined, bestMs = -Infinity;
  const consider = (value?: string) => {
    const ms = value ? Date.parse(value) : NaN;
    if (!Number.isNaN(ms) && ms > bestMs) { bestMs = ms; best = value; }
  };
  for (const n of g.nodes) {
    const r = n.remote;
    if (!r) continue;
    consider(r.updated_at); consider(r.committed_at); consider(r.agent_started_at); consider(r.intent_saved_at);
    consider(r.gateway?.last_exchange?.finished_at); consider(r.gateway?.last_error?.finished_at); consider(r.discovery?.checked_at);
  }
  return best;
}

// The latest time recorded for one Agent Bus step's call.
export function lastObservedStep(n: GraphNode): string | undefined {
  return lastObserved({ nodes: [n] } as Graph);
}

const clock = (value: string) => new Date(value).toLocaleTimeString("zh-CN", { hour12: false });
const fullTime = (value: string) => new Date(value).toLocaleString("zh-CN", { hour12: false });
const stepWhen = (at?: string) => at ? `最后观察 ${fullTime(at)}：本步骤远端调用最近一次有记录的时间（浏览器时区）。` : "本步骤远端调用没有带时间的记录。";
const recent = (n: GraphNode) => n.activity ? `最近记录：${n.activity}。` : "";

// The status line of a stopped step and the hover text that explains it.
export function stoppedLine(g: Graph, activity?: string): { text: string; title: string } {
  const at = lastObserved(g);
  const text = at ? `已停止 · 最后观察 ${clock(at)}` : "已停止";
  const when = at ? `最后观察 ${fullTime(at)}：本任务最近一次有记录的远端调用时间（浏览器时区）。` : "本任务没有带时间的远端调用记录。";
  return { text, title: `任务已停止，本步骤不再推进。${activity ? `停止前记录：${activity}。` : ""}${when}` };
}

// "unconfirmed": registered, and maybe sent, but nothing shows it arrived.
export function unconfirmedLine(n: GraphNode): { text: string; title: string } {
  const r = n.remote, gw = r?.gateway;
  const registered = r?.intent_saved_at ? `已登记派发意图（${fullTime(r.intent_saved_at)}）` : "已登记派发意图";
  const sent = gw?.first_send
    ? `Gateway 记录了 ${gw.send_count} 次发送，最近一次：${exchangeDescription(gw.last_exchange)}`
    : "没有 Gateway 发送记录";
  return { text: "已登记，发送待确认",
    title: `任务已暂停派发。本步骤${registered}；${sent}；没有对端接收记录，不能确认请求已到达专家端。${recent(n)}${stepWhen(lastObservedStep(n))}` };
}

// "inflight": the expert side has the request; the time is this step's own
// latest recorded call time.
export function inflightLine(n: GraphNode): { text: string; title: string } {
  const r = n.remote, at = lastObservedStep(n);
  const proof = [r?.gateway?.first_receipt && "Gateway 任务回执", r?.task_id && "远端任务号", r?.agent_started_at && "专家已开始执行"].filter(Boolean).join("、");
  const text = at ? `远端在途 · 最后观察 ${clock(at)}` : "远端在途";
  return { text, title: `任务已暂停派发；本步骤的请求已有对端接收记录${proof ? `（${proof}）` : ""}，远端任务可能仍在执行，恢复后按原任务查询结果。${recent(n)}${stepWhen(at)}` };
}

// "unknown" (served by Go): the result is unconfirmed; the original request is
// kept and only that task is queried, nothing is sent again.
export function unknownLine(n: GraphNode): { text: string; title: string } {
  const r = n.remote, at = lastObservedStep(n);
  const text = at ? `结果未知 · 最后观察 ${clock(at)}` : "结果未知";
  return { text, title: `本步骤远端调用的结果未确认。保留原请求${r?.request_id ? `（${r.request_id}）` : ""}，不会自动重发，只按原请求查询原任务。${recent(n)}${stepWhen(at)}` };
}

// The snapshot as drawn: in a halted task, an Agent Bus step whose call is
// running or waiting becomes "inflight" or "unconfirmed" by its evidence;
// other running steps and started forks become "stopped"; anything else is
// returned as is.
export function stoppedView(g: Graph): Graph {
  if (!halted(g)) return g;
  let changed = false;
  const nodes = g.nodes.map((n) => {
    const forks = n.branches?.some((b) => b.status === "started");
    const out = !!n.remote && (n.status === "running" || n.status === "waiting");
    if (n.status !== "running" && !forks && !out) return n;
    changed = true;
    return {
      ...n,
      status: out ? (callEvidence(n.remote!) === "receipt" ? "inflight" : "unconfirmed") : n.status === "running" ? "stopped" : n.status,
      branches: forks ? n.branches!.map((b) => (b.status === "started" ? { ...b, status: "stopped" } : b)) : n.branches,
    };
  });
  return changed ? { ...g, nodes } : g;
}
