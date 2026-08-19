import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import type { SessionOptions, ZotEvent, ZotSession } from "@zot/native-core";
export type { SessionOptions, ZotEvent, ZotSession } from "@zot/native-core";

export async function createSession(options: SessionOptions): Promise<ZotSession> {
  const id = await invoke<string>("plugin:zot|create_session", { options });
  return {
    id,
    prompt: message => invoke("plugin:zot|prompt", { sessionId: id, message }),
    abort: () => invoke("plugin:zot|abort", { sessionId: id }),
    close: () => invoke("plugin:zot|close_session", { sessionId: id }),
    exportHistory: () => invoke<string>("plugin:zot|export_history", { sessionId: id }),
    importHistory: history => invoke("plugin:zot|import_history", { sessionId: id, history }),
    onEvent(listener) {
      let disposed = false;
      let dispose = () => { disposed = true; };
      void listen<{ sessionId: string; event: ZotEvent }>("zot:event", value => {
        if (!disposed && value.payload.sessionId === id) listener(value.payload.event);
      }).then(unlisten => {
        if (disposed) unlisten();
        else dispose = () => {
          disposed = true;
          unlisten();
        };
      });
      return () => dispose();
    }
  };
}

export const extractOpenAIAccountId = (idToken: string) =>
  invoke<string>("plugin:zot|extract_openai_account_id", { idToken });
