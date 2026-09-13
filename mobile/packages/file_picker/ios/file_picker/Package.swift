// swift-tools-version: 5.9

import PackageDescription

let package = Package(
    name: "file_picker",
    platforms: [
        .iOS("13.0")
    ],
    products: [
        .library(name: "file-picker", targets: ["file_picker"])
    ],
    dependencies: [
        .package(name: "FlutterFramework", path: "../FlutterFramework")
    ],
    targets: [
        .target(
            name: "file_picker",
            dependencies: [
                .product(name: "FlutterFramework", package: "FlutterFramework")
            ]
        )
    ]
)
