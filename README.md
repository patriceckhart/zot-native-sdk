# zot native SDK

Use the [zot](https://github.com/patriceckhart/zot) agent runtime from native application frameworks. The SDK supports streaming, cancellation, stateful conversations, API keys, and Claude or ChatGPT subscription OAuth credentials.

iOS, Android, and macOS embed zot in-process and do not require a backend when users provide their own credentials. React Native Windows, Linux, and web use the included `zot-gateway` WebSocket service because those runtimes cannot load the gomobile artifacts. Never embed developer-owned API keys or reusable OAuth credentials in a distributed application.

## Status

Version [`v0.0.1`](https://github.com/patriceckhart/zot-native-sdk/releases/tag/v0.0.1) is available from GitHub Releases. The multiplatform React Native implementation is complete, with these constraints:

- Packages and binaries are distributed through GitHub Releases, not npmjs, pub.dev, or crates.io.
- Generated XCFrameworks, Android JNI libraries, sidecars, and gateway executables are not committed to Git. Tagged releases build and attach them automatically.
- Live provider requests require externally supplied API keys or OAuth test credentials.
- React Native 0.82 iOS remains blocked by an upstream `fmt` compilation failure with Xcode 26 before the zot target compiles.
- Production applications must apply the appropriate platform signing, notarization, authentication, and credential-storage controls.

## Supported integrations

| Consumer | Package or artifact | Runtime |
|---|---|---|
| React Native iOS, Android, macOS | `@zot/native-react-native` | In-process XCFramework or Android JNI bindings |
| React Native Windows, Linux, web | `@zot/native-react-native` | WebSocket connection to `zot-gateway` |
| Flutter mobile | `zot_native` | In-process iOS or Android bindings |
| Flutter desktop | `ZotDesktopClient` from `zot_native` | Host-provided desktop sidecar |
| Electron | `@zot/native-electron` | Bundled desktop sidecar |
| Tauri 2 | `@zot/native-tauri` and `tauri-plugin-zot` | Host-provided release sidecar |
| Vercel Labs Native SDK | `@zot/native-sdk` | Optional replay-aware adapter for Vercel Labs Native SDK |
| Swift directly | `apple/Package.swift` | XCFramework |
| Kotlin or Java directly | `artifacts/zot.aar` | AAR |

The Go package in `bridge/` is the shared implementation. Mobile bindings are generated with `gomobile`. Desktop clients communicate with `cmd/zot-bridge` through newline-delimited JSON over standard input and output. Browser and remote React Native clients communicate with `cmd/zot-gateway` over WebSocket.

No filesystem, shell, or subprocess tools are registered in embedded zot sessions.

## Platform requirements

- iOS 15 or newer
- macOS 12 or newer
- Android API 24 or newer
- JDK 17 for Android builds
- Go 1.25 or newer
- Xcode for Apple artifacts
- Android SDK and NDK for building Android consumers and generated bindings
- Flutter SDK for Flutter validation
- Rust stable for the Tauri plugin
- Bun or Node.js 20 or newer for TypeScript packages
- CocoaPods when using Flutter or React Native CocoaPods integration

The generated binaries are architecture-specific. Sidecars and gateways support macOS, Linux, and Windows on arm64 and x64. Gomobile artifacts contain iOS, iOS simulator, macOS, and Android architectures for their respective consumers.

## Providers

API key sessions support:

- `anthropic`
- `openai`
- `openai-responses`
- `gemini`

OAuth sessions support:

- `anthropic` for Claude Pro or Max
- `openai-codex` for ChatGPT Plus or Pro

OpenAI Codex requires the ChatGPT account ID from the OAuth ID token. Use the platform adapter's `extractOpenAIAccountId` helper when needed.

OAuth browser authorization, callback handling, token refresh, revocation, account selection, and secure storage are responsibilities of the host application. Provider subscription routes may be restricted by provider terms or revoked without notice.

## Installation

All first-party packages and binaries are installed from the repository's GitHub Releases. Replace `v0.0.1` with the desired release.

### React Native

```sh
bun add https://github.com/patriceckhart/zot-native-sdk/releases/download/v0.0.1/zot-native-react-native-0.0.1.tgz
```

The archive contains the shared core, Apple XCFramework, Android JNI libraries, and Java bindings.

### Electron

```sh
bun add https://github.com/patriceckhart/zot-native-sdk/releases/download/v0.0.1/zot-native-electron-0.0.1.tgz
```

The archive contains all six macOS, Linux, and Windows sidecars.

### Tauri

```sh
bun add https://github.com/patriceckhart/zot-native-sdk/releases/download/v0.0.1/zot-native-tauri-0.0.1.tgz
```

Use the Rust plugin directly from GitHub and download the sidecar matching the target platform from the same release:

```toml
[dependencies]
tauri-plugin-zot = { git = "https://github.com/patriceckhart/zot-native-sdk", tag = "v0.0.1" }
```

### Flutter

Download and extract `zot-native-flutter-0.0.1.tar.gz`, then reference it as a path dependency:

```yaml
dependencies:
  zot_native:
    path: vendor/zot-native-flutter
```

The archive contains the staged iOS XCFramework and Android JNI bindings. A plain Git dependency cannot include generated native artifacts because they are intentionally excluded from Git.

### Swift

Download and extract `zot-native-swift-0.0.1.zip`, then add the extracted directory as a local Swift package. It contains `Package.swift` and `Zot.xcframework`.

### Android

Download `zot-native-android-0.0.1.aar` and add it as a Gradle file dependency:

```kotlin
dependencies {
    implementation(files("libs/zot-native-android-0.0.1.aar"))
}
```

### Gateway for React Native Windows, Linux, and web

Download the `zot-gateway` executable matching the server operating system and architecture from the release. For example:

```sh
curl -L -o zot-gateway \
  https://github.com/patriceckhart/zot-native-sdk/releases/download/v0.0.1/zot-gateway-linux-x64
chmod +x zot-gateway
ZOT_GATEWAY_TOKEN="replace-me" ./zot-gateway -addr 0.0.0.0:8787
```

Use the corresponding `.exe` asset on Windows. Put the gateway behind TLS and use `wss://` in production.

### Vercel Labs Native SDK

`@zot/native-sdk` is only for the separate Vercel Labs Native SDK runtime. Most users do not need it:

```sh
bun add https://github.com/patriceckhart/zot-native-sdk/releases/download/v0.0.1/zot-native-sdk-0.0.1.tgz
```

No zot package is published to npmjs, pub.dev, or crates.io.

## Basic usage

### React Native

```ts
import { createSession } from "@zot/native-react-native";

const session = await createSession({
  provider: "anthropic",
  apiKey,
  model: "claude-sonnet-4-5",
  systemPrompt: "You are concise.",
});

const unsubscribe = session.onEvent(event => {
  if (event.type === "text") console.log(event.delta);
  if (event.type === "error") console.error(event.message);
});

await session.prompt("Hello");
unsubscribe();
```

### React Native Windows, Linux, and web

These platforms use the same API through a gateway. Start the gateway on a trusted host:

```sh
ZOT_GATEWAY_TOKEN="replace-me" \
ZOT_GATEWAY_ORIGINS="https://app.example.com" \
go run ./cmd/zot-gateway -addr 0.0.0.0:8787
```

Use TLS in production, then configure the session:

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

The gateway defaults to `127.0.0.1:8787`, exposes WebSocket RPC at `/v1/zot`, and has a health endpoint at `/healthz`. Configure allowed browser origins with the comma-separated `ZOT_GATEWAY_ORIGINS` environment variable. Use authenticated, short-lived per-user gateway credentials in production.

### Electron

Create the client in the Electron main process, not in an unrestricted renderer:

```ts
import { createClient } from "@zot/native-electron";

const client = createClient();
const session = await client.createSession({
  provider: "anthropic",
  apiKey,
});

session.onEvent(event => {
  if (event.type === "text") process.stdout.write(event.delta);
});

await session.prompt("Hello");
```

When packaging with ASAR, sidecars must remain executable outside the archive. For electron-builder:

```json
{
  "build": {
    "asarUnpack": ["node_modules/@zot/native-electron/bin/**"]
  }
}
```

`bundledBinaryPath()` automatically resolves the matching `app.asar.unpacked` path.

### Flutter mobile

```dart
import 'package:zot_native/zot_native.dart';

final session = await ZotNative.createSession(
  ZotSessionOptions(provider: 'anthropic', apiKey: apiKey),
);

final subscription = session.events.listen((event) {
  if (event.type == 'text') {
    print(event.payload['delta']);
  }
});

await session.prompt('Hello');
```

### Flutter desktop

Flutter desktop does not register a native plugin automatically. Start the packaged sidecar explicitly:

```dart
final client = await ZotDesktopClient.start(sidecarPath);
final session = await client.createSession(
  ZotSessionOptions(provider: 'anthropic', apiKey: apiKey),
);
await session.prompt('Hello');
```

The host application must package the correct sidecar for its operating system and architecture and close `ZotDesktopClient` during shutdown.

### Tauri 2

Configure `tauri-plugin-zot` in Rust with the packaged sidecar path, then use `createSession` from `@zot/native-tauri` in the webview. See `packages/tauri/README.md` and `crates/tauri-plugin-zot/README.md` for the integration surface.

```rust
fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_zot::init("/path/to/zot-bridge".into()))
        .run(tauri::generate_context!())
        .expect("failed to run application");
}
```

Package one sidecar per target operating system and architecture. Do not accept a sidecar path from untrusted web content.

### Vercel Labs Native SDK

Native SDK uses `zot-bridge oneshot-native` through replay-aware `Cmd.spawn`. Its restricted TypeScript build does not read `node_modules`, so copy the adapter source into the application as documented in `packages/native-sdk/README.md`. Desktop is supported through the sidecar. Native SDK mobile consumers should use the XCFramework or Android bindings instead of subprocesses.

### Direct Swift

Download and extract `zot-native-swift-0.0.1.zip`, then add that directory as a local Swift package. Source builds can generate the same package with `make apple`:

```swift
import Zot

var error: NSError?
let session = ZotNewSession(
    "anthropic",
    apiKey,
    "",
    "You are concise.",
    &error
)
```

Conform to `ZotStreamProtocol`, call `prompt` away from the UI thread, and use `abort()` for cancellation. See `apple/README.md`.

### Direct Kotlin or Java

Download `zot-native-android-0.0.1.aar` from the GitHub Release and add it as an Android application dependency. Source builds can generate `artifacts/zot.aar` with `make android`:

```kotlin
import zot.Zot

val session = Zot.newSession("anthropic", apiKey, "", "You are concise.")
session.prompt("Hello", stream)
session.abort()
```

Java uses the generated `zot.Zot`, `zot.Session`, and `zot.Stream` classes. Run `prompt` on a worker thread. See `android/README.md`.

### Subscription OAuth

OAuth credentials use the same session API:

```ts
const session = await createSession({
  provider: "openai-codex",
  accessToken,
  accountId,
});
```

Do not log access tokens, ID tokens, account IDs, prompts, or transcripts unless the user has explicitly opted in.

## Cancellation and conversation persistence

Prompts stream events asynchronously. A prompt call acknowledges that the turn started, while output arrives through the event listener.

```ts
const unsubscribe = session.onEvent(event => {
  switch (event.type) {
    case "text":
      process.stdout.write(event.delta);
      break;
    case "history":
      saveTranscript(event.history);
      break;
    case "error":
      console.error(event.message);
      break;
    case "done":
      console.log("turn complete");
      break;
  }
});

await session.prompt("Explain this code");
await session.abort();

const history = await session.exportHistory();
await session.importHistory(history);

unsubscribe();
await session.close();
```

`abort()` is safe when no prompt is active. Prompt calls on the same in-process session are serialized. Exported history is provider-neutral JSON and imports are limited to 16 MiB.

## Building from source

Build the Windows, Linux, and web React Native gateway with `make gateway`, or run it directly with `go run ./cmd/zot-gateway`.

Install the required toolchains, then initialize `gomobile`:

```sh
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest
gomobile init
bun install
```

Run checks:

```sh
go test -race ./...
go vet ./...
bun run typecheck
bun run build
cargo fmt --manifest-path crates/tauri-plugin-zot/Cargo.toml -- --check
cargo check --locked --manifest-path crates/tauri-plugin-zot/Cargo.toml
cd packages/flutter && flutter analyze
```

Generate and stage every native artifact:

```sh
make stage
```

`make stage` performs all of the following:

- Builds iOS 15 and macOS 12 XCFrameworks.
- Builds the Android API 24 AAR.
- Stages XCFrameworks into the Swift, React Native, and Flutter packages.
- Expands the Android AAR into `zot.jar` and JNI libraries for React Native and Flutter.
- Builds and stages sidecars for macOS, Linux, and Windows on arm64 and x64.

Generated native artifacts are intentionally ignored by Git. A fresh clone is not directly consumable by native applications until `make stage` succeeds or release artifacts are installed. The original direct-consumer Android artifact remains at `artifacts/zot.aar`.

Useful individual targets are:

```sh
make test
make sidecar
make gateway
make desktop
make desktop-all
make apple
make android
```

## Desktop sidecar protocol

Requests and responses are one JSON object per line. `prompt` acknowledges immediately and then emits text, tool, usage, updated history, error, and completion events.

Supported methods are:

- `create_session`
- `prompt`
- `abort`
- `close_session`
- `import_history`
- `export_history`
- `extract_openai_account_id`

Diagnostics are written to standard error, never standard output. Input and history sizes are bounded. `oneshot-native` provides a deterministic hex-framed protocol for the restricted Vercel Labs Native SDK TypeScript subset.

## Validation status

The generated artifacts and adapters have been validated with:

- React Native 0.82 Android debug application
- Flutter Android debug application
- Flutter iOS simulator application through CocoaPods
- Flutter iOS 15 Swift Package Manager simulator application
- Vercel Labs Native SDK 0.8.4 subset and markup checker
- Packaged Electron application launched against its unpacked bundled sidecar
- Packaged Tauri 2 macOS application
- Direct Android AAR API inspection
- Direct Swift Package Manager consumer build and launch
- Sidecar protocol, lifecycle, history, malformed-input, and event-ordering tests
- Go race detector and `go vet`
- TypeScript type checks and builds
- Rust formatting, checking, and package verification
- Flutter analysis and pub.dev dry run with zero warnings
- npm package dry runs
- Cross-compilation of six desktop sidecars
- Authenticated WebSocket gateway protocol tests
- Gateway release builds for macOS, Linux, and Windows on arm64 and x64

The React Native iOS blocker is in the upstream React Native `fmt` dependency under Xcode 26. Zot's Swift bridge and generated simulator XCFramework compile successfully through both Flutter CocoaPods and Swift Package Manager consumers.

Live provider streaming has not been validated in this repository because no API or OAuth credentials were supplied.

## Security model

- Never hardcode provider credentials in source code or application bundles.
- Use Keychain, Android Keystore backed storage, or an equivalent platform credential store.
- Prefer user-owned credentials for direct-to-provider applications.
- Keep developer-owned credentials on a trusted backend.
- Keep OAuth login, refresh, revocation, and callback processing in the host application.
- Redact tokens, prompts, model responses, account IDs, and transcripts from logs and crash reports.
- Validate imported history before accepting it from untrusted sources.
- Expose only application-specific Electron or Tauri commands to web content.
- Apply platform signing, sandboxing, hardened runtime, and least-privilege permissions before distribution.
- Review provider terms before enabling subscription OAuth routes.

## Release checklist

Before declaring a production release:

1. Run all CI and consumer builds from a clean clone.
2. Run credentialed provider smoke tests using non-production test accounts.
3. Resolve or document the React Native iOS compatibility matrix.
4. Build native artifacts from the tagged commit.
5. Verify artifact checksums and minimum platform versions.
6. Sign and notarize desktop and Apple artifacts where required.
7. Sign Android release artifacts and applications where required.
8. Build JavaScript, Flutter, Swift, Android, sidecar, and gateway archives from the tagged commit.
9. Publish all generated files in a GitHub Release.
10. Verify every documented GitHub installation path.

## License

MIT
