// Turns a graph snapshot into React Flow nodes and edges laid out with dagre,
// the same way WorkSwarm's SwarmFlow graph does. Node heights are fixed by the
// stylesheet (see the matching numbers in style.css), so the layout is known
// before anything is drawn and never has to be measured and redone.
import dagre from "@dagrejs/dagre";
import type { Edge, Node } from "@xyflow/react";
import { condLabel, decisionLabel, finished, receiptLabel, started, type Branch, type Graph, type GraphNode, type Tone } from "../types";

export const NODE_W = 212;
export const BRANCH_W = 196;
const nodeH = (rows: number) => 74 + rows * 22; // padding 18 + border 2 + head 18 + gap 4 + two task lines 32
const BRANCH_H = 50, BRANCH_GAP = 8, MAX_BRANCHES = 5, SLOTS = 3; // room kept for forks before they come
const LF_TOP = 30, LF_PAD = 10, LF_GAP = 44, LF_MORE = 18;

// The one status line under the task: what a running step is doing, a
// judgment or receipt, how a step ended, or (in a plan) the closed black box.
export type Line = { kind: "act" | "res" | "note" | "box"; text: string; tone?: Tone };
export interface ExpertData extends Record<string, unknown> {
  n: GraphNode; status: string; denied: boolean; line?: Line; delay: number;
  host?: boolean; // the black box inside its Loop Forest group
}
export interface GroupData extends Record<string, unknown> { n: GraphNode; forks: number; more: number; delay: number }
export interface BranchData extends Record<string, unknown> { b: Branch; parent: string; delay: number }
export type EdgeState = "idle" | "active" | "done" | "skip" | "bad" | "lf" | "lf-active" | "lf-off";
export interface EdgeData extends Record<string, unknown> { state: EdgeState; label?: string; labelOn?: boolean; labelOff?: boolean; delay: number }

export interface Built { nodes: Node[]; edges: Edge<EdgeData>[]; width: number; height: number; key: string }

// Enter delays are fixed the first time an element appears, so later
// snapshots never restart its entrance.
export class Delays {
  private readonly seen = new Map<string, number>();
  first = true;
  get(id: string, compute: () => number) {
    let d = this.seen.get(id);
    if (d === undefined) { d = compute(); this.seen.set(id, d); }
    return d;
  }
}

const hostId = (id: string) => `${id}/host`;
const branchId = (id: string, i: number) => `${id}/b${i}`;

