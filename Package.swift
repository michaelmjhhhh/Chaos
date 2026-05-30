// swift-tools-version: 6.0

import PackageDescription

let package = Package(
    name: "Chaos",
    platforms: [.macOS(.v15)],
    targets: [
        .executableTarget(
            name: "Chaos",
            path: "Chaos",
            exclude: ["Info.plist"],
            resources: [
                .process("Resources"),
            ],
            swiftSettings: [
                .swiftLanguageMode(.v5),
            ]
        ),
        .testTarget(
            name: "ChaosTests",
            dependencies: ["Chaos"],
            path: "Tests/ChaosTests",
            swiftSettings: [
                .swiftLanguageMode(.v5),
            ]
        ),
    ]
)
