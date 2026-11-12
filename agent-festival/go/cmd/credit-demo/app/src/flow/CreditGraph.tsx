// The plan card: header, in-card approval, the graph on a React Flow canvas,
// verification results and a detail panel for the clicked node.
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { Background, Controls, ReactFlow, useReactFlow, type Node } from "@xyflow/react";
import { CircleCheck, CircleX, RotateCcw, X } from "lucide-react";
import { finished, type Graph, type GraphNode, type Options } from "../types";
import { associationText, busOverviewHTML, executionDetailHTML } from "./bus-detail";
import { build, Delays, type Built } from "./layout";
import { edgeTypes, nodeTypes } from "./nodes";

const PAD = 14, MIN_ZOOM = 0.55, MOVE_MS = 420;
const ease = (t: number) => 1 - Math.pow(1 - t, 3);

export function CreditGraph({ g, opts }: { g: Graph; opts: Options }) {
  const delays = useRef(new Delays());
  const mode = g.run ? "run" : "plan";
  // Approval swaps the whole plan for the run view: start the entrances over.
  const lastMode = useRef(mode);
  if (lastMode.current !== mode) { lastMode.current = mode; delays.current = new Delays(); }
  const built = useMemo(() => build(g, delays.current), [g]);
  const [detail, setDetail] = useState<string>();
  const [gate, setGate] = useState(false);
  const canApprove = !!g.approve && !!opts.approve && !opts.page;
  useEffect(() => { if (!canApprove) setGate(false); }, [canApprove]);

  return (
    <div className={`cg m-${mode}${g.run?.stalled ? " m-stalled" : ""}`}>
      <Head g={g} canApprove={canApprove} onApprove={() => setGate((x) => !x)} />
      {gate && canApprove && <Gate g={g} opts={opts} onClose={() => setGate(false)} />}
      {g.bus && <BusOverview g={g} onInspect={(id) => setDetail(id)} />}
      <Canvas built={built} empty={!built.nodes.length} blank={!g.nodes.length} onPick={(id) => setDetail((cur) => (cur === id ? undefined : id))} />
      <Foot g={g} />
      {detail && <Detail g={g} id={detail} onClose={() => setDetail(undefined)} />}
    </div>
  );
}

// ---------- header ----------

function Head({ g, canApprove, onApprove }: { g: Graph; canApprove: boolean; onApprove: () => void }) {
  const total = g.nodes.length;
  const settled = g.nodes.filter((n) => finished(n.status) || n.status === "skipped").length;
  const reused = g.nodes.filter((n) => n.status === "reused").length;
  let chip: [string, string];
  if (g.verify) {
    const pass = g.verify.checks.filter((c) => c.ok).length;
    chip = g.verify.ok ? ["ok", `验证通过 ${pass}/${g.verify.checks.length}`] : ["bad", `验证未通过 ${pass}/${g.verify.checks.length}`];
  } else if (g.run) {
    chip = ({ queued: ["wait", "排队中"], running: ["run", `执行中 ${settled}/${total}`], waiting: ["wait", "等待核对远端结果"], done: ["ok", "已完成"], failed: ["bad", "执行失败"] } as Record<string, [string, string]>)[g.run.status] ?? ["mute", g.run.status];
    if (g.run.stalled) chip = ["bad", "执行器无响应 · 等待接管"];
  } else if (!g.ok) chip = ["bad", "预检未通过"];
  else if (g.superseded) chip = ["mute", `已更新为 v${g.superseded}`];
  else chip = ["wait", "待批准"];
  const running = g.run?.status === "running" && !g.run.stalled ? g.nodes.filter((n) => n.status === "running").map((n) => n.name) : [];
  const progress = g.run ? settled / Math.max(total, 1) : 0;
  return (
    <div className="cg-head">
      <div className="cg-line">
        <span className="cg-goal" title={g.goal}>{g.goal || `${g.case} 授信尽调`}</span>
        {g.run && g.run.leases > 1 && <span className="chip soft"><RotateCcw className="ci" />执行器重启 · 复用 {reused} 步</span>}
        <span key={chip[1]} className={`chip ${chip[0]}`}><i />{chip[1]}</span>
        {canApprove && <button className="cg-btn" onClick={onApprove}>批准</button>}
      </div>
      <div className="cg-sub">
        {running.length
          ? <span key={running.join()} className="cg-now"><i />正在：{running.join("、")}</span>
          : <span>{g.case} · {g.plan} v{g.version}</span>}
      </div>
      <div className={`cg-bar${g.run?.status === "done" ? " ok" : ""}`}><b style={{ transform: `scaleX(${progress})` }} /></div>
    </div>
  );
}

