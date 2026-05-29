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
    ]
)
