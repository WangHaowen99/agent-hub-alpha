// The plan graph: one renderer for the MCP App in WorkSwarm and for the
// confirm/run pages. It draws the view model served by /api/graph and
// animates what changes between two snapshots. Before approval it shows the
// whole plan; once the run starts it shows only the experts that have started,
// each popping in at the end of the edges that feed it. The black box opens
// into a Loop Forest group whose branches fork out of it one by one.
import { css } from "./style";

export interface Branch {
  goal: string; status: string; title: string; relation?: string; activity?: string; finding?: string; tone?: string; retried?: boolean;
}
export interface RunIf { step: string; contains: string; negate?: boolean }
export interface GraphNode {
  id: string; name: string; kind: string; role?: string; task: string; deps?: string[]; when?: RunIf;
  effect?: string; reads?: string[]; contract?: string; denied?: string; status?: string; activity?: string; output?: string; branches?: Branch[];
}
export interface Check { name: string; ok: boolean; detail: string; items?: string[] }
export interface Graph {
  plan: string; version: number; case: string; goal: string; layers: string[][]; nodes: GraphNode[];
  ok: boolean; problems?: string[]; hash: string; approve?: string; confirm?: string; superseded?: number;
  run?: { id: string; status: string; decision?: string; receipt?: string; leases: number; stalled?: boolean; url: string };
  verify?: { ok: boolean; summary: string; checks: Check[] };
  live?: string;
}
export interface Options {
  // Approve in the card: resolves to an error message, or undefined when approved.
  approve?: (g: Graph, user: string, password: string) => Promise<string | undefined>;
  onApproved?: () => void;
  page?: boolean; // embedded in the confirm page, which has its own approve button
}

const W = 184, GROUP_W = 404, GX = 64, GY = 18, PAD = 22, MAX_BRANCHES = 5, MOVE_MS = 520, LEAVE_MS = 320;
const TICK = `<svg viewBox="0 0 12 12"><path d="M2.5 6.3l2.3 2.3 4.7-5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>`;
const CROSS = `<svg viewBox="0 0 12 12"><path d="M3 3l6 6M9 3l-6 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>`;
const SVGNS = "http://www.w3.org/2000/svg";
const EDGE_STATES = ["idle", "active", "done", "skip", "bad"];
const EDGE_COLORS: Record<string, string> = { idle: "#cfd4dc", active: "#93b4f5", done: "#8fd1a6", skip: "#dcdfe4", bad: "#f2a7a7" };

const esc = (s: string) => s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]!);
const finished = (s?: string) => s === "done" || s === "reused";
// Once a run exists, an expert appears when it starts.
const started = (s?: string) => s === "running" || s === "failed" || finished(s);
// Phase for looping animations in elements that are rebuilt on every change,
// so spinners and flowing dashes carry on instead of jumping back to the start.
const phase = () => `-${Math.round(performance.now() % 7000)}ms`;
const ease = (t: number) => 1 - Math.pow(1 - t, 3);
let instances = 0;

function ensureStyle(doc: Document) {
  if (doc.getElementById("cg-style")) return;
  const st = doc.createElement("style");
  st.id = "cg-style";
  st.textContent = css;
  doc.head.appendChild(st);
}

export function decisionLabel(level?: string): [string, string] | undefined {
  if (!level) return undefined;
  const m = /^grant-(\d+)$/.exec(level);
  if (m) return ["ok", `同意 ${Number(m[1]).toLocaleString("en-US")}`];
  if (level === "deny") return ["bad", "拒绝"];
  if (level === "on_missing_evidence") return ["mute", "证据不足"];
  if (level === "on_undefined_rule") return ["mute", "规则未覆盖"];
  return ["mute", level];
}

function receiptLabel(r?: string): [string, string] | undefined {
  switch (r) {
    case "approved": return ["ok", "银行已批准"];
    case "declined": return ["bad", "银行拒绝"];
    case "cancelled": return ["mute", "已撤销"];
    case "unknown": return ["mute", "结果未知 · 查询中"];
  }
  return undefined;
}

function condLabel(w: RunIf) {
  if (!w.negate && /GRANT/.test(w.contains)) return "同意时";
  return (w.negate ? "不含 " : "含 ") + w.contains;
}

