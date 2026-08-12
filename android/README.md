# Direct Kotlin and Java integration

Run `make android` and add `artifacts/zot.aar` to the Android application's dependencies.

```kotlin
import zot.Zot

val session = Zot.newSession("anthropic", apiKey, "", "You are concise.")
session.prompt("Hello", stream)
session.abort()
```

Java uses the same generated `zot.Zot`, `zot.Session`, and `zot.Stream` classes. Run `prompt` on a worker thread.
