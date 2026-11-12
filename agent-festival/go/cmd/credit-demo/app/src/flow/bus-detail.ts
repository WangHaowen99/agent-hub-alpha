// From the Agent Bus (W05) branch: what a node's RDC binding, Gateway calls,
// Agent run and saved result say, rendered as escaped HTML. Kept as is so its
// wording rules (never claim what was not recorded) and tests carry over.
import type { Graph, GraphNode } from "../types";

export interface CapabilityBinding {
  agent_id: string; version: string; card_digest: string; contract_digest: string;
}
export interface ExecutionBinding {
  kind: "agent_bus" | "host" | "loop_forest"; path_mode?: string; binding?: CapabilityBinding;
}
export interface GatewayExchange {
  rpc_id: string; method: string; started_at: string; finished_at: string;
  http_status?: number; outcome: string; error_code?: string; sequence: number;
}
export interface RemoteExecution {
  agent_id: string; task_id: string; status: string; model_calls: number;
  node_execution_id: string; attempt: number; request_id: string; context_id: string;
  input_digest: string; artifact_id: string; artifact_digest: string;
  agent_started_at: string; intent_saved_at?: string; committed_at?: string;
  reason?: string; reused_count?: number;
  discovery?: { checked_at: string; candidates: { instance_id: string; health: string; capability_version: string; contract_version: string; eligible: boolean }[] };
  gateway?: { source: string; protocol: string; path_mode: string; send_count: number; query_count: number;
    first_send?: GatewayExchange; first_receipt?: GatewayExchange; last_exchange?: GatewayExchange; last_error?: GatewayExchange };
}
export interface BusSummary {
  planned_nodes: number; services: number; linked_tasks: number; completed: number;
  unresolved: number; failed: number; reused: number;
}

const esc = (v: unknown) => String(v ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!);
const time = (value?: string) => value ? esc(new Date(value).toLocaleString("zh-CN", { hour12: false })) : "未采集";
const row = (name: string, value: unknown) => `<div class="cg-fact"><dt>${esc(name)}</dt><dd>${esc(value || "未采集")}</dd></div>`;
const timed = (name: string, value?: string) => `<div class="cg-fact"><dt>${esc(name)}</dt><dd><time title="${esc(value)}">${time(value)}</time></dd></div>`;
const stage = (i: number, name: string, state: string, tone: string, body: string) =>
  `<section class="cg-proof"><header><span class="cg-proof-number">${i}</span><h5>${esc(name)}</h5><span class="cg-proof-state ${tone}">${esc(state)}</span></header>${body}</section>`;
const stateLabel = (s?: string) => ({ pending: "尚未派发", running: "执行中", waiting: "等待结果", unknown: "待核对", done: "已完成", reused: "结果复用", failed: "失败", skipped: "未执行" }[s ?? ""] ?? "待批准");

export function executionLabel(n: GraphNode): string {
  if (n.execution?.kind === "agent_bus" || n.remote) return "Agent Bus · DIRECT";
  return n.execution?.kind === "loop_forest" || n.kind === "blackbox" ? "宿主内 · Loop Forest" : "宿主内执行";
}

export function busOverviewHTML(g: Graph): string {
  if (!g.bus) return "";
  const s = g.bus;
  const counts = g.run
    ? `关联任务 ${s.linked_tasks} · 已保存 ${s.completed} · 待核对 ${s.unresolved}${s.failed ? ` · 失败 ${s.failed}` : ""}${s.reused ? ` · 复用 ${s.reused}（包含在已保存中）` : ""}`
    : "待用户批准 · 尚未派发";
  return `<details class="cg-bus-list"><summary><span class="cg-bus-title">Agent Bus <b>DIRECT</b></span><span>RDC 能力绑定 · ${s.services} 类服务 · ${s.planned_nodes} 个远端节点</span><span class="cg-bus-counts">${esc(counts)}</span><span class="cg-bus-expand">调用清单</span></summary>
    <div class="cg-call-list">${g.nodes.filter(n => n.execution?.kind === "agent_bus").map(n => `<button type="button" data-inspect="${esc(n.id)}"><span>${esc(n.name)}</span><code>${esc(n.execution?.binding?.agent_id)}</code><span>${esc(stateLabel(n.status))}</span><span aria-hidden="true">→</span></button>`).join("")}</div>
    <p class="cg-proof-note">统计来自本任务的持久记录；实例健康仅表示发现时的快照。</p></details>`;
}

