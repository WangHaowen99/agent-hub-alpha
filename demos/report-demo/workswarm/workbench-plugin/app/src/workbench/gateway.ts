// The plugin page is served from the WorkSwarm origin, so it opens its own
// gateway WebSocket with the identity the WorkSwarm app stored, and calls the
// plugin's RPC methods over it: {type:"req"} out, {type:"res"} back.
type Waiter = { resolve: (v: unknown) => void; reject: (e: Error) => void; timer: number };

export class GatewayError extends Error {
  constructor(message: string, readonly code = "") { super(message); }
}

const stored = (key: string) => { try { return localStorage.getItem(key)?.trim() ?? ""; } catch { return ""; } };

export class Gateway {
  private ws?: WebSocket;
  private opening?: Promise<WebSocket>;
  private seq = 0;
  private readonly waiting = new Map<string, Waiter>();

  private url(): string {
    const q = new URLSearchParams();
    if (stored("jiuwenswarm:user_id")) q.set("user_id", stored("jiuwenswarm:user_id"));
    if (stored("jiuwenswarm.auth.sessionId")) q.set("auth_session", stored("jiuwenswarm.auth.sessionId"));
    const query = q.toString();
    return `${location.protocol === "https:" ? "wss" : "ws"}://${location.host}/ws${query ? `?${query}` : ""}`;
  }

  private open(): Promise<WebSocket> {
    if (this.ws?.readyState === WebSocket.OPEN) return Promise.resolve(this.ws);
    this.opening ??= new Promise<WebSocket>((resolve, reject) => {
      const ws = new WebSocket(this.url());
      ws.onopen = () => { this.ws = ws; this.opening = undefined; resolve(ws); };
      ws.onerror = () => { this.opening = undefined; reject(new GatewayError("连不上 WorkSwarm", "WS")); };
      ws.onclose = () => {
        if (this.ws === ws) this.ws = undefined;
        for (const [id, w] of this.waiting) { clearTimeout(w.timer); w.reject(new GatewayError("连接断开", "WS")); this.waiting.delete(id); }
      };
      ws.onmessage = (e) => {
        let m: { type?: string; id?: string; ok?: boolean; payload?: unknown; error?: string; code?: string };
        try { m = JSON.parse(String(e.data)); } catch { return; }
        if (m.type !== "res" || !m.id) return;
        const w = this.waiting.get(m.id);
        if (!w) return;
        this.waiting.delete(m.id);
        clearTimeout(w.timer);
        if (m.ok) w.resolve(m.payload);
        else w.reject(new GatewayError(m.error ?? "请求失败", m.code));
      };
    });
    return this.opening;
  }

  async call<T>(method: string, params: Record<string, unknown> = {}, timeoutMs = 20000): Promise<T> {
    const ws = await this.open();
    const id = `cw_${Date.now().toString(36)}_${++this.seq}`;
    return new Promise<T>((resolve, reject) => {
      const timer = window.setTimeout(() => { this.waiting.delete(id); reject(new GatewayError("请求超时", "TIMEOUT")); }, timeoutMs);
      this.waiting.set(id, { resolve: resolve as (v: unknown) => void, reject, timer });
      ws.send(JSON.stringify({ type: "req", id, method, params }));
    });
  }
}