interface NodeView {
  slot: HTMLDivElement; el: HTMLDivElement; sig: string; branches: string[]; wide: boolean;
  x: number; y: number; fx: number; fy: number; tx: number; ty: number; w: number; h: number; placed: boolean;
}
interface EdgeView { from: string; to: string; g: SVGGElement; base: SVGPathElement; flow: SVGPathElement; cond?: HTMLDivElement; drawn: boolean }

export class GraphView {
  private readonly opts: Options;
  private readonly id = `cg${++instances}`;
  private readonly el: HTMLDivElement;
  private readonly head: HTMLDivElement;
  private readonly canvas: HTMLDivElement;
  private readonly stage: HTMLDivElement;
  private readonly svg: SVGSVGElement;
  private readonly foot: HTMLDivElement;
  private shown: GraphNode[] = [];
  private nodes = new Map<string, NodeView>();
  private edges = new Map<string, EdgeView>();
  private g?: Graph;
  private first = true;
  private headSig = "";
  private footSig = "";
  private frame = 0;
  private pop?: HTMLDivElement;
  private gate?: HTMLFormElement;
  private pending = 0;
  // A node changes size when its iframe becomes visible again (WorkSwarm unmounts
  // off-screen messages) or the black box opens: lay out again when that happens.
  private readonly sizes = new ResizeObserver(() => {
    cancelAnimationFrame(this.pending);
    this.pending = requestAnimationFrame(() => {
      if ([...this.nodes.values()].some((v) => v.el.offsetHeight !== v.h)) this.layout(true);
    });
  });

  constructor(root: HTMLElement, opts: Options = {}) {
    this.opts = opts;
    ensureStyle(root.ownerDocument);
    this.el = document.createElement("div");
    this.el.className = "cg";
    this.el.innerHTML = `<div class="cg-head"></div><div class="cg-canvas"><div class="cg-stage"><svg class="cg-edges"><defs>${EDGE_STATES.map((s) =>
      `<marker id="${this.id}-ar-${s}" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M1 1.5L8.5 5L1 8.5" fill="none" stroke="${EDGE_COLORS[s]}" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></marker>`).join("")}</defs></svg></div><div class="cg-wait"><i></i><i></i><i></i></div></div><div class="cg-foot"></div>`;
    root.replaceChildren(this.el);
    this.head = this.el.querySelector(".cg-head")!;
    this.canvas = this.el.querySelector(".cg-canvas")!;
    this.stage = this.el.querySelector(".cg-stage")!;
    this.svg = this.el.querySelector("svg")!;
    this.foot = this.el.querySelector(".cg-foot")!;
    let width = 0;
    new ResizeObserver(() => {
      if (this.g && this.canvas.clientWidth !== width) {
        width = this.canvas.clientWidth;
        this.layout(false);
      }
    }).observe(this.canvas);
    this.el.addEventListener("click", (e) => {
      const slot = (e.target as HTMLElement).closest<HTMLElement>(".cg-slot");
      if (slot && !(e.target as HTMLElement).closest(".cg-pop")) this.togglePop(slot.dataset.id!);
      else if (!(e.target as HTMLElement).closest(".cg-pop")) this.closePop();
    });
    root.ownerDocument.addEventListener("keydown", (e) => { if (e.key === "Escape") this.closePop(); });
  }

  current() { return this.g; }

  render(g: Graph) {
    this.g = g;
    this.el.classList.toggle("m-run", !!g.run);
    this.el.classList.toggle("m-plan", !g.run);
    this.el.classList.toggle("m-stalled", !!g.run?.stalled);
    this.renderHead(g);
    const layerOf = new Map<string, number>();
    g.layers.forEach((l, i) => l.forEach((id) => layerOf.set(id, i)));

    this.shown = g.run ? g.nodes.filter((n) => started(n.status)) : g.nodes;
    const seen = new Set<string>();
    this.shown.forEach((n, i) => {
      seen.add(n.id);
      let v = this.nodes.get(n.id);
      if (!v) {
        const slot = document.createElement("div");
        slot.className = "cg-slot";
        slot.dataset.id = n.id;
        const el = document.createElement("div");
        slot.appendChild(el);
        this.stage.appendChild(slot);
        this.sizes.observe(el);
        // Edges that change the node's right side follow its width as it grows.
        el.addEventListener("transitionend", (e) => { if (e.target === el && e.propertyName === "width") this.drawEdges(); });
        v = { slot, el, sig: "", branches: [], wide: false, x: 0, y: 0, fx: 0, fy: 0, tx: 0, ty: 0, w: W, h: 0, placed: false };
        this.nodes.set(n.id, v);
        // A node that starts mid-run waits for the edges into it to draw first.
        const delay = this.first ? (layerOf.get(n.id) ?? 0) * 110 + i * 25 : (n.deps ?? []).some((d) => this.nodes.has(d)) ? 240 : 0;
        el.style.setProperty("--d", `${delay}ms`);
        el.style.animationDelay = `${delay}ms`;
        el.dataset.enter = "1";
      }
      this.renderNode(v, n, g);
    });
    for (const [id, v] of this.nodes) {
      if (seen.has(id)) continue;
      // Approval hides the plan: experts leave and come back as they start.
      this.sizes.unobserve(v.el);
      this.nodes.delete(id);
      v.el.classList.remove("enter");
      v.el.classList.add("leave");
      setTimeout(() => v.slot.remove(), LEAVE_MS);
    }

    this.renderEdges(g, layerOf);
    this.renderFoot(g);
    this.layout(!this.first);
    for (const v of this.nodes.values()) if (v.wide) this.drawFan(v);
    if (this.pop) this.togglePop(this.pop.dataset.id!, true);
    this.first = false;
  }

