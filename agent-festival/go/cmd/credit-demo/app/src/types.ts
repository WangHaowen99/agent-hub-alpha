// The view model served by /api/graph (graph.go), shared by the MCP App,
// the confirm and run pages.

export interface Branch {
  goal: string; status: string; title: string; relation?: string; activity?: string; finding?: string; tone?: string; retried?: boolean;
}
export interface RunIf { step: string; contains: string; negate?: boolean }
export type { BusSummary, CapabilityBinding, ExecutionBinding, GatewayExchange, RemoteExecution } from "./flow/bus-detail";
import type { BusSummary, ExecutionBinding, RemoteExecution } from "./flow/bus-detail";
export interface GraphNode {
  id: string; name: string; kind: string; role?: string; task: string; deps?: string[]; when?: RunIf;
  effect?: string; reads?: string[]; contract?: string; denied?: string; status?: string; activity?: string; output?: string; branches?: Branch[];
  // Set when the node runs through Agent Bus: the capability it is bound to and what its call recorded.
  execution?: ExecutionBinding; remote?: RemoteExecution;
}
export interface Check { name: string; ok: boolean; detail: string; items?: string[] }
export interface Graph {
  plan: string; version: number; case: string; goal: string; layers: string[][]; nodes: GraphNode[];
  ok: boolean; problems?: string[]; hash: string; approve?: string; confirm?: string; superseded?: number;
  run?: { id: string; status: string; decision?: string; receipt?: string; leases: number; stalled?: boolean; url: string };
  verify?: { ok: boolean; summary: string; checks: Check[] };
  live?: string;
  bus?: BusSummary;
  rev?: number; // state revision the snapshot was read at (see order.ts)
}
export interface Options {
  // Approve in the card: resolves to an error message, or undefined when approved.
  approve?: (g: Graph, user: string, password: string) => Promise<string | undefined>;
  onApproved?: () => void;
  page?: boolean; // embedded in the confirm page, which has its own approve button
}

export const finished = (s?: string) => s === "done" || s === "reused";
// Once a run exists, an expert appears when it starts. A remote node may be
// waiting for its result, or have one that still has to be checked (unknown).
export const started = (s?: string) => s === "running" || s === "waiting" || s === "unknown" || s === "failed" || finished(s);

export type Tone = "ok" | "bad" | "mute";

export function decisionLabel(level?: string): [Tone, string] | undefined {
  if (!level) return undefined;
  const m = /^grant-(\d+)$/.exec(level);
  if (m) return ["ok", `同意 ${Number(m[1]).toLocaleString("en-US")}`];
  if (level === "deny") return ["bad", "拒绝"];
  if (level === "on_missing_evidence") return ["mute", "证据不足"];
  if (level === "on_undefined_rule") return ["mute", "规则未覆盖"];
  return ["mute", level];
}

export function receiptLabel(r?: string): [Tone, string] | undefined {
  switch (r) {
    case "approved": return ["ok", "银行已批准"];
    case "declined": return ["bad", "银行拒绝"];
    case "cancelled": return ["mute", "已撤销"];
    case "unknown": return ["mute", "结果未知 · 查询中"];
  }
  return undefined;
}

export function condLabel(w: RunIf) {
  if (!w.negate && /GRANT/.test(w.contains)) return "同意时";
  return (w.negate ? "不含 " : "含 ") + w.contains;
}
