// MCP App entry: WorkSwarm hands over the tool result; the graph comes in
// structuredContent, then keeps itself current through the live URL. Its
// links are paths on the WorkSwarm origin (the front proxy serves the credit
// service under /credit), resolved against the host page in a srcdoc iframe.
import { Bridge } from "./bridge";
import { GraphView, startLive, type Graph } from "./graph";

// Where the service is now; it writes this into the document when serving it.
const home = new URL(document.querySelector<HTMLMetaElement>('meta[name="credit-base"]')?.content ?? "", document.baseURI);
const prefix = home.pathname.replace(/\/$/, "");

// Tool results stay in chat history and keep the address the service had
// when they were written. Every link is one of this service's paths, so keep
// the path and put it under today's address.
function here(url: string): string {
  const u = new URL(url, home);
  const path = !prefix || u.pathname.startsWith(`${prefix}/`) ? u.pathname : prefix + u.pathname;
  return new URL(path + u.search, home).href;
}

const rebase = (g: Graph): Graph => ({
  ...g,
  live: g.live && here(g.live),
  confirm: g.confirm && here(g.confirm),
  approve: g.approve && here(g.approve),
  run: g.run && { ...g.run, url: here(g.run.url) },
});

const app = new Bridge();
let stop: (() => void) | undefined;

async function approve(g: Graph, user: string, password: string): Promise<string | undefined> {
  const res = await fetch(here(g.approve!), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ plan: g.plan, version: g.version, hash: g.hash, user, password }),
  });
  if (res.ok) return undefined;
  const body = await res.json().catch(() => ({}));
  return body.error ?? `批准失败（${res.status}）`;
}

const view = new GraphView(document.getElementById("root")!, {
  approve,
  // Pick up the new run at once instead of waiting for the next poll.
  onApproved: () => { const g = view.current(); if (g) { stop?.(); stop = startLive(view, g, here, 1200, true); } },
});

function show(g: Graph) {
  stop?.();
  g = rebase(g);
  view.render(g);
  stop = startLive(view, g, here);
}

// Hosts that drop structuredContent: rebuild the graph URL from the text.
async function fromText(text: string): Promise<Graph | undefined> {
  const v = JSON.parse(text);
  const q = v.plan_id && v.version ? `plan=${v.plan_id}&v=${v.version}` : v.run_id ? `run=${v.run_id}` : "";
  if (!q) return undefined;
  const res = await fetch(here(`/api/graph?${q}${v.checks ? "&verify=1" : ""}`), { cache: "no-store" });
  return res.ok ? ((await res.json()) as Graph) : undefined;
}

app.ontoolresult = async (r) => {
  const g = r.structuredContent as Graph | undefined;
  if (g?.nodes) return show(g);
  const text = r.content?.find((c) => c.type === "text")?.text;
  if (text) {
    const fetched = await fromText(text).catch(() => undefined);
    if (fetched) show(fetched);
  }
};

void app.connect({ name: "credit-plan-graph", version: "0.2.0" });