  // ---------- header ----------

  private renderHead(g: Graph) {
    const total = g.nodes.length;
    const settled = g.nodes.filter((n) => finished(n.status) || n.status === "skipped").length;
    const reused = g.nodes.filter((n) => n.status === "reused").length;
    let chip: [string, string];
    if (g.verify) {
      const pass = g.verify.checks.filter((c) => c.ok).length;
      chip = g.verify.ok ? ["ok", `验证通过 ${pass}/${g.verify.checks.length}`] : ["bad", `验证未通过 ${pass}/${g.verify.checks.length}`];
    } else if (g.run) {
      chip = ({ queued: ["wait", "排队中"], running: ["run", `执行中 ${settled}/${total}`], done: ["ok", "已完成"], failed: ["bad", "执行失败"] } as Record<string, [string, string]>)[g.run.status] ?? ["mute", g.run.status];
      if (g.run.stalled) chip = ["bad", "执行器无响应 · 等待接管"];
    } else if (!g.ok) chip = ["bad", "预检未通过"];
    else if (g.superseded) chip = ["mute", `已更新为 v${g.superseded}`];
    else chip = ["wait", "待批准"];
    const restart = g.run && g.run.leases > 1 ? `<span class="cg-chip soft">↻ 执行器重启 · 复用 ${reused} 步</span>` : "";
    const canApprove = !!g.approve && !!this.opts.approve && !this.opts.page;
    const approve = canApprove ? `<button class="cg-btn" data-approve>批准</button>` : "";
    const running = g.run?.status === "running" && !g.run.stalled ? g.nodes.filter((n) => n.status === "running").map((n) => n.name) : [];
    const ver = running.length
      ? `<span class="cg-now"><i></i>正在：${esc(running.join("、"))}</span>`
      : `<span class="cg-ver">${esc(g.case)} · ${esc(g.plan)} v${g.version}</span>`;
    const sig = [g.goal, ver, chip.join(), restart, approve].join("|");
    if (sig !== this.headSig) {
      this.headSig = sig;
      this.head.innerHTML = `<div class="cg-title"><span class="cg-goal">${esc(g.goal || `${g.case} 授信尽调`)}</span>${ver}</div>
        <div class="cg-meta">${restart}<span class="cg-chip ${chip[0]}"><i></i>${esc(chip[1])}</span>${approve}</div><div class="cg-bar"><b></b></div>`;
      this.head.querySelector<HTMLButtonElement>("[data-approve]")?.addEventListener("click", (e) => {
        e.stopPropagation();
        this.toggleGate();
      });
    }
    if (!canApprove) this.closeGate();
    const bar = this.head.querySelector<HTMLElement>(".cg-bar")!;
    bar.classList.toggle("ok", g.run?.status === "done");
    requestAnimationFrame(() => { (bar.firstElementChild as HTMLElement).style.width = g.run ? `${(settled / Math.max(total, 1)) * 100}%` : "0"; });
  }

  // ---------- nodes ----------

