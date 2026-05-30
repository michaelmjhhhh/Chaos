// swift-tools-version: 6.0

import PackageDescription

let package = Package(
    name: "VibeShot",
    platforms: [.macOS(.v15)],
    targets: [
        .executableTarget(
            name: "VibeShot",
            path: "VibeShot",
            exclude: ["Info.plist"],
            resources: [
                .process("Resources"),
            ],
            swiftSettings: [
                .swiftLanguageMode(.v5),
            ]
        ),
        .testTarget(
            name: "VibeShotTests",
            dependencies: ["VibeShot"],
            path: "Tests/VibeShotTests",
            swiftSettings: [
                .swiftLanguageMode(.v5),
            ]
        ),
    ]
)
