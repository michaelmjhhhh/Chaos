# Audit cleanup design

The user approved implementing all 21 findings from the 15 October 2026 audit of
commit `9cb71b49b3b03840511cc2b83372dbe0327f826c`, followed by a pull request.
The complete local evidence is in the original checkout's
`artifacts/slop-audit-2026-10-15.md` and `artifacts/slop-audit-evidence/`.

## Outcome

Reject unknown provider identities without transmitting their credentials; retain
the latest watch-directory intent during startup/rebind; clear errors only when
their owning operation recovers; validate imported dates before changing history;
bound/cancel thumbnail work and invalidate reused image paths; run retry intake
off the UI thread. Remove abandoned APIs/state and consolidate durable writes,
IDs, thumbnail loading, and UI effects. Strengthen behavioral tests, stream
analytics over necessary fields, and correct current documentation/packaging.

## Binding constraints

- Keep Go 1.27.2 and MyGo v0.4.6; no new runtime dependencies.
- Preserve bundle ID `com.chaos.app`, config snake_case, SQLite/GRDB schema,
  Swift reference-date and enum migration, full history, and the 500-row UI limit.
- Preserve persisted/imported `Preferences.SessionNumber` and `Config.Appearance`.
- Preserve prepublication journal durability, source identity/hash verification,
  no-clobber operations, ACL/metadata, watcher suppression, and serialized work.
- Preserve journal kind strings `insert`, `update`, `delete` on disk.
- UI state stays on the UI thread; filesystem/network/image work stays off it.
- Tests use temporary directories, synthetic images, and local/fake transports;
  never read production UserDefaults or use a personal screenshot directory.
- Keep useful migration and safety tests; no tests policing removed source text.
- Create a PR; do not merge, deploy, or release.

## Scope mapping

1. Explicit provider lookup and credential isolation.
2. Reconcile desired watcher state through startup/rebind.
3. Owned settings/read/recovery errors and dismissible warnings.
4. Transaction-safe legacy timestamp range validation.
5. Thumbnail state, invalidation, bounded work, and teardown.
6. Asynchronous retry-all with existing intake validation.
7. Essential request payload and exact response assertions.
8. Completion-based concurrency tests.
9. Delete unused/test-only file and processing APIs; migrate useful tests.
10. Delete unused snapshot/UI/error/config projections; use Insights directly.
11. Share atomic writes and ID generation.
12. One context-aware production thumbnail loader.
13. Typed request structures.
14. Typed control-flow kinds/actions/stages and classified errors.
15. Stream analytics inputs; preserve timezone/full-history behavior; benchmark.
16. One owner for replay and appearance effects.
17. Honest render smoke tests with meaningful control bounds.
18. Independent screenshot artifact export.
19. Remove incidental provider-count/substrings assertions.
20. Separate historical validation from current executable checks.
21. Derive/check preview archive version against app metadata.

Additional targeted coverage: redirects/response limits/HTTP cancellation,
legacy preference import, retry behavior, and snapshot mutable-member ownership.

## Implementation choices

Retain current package boundaries. Public Config.Resolve(home), Processor.Prepare,
and core.AtomicWrite signatures remain stable. Snapshot.StorageError remains a
derived display string while private error categories own clearing. Small named
types replace dispatch strings; persisted representations remain unchanged.
Desktop requires an injected context-aware thumbnail loader when dispatch exists
and owns its worker lifetime. The shell cancels that lifetime before shutdown.
Analytics uses streaming aggregation rather than adding a persistent cache/schema.
Each substantive bug gets a regression that fails before its fix; deletion and
mechanical consolidation use existing behavior tests without removal assertions.
