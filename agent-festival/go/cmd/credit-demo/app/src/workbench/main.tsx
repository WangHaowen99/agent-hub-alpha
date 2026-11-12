// 编排台：WorkSwarm 应用插件页面。左侧是最近的编排单，右侧是选中的
// 编排图；读状态和确认都走插件 RPC，由插件后端转给编排服务。
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { ReactFlowProvider } from "@xyflow/react";
import { CreditGraph } from "../flow/CreditGraph";
import { ensureStyle } from "../flow/styles";
import type { Graph, Options } from "../types";
import { Gateway } from "./gateway";
import pageCss from "./page.css";

interface PlanRow { plan: string; version: number; case: string; goal: string; ok: boolean; run?: string; run_status?: string }
type Pick = { plan: string; v: number };

const gw = new Gateway();
const POLL_MS = 1200, LIST_MS = 5000;

function statusOf(p: PlanRow): [string, string] {
  if (p.run_status === "done") return ["ok", "已完成"];
  if (p.run_status === "failed") return ["bad", "执行失败"];
  if (p.run_status) return ["run", "执行中"];
  if (!p.ok) return ["bad", "预检未通过"];
  return ["wait", "待批准"];
}

// The selection lives in the URL hash, so a reload comes back to it.
const fromHash = (): Pick | undefined => {
  const m = /^#(P\d+)\/(\d+)$/.exec(location.hash);
  return m ? { plan: m[1], v: Number(m[2]) } : undefined;
};

function App() {
  const [plans, setPlans] = useState<PlanRow[]>();
  const [pick, setPick] = useState<Pick | undefined>(fromHash);
  const [g, setG] = useState<Graph>();
  const [err, setErr] = useState("");
  const [nonce, setNonce] = useState(0);
  const shown = useRef("");

  useEffect(() => {
    let stop = false, timer = 0;
    const load = async () => {
      try {
        const r = await gw.call<{ plans: PlanRow[] }>("plugin.credit_workbench.plans");
        if (stop) return;
        setPlans(r.plans);
        setErr("");
        setPick((p) => p ?? (r.plans[0] && { plan: r.plans[0].plan, v: r.plans[0].version }));
      } catch (e) {
        if (!stop) setErr((e as Error).message);
      }
      if (!stop) timer = window.setTimeout(load, LIST_MS);
    };
    void load();
    return () => { stop = true; clearTimeout(timer); };
  }, []);

  // Poll the selected plan while it can still change; follow its run once
  // approved and fetch the verification once it is done.
  useEffect(() => {
    if (!pick) return;
    history.replaceState(null, "", `#${pick.plan}/${pick.v}`);
    let stop = false, timer = 0;
    let target: Record<string, unknown> = { plan: pick.plan, v: String(pick.v) };
    const tick = async () => {
      try {
        const next = await gw.call<Graph>("plugin.credit_workbench.graph", target);
        if (stop) return;
        setErr("");
        const text = JSON.stringify(next);
        if (text !== shown.current) { shown.current = text; setG(next); }
        if (next.run) {
          const done = next.run.status === "done";
          if (done && !next.verify) target = { run: next.run.id, verify: true };
          else if (done || next.run.status === "failed") return;
          else target = { run: next.run.id };
        } else if (!next.ok || next.superseded) {
          return;
        }
      } catch (e) {
        if (!stop) setErr((e as Error).message);
      }
      if (!stop) timer = window.setTimeout(tick, POLL_MS);
    };
    void tick();
    return () => { stop = true; clearTimeout(timer); };
  }, [pick?.plan, pick?.v, nonce]);

  const approve = useCallback(async (graph: Graph, _user: string, password: string) => {
    try {
      await gw.call("plugin.credit_workbench.approve", { plan: graph.plan, version: graph.version, hash: graph.hash, password });
      return undefined;
    } catch (e) {
      return (e as Error).message;
    }
  }, []);
  const opts = useMemo<Options>(() => ({ approve, onApproved: () => setNonce((n) => n + 1) }), [approve]);

  const choose = (p: PlanRow) => {
    if (pick?.plan === p.plan && pick.v === p.version) return;
    shown.current = "";
    setG(undefined);
    setPick({ plan: p.plan, v: p.version });
  };

  return (
    <div className="wb">
      <aside className="wb-list">
        <div className="wb-head">编排单</div>
        {plans === undefined ? <div className="wb-empty">加载中…</div>
          : plans.length === 0 ? <div className="wb-empty">还没有编排单</div>
            : plans.map((p) => {
              const [tone, text] = statusOf(p);
              return (
                <button key={p.plan} className={`wb-item${pick?.plan === p.plan ? " on" : ""}`} onClick={() => choose(p)}>
                  <div className="wb-row"><b>{p.plan} v{p.version}</b><span className={`chip ${tone}`}><i />{text}</span></div>
                  <div className="wb-goal" title={p.goal}>{p.goal}</div>
                </button>
              );
            })}
      </aside>
      <main className="wb-main">
        {err && <div className="wb-err">{err}</div>}
        {g ? (
          <ReactFlowProvider key={`${g.plan}/${g.version}`}><CreditGraph g={g} opts={opts} /></ReactFlowProvider>
        ) : (
          <div className="wb-empty">{pick ? "加载中…" : ""}</div>
        )}
      </main>
    </div>
  );
}

ensureStyle(document);
const st = document.createElement("style");
st.textContent = pageCss;
document.head.appendChild(st);
createRoot(document.getElementById("root")!).render(<App />);