  private renderNode(v: NodeView, n: GraphNode, g: Graph) {
    const status = n.status ?? "plan";
    const denied = !g.ok && !g.run && !!n.denied;
    const isJudge = n.kind === "judge", isEffect = n.kind === "effect";
    const result = isJudge && finished(status) ? decisionLabel(g.run?.decision)
      : isEffect && finished(status) ? receiptLabel(g.run?.receipt) : undefined;
    const branches = (n.branches ?? []).slice(0, MAX_BRANCHES);
    const more = (n.branches?.length ?? 0) - branches.length;
    const group = !!g.run && n.kind === "blackbox" && branches.length > 0;
    const activity = status === "running" ? n.activity ?? "" : "";
    const sig = JSON.stringify([n.name, n.task, status, denied, result, activity, more,
      branches.map((b) => [b.status, b.title, b.activity, b.finding, b.retried])]);
    v.wide = group;
    v.el.className = `cg-node k-${n.kind} s-${status}${denied ? " denied" : ""}${group ? " group" : ""}${v.el.dataset.enter ? " enter" : ""}`;
    if (sig === v.sig) return;
    const changedStatus = v.sig !== "" && JSON.parse(v.sig)[2] !== status;
    const wasDenied = v.sig === "" ? false : JSON.parse(v.sig)[3];
    v.sig = sig;
    if (v.el.dataset.enter && !v.el.dataset.listening) {
      v.el.dataset.listening = "1";
      const done = (e: AnimationEvent) => {
        if (e.target !== v.el) return; // badges and dots inside bubble up too
        v.el.removeEventListener("animationend", done);
        delete v.el.dataset.enter;
        v.el.classList.remove("enter");
        v.el.style.animationDelay = "";
      };
      v.el.addEventListener("animationend", done);
    }
    const late = changedStatus || !this.first ? "0ms" : undefined;
    if (late) v.el.style.setProperty("--d", late);

    const tags: string[] = [];
    if (n.kind === "blackbox") tags.push(`<span class="cg-tag bb">${group ? "黑盒 · Loop Forest" : "黑盒"}</span>`);
    if (n.effect === "irreversible") tags.push(`<span class="cg-tag irrev">不可逆</span>`);
    if (denied) tags.push(`<span class="cg-tag deny pop">无权限</span>`);
    if (status === "reused") tags.push(`<span class="cg-tag reuse pop">复用</span>`);
    if (status === "skipped") tags.push(`<span class="cg-tag skip">未执行</span>`);
    const badge = status === "done" ? `<span class="cg-ok">${TICK}</span>`
      : status === "reused" ? `<span class="cg-ok reused">${TICK}</span>`
      : status === "failed" ? `<span class="cg-ok fail">${CROSS}</span>` : "";

    const res = (result ? `<div class="cg-res ${result[0]}">${esc(result[1])}</div>` : "")
      + (activity ? `<div class="cg-act" title="${esc(activity)}"><i></i><span>${esc(activity)}</span></div>` : "");
    const card = `<div class="cg-av">${esc([...n.name][0] ?? "?")}</div><div class="cg-body"><div class="cg-name">${esc(n.name)}</div><div class="cg-task">${esc(n.task)}</div>${res}</div>`;

    let html: string;
    if (group) {
      html = this.renderGroup(v, card, branches, more);
    } else {
      // Before approval the box stays shut: what is inside is decided at run time.
      const closed = n.kind === "blackbox" && !g.run
        ? `<div class="cg-bb closed"><i></i><i></i><i></i><span>运行时展开</span></div>` : "";
      html = card + closed;
      v.branches = [];
    }
    v.el.innerHTML = `${html}${tags.length ? `<div class="cg-tags">${tags.join("")}</div>` : ""}${badge}`;
    v.el.querySelectorAll<HTMLElement>(".cg-ok, .cg-res, .cg-tag.pop").forEach((e) => {
      if (!e.closest(".cg-lfb")) e.style.animationDelay = `calc(var(--d, 0ms) + 280ms)`;
    });
    if (group) {
      this.drawFan(v);
      // New fork edges draw on, then their branch slides out of the end.
      requestAnimationFrame(() => requestAnimationFrame(() => v.el.querySelectorAll(".cg-fan-edges .fb:not(.drawn)").forEach((p) => p.classList.add("drawn"))));
    }
    if (denied && !wasDenied) {
      v.el.classList.remove("shake");
      setTimeout(() => v.el.classList.add("shake"), this.first ? 900 : 50);
    }
  }

