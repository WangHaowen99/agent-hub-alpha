// A minimal view side of MCP Apps (protocol 2026-01-26): handshake, tool
// result, open-link and size reports. The official SDK bundles zod and the
// MCP client (~600 KB per card); over a thin public link every card paid for it.
const PROTOCOL = "2026-01-26";

interface Message { jsonrpc: "2.0"; id?: number | string; method?: string; params?: any; result?: any; error?: { message?: string } }
export interface ToolResult { content?: { type: string; text?: string }[]; structuredContent?: unknown; isError?: boolean }

export class Bridge {
  ontoolresult?: (r: ToolResult) => void;
  private next = 1;
  private readonly waiting = new Map<number | string, (m: Message) => void>();
  private readonly host: Window = window.parent;

  constructor() {
    window.addEventListener("message", (e) => {
      if (e.source !== this.host) return;
      const m = e.data as Message;
      if (!m || m.jsonrpc !== "2.0") return;
      if (m.method === undefined) {
        if (m.id !== undefined) { this.waiting.get(m.id)?.(m); this.waiting.delete(m.id); }
        return;
      }
      if (m.method === "ui/notifications/tool-result") this.ontoolresult?.(m.params as ToolResult);
      else if (m.id !== undefined) this.post({ jsonrpc: "2.0", id: m.id, result: {} }); // ping, teardown: acknowledge
    });
  }

  async connect(appInfo: { name: string; version: string }) {
    await this.request("ui/initialize", { appInfo, appCapabilities: {}, protocolVersion: PROTOCOL });
    this.post({ jsonrpc: "2.0", method: "ui/notifications/initialized", params: {} });
    this.reportSize();
  }

  openLink(url: string) {
    return this.request("ui/open-link", { url });
  }

  private request(method: string, params: unknown): Promise<unknown> {
    const id = this.next++;
    return new Promise((resolve, reject) => {
      this.waiting.set(id, (m) => (m.error ? reject(new Error(m.error.message ?? method)) : resolve(m.result)));
      this.post({ jsonrpc: "2.0", id, method, params });
    });
  }

  private post(m: Message) {
    this.host.postMessage(m, "*");
  }

  // Same measurement as the SDK: the document's max-content height.
  private reportSize() {
    let queued = false, width = 0, height = 0;
    const measure = () => {
      if (queued) return;
      queued = true;
      requestAnimationFrame(() => {
        queued = false;
        const root = document.documentElement, saved = root.style.height;
        root.style.height = "max-content";
        const h = Math.ceil(root.getBoundingClientRect().height);
        root.style.height = saved;
        const w = Math.ceil(window.innerWidth);
        if (w !== width || h !== height) {
          width = w; height = h;
          this.post({ jsonrpc: "2.0", method: "ui/notifications/size-changed", params: { width: w, height: h } });
        }
      });
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(document.documentElement);
    ro.observe(document.body);
  }
}