function Gate({ g, opts, onClose }: { g: Graph; opts: Options; onClose: () => void }) {
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [shake, setShake] = useState(0);
  const [done, setDone] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => input.current?.focus(), []);
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const pw = input.current?.value ?? "";
    if (!pw) { input.current?.focus(); return; }
    setBusy(true);
    setErr("");
    const msg = await opts.approve!(g, "alice", pw).catch((x) => String(x));
    setBusy(false);
    if (msg) { setErr(msg); setShake((x) => x + 1); input.current?.select(); return; }
    setDone(true);
    opts.onApproved?.();
    setTimeout(onClose, 1600);
  };
  if (done) return <div className="cg-gate ok"><CircleCheck className="ci pop t-ok" />已批准，开始执行</div>;
  return (
    <form key={shake} className={`cg-gate${shake ? " shake" : ""}`} onSubmit={submit}>
      <span className="who">以 <b>alice</b>（客户经理）批准</span>
      <input ref={input} type="password" placeholder="密码" autoComplete="current-password" aria-label="密码" />
      <button className="cg-btn" type="submit" disabled={busy}>{busy ? "批准中…" : "批准执行"}</button>
      {err && <span className="msg">{err}</span>}
      <span className="hash" title={g.hash}>绑定 {g.hash.replace("sha256:", "").slice(0, 8)}</span>
    </form>
  );
}

// ---------- canvas ----------

function Canvas({ built, empty, blank, onPick }: { built: Built; empty: boolean; blank: boolean; onPick: (id: string) => void }) {
  const rf = useReactFlow();
  const wrap = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(0);
  const [nodes, setNodes] = useState<Node[]>([]);
  const at = useRef(new Map<string, { x: number; y: number }>());
  const userMoved = useRef(false);
  const fitted = useRef(false);

  useLayoutEffect(() => {
    const el = wrap.current!;
    const ro = new ResizeObserver(() => setWidth((w) => (Math.abs(w - el.clientWidth) > 1 ? el.clientWidth : w)));
    ro.observe(el);
    setWidth(el.clientWidth);
    return () => ro.disconnect();
  }, []);

  // Glide nodes that moved to their new place; new ones appear where they belong.
  useEffect(() => {
    const from = new Map(at.current);
    const moving = built.nodes.some((n) => { const p = from.get(n.id); return p && (p.x !== n.position.x || p.y !== n.position.y); });
    const place = (k: number) => built.nodes.map((n) => {
      const p = from.get(n.id);
      const pos = p && k < 1 ? { x: p.x + (n.position.x - p.x) * k, y: p.y + (n.position.y - p.y) * k } : n.position;
      at.current.set(n.id, pos);
      return pos === n.position ? n : { ...n, position: pos };
    });
    at.current = new Map();
    if (!moving) { setNodes(place(1)); return; }
    let frame = 0;
    const t0 = performance.now();
    const step = (now: number) => {
      const k = ease(Math.min(1, (now - t0) / MOVE_MS));
      at.current = new Map();
      setNodes(place(k));
      if (k < 1) frame = requestAnimationFrame(step);
    };
    frame = requestAnimationFrame(step);
    return () => cancelAnimationFrame(frame);
  }, [built]);

  // Fit the graph to the card width; the card grows to the graph's height.
  const zoom = width ? Math.max(MIN_ZOOM, Math.min(1, (width - PAD * 2) / Math.max(built.width, 1))) : 1;
  // A queued run keeps the room its graph will take.
  const height = blank ? 84 : Math.ceil(built.height * zoom + PAD * 2);
  const vx = Math.max(PAD, (width - built.width * zoom) / 2);
  useEffect(() => {
    if (!width || userMoved.current) return;
    void rf.setViewport({ x: vx, y: PAD, zoom }, { duration: fitted.current ? MOVE_MS : 0 });
    fitted.current = true;
  }, [rf, vx, zoom, width, built.key]);

  return (
    <div ref={wrap} className={`cg-canvas${empty ? " empty" : ""}`} style={{ height }}>
      <ReactFlow
        nodes={nodes} edges={built.edges} nodeTypes={nodeTypes} edgeTypes={edgeTypes}
        nodesDraggable={false} nodesConnectable={false} elementsSelectable={false} nodesFocusable={false} edgesFocusable={false}
        panOnDrag zoomOnScroll={false} zoomOnPinch zoomOnDoubleClick={false} preventScrolling={false}
        minZoom={0.3} maxZoom={1.5} proOptions={{ hideAttribution: true }}
        onMoveStart={(e) => { if (e) userMoved.current = true; }}
        onNodeClick={(_, n) => onPick(n.type === "branch" ? n.id : n.parentId ?? n.id)}
      >
        <Background color="#dfe3e8" gap={20} size={1.2} />
        <Controls showInteractive={false} position="bottom-right" fitViewOptions={{ padding: 0.08, maxZoom: 1, duration: MOVE_MS }}
          onFitView={() => { userMoved.current = false; }} />
      </ReactFlow>
      {empty && <div className="cg-wait"><i /><i /><i /></div>}
    </div>
  );
}

// ---------- verification and problems ----------

