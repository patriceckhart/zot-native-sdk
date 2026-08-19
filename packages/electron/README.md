# @zot/native-electron

Electron main-process client for the zot sidecar. Release packages contain sidecars for macOS, Linux, and Windows on arm64 and x64.

```ts
const client = createClient();
const session = await client.createSession({ provider: "anthropic", apiKey });
session.onEvent(event => {});
await session.prompt("Hello");
```

Expose only the application-specific methods needed by the renderer through Electron context isolation. Do not expose credentials or this complete client over an unrestricted renderer bridge.

When packaging with ASAR, unpack the executable sidecars so the operating system can launch them. For electron-builder:

```json
{
  "build": {
    "asarUnpack": ["node_modules/@zot/native-electron/bin/**"]
  }
}
```

`bundledBinaryPath()` automatically resolves the corresponding `app.asar.unpacked` path.
