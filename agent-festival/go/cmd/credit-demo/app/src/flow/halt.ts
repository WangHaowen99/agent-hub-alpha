// A task that is waiting for a check, has failed or is blocked no longer
// advances, but the store can still list a step as running: a local step
// (the Loop Forest black box) whose thread was not stopped with the task, or
// a fork it had started. Drawn as running it would look like work in
// progress, so the view shows such steps as stopped, with the latest time
// this snapshot records. Display only: the service's state is not changed.
import type { Graph } from "../types";

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

const clock = (value: string) => new Date(value).toLocaleTimeString("zh-CN", { hour12: false });

// The status line of a stopped step and the hover text that explains it.
export function stoppedLine(g: Graph, activity?: string): { text: string; title: string } {
  const at = lastObserved(g);
  const text = at ? `已停止 · 最后观察 ${clock(at)}` : "已停止";
  const when = at ? `最后观察 ${new Date(at).toLocaleString("zh-CN", { hour12: false })}：本任务最近一次有记录的远端调用时间（浏览器时区）。` : "本任务没有带时间的远端调用记录。";
  return { text, title: `任务已停止，本步骤不再推进。${activity ? `停止前记录：${activity}。` : ""}${when}` };
}

// The snapshot as drawn: running steps and started forks of a halted task
// become "stopped"; anything else is returned as is.
export function stoppedView(g: Graph): Graph {
  if (!halted(g)) return g;
  let changed = false;
  const nodes = g.nodes.map((n) => {
    const forks = n.branches?.some((b) => b.status === "started");
    if (n.status !== "running" && !forks) return n;
    changed = true;
    return {
      ...n,
      status: n.status === "running" ? "stopped" : n.status,
      branches: forks ? n.branches!.map((b) => (b.status === "started" ? { ...b, status: "stopped" } : b)) : n.branches,
    };
  });
  return changed ? { ...g, nodes } : g;
}
