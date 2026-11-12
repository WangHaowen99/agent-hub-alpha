// graph-flow.js: the plan card as a script both the MCP App and the confirm
// and run pages load from /static. It exposes window.CreditGraph and, on a
// page, draws every [data-graph] element from the URL it names.
import { useSyncExternalStore } from "react";
import { createRoot } from "react-dom/client";
import { ReactFlowProvider } from "@xyflow/react";
import type { Graph, Options } from "../types";
import { CreditGraph } from "./CreditGraph";
import { newer } from "../order";
import { ensureStyle } from "./styles";

export interface View { render(g: Graph): void; current(): Graph | undefined }

export function mount(el: HTMLElement, opts: Options = {}): View {
  ensureStyle(el.ownerDocument);
  let g: Graph | undefined, text = "";
  const subs = new Set<() => void>();
  const subscribe = (f: () => void) => { subs.add(f); return () => subs.delete(f); };
  function App() {
    const cur = useSyncExternalStore(subscribe, () => g);
    return cur ? <ReactFlowProvider><CreditGraph g={cur} opts={opts} /></ReactFlowProvider> : null;
  }
  createRoot(el).render(<App />);
  return {
    // Polls repeat the same snapshot most of the time: only a change renders.
    render(next) {
      if (!newer(g, next)) return;
      const t = JSON.stringify(next);
      if (t === text) return;
      text = t;
      g = next;
      subs.forEach((f) => f());
    },
    current: () => g,
  };
}

// startLive polls the graph while it can still change (awaiting approval or
// running) and stops by itself once the server stops handing out a live URL.
// resolve maps the links in each snapshot to where the service is reachable.
export function startLive(view: View, g: Graph, resolve = (u: string) => u, everyMs = 1200, immediate = false): () => void {
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

declare global { interface Window { CreditGraph: { mount: typeof mount; startLive: typeof startLive } } }
window.CreditGraph = { mount, startLive };

for (const el of document.querySelectorAll<HTMLElement>("[data-graph]")) {
  const view = mount(el, { page: true });
  fetch(el.dataset.graph!, { cache: "no-store" })
    .then((r) => r.json() as Promise<Graph>)
    .then((g) => { view.render(g); startLive(view, g); });
}