  // The black box at run time: the host on the left, the branches Loop Forest
  // forked fanning out on the right. A finished branch sends its result back
  // along its edge to the host.
  private renderGroup(v: NodeView, card: string, branches: Branch[], more: number): string {
    const prev = v.branches;
    let fresh = 0, returned = false;
    const ph = phase();
    const edges: string[] = [], cards: string[] = [];
    branches.forEach((b, i) => {
      const isNew = i >= prev.length;
      const back = !isNew && prev[i] === "started" && b.status === "finished";
      returned ||= back;
      const delay = isNew ? 220 + fresh++ * 160 : 0;
      edges.push(`<g class="b-${esc(b.status)}"><path class="fb${isNew ? "" : " drawn"}${back ? " flash" : ""}" pathLength="1" style="transition-delay:${isNew ? delay - 220 : 0}ms, 0ms"/><path class="ff" style="animation-delay:${ph}"/><path class="fr${back ? " ret" : ""}" pathLength="1"/></g>`);
      const line = b.status === "started" && b.activity
        ? `<div class="cg-lfb-sub act"><i style="animation-delay:${ph}"></i><span>${esc(b.activity)}</span></div>`
        : b.status === "finished" && b.finding
          ? `<div class="cg-lfb-sub"><b class="find ${esc(b.tone ?? "")}${back || isNew ? " pop" : ""}">${esc(b.finding)}</b></div>`
          : b.status === "interrupted"
            ? `<div class="cg-lfb-sub">执行器重启，已中断</div>`
            : `<div class="cg-lfb-sub"><span>${esc(b.relation ?? "")}</span></div>`;
      const tick = b.status === "finished" ? `<span class="cg-lfb-ok${back ? " pop" : ""}">${TICK}</span>` : "";
      cards.push(`<div class="cg-lfb s-${esc(b.status)}${isNew ? " enter" : ""}" style="animation-delay:${delay}ms;--ph:${ph}" title="${esc(b.goal)}"><div class="cg-lfb-av">${esc([...b.title][0] ?? "?")}</div><div class="cg-lfb-body"><div class="cg-lfb-name">${b.retried ? `<em title="上次执行中断后重新分叉">↻</em>` : ""}${esc(b.title)}</div>${line}</div>${tick}</div>`);
    });
    v.branches = branches.map((b) => b.status);
    return `<svg class="cg-fan-edges">${edges.join("")}</svg><div class="cg-host">${card}${returned ? `<i class="cg-ripple"></i>` : ""}</div><div class="cg-fan">${cards.join("")}${
      more > 0 ? `<div class="cg-lfb-more">另 ${more} 个分支</div>` : ""}</div>`;
  }

  private drawFan(v: NodeView) {
    const host = v.el.querySelector<HTMLElement>(".cg-host");
    if (!host) return;
    const x1 = host.offsetLeft + host.offsetWidth + 2, y1 = host.offsetTop + host.offsetHeight / 2;
    const groups = v.el.querySelectorAll(".cg-fan-edges g");
    v.el.querySelectorAll<HTMLElement>(".cg-lfb").forEach((c, i) => {
      const x2 = c.offsetLeft - 2, y2 = c.offsetTop + c.offsetHeight / 2, dx = (x2 - x1) * 0.6;
      const d = `M${x1},${y1} C${x1 + dx},${y1} ${x2 - dx},${y2} ${x2},${y2}`;
      groups[i]?.querySelectorAll("path").forEach((p) => p.setAttribute("d", d));
    });
  }

  // ---------- edges ----------

