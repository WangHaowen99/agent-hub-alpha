// Page entry: every [data-graph] element draws the graph served at its URL.
import { GraphView, startLive, type Graph } from "./graph";

for (const el of document.querySelectorAll<HTMLElement>("[data-graph]")) {
  const view = new GraphView(el, { page: true });
  fetch(el.dataset.graph!, { cache: "no-store" })
    .then((r) => r.json() as Promise<Graph>)
    .then((g) => { view.render(g); startLive(view, g); });
}
