# tauri-plugin-zot

Tauri 2 plugin that manages the zot desktop sidecar and exposes session lifecycle, streaming events, cancellation, and transcript persistence. Pair it with `@zot-native/tauri`.

Pass the packaged platform-specific `zot-bridge` path to `tauri_plugin_zot::init`. Keep provider credentials in host-managed secure storage.
