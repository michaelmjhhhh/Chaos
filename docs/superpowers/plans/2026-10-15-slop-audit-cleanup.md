# Slop Audit Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Resolve all 21 audited findings and create a verified pull request.

**Architecture:** Keep core/app/desktop/shell boundaries. Consolidate existing
primitives and model real domain states explicitly without generic frameworks.
Three workers own disjoint package trees; integration changes are owned separately.

**Tech Stack:** Go 1.27.2, MyGo v0.4.6, modernc SQLite; no dependency upgrades.

**Spec:** `docs/superpowers/specs/2026-10-15-slop-audit-cleanup-design.md`.

## Global Constraints

Keep Go 1.27.2 and MyGo v0.4.6; no new runtime dependencies. Keep bundle ID
`com.chaos.app`, config snake_case, SQLite/GRDB schema, Swift reference dates/enums,
full history, and 500-row UI limit. Preserve Preferences.SessionNumber and
Config.Appearance persistence. Preserve journal kind strings insert/update/delete,
prepublication durability, source identity/hash, no-clobber moves, ACL/metadata,
watcher suppression, serialized work, health freshness, and live snapshot ownership.
Never use personal data or real provider credentials in tests. Do not merge PR.

## Review Focus

- Unsupported saved provider or switching from it must never leak an old key.
- A directory edit while starting/rebinding must win; Stop/shutdown must also win.
- Successful settings saves must not erase pending journal errors or corrupt input.
- A same-path new image and late thumbnail completion must not restore stale state.
- Huge/negative legacy dates and 100k history rows must preserve migration/DST semantics.

## Shared interfaces and parallel ownership

- Core keeps Config.Resolve(home), ProviderByID(id), Processor.Prepare, AtomicWrite,
  NewID, Insights and RecentFile signatures except removing dead fields/API entries.
  Unknown ProviderByID IDs must not be treated as a valid default; empty IDs may default.
- Core defines `type RecoveryAction string` with RecoveryNone="", RecoverySettings="settings",
  RecoveryRetry="retry", RecoveryInternet="internet"; FriendlyError.Action uses it.
- App defines `type ProcessingStage string`: StageIdle="", StageAnalyzing="analyzing",
  StageRenaming="renaming", StageClipboard="clipboard", StageSuccess="success", StageError="error".
- App defines `type HealthStatus string`: HealthUnknown="N/A", HealthChecking="Checking…",
  HealthOK="OK", HealthFailed="FAIL". Snapshot.CurrentStage/APIStatus use these types;
  Snapshot.RecoveryAction uses core.RecoveryAction. Snapshot.StorageError remains string.
- App adds `DismissWarning()`; desktop interface/fake and banner action consume it.
- App removes Snapshot.Processed/Successes/Errors/AvgLatency/P95Latency/Latencies/
  HourlyThroughput/SuccessWindow/WatcherStartedAt/StartedAt/SessionNumber/HistoryRevision/DataDir.
  UI reads Insights.TodaySuccesses/TodayErrors/TodayAvgDuration instead.
- Desktop changes Actions.LoadThumbnail to `func(context.Context, string) ([]byte, error)`,
  removes Actions.ApplyAppearance, adds UI-thread `Desktop.Close()` to cancel owned workers.
  Shell adapts these exact signatures and calls Close before async controller shutdown.
- Each worker edits only its owned paths, does not commit/push, and reports covering
  test commands and RED/GREEN evidence. Coordinator commits and performs cross-package QA.

### Task 1: Core contracts, file API cleanup, and streaming analytics

**Files:** owns all `internal/core/*.go`, including existing/new tests.
**Interfaces:** produce core RecoveryAction constants above; keep remaining shared APIs stable.
**Audit items:** 1, 4, 7, 9 (core), 10 (core fields), 13, 14 (errors/actions), 15, 19.

- [ ] Add failing unknown-provider tests using captured fake requests: no health or
  generation request may be sent; switching to a known provider cannot transfer the key.
  Empty-provider defaults and retired-provider/known saved keys must retain documented behavior.
- [ ] Add out-of-range finite legacy date tests: no committed rows or archived source;
  retain valid negative fractional Swift dates. Run selected tests and record expected failures.
- [ ] Implement explicit provider lookup/validation and typed/sentinel domain errors;
  ensure public network methods validate unsafe configuration too. Remove Friendly substring
  classification for owned domain errors. Use small typed chat payload structures.
- [ ] Strengthen request tests for exact model, POST, JSON content type, text/image parts,
  MIME/Base64; exact parsed names. Test default-client redirects, oversize response and in-flight
  cancellation via local servers. Remove provider count and broad agnes/vercel substring bans.
- [ ] Delete MoveToWithHook, MoveFile/RenameFile/RevertFile wrappers, MoveError.Source,
  ResolvedConfig.Appearance, Processor.Process. Migrate rejection tests to Prepare and
  no-clobber/ACL tests to hook variants. Controller already owns real filing byte preservation.
- [ ] Add representative 1k/100k Insights benchmark before optimization (record allocations).
  Stream only needed fields into aggregation; do not retain full RecentFile or names arrays,
  release DB transaction before final sorting, avoid duplicate owned duration copies.
  Retain ComputeInsights behavior or refactor it through same accumulator for testability.
