export type ApiKeyProvider = "anthropic" | "openai" | "openai-responses" | "gemini";
export type OAuthProvider = "anthropic" | "openai-codex";

export interface CommonOptions {
  model?: string;
  systemPrompt?: string;
}

export interface ApiKeyOptions extends CommonOptions {
  provider: ApiKeyProvider;
  apiKey: string;
}

export interface OAuthOptions extends CommonOptions {
  provider: OAuthProvider;
  accessToken: string;
  accountId?: string;
}

export type SessionOptions = ApiKeyOptions | OAuthOptions;

export type ZotEvent =
  | { type: "text"; delta: string }
  | { type: "tool_call" | "tool_progress" | "tool_result" | "turn_start" | "turn_end" | "usage"; payload: unknown }
  | { type: "error"; message: string }
  | { type: "history"; history: string }
  | { type: "done" };

export interface ZotSession {
  readonly id: string;
  prompt(message: string): Promise<void>;
  abort(): Promise<void>;
  close(): Promise<void>;
  exportHistory(): Promise<string>;
  importHistory(history: string): Promise<void>;
  onEvent(listener: (event: ZotEvent) => void): () => void;
}