  private renderEdges(g: Graph, layerOf: Map<string, number>) {
    const plan = new Map(g.nodes.map((n) => [n.id, n]));
    const byId = new Map(this.shown.map((n) => [n.id, n]));
    const up = new Map<string, Set<string>>();
    const ancestors = (id: string): Set<string> => {
      let s = up.get(id);
      if (s) return s;
      s = new Set();
      up.set(id, s);
      for (const d of plan.get(id)?.deps ?? []) { s.add(d); ancestors(d).forEach((a) => s!.add(a)); }
      return s;
    };
    const keep = new Set<string>();
    for (const n of this.shown) {
      const deps = (n.deps ?? []).filter((d) => byId.has(d));
      for (const d of deps) {
        // Draw only direct needs: an edge implied through another dependency is noise.
        if (n.when?.step !== d && deps.some((o) => o !== d && ancestors(o).has(d))) continue;
        const key = `${d}>${n.id}`;
        keep.add(key);
        let e = this.edges.get(key);
        if (!e) {
          const grp = document.createElementNS(SVGNS, "g");
          const base = document.createElementNS(SVGNS, "path");
          base.setAttribute("class", "base");
          base.setAttribute("pathLength", "1");
          const flow = document.createElementNS(SVGNS, "path");
          flow.setAttribute("class", "flow");
          grp.append(base, flow);
          this.svg.appendChild(grp);
          e = { from: d, to: n.id, g: grp, base, flow, drawn: false };
          this.edges.set(key, e);
          const delay = this.first ? 260 + (layerOf.get(d) ?? 0) * 120 : 0;
          base.style.transitionDelay = `${delay}ms, 0ms`;
        }
        if (n.when && n.when.step === d && !e.cond) {
          e.cond = document.createElement("div");
          e.cond.className = "cg-cond";
          e.cond.textContent = condLabel(n.when);
          this.stage.appendChild(e.cond);
        }
        const src = byId.get(d)!;
        let state = "idle";
        if (g.run) {
          if (n.status === "skipped") state = "skip";
          else if (finished(src.status) && n.status === "running") state = "active";
          else if (finished(src.status) && finished(n.status)) state = "done";
        } else if (!g.ok && n.denied) state = "bad";
        e.g.setAttribute("class", `e-${state}`);
        e.base.setAttribute("marker-end", `url(#${this.id}-ar-${state})`);
        if (e.cond) {
          const decided = g.run?.decision;
          e.cond.classList.toggle("on", !!decided && /^grant/.test(decided) && !n.when?.negate);
          e.cond.classList.toggle("off", n.status === "skipped");
        }
      }
    }
    for (const [key, e] of this.edges) {
      if (keep.has(key)) continue;
      this.edges.delete(key);
      e.g.classList.add("gone");
      e.cond?.classList.add("gone");
      setTimeout(() => { e.g.remove(); e.cond?.remove(); }, LEAVE_MS);
    }
  }

  private drawEdges() {
    // Fan edges out along each side instead of piling them on one point.
    const outs = new Map<string, EdgeView[]>(), ins = new Map<string, EdgeView[]>();
    for (const e of this.edges.values()) {
      (outs.get(e.from) ?? outs.set(e.from, []).get(e.from)!).push(e);
      (ins.get(e.to) ?? ins.set(e.to, []).get(e.to)!).push(e);
    }
    const port = (list: EdgeView[], e: EdgeView, v: NodeView, other: (e: EdgeView) => NodeView | undefined) => {
      const sorted = [...list].sort((a, b) => (other(a)?.y ?? 0) - (other(b)?.y ?? 0));
      const k = sorted.length, i = sorted.indexOf(e);
      const gap = k > 1 ? Math.min(9, (v.h * 0.6) / (k - 1)) : 0;
      return v.y + v.h / 2 + (i - (k - 1) / 2) * gap;
    };
    for (const e of this.edges.values()) {
      const s = this.nodes.get(e.from), t = this.nodes.get(e.to);
      if (!s || !t) continue;
      const x1 = s.x + (s.el.offsetWidth || s.w), x2 = t.x - 3;
      const y1 = port(outs.get(e.from)!, e, s, (o) => this.nodes.get(o.to));
      const y2 = port(ins.get(e.to)!, e, t, (o) => this.nodes.get(o.from));
      const dx = Math.max(26, (x2 - x1) * 0.5);
      const d = `M${x1},${y1} C${x1 + dx},${y1} ${x2 - dx},${y2} ${x2},${y2}`;
      e.base.setAttribute("d", d);
      e.flow.setAttribute("d", d);
      if (e.cond) { e.cond.style.left = `${(x1 + x2) / 2}px`; e.cond.style.top = `${(y1 + y2) / 2}px`; }
      if (!e.drawn) {
        e.drawn = true;
        requestAnimationFrame(() => requestAnimationFrame(() => e.base.classList.add("drawn")));
      }
    }
  }

  // ---------- layout and motion ----------