- [ ] Verify full-history >500, DST/month/streak/percentile semantics, targeted regressions,
  core race suite and benchmark after changes. Write task report; do not touch app/desktop/cmd.

### Task 2: Controller lifecycle, error ownership, and persistence cleanup

**Files:** owns all `internal/app/*.go`, including existing/new tests.
**Interfaces:** produce stage/health types, DismissWarning, reduced Snapshot above;
consume core RecoveryAction constants, AtomicWrite and NewID.
**Audit items:** 2, 3 (state), 6, 8, 10 (app), 11, 14 (job/journal kinds).

- [ ] Add deterministic regressions for SaveConfig during Starting/rebind and settings
  failure->repair->successful save. Keep a separate recovery error across successful saves.
  Record correct RED outcomes before implementing.
- [ ] Reconcile latest wanted directory even while Starting; preserve serialized worker
  and cancellation semantics. Track owned config/preferences/history/recovery errors with
  derived StorageError; clear only matching successful operation. Add DismissWarning.
- [ ] Move all RetryAll validation through asynchronous intake, preserving dedup and
  separate history attempts. Add real fail->retry tests for single/all, duplicates/missing paths.
- [ ] Delete dead/duplicated snapshot fields and writers; keep Preferences.SessionNumber.
  Replace app atomicWrite/newUUID with existing core helpers; preserve file modes/fsync.
- [ ] Add typed job/operation kinds and explicit unknown dispatch rejection, preserving
  serialized journal values. Add agreed stage/health types; consume typed recovery actions.
- [ ] Replace health sleep with bounded started/completed signals; deterministic watcher
  progress where needed. Keep at least one real ticker integration case.
- [ ] Test legacyPreferences with existing commandRunner fake (never real UserDefaults),
  one-time preference persistence and mutable snapshot slices/maps/pointers.
- [ ] Run app race/recovery suites and write report; no edits outside internal/app.

### Task 3: Native view state, thumbnail ownership, and useful UI checks

**Files:** owns all `internal/desktop/*.go`, including existing/new tests.
**Interfaces:** consume shared Snapshot/types/DismissWarning above; produce context-aware
Actions.LoadThumbnail and Desktop.Close; remove ApplyAppearance.
**Audit items:** 3 (banners), 5, 10 (UI), 12, 14 (UI consumers), 16 (appearance), 17, 18.

- [ ] Add failing controlled-loader tests for missing->new intake same path, successful
  same-path replacement, stale result, bounded queued work, and shutdown cancellation.
  Dispatch results on test-owned UI queue; no timing-only false-positive assertions.
- [ ] Replace pending bool with explicit entries/generations and a bounded worker queue.
  New intake/history identity invalidates paths; no file Stat/decode on UI thread.
  Desktop.Close cancels owned workers and late UI results cannot mutate closed state.
- [ ] Require injected loader with dispatcher; remove PrepareImage/Base64 fallback.
  Keep current production 640px PNG loader behavior via the new callback signature.
- [ ] Remove notice/banner title and duplicate metric projections; consume typed statuses
  and actions. Warning-only banners get a working Dismiss action. Drop direct appearance
  callback so persisted snapshots are the single shell effect path.
- [ ] Rename layout tests as smoke tests; assert important controls' bounds/visibility
  at 1000x800 and supported narrow width, plus welcome and tray action availability.
  Shared optional PNG writer independently creates directories and checks failures.
- [ ] Run desktop race tests plus standalone/shuffled artifact generation into fresh
  temp dirs. Report any real layout defect rather than weakening assertions. No cmd edits.

### Task 4: Shell integration, packaging metadata, and current documentation

**Files:** coordinator owns `cmd/chaos/*`, `scripts/package-preview-release.sh`,
`docs/migration/validation.md`, `README.md`, `CLAUDE.md`, this plan/spec.
**Interfaces:** adapt Task 3 loader/Close and reduced Snapshot; no package-tree conflicts.
**Audit items:** 16 (replay/shell), 20, 21 plus integration validation.

- [ ] Remove the duplicate native ReplayOnboarding invocation, retain ShowWelcome owner.
  Remove ApplyAppearance wiring, keep snapshot theme effect. Make thumbnail accept caller
  context with bounded decode timeout; cancel Desktop workers on shutdown/error exit.
- [ ] Validate/derive preview ZIP version from mygo.json before build work. Execute with
  mismatched/matching fixture metadata and controlled stub build commands; no text-policing test.
- [ ] Separate historical validation from current commands; remove Redis/current deleted
  service claims. Record new checks accurately and update CLAUDE historical-plans description.
- [ ] Verify production thumbnail resize/PNG output/cancel behavior with synthetic fixtures.
  Run full race suite, vet, staticcheck, deadcode, formatting, native production and Linux/
  Windows builds, packaged macOS app verification, and appropriate isolated smoke checks.
- [ ] Obtain independent task/spec+quality reviews, fix findings, then whole-branch review.
  Update plan outcomes and benchmark evidence. Commit logical scopes, push feature branch,
  create PR with concrete behavior/validation summary, link it using T3, verify thread link.
