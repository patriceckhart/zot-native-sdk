// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "ZotNative",
    platforms: [.iOS(.v15), .macOS(.v12)],
    products: [.library(name: "Zot", targets: ["Zot"])],
    targets: [.binaryTarget(name: "Zot", path: "Zot.xcframework")]
)
