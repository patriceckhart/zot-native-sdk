import { NativeEventEmitter, NativeModules } from "react-native";
import type { SessionOptions, ZotEvent, ZotSession } from "@zot-native/core";

const NativeZot = NativeModules.ZotNative;
if (!NativeZot) throw new Error("ZotNative is not linked. Rebuild the native application.");
const emitter = new NativeEventEmitter(NativeZot);

export async function createNativeSession(options: SessionOptions): Promise<ZotSession> {
  const id: string = await NativeZot.createSession(options);
  return {
    id,
    prompt: message => NativeZot.prompt(id, message),
    abort: () => NativeZot.abort(id),
    close: () => NativeZot.closeSession(id),
    exportHistory: () => NativeZot.exportHistory(id),
    importHistory: history => NativeZot.importHistory(id, history),
    onEvent(listener) {
      const subscription = emitter.addListener("ZotEvent", (value: { sessionId: string; event: ZotEvent }) => {
        if (value.sessionId !== id) return;
        const event = value.event as ZotEvent & { payload?: unknown };
        if (typeof event.payload === "string") {
          try { event.payload = JSON.parse(event.payload); } catch { /* preserve non-JSON payloads */ }
        }
        listener(event);
      });
      return () => subscription.remove();
    }
  };
}

export const extractNativeOpenAIAccountId = (idToken: string): Promise<string> =>
  NativeZot.extractOpenAIAccountId(idToken);
