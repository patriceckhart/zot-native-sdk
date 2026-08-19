import type { GatewayOptions, SessionOptions, ZotEvent, ZotSession } from "@zot-native/core";

type Listener = (event: ZotEvent) => void;
type Pending = { resolve(value: any): void; reject(error: Error): void };
type SocketMessage = { data: unknown };
type SocketLike = {
  readonly readyState: number;
  onopen: (() => void) | null;
  onmessage: ((event: SocketMessage) => void) | null;
  onerror: (() => void) | null;
  onclose: (() => void) | null;
  send(data: string): void;
  close(): void;
};
type SocketConstructor = new (url: string) => SocketLike;

const clients = new Map<string, GatewayClient>();
let sequence = 0;

function identifier(): string {
  sequence += 1;
  return `${Date.now().toString(36)}-${sequence.toString(36)}-${Math.random().toString(36).slice(2)}`;
}

function clientFor(gateway: GatewayOptions): GatewayClient {
  const key = `${gateway.url}\n${gateway.token ?? ""}`;
  let client = clients.get(key);
  if (!client) {
    client = new GatewayClient(gateway);
    clients.set(key, client);
  }
  return client;
}

export async function createRemoteSession(options: SessionOptions): Promise<ZotSession> {
  if (!options.gateway?.url) {
    throw new Error("A gateway URL is required on web, Windows, and Linux. Set options.gateway.url to a zot-gateway WebSocket endpoint.");
  }
  return clientFor(options.gateway).createSession(options);
}

export async function extractRemoteOpenAIAccountId(idToken: string, gateway?: GatewayOptions): Promise<string> {
  if (!gateway?.url) throw new Error("A gateway URL is required to extract an OpenAI account ID on this platform.");
  const value = await clientFor(gateway).call("extract_openai_account_id", { id_token: idToken });
  return String(value.account_id ?? "");
}

class GatewayClient {
  private socket?: SocketLike;
  private connecting?: Promise<SocketLike>;
  private nextId = 1;
  private pending = new Map<number, Pending>();
  private listeners = new Map<string, Set<Listener>>();

  constructor(private readonly gateway: GatewayOptions) {}

  async createSession(options: SessionOptions): Promise<ZotSession> {
    const id = identifier();
    await this.call("create_session", {
      session_id: id,
      provider: options.provider,
      model: options.model ?? "",
      system_prompt: options.systemPrompt ?? "",
      ...( "apiKey" in options
        ? { api_key: options.apiKey }
        : { access_token: options.accessToken, account_id: options.accountId ?? "" })
    });
    return new RemoteSession(this, id);
  }

  async call(method: string, params: Record<string, unknown> = {}): Promise<any> {
    const socket = await this.connect();
    const id = this.nextId++;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      try {
        socket.send(JSON.stringify({ id, method, rpc_token: this.gateway.token ?? "", ...params }));
      } catch (error) {
        this.pending.delete(id);
        reject(error instanceof Error ? error : new Error(String(error)));
      }
    });
  }

  subscribe(sessionId: string, listener: Listener): () => void {
    const listeners = this.listeners.get(sessionId) ?? new Set<Listener>();
    listeners.add(listener);
    this.listeners.set(sessionId, listeners);
    return () => {
      listeners.delete(listener);
      if (listeners.size === 0) this.listeners.delete(sessionId);
    };
  }

  private connect(): Promise<SocketLike> {
    if (this.socket?.readyState === 1) return Promise.resolve(this.socket);
    if (this.connecting) return this.connecting;
    const Socket = (globalThis as unknown as { WebSocket?: SocketConstructor }).WebSocket;
    if (!Socket) return Promise.reject(new Error("WebSocket is unavailable in this JavaScript runtime."));
    this.connecting = new Promise((resolve, reject) => {
      const socket = new Socket(this.gateway.url);
      socket.onopen = () => {
        this.socket = socket;
        this.connecting = undefined;
        resolve(socket);
      };
      socket.onmessage = event => this.receive(event.data);
      socket.onerror = () => {
        if (this.connecting) {
          this.connecting = undefined;
          reject(new Error("Could not connect to the zot gateway."));
        }
      };
      socket.onclose = () => {
        this.socket = undefined;
        this.connecting = undefined;
        this.failAll(new Error("The zot gateway connection closed."));
      };
    });
    return this.connecting;
  }

  private receive(raw: unknown): void {
    if (typeof raw !== "string") return;
    let message: any;
    try { message = JSON.parse(raw); } catch { return; }
    if (message.id != null) {
      const pending = this.pending.get(message.id);
      if (!pending) return;
      this.pending.delete(message.id);
      message.error ? pending.reject(new Error(String(message.error))) : pending.resolve(message.result);
      return;
    }
    const listeners = this.listeners.get(String(message.session_id));
    if (!listeners) return;
    const event: ZotEvent = message.event === "text"
      ? { type: "text", delta: String(message.payload?.delta ?? "") }
      : message.event === "error"
        ? { type: "error", message: String(message.payload?.message ?? "Unknown error") }
        : message.event === "history"
          ? { type: "history", history: String(message.payload?.history ?? "[]") }
          : message.event === "done"
            ? { type: "done" }
            : { type: message.event, payload: message.payload };
    listeners.forEach(listener => listener(event));
  }

  private failAll(error: Error): void {
    this.pending.forEach(pending => pending.reject(error));
    this.pending.clear();
  }
}

class RemoteSession implements ZotSession {
  constructor(private readonly client: GatewayClient, readonly id: string) {}
  async prompt(message: string): Promise<void> {
    await this.client.call("prompt", { session_id: this.id, message });
  }
  async abort(): Promise<void> {
    await this.client.call("abort", { session_id: this.id });
  }
  async close(): Promise<void> {
    await this.client.call("close_session", { session_id: this.id });
  }
  async exportHistory(): Promise<string> {
    const value = await this.client.call("export_history", { session_id: this.id });
    return String(value.history);
  }
  async importHistory(history: string): Promise<void> {
    await this.client.call("import_history", { session_id: this.id, history });
  }
  onEvent(listener: Listener): () => void {
    return this.client.subscribe(this.id, listener);
  }
}
