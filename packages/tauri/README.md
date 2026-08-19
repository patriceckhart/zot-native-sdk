# @zot/native-tauri

TypeScript client for `tauri-plugin-zot`. Install the client from GitHub Releases:

```sh
bun add https://github.com/patriceckhart/zot-native-sdk/releases/download/v0.0.1/zot-native-tauri-0.0.1.tgz
```

Configure the Rust plugin from the `v0.0.1` Git tag and provide the platform-specific `zot-bridge` release sidecar path, then create sessions from the webview:

```ts
import { createSession } from "@zot/native-tauri";

const session = await createSession({ provider: "anthropic", apiKey });
const unsubscribe = session.onEvent(event => {
  if (event.type === "text") console.log(event.delta);
});
await session.prompt("Hello");
```

OAuth login, token refresh, and secure credential storage are responsibilities of the host application.