export function build(g: Graph, delays: Delays): Built {
  const shown = g.run ? g.nodes.filter((n) => started(n.status)) : g.nodes;
  const plan = new Map(g.nodes.map((n) => [n.id, n]));
  const byId = new Map(shown.map((n) => [n.id, n]));
  const order = new Map(g.nodes.map((n, i) => [n.id, i]));
  const layerOf = new Map<string, number>();
  g.layers.forEach((l, i) => l.forEach((id) => layerOf.set(id, i)));

  // Draw only direct needs: an edge implied through another dependency is noise.
  const up = new Map<string, Set<string>>();
  const ancestors = (id: string): Set<string> => {
    let s = up.get(id);
    if (s) return s;
    s = new Set();
    up.set(id, s);
    for (const d of plan.get(id)?.deps ?? []) { s.add(d); ancestors(d).forEach((a) => s!.add(a)); }
    return s;
  };
  const linksAmong = (nodes: GraphNode[]) => {
    const ids = new Set(nodes.map((n) => n.id));
    const out: [string, GraphNode][] = [];
    for (const n of nodes) {
      const deps = (n.deps ?? []).filter((d) => ids.has(d));
      for (const d of deps) {
        if (n.when?.step !== d && deps.some((o) => o !== d && ancestors(o).has(d))) continue;
        out.push([d, n]);
      }
    }
    return out;
  };
  const links = linksAmong(shown);

  // Sizes: a black box that has forked becomes a Loop Forest group.
  const size = new Map<string, { w: number; h: number }>();
  const groups = new Map<string, { branches: Branch[]; more: number; inner: number; stack: number; hostH: number }>();
  // During a run every card carries its status line, so cards keep their
  // height as steps start and finish; the black box is a Loop Forest group
  // from the start, with room for its forks.
  const rowsOf = (n: GraphNode) => (g.run || n.kind === "blackbox" ? 1 : 0);
  const stackH = (k: number) => k * BRANCH_H + (k - 1) * BRANCH_GAP;
  for (const n of g.nodes) {
    const branches = (n.branches ?? []).slice(0, MAX_BRANCHES);
    if (g.run && n.kind === "blackbox") {
      const hostH = nodeH(rowsOf(n));
      const stack = stackH(Math.max(branches.length, 1));
      const more = (n.branches?.length ?? 0) - branches.length;
      const inner = Math.max(hostH, stackH(Math.max(branches.length, SLOTS)));
      groups.set(n.id, { branches, more, inner, stack, hostH });
      size.set(n.id, { w: LF_PAD + NODE_W + LF_GAP + BRANCH_W + LF_PAD, h: LF_TOP + inner + LF_PAD + (more > 0 ? LF_MORE : 0) });
    } else {
      size.set(n.id, { w: NODE_W, h: nodeH(rowsOf(n)) });
    }
  }

  const dg = new dagre.graphlib.Graph();
  dg.setDefaultEdgeLabel(() => ({}));
  dg.setGraph({ rankdir: "LR", nodesep: 16, ranksep: 64, marginx: 0, marginy: 0 });
  // Lay out the whole plan even while a run shows only the experts that have
  // started: each appears in its final place and the card keeps its size.
  for (const n of g.nodes) dg.setNode(n.id, { width: size.get(n.id)!.w, height: size.get(n.id)!.h });
  for (const [d, n] of linksAmong(g.nodes)) dg.setEdge(d, n.id);
  dagre.layout(dg);

  // dagre reorders a rank to cut crossings; keep the plan's order instead so
  // nodes do not swap places between snapshots, stacked around the rank center.
  const pos = new Map<string, { x: number; y: number }>();
  // A rank is the nodes dagre centered on the same x; wider ones (a Loop
  // Forest group) stay centered on it.
  const ranks = new Map<number, string[]>();
  for (const n of g.nodes) {
    const cx = Math.round(dg.node(n.id).x);
    (ranks.get(cx) ?? ranks.set(cx, []).get(cx)!).push(n.id);
  }
  let height = 0, width = 0;
  const heights: number[] = [];
  for (const ids of ranks.values()) heights.push(ids.reduce((s, id) => s + size.get(id)!.h, 0) + (ids.length - 1) * 16);
  const tallest = Math.max(...heights, 0);
  let left = Infinity;
  for (const [cx, ids] of ranks) for (const id of ids) left = Math.min(left, cx - size.get(id)!.w / 2);
  for (const [cx, ids] of ranks) {
    ids.sort((a, b) => order.get(a)! - order.get(b)!);
    const total = ids.reduce((s, id) => s + size.get(id)!.h, 0) + (ids.length - 1) * 16;
    let y = Math.round((tallest - total) / 2);
    for (const id of ids) {
      const x = Math.round(cx - size.get(id)!.w / 2 - left);
      pos.set(id, { x, y });
      y += size.get(id)!.h + 16;
      width = Math.max(width, x + size.get(id)!.w);
      height = Math.max(height, pos.get(id)!.y + size.get(id)!.h);
    }
  }

  const first = delays.first;
  const enter = (id: string, layer: number, i: number, hasUp: boolean) =>
    delays.get(id, () => (first ? layer * 110 + i * 30 : hasUp ? 260 : 0));

  const nodes: Node[] = [];
  shown.forEach((n, i) => {
    const p = pos.get(n.id)!, s = size.get(n.id)!;
    const hasUp = links.some(([, t]) => t.id === n.id);
    const delay = enter(n.id, layerOf.get(n.id) ?? 0, i, hasUp);
    const grp = groups.get(n.id);
    if (!grp) {
      nodes.push({ id: n.id, type: "expert", position: p, width: s.w, height: s.h, data: expertData(g, n, delay) });
      return;
    }
    nodes.push({ id: n.id, type: "lf", position: p, width: s.w, height: s.h, data: { n, forks: n.branches?.length ?? 0, more: grp.more, delay } satisfies GroupData });
    nodes.push({
      id: hostId(n.id), type: "expert", parentId: n.id, extent: "parent", position: { x: LF_PAD, y: LF_TOP + (grp.inner - grp.hostH) / 2 },
      width: NODE_W, height: grp.hostH, data: { ...expertData(g, n, 0), host: true },
    });
    const top = LF_TOP + (grp.inner - grp.stack) / 2;
    grp.branches.forEach((b, k) => {
      const id = branchId(n.id, k);
      nodes.push({
        id, type: "branch", parentId: n.id, extent: "parent",
        position: { x: LF_PAD + NODE_W + LF_GAP, y: top + k * (BRANCH_H + BRANCH_GAP) },
        width: BRANCH_W, height: BRANCH_H,
        data: { b, parent: n.id, delay: delays.get(id, () => 120 + k * 140) } satisfies BranchData,
      });
    });
  });

  const edges: Edge<EdgeData>[] = [];
  for (const [d, n] of links) {
    const src = byId.get(d)!;
    let state: EdgeState = "idle";
    if (g.run) {
      if (n.status === "skipped") state = "skip";
      else if (finished(src.status) && n.status === "running") state = "active";
      else if (finished(src.status) && finished(n.status)) state = "done";
    } else if (!g.ok && n.denied) state = "bad";
    const id = `${d}>${n.id}`;
    const cond = n.when && n.when.step === d ? n.when : undefined;
    const decided = g.run?.decision;
    edges.push({
      id, source: d, target: n.id, type: "flow",
      data: {
        state, delay: delays.get(id, () => (first ? 200 + (layerOf.get(d) ?? 0) * 110 : 0)),
        label: cond && condLabel(cond), labelOn: !!cond && !!decided && /^grant/.test(decided) && !cond.negate, labelOff: n.status === "skipped",
      },
    });
  }
  for (const [id, grp] of groups) {
    grp.branches.forEach((b, k) => {
      const eid = `${id}/e${k}`;
      edges.push({
        id: eid, source: hostId(id), target: branchId(id, k), type: "flow",
        data: { state: b.status === "started" ? "lf-active" : b.status === "interrupted" ? "lf-off" : "lf", delay: delays.get(eid, () => k * 140) },
      });
    });
  }
  delays.first = false;
  return { nodes, edges, width, height, key: JSON.stringify([width, height, [...pos].map(([id, p]) => [id, p.x, p.y])]) };
}

