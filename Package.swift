// swift-tools-version: 6.0

import PackageDescription

let package = Package(
    name: "Chaos",
    platforms: [.macOS(.v15)],
    dependencies: [
        .package(url: "https://github.com/groue/GRDB.swift", from: "7.0.0")
    ],
    targets: [
        .executableTarget(
            name: "Chaos",
            dependencies: [
                .product(name: "GRDB", package: "GRDB.swift")
            ],
            path: "Chaos",
            exclude: ["Info.plist"],
            resources: [
                .process("Resources")
            ],
            swiftSettings: [
                .swiftLanguageMode(.v5)
            ]
        ),
        .testTarget(
            name: "ChaosTests",
            dependencies: ["Chaos"],
            path: "Tests/ChaosTests",
            swiftSettings: [
                .swiftLanguageMode(.v5)
            ]
        )
    ]
)
