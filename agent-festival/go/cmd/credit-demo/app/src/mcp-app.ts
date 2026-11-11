// MCP App entry: WorkSwarm hands over the tool result; the graph comes in
// structuredContent, then keeps itself current through the live URL.
import { App } from "@modelcontextprotocol/ext-apps/app-with-deps";
import { GraphView, startLive, type Graph } from "./graph";

const app = new App({ name: "credit-plan-graph", version: "0.1.0" }, {});
const view = new GraphView(document.getElementById("root")!, { openLink: (url) => void app.openLink({ url }) });
let stop: (() => void) | undefined;

function show(g: Graph) {
  stop?.();
  view.render(g);
  stop = startLive(view, g);
}

// Hosts that drop structuredContent: rebuild the graph URL from the text.
async function fromText(text: string): Promise<Graph | undefined> {
  const v = JSON.parse(text);
  const link: string | undefined = v.confirm_url ?? v.run_url ?? v.verify_url;
  if (!link) return undefined;
  const origin = new URL(link).origin;
  const q = v.plan_id && v.version ? `plan=${v.plan_id}&v=${v.version}` : v.run_id ? `run=${v.run_id}` : "";
  if (!q) return undefined;
  const res = await fetch(`${origin}/api/graph?${q}${v.checks ? "&verify=1" : ""}`, { cache: "no-store" });
  return res.ok ? ((await res.json()) as Graph) : undefined;
}

app.ontoolresult = async (r) => {
  const g = r.structuredContent as Graph | undefined;
  if (g?.nodes) return show(g);
  const text = r.content?.find((c) => c.type === "text");
  if (text && "text" in text) {
    const fetched = await fromText(text.text).catch(() => undefined);
    if (fetched) show(fetched);
  }
};

void app.connect();
