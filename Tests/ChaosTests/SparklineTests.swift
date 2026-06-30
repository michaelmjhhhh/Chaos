import XCTest
@testable import Chaos

final class SparklineTests: XCTestCase {
    func testSummaryReportsNoDataWhenInsufficientPoints() {
        XCTAssertEqual(Sparkline(values: [], caption: "Latency").accessibilitySummary, "No data yet.")
        XCTAssertEqual(Sparkline(values: [1.0], caption: "Latency").accessibilitySummary, "No data yet.")
        // All-equal values have no visible trend, so they read as no data.
        XCTAssertEqual(Sparkline(values: [2.0, 2.0], caption: "Latency").accessibilitySummary, "No data yet.")
    }

    func testSummaryDescribesTrendAndLatest() {
        let up = Sparkline(values: [1.0, 2.0, 3.0], caption: "Latency", lastValueText: "3.0s")
        XCTAssertEqual(up.accessibilitySummary, "latest 3.0s, 3 points, trending up.")

        let down = Sparkline(values: [3.0, 2.0, 1.0], caption: "Latency", lastValueText: "1.0s")
        XCTAssertEqual(down.accessibilitySummary, "latest 1.0s, 3 points, trending down.")
    }

    func testSummaryOmitsLatestWhenNotProvided() {
        let s = Sparkline(values: [1.0, 5.0], caption: "Throughput")
        XCTAssertEqual(s.accessibilitySummary, "2 points, trending up.")
    }
}
