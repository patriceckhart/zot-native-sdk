// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "zot_native",
    platforms: [.iOS(.v15)],
    products: [.library(name: "zot-native", targets: ["zot_native"])],
    dependencies: [.package(name: "FlutterFramework", path: "../FlutterFramework")],
    targets: [
        .binaryTarget(name: "Zot", path: "Zot.xcframework"),
        .target(
            name: "zot_native",
            dependencies: [
                "Zot",
                .product(name: "FlutterFramework", package: "FlutterFramework")
            ]
        )
    ]
)
