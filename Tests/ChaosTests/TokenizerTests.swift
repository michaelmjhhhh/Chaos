import XCTest
@testable import Chaos

final class TokenizerTests: XCTestCase {
    func testExtractsTopNounsByFrequency() {
        let slugs = [
            "terminal-git-log",
            "terminal-vim-config",
            "settings-dialog",
            "login-screen",
            "login-error"
        ]
        let top = Tokenizer.topNouns(from: slugs, limit: 3)
        // terminal and login both appear twice; the third slot goes to a
        // one-off token. Order between the two-counts is implementation-defined.
        let topSet = Set(top.prefix(2))
        XCTAssertEqual(topSet, Set(["terminal", "login"]))
        XCTAssertEqual(top.count, 3)
    }

    func testStripsStopwords() {
        let slugs = ["the-and-of-terminal", "the-the-and"]
        let top = Tokenizer.topNouns(from: slugs, limit: 5)
        XCTAssertFalse(top.contains("the"))
        XCTAssertFalse(top.contains("and"))
        XCTAssertFalse(top.contains("of"))
        XCTAssertTrue(top.contains("terminal"))
    }

    func testStripsShortTokens() {
        let slugs = ["a-b-c-terminal"]
        let top = Tokenizer.topNouns(from: slugs, limit: 5)
        XCTAssertFalse(top.contains("a"))
        XCTAssertFalse(top.contains("b"))
        XCTAssertEqual(top.first, "terminal")
    }

    func testStripsNumericSuffixes() {
        let slugs = ["screenshot_143022", "terminal_120000"]
        let top = Tokenizer.topNouns(from: slugs, limit: 5)
        XCTAssertFalse(top.contains("143022"))
        XCTAssertFalse(top.contains("120000"))
    }

    func testEmptyInputReturnsEmptyArray() {
        XCTAssertEqual(Tokenizer.topNouns(from: [], limit: 5), [])
    }

    func testHonorsLimit() {
        let slugs = ["alpha-beta-gamma-delta-epsilon"]
        let top = Tokenizer.topNouns(from: slugs, limit: 2)
        XCTAssertEqual(top.count, 2)
    }
}
