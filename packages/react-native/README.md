# @zot-native/react-native

Native iOS and Android bindings for zot with streaming, cancellation, provider API keys, and Claude or ChatGPT subscription OAuth credentials.

```ts
const session = await createSession({ provider: "anthropic", apiKey: "..." });
session.onEvent(event => {});
await session.prompt("Hello");
```

Requires iOS 15 or Android API 24. OAuth login, refresh, and secure credential storage remain the application's responsibility.