export function exchangeDescription(e?: GatewayExchange): string {
  if (!e) return "未采集";
  const state = ({ task_received: "已校验任务回执", http_error: "HTTP 调用失败", protocol_error: "协议错误", transport_error: "未收到 HTTP 回执", invalid_response: "回执未通过校验" } as Record<string, string>)[e.outcome] ?? "结果待核对";
  return `${e.method} · ${e.http_status ? `HTTP ${e.http_status} · ` : ""}${state}${e.error_code ? ` · ${e.error_code}` : ""}`;
}

export function associationText(n: GraphNode): string {
  const r = n.remote;
  return JSON.stringify({ node: n.id, execution: n.execution ? { kind: n.execution.kind, path_mode: n.execution.path_mode, binding: n.execution.binding ? { agent_id: n.execution.binding.agent_id, version: n.execution.binding.version, card_digest: n.execution.binding.card_digest, contract_digest: n.execution.binding.contract_digest } : undefined } : undefined, request_id: r?.request_id,
    task_id: r?.task_id, context_id: r?.context_id, node_execution_id: r?.node_execution_id,
    attempt: r?.attempt, input_digest: r?.input_digest, artifact_id: r?.artifact_id,
    artifact_digest: r?.artifact_digest }, null, 2);
}

export function executionDetailHTML(n: GraphNode): string {
  const r = n.remote, b = n.execution?.binding, d = r?.discovery, gw = r?.gateway;
  const remote = n.execution?.kind === "agent_bus" || !!r;
  const saved = !!r?.artifact_id && (n.status === "done" || n.status === "reused");
  const reused = n.status === "reused";
  const heading = `<header class="cg-detail-head"><div><h4 tabindex="-1">${esc(n.name)} · 执行详情</h4><p>${esc(executionLabel(n))} · ${esc(stateLabel(n.status))}</p></div><button type="button" class="cg-detail-close" data-detail-close aria-label="关闭执行详情">关闭 ×</button></header>`;
  let body = `<p class="cg-detail-task">${esc(n.task)}</p>`;
  if (!remote) {
    body += `<div class="cg-local-execution"><b>${esc(executionLabel(n))}</b><p>${n.kind === "blackbox" ? "在 Java 宿主的已授权节点内展开 Loop Forest。" : "由 Java Graph 宿主执行本节点。"}</p></div>`;
  } else {
    if (reused) body += `<p class="cg-detail-notice">本次读取已保存产物；下方保留原调用记录。累计复用 ${r?.reused_count ?? 1} 次。</p>`;
    if (r?.reason) body += `<p class="cg-detail-notice ${n.status === "unknown" || n.status === "failed" ? "warn" : ""}">${esc(r.reason)}</p>`;
    const candidates = d ? `<details data-detail-section="candidates"><summary>当时的候选实例（${d.candidates.length}）</summary><ul class="cg-candidates">${d.candidates.map(c => `<li><code>${esc(c.instance_id || "实例标识未采集")}</code><span>${esc(c.health)} · ${esc(c.capability_version)} · ${c.eligible ? "符合绑定" : "未纳入匹配"}</span></li>`).join("")}</ul><p class="cg-proof-note">此列表是发现快照，实际执行实例需另行回传。</p></details>` : `<p class="cg-proof-note">${r ? "该调用的发现快照未采集；能力版本取自已保存绑定。" : "能力已绑定，派发前将再次查询 RDC 并记录快照。"}</p>`;
    const discover = stage(1, "RDC 能力绑定", d ? "派发前已核对" : "已保存绑定", d ? "ok" : "muted",
      `<dl>${row("Agent ID", b?.agent_id ?? r?.agent_id)}${row("能力版本", b?.version)}${timed("查询时刻", d?.checked_at)}</dl>${candidates}`);
    const gateState = !r ? "尚未派发" : n.status === "unknown" ? "结果待核对" : n.status === "failed" ? "调用结束 · 失败" : gw?.first_receipt ? "已收到任务回执" : r?.task_id ? "已有任务关联" : r ? "回执待核对" : "尚未派发";
    const gateTone = n.status === "unknown" || n.status === "failed" ? "warn" : gw?.first_receipt ? "ok" : "muted";
    let gateBody = gw ? `<dl>${row("协议", gw.protocol)}${timed("首次派发尝试", gw.first_send?.started_at)}${timed("首次任务回执", gw.first_receipt?.finished_at)}${row("已记录请求", `派发尝试 ${gw.send_count} 次 · 原任务查询 ${gw.query_count} 次`)}${row("最近观测", exchangeDescription(gw.last_exchange))}${timed("最近观测时刻", gw.last_exchange?.finished_at)}</dl><p class="cg-proof-note">来源：Java 宿主观测。${!gw.first_send ? "原始派发观测未采集。" : ""}</p>`
      : `<p>${r ? "Gateway 请求与回执时间未采集。" : "执行后记录 Gateway 请求和有效任务回执。"}</p>`;
    if (gw?.last_error) gateBody += `<details data-detail-section="gateway-error"><summary>最近错误${saved ? "（后续已核对并保存结果）" : ""}</summary><p class="cg-proof-error">${esc(exchangeDescription(gw.last_error))}</p><p class="cg-proof-note">${time(gw.last_error.finished_at)}</p></details>`;
    const gateway = stage(2, "Gateway DIRECT", gateState, gateTone, gateBody);
    const runtime = stage(3, "专业 Agent 执行", saved ? "执行已完成" : r?.agent_started_at ? "已领取节点输入" : "暂无开始记录", saved ? "ok" : r?.agent_started_at ? "active" : "muted",
      `<dl>${row("逻辑 Agent", b?.agent_id ?? r?.agent_id)}${timed("开始记录", r?.agent_started_at)}${row("模型调用尝试", r ? `${r.model_calls} 次（已记录）` : "尚无记录")}${row("实际执行实例", "未采集")}</dl>`);
    const result = stage(4, "正式结果", reused ? "复用已保存产物" : saved ? "已持久保存" : "尚未提交", saved ? "ok" : "muted",
      saved ? `<dl>${timed("提交时间", r?.committed_at)}${row("产物标识", r?.artifact_id)}</dl><p class="cg-proof-note">输入、任务与上下文校验后持久提交。</p>` : `<p>正式结果提交成功后，下游节点才可消费。</p>`);
    body += `<div class="cg-proof-grid">${discover}${gateway}${runtime}${result}</div>
      <details class="cg-identifiers" data-detail-section="ids"><summary>关联标识与摘要</summary><dl>${row("节点执行 ID", r?.node_execution_id)}${row("Request ID", r?.request_id)}${row("Task ID", r?.task_id)}${row("Context ID", r?.context_id)}${row("Attempt", r?.attempt)}${row("能力卡摘要", b?.card_digest)}${row("契约摘要", b?.contract_digest)}${row("输入摘要", r?.input_digest)}${row("产物摘要", r?.artifact_digest)}${timed("派发意图落盘", r?.intent_saved_at)}</dl></details>
      <div class="cg-detail-actions"><button type="button" data-copy-evidence>复制关联信息</button><span role="status" data-copy-status></span><span class="cg-proof-note">时间按浏览器时区显示</span></div>`;
  }
  if (n.denied) body += `<p class="cg-detail-notice warn">${esc(n.denied)}</p>`;
  if (n.reads?.length || n.contract) body += `<details data-detail-section="permissions"><summary>读取权限与节点契约</summary><dl>${row("读取范围", n.reads?.join("、"))}${row("契约摘要", n.contract)}</dl></details>`;
  if (n.output) body += `<details data-detail-section="output"><summary>查看产物内容摘要</summary><pre class="cg-detail-output">${esc(n.output)}</pre></details>`;
  return `${heading}<div class="cg-detail-body">${body}</div>`;
}
