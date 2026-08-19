# @zot-native/react-native

Use zot from one React Native API on iOS, Android, macOS, Windows, Linux, and web.

- iOS, Android, and macOS embed the Go runtime in-process.
- Windows, Linux, and web connect to `zot-gateway` over WebSocket.
- Streaming, cancellation, stateful history, API keys, and Claude or ChatGPT subscription OAuth are supported by both transports.

## Installation

The package is not published to npm yet. Build the repository first:

```sh
bun install
bun run build
make stage
```

Then add both local packages to the React Native application:

```sh
bun add /absolute/path/to/zot-native-sdk/packages/core
bun add /absolute/path/to/zot-native-sdk/packages/react-native
```

Run CocoaPods for iOS or macOS, then rebuild the native application. Once published, installation will be `bun add @zot-native/react-native`.

## Mobile and macOS

Native platforms do not require a gateway:

```ts
import { createSession } from "@zot-native/react-native";

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

Build and start a gateway on a trusted server:

```sh
make gateway
```

```sh
ZOT_GATEWAY_TOKEN="replace-me" \
ZOT_GATEWAY_ORIGINS="https://app.example.com" \
go run ./cmd/zot-gateway -addr 0.0.0.0:8787
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
