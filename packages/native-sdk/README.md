# Zot for Vercel Labs Native SDK

A replay-safe adapter for Native SDK 0.8.4 and later. It uses `Cmd.spawn` with `zot-bridge oneshot-native`, so no Zig host customization is required on desktop.

Native SDK intentionally builds without reading `node_modules`. Copy `src/index.ts` from this package into the application as `src/zot.ts`, then import its pure encoder and line decoder.

```ts
import { Cmd, asciiBytes } from "@native-sdk/core";
import { encodeZotOneShot, decodeZotNativeLine } from "./zot.ts";

Cmd.spawn([asciiBytes("/path/to/zot-bridge"), asciiBytes("oneshot-native")], {
  key: "zot",
  stdin: encodeZotOneShot({
    provider: asciiBytes("anthropic"),
    apiKey: model.apiKey,
    accessToken: new Uint8Array(0),
    accountId: new Uint8Array(0),
    model: new Uint8Array(0),
    systemPrompt: asciiBytes("Be concise."),
    history: model.history,
    message: model.prompt,
  }),
  line: "zot_line",
  exit: "zot_exit",
  err: "zot_process_error",
});
```

In the `zot_line` message arm, call `decodeZotNativeLine(msg.line)`. Tags are 1 text, 2 zot event, 3 error, 4 updated history, and 5 done. Store tag 4's payload in the model and pass it into the next turn. Native SDK journals subprocess lines, so recording and replay remain deterministic.

The adapter and a representative model core pass `native check` with Native SDK 0.8.4. Desktop is supported. Native SDK mobile should use the linked XCFramework or Android library because mobile hosts cannot rely on subprocess sidecars.