function Foot({ g }: { g: Graph }) {
  if (g.verify) {
    const failed = g.verify.checks.filter((c) => !c.ok);
    return (
      <div className="cg-foot">
        <div className="cg-checks">
          {g.verify.checks.map((c, i) => (
            <div key={c.name} className={`cg-check${c.ok ? "" : " no"}`} style={{ animationDelay: `${i * 70}ms` }} title={c.detail}>
              {c.ok ? <CircleCheck className="ci t-ok" /> : <CircleX className="ci t-bad" />}<span>{c.name}</span>
            </div>
          ))}
        </div>
        {failed.map((c) => {
          const items = c.items ?? [];
          const bad = items.filter((x) => x.includes("✗"));
          return (bad.length ? bad : items.length ? items : [c.detail]).map((x) => <div key={c.name + x} className="cg-prob">{c.name}：{x}</div>);
        })}
      </div>
    );
  }
  if (!g.ok && g.problems?.length) {
    return <div className="cg-foot">{g.problems.map((p, i) => <div key={p} className="cg-prob" style={{ animationDelay: `${i * 70}ms` }}>{p}</div>)}</div>;
  }
  return null;
}

// ---------- detail ----------

function Detail({ g, id, onClose }: { g: Graph; id: string; onClose: () => void }) {
  const m = /^(.+)\/b(\d+)$/.exec(id);
  if (m) {
    const b = g.nodes.find((x) => x.id === m[1])?.branches?.[Number(m[2])];
    if (!b) return null;
    return (
      <div className="cg-detail">
        <button className="x" onClick={onClose} aria-label="关闭"><X className="ci" /></button>
        <h4>{b.title}<small>Loop Forest 分支</small></h4>
        <p>{b.goal}</p>
        {b.finding && <><div className="k">结论</div><p>{b.finding}</p></>}
      </div>
    );
  }
  const n = g.nodes.find((x) => x.id === id);
  if (!n) return null;
  if (n.execution || n.remote) return <BusDetail n={n} onClose={onClose} />;
  return (
    <div className="cg-detail">
      <button className="x" onClick={onClose} aria-label="关闭"><X className="ci" /></button>
      <h4>{n.name}{n.role && n.role !== n.name && <small>{n.role}</small>}</h4>
      <p>{n.task}</p>
      {n.denied && !g.ok && <p className="t-bad">{n.denied}</p>}
      {!!n.reads?.length && <><div className="k">读取</div>{n.reads.map((r) => <code key={r}>{r}</code>)}</>}
      {n.contract && <><div className="k">契约</div><code>{n.contract}</code></>}
      {n.output && <><div className="k">产物</div><div className="out">{n.output}</div></>}
    </div>
  );
}

// ---------- Agent Bus (W05) ----------
// The overview and the execution detail come from bus-detail.ts as escaped
// HTML; set it by hand so a live poll does not collapse what the user opened.

function useHTML(html: string, keep: (el: HTMLElement) => () => void) {
  const ref = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    const el = ref.current!;
    const restore = keep(el);
    el.innerHTML = html;
    restore();
  }, [html]);
  return ref;
}

const openSections = (el: HTMLElement) => {
  const open = [...el.querySelectorAll<HTMLDetailsElement>("details")].filter((d) => d.open).map((d) => d.dataset.detailSection ?? d.className);
  const body = el.querySelector(".cg-detail-body");
  const scroll = body?.scrollTop ?? 0;
  return () => {
    el.querySelectorAll<HTMLDetailsElement>("details").forEach((d) => { d.open = open.includes(d.dataset.detailSection ?? d.className); });
    const b = el.querySelector(".cg-detail-body");
    if (b) b.scrollTop = scroll;
  };
};

function BusOverview({ g, onInspect }: { g: Graph; onInspect: (id: string) => void }) {
  const ref = useHTML(busOverviewHTML(g), openSections);
  return (
    <div ref={ref} className="cg-bus-overview" onClick={(e) => {
      const b = (e.target as HTMLElement).closest<HTMLElement>("[data-inspect]");
      if (b) onInspect(b.dataset.inspect!);
    }} />
  );
}

function BusDetail({ n, onClose }: { n: GraphNode; onClose: () => void }) {
  const ref = useHTML(executionDetailHTML(n), openSections);
  const copy = async (status: HTMLElement | null) => {
    const text = associationText(n);
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      const t = document.createElement("textarea");
      t.value = text;
      document.body.appendChild(t);
      t.select();
      document.execCommand("copy");
      t.remove();
    }
    if (status) status.textContent = "已复制";
  };
  return (
    <div ref={ref} className="cg-detail bus" onClick={(e) => {
      const t = e.target as HTMLElement;
      if (t.closest("[data-detail-close]")) onClose();
      else if (t.closest("[data-copy-evidence]")) void copy(ref.current!.querySelector("[data-copy-status]"));
    }} />
  );
}
