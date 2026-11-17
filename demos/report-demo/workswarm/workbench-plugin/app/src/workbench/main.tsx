// 编排台：WorkSwarm 应用插件页面。左侧是最近的编排单，右侧是选中的
// 编排图；读状态和确认都走插件 RPC，由插件后端转给编排服务。
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { ReactFlowProvider } from "@xyflow/react";
import { CreditGraph } from "../flow/CreditGraph";
import { ensureStyle } from "../flow/styles";
import type { Graph, Options } from "../types";
import { newer } from "../order";
import { Gateway, type GatewayError } from "./gateway";
import pageCss from "./page.css";
import { HumanReviewNotice } from "./HumanReviewNotice";

interface PlanRow { plan: string; version: number; case: string; goal: string; ok: boolean; run?: string; run_status?: string }
type Pick = { plan: string; v: number };

const gw = new Gateway();
const lost = (e: unknown) => ((e as GatewayError).code === "WS" ? "与 WorkSwarm 的连接中断，正在重连…" : (e as Error).message);
const POLL_MS = 1200, LIST_MS = 5000;

function statusOf(p: PlanRow): [string, string] {
  if (p.run_status === "done") return ["ok", "已完成"];
  if (p.run_status === "failed") return ["bad", "执行失败"];
  if (p.run_status === "blocked") return ["bad", "任务受阻"];
  if (p.run_status === "waiting") return ["wait", "等待核对"];
  if (p.run_status) return ["run", "执行中"];
  if (!p.ok) return ["bad", "预检未通过"];
  return ["wait", "待批准"];
}

// The selection survives a WorkSwarm reload. WorkSwarm loads this page from
// a fixed URL, so it is kept in localStorage (same origin), not the hash.
const PICK_KEY = "credit-workbench:pick";
const savedPick = (): Pick | undefined => {
  let m: RegExpExecArray | null = null;
  try { m = /^(T-[0-9a-f]{32})\/(\d+)$/.exec(localStorage.getItem(PICK_KEY) ?? ""); } catch { /* storage blocked */ }
  return m ? { plan: m[1], v: Number(m[2]) } : undefined;
};

// A human command through the existing plugin channel. Never auto-retry a
// timeout: the original run is polled to reconcile whether it was accepted.
function ResumeControl({ run, onAccepted, onMessage }: { run: string; onAccepted: () => void; onMessage: (message: string) => void }) {
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const pending = useRef(false);
  return <form className="wb-resume" onSubmit={async (e) => {
    e.preventDefault();
    if (pending.current) return;
    pending.current = true; setBusy(true); onMessage("");
    try {
      await gw.call("plugin.credit_workbench.resume", { run, password });
      onMessage("恢复请求已接受，正在核对原任务；这不表示任务已完成。");
      onAccepted();
    } catch (error) {
      onMessage(`${(error as Error).message}。请先查看原任务最新状态，勿连续重试。`);
    } finally {
      setPassword(""); pending.current = false; setBusy(false);
    }
  }}>
    <span>{run} 正在等待核对。使用Go 配置的演示账号 恢复原任务。</span>
    <label>演示密码 <input type="password" value={password} autoComplete="off" disabled={busy}
      onChange={(e) => setPassword(e.target.value)} /></label>
    <button type="submit" disabled={busy || !password}>{busy ? "提交中…" : "核对原任务并继续"}</button>
  </form>;
}

