# @zot-native/tauri

TypeScript client for `tauri-plugin-zot`. Configure the Rust plugin with the platform-specific `zot-bridge` sidecar path, then create sessions from the webview:

```ts
import { createSession } from "@zot-native/tauri";

const session = await createSession({ provider: "anthropic", apiKey });
const unsubscribe = session.onEvent(event => {
  if (event.type === "text") console.log(event.delta);
});
await session.prompt("Hello");
```

OAuth login, token refresh, and secure credential storage are responsibilities of the host application.
