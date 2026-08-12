import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { createInterface } from "node:readline";
import { randomUUID } from "node:crypto";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import type { SessionOptions, ZotEvent, ZotSession } from "@zot-native/core";
export type { SessionOptions, ZotEvent, ZotSession } from "@zot-native/core";

type Listener = (event: ZotEvent) => void;

export class ZotClient {
  private child: ChildProcessWithoutNullStreams;
  private nextId = 1;
  private pending = new Map<number, { resolve(value: any): void; reject(error: Error): void }>();
  private listeners = new Map<string, Set<Listener>>();
  private closed = false;

  constructor(binaryPath: string) {
    this.child = spawn(binaryPath, [], { stdio: ["pipe", "pipe", "pipe"] });
    createInterface({ input: this.child.stdout }).on("line", line => this.receive(line));
    this.child.stderr.resume();
    this.child.on("exit", code => this.failAll(new Error(`zot bridge exited with code ${code}`)));
    this.child.on("error", error => this.failAll(error));
  }

  async createSession(options: SessionOptions): Promise<ZotSession> {
    const id = randomUUID();
    await this.call("create_session", {
      session_id: id,
      provider: options.provider,
      model: options.model ?? "",
      system_prompt: options.systemPrompt ?? "",
      ...( "apiKey" in options
        ? { api_key: options.apiKey }
        : { access_token: options.accessToken, account_id: options.accountId ?? "" })
    });
    return new DesktopSession(this, id);
  }

  async extractOpenAIAccountId(idToken: string): Promise<string> {
    const value = await this.call("extract_openai_account_id", { id_token: idToken });
    return String(value.account_id ?? "");
  }

  call(method: string, params: Record<string, unknown> = {}): Promise<any> {
    if (this.closed) return Promise.reject(new Error("zot bridge is closed"));
    const id = this.nextId++;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.child.stdin.write(JSON.stringify({ id, method, ...params }) + "\n", error => {
        if (!error) return;
        const pending = this.pending.get(id);
        if (!pending) return;
        this.pending.delete(id);
        pending.reject(error);
      });
    });
  }

  subscribe(sessionId: string, listener: Listener): () => void {
    const set = this.listeners.get(sessionId) ?? new Set<Listener>();
    set.add(listener);
    this.listeners.set(sessionId, set);
    return () => set.delete(listener);
  }

  close(): void {
    if (this.closed) return;
    this.closed = true;
    this.failAll(new Error("zot bridge is closed"));
    this.listeners.clear();
    this.child.stdin.end();
    this.child.kill();
  }

  private receive(line: string): void {
    let message: any;
    try { message = JSON.parse(line); } catch { return; }
    if (message.id != null) {
      const pending = this.pending.get(message.id);
      if (!pending) return;
      this.pending.delete(message.id);
      message.error ? pending.reject(new Error(message.error)) : pending.resolve(message.result);
      return;
    }
    const listeners = this.listeners.get(message.session_id);
    if (!listeners) return;
    const event: ZotEvent = message.event === "text"
      ? { type: "text", delta: message.payload.delta }
      : message.event === "error"
        ? { type: "error", message: message.payload.message }
        : message.event === "history"
          ? { type: "history", history: message.payload.history }
        : message.event === "done"
          ? { type: "done" }
          : { type: message.event, payload: message.payload };
    listeners.forEach(listener => listener(event));
  }

  private failAll(error: Error): void {
    this.closed = true;
    this.pending.forEach(pending => pending.reject(error));
    this.pending.clear();
  }
}

export function bundledBinaryPath(): string {
  const extension = process.platform === "win32" ? ".exe" : "";
  const platform = process.platform === "win32" ? "windows" : process.platform === "darwin" ? "darwin" : "linux";
  const packageDirectory = join(dirname(fileURLToPath(import.meta.url)), "..");
  const unpackedDirectory = packageDirectory.replace(`${process.platform === "win32" ? "\\" : "/"}app.asar${process.platform === "win32" ? "\\" : "/"}`, `${process.platform === "win32" ? "\\" : "/"}app.asar.unpacked${process.platform === "win32" ? "\\" : "/"}`);
  return join(unpackedDirectory, "bin", `zot-bridge-${platform}-${process.arch}${extension}`);
}

export const createClient = (binaryPath = bundledBinaryPath()): ZotClient => new ZotClient(binaryPath);

class DesktopSession implements ZotSession {
  constructor(private client: ZotClient, readonly id: string) {}
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
