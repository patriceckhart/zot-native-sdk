# @zot/native-react-native

Use zot from one React Native API on iOS, Android, macOS, Windows, Linux, and web.

- iOS, Android, and macOS embed the Go runtime in-process.
- Windows, Linux, and web connect to `zot-gateway` over WebSocket.
- Streaming, cancellation, stateful history, API keys, and Claude or ChatGPT subscription OAuth are supported by both transports.

## Installation

Install the package archive directly from GitHub Releases. The shared core is bundled, so no second zot package is required:

```sh
bun add https://github.com/patriceckhart/zot-native-sdk/releases/download/v0.0.1/zot-native-react-native-0.0.1.tgz
```

Run CocoaPods for iOS or macOS, then rebuild the native application. Zot packages are not published to npmjs.

## Mobile and macOS

Native platforms do not require a gateway:

```ts
import { createSession } from "@zot/native-react-native";

const session = await createSession({
  provider: "anthropic",
  apiKey,
});

const unsubscribe = session.onEvent(event => {
  if (event.type === "text") console.log(event.delta);
});

await session.prompt("Hello");
```

Requires iOS 15, macOS 12, or Android API 24. Run CocoaPods after installation on Apple platforms and rebuild the native application.

## Windows, Linux, and web

Download the gateway executable matching the trusted server's operating system and architecture from GitHub Releases. For example:

```sh
curl -L -o zot-gateway \
  https://github.com/patriceckhart/zot-native-sdk/releases/download/v0.0.1/zot-gateway-linux-x64
chmod +x zot-gateway
ZOT_GATEWAY_TOKEN="replace-me" \
ZOT_GATEWAY_ORIGINS="https://app.example.com" \
./zot-gateway -addr 0.0.0.0:8787
```

Put it behind TLS and use `wss://` outside local development. Then provide the endpoint when creating a session:

```ts
const session = await createSession({
  provider: "anthropic",
  apiKey,
  gateway: {
    url: "wss://api.example.com/v1/zot",
    token: userGatewayToken,
  },
});
```

The gateway token is access control, not a secret in a distributed application. Use authenticated, short-lived per-user gateway credentials in production. Never embed a shared provider API key or reusable OAuth token in an application.

`prompt()` resolves when the turn is accepted. Output and completion are delivered through `onEvent`, including the final `done` event.

OAuth browser login, token refresh, revocation, and secure credential storage remain the host application's responsibility.
