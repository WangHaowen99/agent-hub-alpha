// A task that is waiting for a check, has failed or is blocked no longer
// advances, but the store can still list a step as running: a local step
// (the Loop Forest black box) whose thread was not stopped with the task, or
// a fork it had started. Drawn as running it would look like work in
// progress, so the view shows such steps as stopped, with the latest time
// this snapshot records. An Agent Bus step whose call was already sent is
// different: dispatch has paused, but the remote task may still be running.
// It is drawn as "in flight" with its own last observation, never animated.
// Display only: the service's state is not changed.
import type { Graph, GraphNode } from "../types";

const HALTED = new Set(["waiting", "failed", "blocked"]);
export const halted = (g: Graph) => !!g.run && HALTED.has(g.run.status);

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

// The status line of a stopped step and the hover text that explains it.
export function stoppedLine(g: Graph, activity?: string): { text: string; title: string } {
  const at = lastObserved(g);
  const text = at ? `已停止 · 最后观察 ${clock(at)}` : "已停止";
  const when = at ? `最后观察 ${fullTime(at)}：本任务最近一次有记录的远端调用时间（浏览器时区）。` : "本任务没有带时间的远端调用记录。";
  return { text, title: `任务已停止，本步骤不再推进。${activity ? `停止前记录：${activity}。` : ""}${when}` };
}

// The status line of an Agent Bus step whose call is still out when the task
// stopped: the time is this step's own latest recorded call time.
export function inflightLine(n: GraphNode): { text: string; title: string } {
  const at = lastObservedStep(n);
  const text = at ? `远端在途 · 最后观察 ${clock(at)}` : "远端在途";
  const when = at ? `最后观察 ${fullTime(at)}：本步骤远端调用最近一次有记录的时间（浏览器时区）。` : "本步骤远端调用没有带时间的记录。";
  return { text, title: `任务已暂停派发；本步骤的远端调用已发出，远端任务可能仍在执行，恢复后按原任务查询结果。${n.activity ? `最近记录：${n.activity}。` : ""}${when}` };
}

// An Agent Bus step with a recorded call (the call was at least begun).
const called = (n: GraphNode) => !!n.remote;

// The snapshot as drawn: in a halted task, an Agent Bus step whose call is
// running or waiting becomes "inflight"; other running steps and started
// forks become "stopped"; anything else is returned as is.
export function stoppedView(g: Graph): Graph {
  if (!halted(g)) return g;
  let changed = false;
  const nodes = g.nodes.map((n) => {
    const forks = n.branches?.some((b) => b.status === "started");
    const inflight = called(n) && (n.status === "running" || n.status === "waiting");
    if (n.status !== "running" && !forks && !inflight) return n;
    changed = true;
    return {
      ...n,
      status: inflight ? "inflight" : n.status === "running" ? "stopped" : n.status,
      branches: forks ? n.branches!.map((b) => (b.status === "started" ? { ...b, status: "stopped" } : b)) : n.branches,
    };
  });
  return changed ? { ...g, nodes } : g;
}
