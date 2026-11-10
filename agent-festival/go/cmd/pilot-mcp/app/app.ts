import { App } from "@modelcontextprotocol/ext-apps/app-with-deps";

type Req = { request_id: string; amount_minor: number; provider_status: string; attempts: unknown[]; observations: { applied: string }[]; cancel?: string };
type View = { task: { task_id: string; command_id: string; command_source: string; issuer: string; judgment: { kind: string }; request?: Req }; control_state?: string; conflicts: number };

const JUDGMENT: Record<string, string> = { deny: "拒绝", "grant-50000": "批 5 万美元", "grant-10000": "批 1 万美元", on_missing_evidence: "证据不足", on_undefined_rule: "规则未覆盖" };
const CONTROL: Record<string, [string, string]> = { adopted: ["已采纳", ""], dispatched: ["已派发", "wait"], unresolved: ["结果未知", "unknown"], completed: ["已终结", "ok"], failed: ["未生效", ""] };
const PROVIDER: Record<string, [string, string]> = { pending: ["业务方处理中", "wait"], approved: ["业务方已批准", "ok"], declined: ["业务方已拒绝", ""], cancelled: ["业务方已取消", ""], "": ["业务方状态未知", "unknown"] };
const RECEIPT: Record<string, [string, string]> = { accepted: ["已受理", "ok"], duplicate: ["重复命令，返回原回执", "ok"], conflict: ["冲突：同号不同内容，未生效", "bad"] };

const app = new App({ name: "credit-pilot-console", version: "0.1.0" }, {});
const el = (id: string) => document.getElementById(id) as HTMLElement;
let last: { id: string; args: Record<string, unknown> } | null = null;
let busy = false;

function newId(): string {
  const b = crypto.getRandomValues(new Uint8Array(6));
  return "app-" + Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("");
}

function formArgs(): Record<string, unknown> {
  const f = el("f") as HTMLFormElement;
  const v = (n: string) => (f.elements.namedItem(n) as HTMLInputElement);
  return {
    customer_id: v("customer_id").value,
    assets_wan: Number(v("assets_wan").value),
    longest_overdue_months: Number(v("longest_overdue_months").value),
    latest_overdue_months_ago: Number(v("latest_overdue_months_ago").value),
    asset_report_missing: !v("asset_report_ok").checked,
    scenario: (f.elements.namedItem("scenario") as HTMLSelectElement).value,
  };
}

function data(r: { structuredContent?: unknown; content?: { type: string; text?: string }[] }): any {
  if (r.structuredContent) return r.structuredContent;
  const t = r.content?.find((c) => c.type === "text")?.text;
  try { return t ? JSON.parse(t) : null; } catch { return null; }
}

function chip([text, cls]: [string, string]): string {
  return `<span class="chip ${cls}">${text}</span>`;
}

function showReceipt(d: any) {
  if (!d?.status) return;
  el("receipt").innerHTML = chip(RECEIPT[d.status] ?? [d.status, ""]) + (d.task_id ? ` <span class="muted">${d.task_id}</span>` : "");
}

function render(views: View[]) {
  el("tasks").innerHTML = views.map((v) => {
    const t = v.task, r = t.request;
    const req = r
      ? `${r.request_id} · ${(r.amount_minor / 100).toLocaleString()} USD ${chip(CONTROL[v.control_state ?? ""] ?? [v.control_state ?? "", ""])} ${chip(PROVIDER[r.provider_status] ?? [r.provider_status, ""])}
         <span class="muted">尝试 ${r.attempts.length} · 回执 ${r.observations.length}${r.observations.some((o) => o.applied !== "applied") ? "（含重复/迟到，已忽略）" : ""}${r.cancel ? " · 取消：" + r.cancel : ""}</span>
         ${r.provider_status === "pending" ? `<button data-cancel="${r.request_id}">申请取消</button>` : ""}`
      : `<span class="muted">无业务请求</span>`;
    return `<div class="task"><div class="row"><span class="id">${t.task_id}</span>${chip([JUDGMENT[t.judgment.kind] ?? t.judgment.kind, ""])}${req}</div>
      <div class="muted">命令 ${t.command_id}（${t.command_source}）· ${t.issuer}${v.conflicts ? ` · 冲突 ${v.conflicts}` : ""}</div></div>`;
  }).join("");
  el("tasks").querySelectorAll<HTMLButtonElement>("button[data-cancel]").forEach((b) => {
    b.onclick = async () => { await app.callServerTool({ name: "pilot_request_cancel", arguments: { request_id: b.dataset.cancel } }); refresh(); };
  });
}

async function refresh() {
  if (busy) return;
  busy = true;
  try {
    const d = data(await app.callServerTool({ name: "pilot_list_tasks", arguments: {} }));
    if (d?.tasks) render(d.tasks);
  } finally { busy = false; }
}

async function submit(id: string, args: Record<string, unknown>) {
  last = { id, args };
  el("cmd").style.display = "flex";
  el("cmdid").textContent = id;
  showReceipt(data(await app.callServerTool({ name: "pilot_submit_application", arguments: { ...args, command_id: id, command_source: "app" } })));
  refresh();
}

el("f").addEventListener("submit", (e) => { e.preventDefault(); submit(newId(), formArgs()); });
el("resend").onclick = () => { if (last) submit(last.id, last.args); };
el("tamper").onclick = () => { if (last) submit(last.id, { ...last.args, assets_wan: Number(last.args.assets_wan) + 1 }); };

app.ontoolresult = (p) => {
  const d = data(p as any);
  if (d?.tasks) render(d.tasks);
  showReceipt(d);
};

(async () => {
  await app.connect();
  refresh();
  setInterval(refresh, 3000);
})();
