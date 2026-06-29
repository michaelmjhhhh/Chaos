import XCTest
@testable import Chaos

final class AppearancePreferenceTests: XCTestCase {
    func testFromParsesKnownValues() {
        XCTAssertEqual(AppearancePreference.from("light"), .light)
        XCTAssertEqual(AppearancePreference.from("dark"), .dark)
        XCTAssertEqual(AppearancePreference.from("system"), .system)
    }

    func testFromDefaultsToSystemForNilOrUnknown() {
        XCTAssertEqual(AppearancePreference.from(nil), .system)
        XCTAssertEqual(AppearancePreference.from(""), .system)
        XCTAssertEqual(AppearancePreference.from("sepia"), .system)
    }

    func testAllCasesAreSelectableAndLabeled() {
        XCTAssertEqual(AppearancePreference.allCases, [.system, .light, .dark])
        XCTAssertEqual(AppearancePreference.light.label, "Light")
    }
}
