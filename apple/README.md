# Direct Swift integration

Run `make apple`, copy `artifacts/Zot.xcframework` beside `Package.swift`, then add the `apple` directory as a local Swift package. Release automation will publish the same XCFramework as a remote Swift package artifact.

```swift
import Zot

var error: NSError?
let session = ZotNewSession("anthropic", apiKey, "", "You are concise.", &error)
```

Conform to `ZotStreamProtocol`, call `session.prompt` from a background task, and use `session.abort()` to cancel.
