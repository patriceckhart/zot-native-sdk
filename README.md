# zot native sdk

Use the [zot](https://github.com/patriceckhart/zot) agent runtime from native application frameworks. The SDK supports provider API keys and Claude or ChatGPT subscription OAuth credentials without requiring an application backend.

## Packages

| Consumer | Package | Runtime |
|---|---|---|
| React Native | `@zot-native/react-native` | In-process XCFramework or AAR |
| Flutter | `zot_native` | In-process mobile bindings or desktop sidecar |
| Electron | `@zot-native/electron` | Bundled desktop sidecar |
| Tauri 2 | `@zot-native/tauri` and `tauri-plugin-zot` | Bundled desktop sidecar |
| Vercel Labs Native SDK | `@zot-native/native-sdk` | Replay-aware `Cmd.spawn` sidecar |
| Swift directly | `apple/Package.swift` | XCFramework |
| Kotlin or Java directly | `artifacts/zot.aar` | AAR |

The Go package in `bridge/` is the single source of truth. Mobile bindings are generated with `gomobile`. Desktop clients communicate with `cmd/zot-bridge` using newline-delimited JSON over standard input and output.

## Providers

API key sessions support `anthropic`, `openai`, `openai-responses`, and `gemini`. OAuth sessions support `anthropic` for Claude Pro/Max and `openai-codex` for ChatGPT Plus/Pro. OpenAI Codex also needs the ChatGPT account ID from the OAuth ID token.

```ts
const session = await createSession({
  provider: "anthropic",
  apiKey: "...",
  model: "claude-sonnet-4-5",
  systemPrompt: "You are concise.",
});

session.onEvent(event => {
  if (event.type === "text") process.stdout.write(event.delta);
});
await session.prompt("Hello");
```

Subscription credentials use the same API shape:

```ts
await createSession({
  provider: "openai-codex",
  accessToken,
  accountId,
});
```

## Build

Requirements are Go 1.25+, Node or Bun, and `gomobile` for mobile artifacts.

```sh
go mod tidy
go install golang.org/x/mobile/cmd/gomobile@latest
go install golang.org/x/mobile/cmd/gobind@latest
gomobile init
make test
make sidecar
make apple
make android
```

Stage generated mobile artifacts into every adapter before local consumption or publishing:

```sh
make stage
```

This copies the XCFramework into the Swift, React Native, and Flutter packages. It also expands the Android AAR into its Java classes and JNI libraries so Android Gradle can package the native code correctly. The original `artifacts/zot.aar` remains available for direct Kotlin and Java use.

## Desktop protocol

Requests and responses are one JSON object per line. `prompt` acknowledges immediately and then emits `text`, tool, usage, updated history, error, and `done` events. Methods are `create_session`, `prompt`, `abort`, `close_session`, `import_history`, `export_history`, and `extract_openai_account_id`. Diagnostics go to stderr, never stdout. `oneshot-native` provides a line-framed, replay-safe protocol for Vercel Labs Native SDK.

## Validation status

The generated artifacts and adapters are validated in real consumer builds:

- React Native 0.82 Android debug application
- Flutter Android debug application
- Flutter iOS simulator application through CocoaPods
- Flutter iOS 15 Swift Package Manager simulator application
- Vercel Labs Native SDK 0.8.4 subset and markup checker
- Packaged Electron application launch against its unpacked bundled sidecar
- Packaged Tauri 2 macOS application
- Direct Android AAR API inspection and Swift XCFramework package inspection
- Go tests, `go vet`, TypeScript checks, Flutter analysis, and package dry runs

React Native's full iOS sample currently encounters an upstream React Native 0.82 `fmt` compile failure with Xcode 26 before the zot target is compiled. The same Zot Swift bridge and generated simulator framework compile successfully through Flutter CocoaPods and Swift Package Manager consumers. Live provider validation still requires externally supplied API or OAuth test credentials. Publishing requires registry credentials and an external repository release; local artifacts are staged and directly consumable.

## Security

Never hardcode credentials in a distributed app. Prefer user-provided credentials and platform secure storage. The SDK accepts OAuth tokens but intentionally does not own browser login, refresh, or secure storage. Subscription routes may be subject to provider terms and revocation. No filesystem, shell, or subprocess tools are registered in zot sessions.

## License

MIT
