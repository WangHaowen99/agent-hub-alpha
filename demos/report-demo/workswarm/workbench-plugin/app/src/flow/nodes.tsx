// Node and edge components, styled after WorkSwarm's SwarmFlow graph nodes:
// a white card with a status icon, a status-tinted border, and a purple
// Loop Forest group with fork nodes for the black box.
import { memo, type CSSProperties } from "react";
import { BaseEdge, EdgeLabelRenderer, getBezierPath, Handle, Position, type EdgeProps, type NodeProps } from "@xyflow/react";
import { Ban, Circle, CircleCheck, CircleDashed, CircleDot, CircleHelp, CircleX, Clock, GitBranch, Loader2 } from "lucide-react";
import type { BranchData, EdgeData, ExpertData, GroupData, Line } from "./layout";

// A denied node shakes once it has landed: the second delay is for that.
const enter = (delay: number, shake = false): CSSProperties => ({ animationDelay: shake ? `${delay}ms, ${delay + 900}ms` : `${delay}ms` });

export function StatusIcon({ status, denied }: { status: string; denied?: boolean }) {
  // Keyed by status so a change remounts the icon and plays its pop.
  if (denied) return <Ban key="denied" className="ci pop t-bad" />;
  switch (status) {
    case "running": return <Loader2 key={status} className="ci spin t-run" />;
    case "waiting": return <Clock key={status} className="ci pop t-warn" />;
    case "unknown": return <CircleHelp key={status} className="ci pop t-warn" />;
    case "done": return <CircleCheck key={status} className="ci pop t-ok" />;
    case "reused": return <CircleCheck key={status} className="ci pop t-reuse" />;
    case "failed": return <CircleX key={status} className="ci pop t-bad" />;
    case "skipped": return <CircleDashed key={status} className="ci t-mute" />;
    case "pending": return <CircleDot key={status} className="ci t-mute" />;
  }
  return <Circle key="plan" className="ci t-mute" />;
}

function StatusLine({ line }: { line: Line }) {
  // Keyed by its text so a new activity slides in instead of just swapping.
  const k = `${line.kind}:${line.text}`;
  switch (line.kind) {
    case "act": return <div key={k} className="cn-row act" title={line.text}><i /><span>{line.text}</span></div>;
    case "res": return <div key={k} className={`cn-row res t-${line.tone}`}><span>{line.text}</span></div>;
    case "box": return <div className="cn-row box"><i /><i /><i /><span>{line.text}</span></div>;
  }
  return <div key={k} className={`cn-row note t-${line.tone ?? "mute"}`} title={line.text}><span>{line.text}</span></div>;
}

export const ExpertNode = memo(({ data }: NodeProps) => {
  const d = data as ExpertData;
  const { n, status } = d;
  const tags: JSX.Element[] = [];
  if (n.kind === "blackbox" && !d.host) tags.push(<span key="bb" className="tag bb">黑盒</span>);
  if (n.execution?.kind === "agent_bus" || n.remote) tags.push(<span key="bus" className="tag bus" title="经 Agent Bus 网关 DIRECT 调用远端 Agent">总线</span>);
  if (status === "unknown") tags.push(<span key="uk" className="tag warn pop">待核对</span>);
  if (n.effect === "irreversible") tags.push(<span key="irr" className="tag irr">不可逆</span>);
  if (status === "reused") tags.push(<span key="re" className="tag reuse pop">复用</span>);
  if (d.denied) tags.push(<span key="dn" className="tag bad pop">无权限</span>);
  return (
    <div className={`cn s-${status} k-${n.kind}${d.denied ? " denied" : ""}${d.host ? " host" : ""}`} style={enter(d.delay, d.denied)}>
      {!d.host && <Handle type="target" position={Position.Left} isConnectable={false} />}
      <Handle type="source" position={Position.Right} isConnectable={false} />
      <div className="cn-head">
        <StatusIcon status={status} denied={d.denied} />
        <span className="cn-name">{n.name}</span>
        {tags}
      </div>
      <div className="cn-task" title={n.task}>{n.task}</div>
      {d.line && <StatusLine line={d.line} />}
    </div>
  );
});

export const GroupNode = memo(({ data }: NodeProps) => {
  const d = data as GroupData;
  const running = d.n.status === "running";
  return (
    <div className={`lf${running ? " on" : ""}`} style={enter(d.delay)}>
      <Handle type="target" position={Position.Left} isConnectable={false} />
      <Handle type="source" position={Position.Right} isConnectable={false} />
      <div className="lf-head">
        <GitBranch className={`ci t-lf${running ? " sway" : ""}`} />
        <span>黑盒 · Loop Forest</span>
        <em>分叉 {d.forks}</em>
      </div>
      {d.forks === 0 && <div className="lf-wait"><i /><i /><i /><span>等待分叉</span></div>}
      {d.more > 0 && <div className="lf-more">另 {d.more} 个分支</div>}
    </div>
  );
});

export const BranchNode = memo(({ data }: NodeProps) => {
  const { b, delay } = data as BranchData;
  const line = b.status === "started" && b.activity
    ? <div className="cb-sub act"><i /><span>{b.activity}</span></div>
    : b.status === "finished" && b.finding
      ? <div className={`cb-sub find t-${b.tone ?? "mute"}`}><span key={b.finding} className="pop">{b.finding}</span></div>
      : b.status === "interrupted"
        ? <div className="cb-sub">执行器重启，已中断</div>
        : <div className="cb-sub">{b.relation ?? ""}</div>;
  const icon = b.status === "started" ? <Loader2 key="s" className="ci spin t-lf" />
    : b.status === "finished" ? <CircleCheck key="f" className="ci pop t-ok" />
      : <CircleDashed key="i" className="ci t-mute" />;
  return (
    <div className={`cb s-${b.status}`} style={enter(delay)} title={b.goal}>
      <Handle type="target" position={Position.Left} isConnectable={false} />
      <div className="cb-head">
        <GitBranch className="ci t-lf" />
        <span className="cb-name">{b.retried && <em title="上次执行中断后重新分叉">↻</em>}{b.title}</span>
        {icon}
      </div>
      {line}
    </div>
  );
});

export const FlowEdge = memo((p: EdgeProps) => {
  const d = p.data as EdgeData;
  const [path, lx, ly] = getBezierPath(p);
  return (
    <>
      <BaseEdge id={p.id} path={path} className={`ce s-${d.state}`} style={{ animationDelay: `${d.delay}ms` }} pathLength={1} />
      {(d.state === "active" || d.state === "lf-active") && <path className={`ce-flow s-${d.state}`} d={path} />}
      {d.label && (
        <EdgeLabelRenderer>
          <div className={`ce-label${d.labelOn ? " on" : ""}${d.labelOff ? " off" : ""}`}
            style={{ transform: `translate(-50%, -50%) translate(${lx}px, ${ly}px)`, animationDelay: `${d.delay + 300}ms` }}>
            {d.label}
          </div>
        </EdgeLabelRenderer>
      )}
    </>
  );
});

export const nodeTypes = { expert: ExpertNode, lf: GroupNode, branch: BranchNode };
export const edgeTypes = { flow: FlowEdge };