function ReportControl({g,refresh}:{g:Graph;refresh:()=>void}) {
 const [password,setPassword]=useState(''),[message,setMessage]=useState(''),[busy,setBusy]=useState(false);
 const pending=useRef(false);
 const receipt=useRef(new Map<string,string>());
 return <div className="wb-notice"><strong>正式状态：{g.task_status}</strong>
 <HumanReviewNotice review={g.human_review} />
 {g.run && ['running','queued'].includes(g.run.status) && <button disabled={busy} onClick={()=>act('pause','')}>暂停新节点</button>}
 {g.reports?.map(r=><details key={r.artifact_id} data-artifact={r.artifact_id}><summary>报告 {r.node} · {r.check_outcome} · {r.digest.slice(0,22)}</summary><pre>{JSON.stringify(r.output,null,2)}</pre>
 <button disabled={busy || !r.can_approve} onClick={()=>act('approve_report',r.artifact_id)}>批准此报告</button> <button disabled={busy || !r.can_approve} onClick={()=>act('deliver',r.artifact_id)}>交付此报告</button>
 <button disabled={busy || g.task_status!=='DELIVERED'} onClick={()=>act('download_report',r.artifact_id)}>下载正式 JSON</button></details>)}
 {(!!g.reports?.length || (g.run && ['running','queued'].includes(g.run.status))) && <label>演示密码 <input type="password" autoComplete="off" value={password} onChange={e=>setPassword(e.target.value)} /></label>}
 <p role="status">{message}</p>{!!g.delivery_receipts?.length&&<pre>{JSON.stringify(g.delivery_receipts,null,2)}</pre>}</div>;
 async function act(action:string,artifact:string){
  if(pending.current||!password)return;pending.current=true;setBusy(true);setMessage('');
  const key=action+':'+artifact;
  if(!receipt.current.has(key))receipt.current.set(key,crypto.randomUUID());
  try{
   const result=await gw.call<{content_base64?:string;digest?:string}>('plugin.credit_workbench.command',{task:g.plan,action,artifact,password,command_id:receipt.current.get(key)});
   if(action==='download_report'){
    const bytes=Uint8Array.from(atob(result.content_base64??''),c=>c.charCodeAt(0));
    const sum='sha256:'+Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes)),v=>v.toString(16).padStart(2,'0')).join('');
    if(sum!==result.digest || sum!==g.reports?.find(r=>r.artifact_id===artifact)?.digest)throw new Error('下载内容摘要不匹配');
    const url=URL.createObjectURL(new Blob([bytes],{type:'application/json'}));const link=document.createElement('a');link.href=url;link.download=g.plan+'-annual-report.json';link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
   }
   setMessage('命令已接受；以原任务的最新状态与回执为准。');refresh();
  }
  catch(e){setMessage((e as Error).message+'。请先核对原任务，勿盲目重试。');}
  finally{setPassword('');pending.current=false;setBusy(false);}
 }
}

function App() {
  const [plans, setPlans] = useState<PlanRow[]>();
  const [pick, setPick] = useState<Pick | undefined>(savedPick);
  const [g, setG] = useState<Graph>();
  const [err, setErr] = useState("");
  // Keep the command receipt after waiting -> queued unmounts ResumeControl.
  // Scope it to its run; it is not a replacement for the polled task status.
  const [notice, setNotice] = useState<{ run: string; message: string }>();
  const [nonce, setNonce] = useState(0);
  const shown = useRef("");
  const shownGraph = useRef<Graph>();

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
        if (!stop) setErr(lost(e));
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
    try { localStorage.setItem(PICK_KEY, `${pick.plan}/${pick.v}`); } catch { /* storage blocked */ }
    let stop = false, timer = 0;
    let target: Record<string, unknown> = { plan: pick.plan, v: String(pick.v) };
    const tick = async () => {
      try {
        const next = await gw.call<Graph>("plugin.credit_workbench.graph", target);
        if (stop) return;
        if (!newer(shownGraph.current,next)) { timer=window.setTimeout(tick,POLL_MS);return; }
        setErr("");
        if (newer(shownGraph.current, next)) {
          const text = JSON.stringify(next);
          if (text !== shown.current) { shown.current = text; shownGraph.current = next; setG(next); }
        }
        if (next.run) {
          const done = next.run.status === "done";
          if (done && !next.verify) target = { run: next.run.id, verify: true };
          else if (done || next.run.status === "failed") return;
          else target = { run: next.run.id };
        } else if (!next.ok || next.superseded) {
          return;
        }
      } catch (e) {
        // Keep the last graph on screen; polling goes on and reconnects.
        if (!stop) setErr(lost(e));
      }
      if (!stop) timer = window.setTimeout(tick, POLL_MS);
    };
    void tick();
    return () => { stop = true; clearTimeout(timer); };
  }, [pick?.plan, pick?.v, nonce]);

  const approve = useCallback(async (graph: Graph, _user: string, password: string) => {
    setNotice(undefined);
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
    shownGraph.current = undefined;
    setG(undefined);
    setNotice(undefined);
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
        {notice && notice.run === g?.run?.id && <div className="wb-notice" role="status">{notice.run}：{notice.message}</div>}
        {g?.run?.status === "blocked" && <div className="wb-blocked" role="status">{g.run.id} 任务受阻。请由维护人员核对绑定版本与配置；此处不能按普通等待任务恢复。</div>}
        {g?.run?.status === "waiting" && <ResumeControl key={`resume:${g.run.id}`} run={g.run.id} onAccepted={() => setNonce((n) => n + 1)}
          onMessage={(message) => {
            const run = g.run!.id;
            if (shownGraph.current?.run?.id === run) setNotice(message ? { run, message } : undefined);
          }} />}
        {g?.selection && <details className="wb-notice"><summary>能力选择记录 {g.selection.id}</summary><pre>{JSON.stringify(g.selection,null,2)}</pre></details>}
        {g && <ReportControl key={`report:${g.plan}`} g={g} refresh={()=>setNonce(n=>n+1)} />}
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