  private layout(animate: boolean) {
    const g = this.g!;
    const byId = new Map(this.shown.map((n) => [n.id, n]));
    const cols = g.layers.map((l) => l.filter((id) => byId.has(id))).filter((l) => l.length);
    const placedIds = new Set(cols.flat());
    const rest = this.shown.map((n) => n.id).filter((id) => !placedIds.has(id));
    if (rest.length) cols.push(rest);
    this.canvas.classList.toggle("empty", !cols.length);
    this.stage.style.transition = animate ? "" : "none";
    if (!cols.length) {
      this.canvas.style.height = "84px";
      return;
    }

    for (const v of this.nodes.values()) { v.h = v.el.offsetHeight; v.w = v.wide ? GROUP_W : W; }
    const colH = cols.map((c) => c.reduce((sum, id) => sum + this.nodes.get(id)!.h, 0) + (c.length - 1) * GY);
    const colX: number[] = [];
    let NW = PAD;
    for (const c of cols) { colX.push(NW); NW += Math.max(...c.map((id) => this.nodes.get(id)!.w)) + GX; }
    NW += PAD - GX;
    const H = Math.max(...colH, 60);
    const center = new Map<string, number>();
    cols.forEach((col, ci) => {
      const order = col.map((id, i) => {
        const ps = (byId.get(id)!.deps ?? []).filter((d) => center.has(d));
        return { id, i, bc: ps.length ? ps.reduce((s, d) => s + center.get(d)!, 0) / ps.length : Infinity };
      });
      if (ci > 0) order.sort((a, b) => (a.bc === b.bc ? a.i - b.i : a.bc - b.bc));
      let y = PAD + (H - colH[ci]) / 2;
      for (const { id } of order) {
        const v = this.nodes.get(id)!;
        v.tx = colX[ci];
        v.ty = y;
        center.set(id, y + v.h / 2);
        y += v.h + GY;
      }
    });

    const NH = H + PAD * 2;
    const avail = this.canvas.clientWidth || NW;
    const k = Math.min(1, avail / NW);
    const ox = Math.max(0, (avail - NW * k) / 2);
    this.stage.style.width = `${NW}px`;
    this.stage.style.height = `${NH}px`;
    this.svg.setAttribute("width", String(NW));
    this.svg.setAttribute("height", String(NH));
    this.stage.style.transform = `translate(${ox}px,0) scale(${k})`;
    this.canvas.style.height = `${Math.ceil(NH * k)}px`;

    cancelAnimationFrame(this.frame);
    for (const v of this.nodes.values()) {
      if (!v.placed || !animate) { v.x = v.tx; v.y = v.ty; v.placed = true; }
      v.fx = v.x; v.fy = v.y;
    }
    const start = performance.now();
    const step = (now: number) => {
      const t = animate ? Math.min(1, (now - start) / MOVE_MS) : 1;
      const e = ease(t);
      for (const v of this.nodes.values()) {
        v.x = v.fx + (v.tx - v.fx) * e;
        v.y = v.fy + (v.ty - v.fy) * e;
        v.slot.style.transform = `translate(${v.x}px,${v.y}px)`;
      }
      this.drawEdges();
      if (t < 1) this.frame = requestAnimationFrame(step);
    };
    step(start);
  }

  // ---------- footer ----------

  private renderFoot(g: Graph) {
    let html = "";
    if (g.verify) {
      const base = this.first ? 700 : 0;
      html = `<div class="cg-checks">${g.verify.checks.map((c, i) =>
        `<div class="cg-check${c.ok ? "" : " no"}" style="animation-delay:${base + i * 90}ms" title="${esc(c.detail)}"><b>${c.ok ? TICK : CROSS}</b><span>${esc(c.name)}</span></div>`).join("")}</div>${
        g.verify.checks.filter((c) => !c.ok).map((c) => {
          const items = c.items ?? [];
          const bad = items.filter((x) => x.includes("✗"));
          return (bad.length ? bad : items.length ? items : [c.detail]).map((x) => `<div class="cg-prob">${esc(`${c.name}：${x}`)}</div>`).join("");
        }).join("")}`;
    } else if (!g.ok && g.problems?.length) {
      html = g.problems.map((p, i) => `<div class="cg-prob" style="animation-delay:${(this.first ? 600 : 0) + i * 80}ms">${esc(p)}</div>`).join("");
    }
    if (html !== this.footSig) {
      this.footSig = html;
      this.foot.innerHTML = html;
    }
  }

  // ---------- approval in the card ----------