function resultOf(g: Graph, n: GraphNode): [Tone, string] | undefined {
  if (n.kind === "judge") return decisionLabel(g.run?.decision);
  if (n.kind === "effect") return receiptLabel(g.run?.receipt);
  return undefined;
}

const ended: Record<string, Line> = {
  done: { kind: "note", text: "已完成", tone: "ok" },
  reused: { kind: "note", text: "复用账本里的结果" },
  failed: { kind: "note", text: "执行失败", tone: "bad" },
  skipped: { kind: "note", text: "条件不满足，未执行", tone: "mute" },
};

// The first sentence of what a step produced, without the step tag and markdown.
function gist(output?: string): string {
  const text = (output ?? "").replace(/^\[step:[^\]]*\]\s*/, "");
  for (const raw of text.split("\n")) {
    const line = raw.replace(/\*\*/g, "").replace(/^\s*[-*#>]+\s*/, "").replace(/\s+/g, " ").trim();
    if (line) return line;
  }
  return "";
}

function lineOf(g: Graph, n: GraphNode, status: string): Line | undefined {
  if (!g.run) return n.kind === "blackbox" ? { kind: "box", text: "运行时展开" } : undefined;
  if (status === "running") return { kind: "act", text: n.activity || "执行中" };
  const res = finished(status) ? resultOf(g, n) : undefined;
  if (res) return { kind: "res", text: res[1], tone: res[0] };
  const said = finished(status) ? gist(n.output) : "";
  return said ? { kind: "note", text: said } : ended[status];
}

function expertData(g: Graph, n: GraphNode, delay: number): ExpertData {
  const status = n.status ?? "plan";
  return { n, status, delay, denied: !g.ok && !g.run && !!n.denied, line: lineOf(g, n, status) };
}
