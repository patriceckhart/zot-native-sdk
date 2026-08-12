# zot_native

Flutter bindings for the zot agent runtime on iOS, Android, macOS, Windows, and Linux. Mobile uses in-process native bindings. Desktop uses an application-packaged `zot-bridge` sidecar. Sessions support streaming, cancellation, provider API keys, Claude subscription OAuth, ChatGPT Codex subscription OAuth, and history persistence.

```dart
final session = await ZotNative.createSession(const ZotSessionOptions(
  provider: 'anthropic',
  apiKey: '...',
));
session.events.listen((event) {});
await session.prompt('Hello');
```

Use iOS 15 or later and Android API 24 or later. CocoaPods and Flutter's Swift Package Manager plugin mode are supported. On desktop, start `ZotDesktopClient` with the packaged sidecar path. OAuth login, refresh, and secure credential storage remain the application's responsibility.