  private toggleGate() {
    if (this.gate) { this.closeGate(); return; }
    const g = this.g!;
    const form = document.createElement("form");
    form.className = "cg-gate";
    form.innerHTML = `<span class="who">以 <b>alice</b>（客户经理）批准</span>
      <input type="password" placeholder="密码" autocomplete="current-password" aria-label="密码">
      <button class="cg-btn" type="submit">批准执行</button>
      <span class="msg"></span>
      <span class="hash" title="${esc(g.hash)}">绑定 ${esc(g.hash.replace("sha256:", "").slice(0, 8))}</span>`;
    const input = form.querySelector("input")!, button = form.querySelector("button")!, msg = form.querySelector<HTMLElement>(".msg")!;
    form.addEventListener("click", (e) => e.stopPropagation());
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      if (!input.value) { input.focus(); return; }
      button.classList.add("busy");
      button.textContent = "批准中…";
      msg.textContent = "";
      const err = await this.opts.approve!(this.g!, "alice", input.value).catch((x) => String(x));
      if (err) {
        button.classList.remove("busy");
        button.textContent = "批准执行";
        msg.textContent = err;
        form.classList.remove("shake");
        void form.offsetWidth;
        form.classList.add("shake");
        input.select();
        return;
      }
      form.classList.add("done");
      form.innerHTML = `<span class="ok">${TICK}已批准，开始执行</span>`;
      this.opts.onApproved?.();
      setTimeout(() => this.closeGate(), 1600);
    });
    this.head.after(form);
    this.gate = form;
    input.focus();
  }

  private closeGate() {
    if (!this.gate) return;
    const gate = this.gate;
    this.gate = undefined;
    gate.classList.add("leave");
    setTimeout(() => gate.remove(), 260);
  }

  // ---------- detail popover ----------

  private togglePop(id: string, refresh = false) {
    if (this.pop && this.pop.dataset.id === id && !refresh) { this.closePop(); return; }
    const n = this.g?.nodes.find((x) => x.id === id);
    const v = this.nodes.get(id);
    if (!n || !v) { this.closePop(); return; }
    const pop = this.pop ?? document.createElement("div");
    const fresh = !this.pop;
    pop.className = "cg-pop";
    pop.dataset.id = id;
    const role = n.role && n.role !== n.name ? `<div class="role">${esc(n.role)}</div>` : "";
    pop.innerHTML = `<h4>${esc(n.name)}</h4>${role}<p>${esc(n.task)}</p>${
      n.denied && !this.g!.ok ? `<p style="color:var(--red)">${esc(n.denied)}</p>` : ""}${
      n.reads?.length ? `<div class="k">读取</div>${n.reads.map((r) => `<code>${esc(r)}</code>`).join("")}` : ""}${
      n.contract ? `<div class="k">契约</div><code>${esc(n.contract)}</code>` : ""}${
      n.output ? `<div class="k">产物</div><div class="out">${esc(n.output)}</div>` : ""}`;
    if (fresh) { this.el.appendChild(pop); this.pop = pop; }
    else if (!refresh) { pop.style.animation = "none"; void pop.offsetWidth; pop.style.animation = ""; }
    const r = v.el.getBoundingClientRect(), root = this.el.getBoundingClientRect();
    const left = Math.min(Math.max(8, r.left - root.left), root.width - pop.offsetWidth - 8);
    let top = r.bottom - root.top + 8;
    if (top + pop.offsetHeight > root.height - 4) top = Math.max(4, r.top - root.top - pop.offsetHeight - 8);
    pop.style.left = `${left}px`;
    pop.style.top = `${top}px`;
  }

  private closePop() {
    this.pop?.remove();
    this.pop = undefined;
  }
}

// startLive polls the graph while it can still change (awaiting approval or
// running) and stops by itself once the server stops handing out a live URL.
// resolve maps the links in each snapshot to where the service is reachable.
export function startLive(view: GraphView, g: Graph, resolve = (u: string) => u, everyMs = 1200, immediate = false): () => void {
  let stopped = false, timer = 0, fails = 0;
  let url = g.live;
  const tick = async () => {
    if (stopped || !url) return;
    try {
      const res = await fetch(url, { cache: "no-store" });
      if (!res.ok) throw new Error(String(res.status));
      const next = (await res.json()) as Graph;
      if (stopped) return;
      fails = 0;
      for (const k of ["live", "confirm", "approve"] as const) if (next[k]) next[k] = resolve(next[k]!);
      if (next.run) next.run.url = resolve(next.run.url);
      view.render(next);
      url = next.live;
    } catch {
      if (++fails > 30) return;
    }
    if (url && !stopped) timer = window.setTimeout(tick, everyMs);
  };
  if (url) timer = window.setTimeout(tick, immediate ? 0 : everyMs);
  return () => { stopped = true; clearTimeout(timer); };
}
