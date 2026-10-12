# Backend migration audit

Audit baseline: the original Swift/SwiftUI application and TypeScript hosted functions in this repository. The Go implementation is under `internal/core`; orchestration and native platform services are separate packages. This document records behavior and compatibility requirements, not an alternative implementation plan.

## Existing product behavior

Chaos organizes screenshots. It does not capture screens, track activity or idle time, run a local HTTP API, export reports, or register login items. “Start watching on launch” starts watching after the user opens the app.

- Folder watching defaults to `~/Desktop`, recognizes `Screenshot`, `屏幕快照` and `截屏` PNG names, requires a regular file, validates PNG magic and 20 KiB–25 MiB size, and excludes existing files.
- Explicit image intake accepts PNG, JPG/JPEG, HEIC and WebP, including uppercase extensions, through multiple selection and drag/drop. Explicit inputs bypass screenshot-name/date restrictions.
- Naming waits for file stability, prepares a small JPEG, calls the selected vision model, sanitizes an English/CJK slug, renders `{slug}`, `{date}` and `{time}`, and moves the original bytes into the configured directory. Day/month grouping and collision suffixes are supported.
- English and Simplified Chinese prompts are built in. A nonblank enabled custom prompt completely replaces the default system instruction. Empty custom text falls back to the language default.
- Providers are Chaos hosted, SiliconRouter, OpenAI, DeepSeek, OpenRouter, OpenAI-compatible and Ollama. Only the custom provider accepts an overridden base URL. Hosted and Ollama require no user key.
- Optional clipboard image copy, success sound and opted-in success/error notifications follow filing. Native services also provide file/folder dialogs, Finder reveal/open and accessibility announcements.
- History includes success and failed attempts, source and destination paths, timestamps and durations. The working list is the newest 500, but durable storage is unlimited. Retry, deduplicated retry-all, rename, revert and case-insensitive original/new/error search are required.
- Lifetime analytics include success/error totals, month comparison, daily heatmap, hourly activity, peak hour, current/longest streak, categories, file types, average/p95/fastest/slowest duration and 30 seconds estimated time saved per successful image. Session activity windows and live processing remain controller responsibilities.
- App preferences include notification/clipboard behavior, start-watching-on-launch, light/dark/system appearance, onboarding completion/replay and session number.

## Data compatibility

The Go version reads `~/Library/Application Support/chaos/history.sqlite` in place. It preserves the existing table, identifiers, GRDB migration metadata and index:

```sql
CREATE TABLE processed_images (
  id TEXT PRIMARY KEY,
  originalName TEXT NOT NULL,
  newName TEXT NOT NULL,
  path TEXT NOT NULL,
  sourcePath TEXT NOT NULL,
  timestamp DATETIME NOT NULL,
  duration DOUBLE NOT NULL,
  isError BOOLEAN NOT NULL,
  errorMessage TEXT,
  ext TEXT NOT NULL
);
CREATE INDEX idx_processed_images_timestamp ON processed_images(timestamp);
```

GRDB migration identifier is `v1_createProcessedImages`. Existing markers are preserved, and Go-created databases receive the same marker so the previous Swift release can reopen them during rollback. GRDB stores UTC timestamps as `yyyy-MM-dd HH:mm:ss.SSS`; this was verified in its checked-out Foundation date encoder. The importer accepts those timestamps and RFC3339 values. New rows retain the original format for compatibility. Error rows preserve empty destination fields and their original source path.

Legacy `history.json` has camelCase record fields. Executing the Swift synthesized encoder confirmed the result forms `{"success":{}}` and `{"error":{"_0":"message"}}`. Timestamps are seconds since 2001-01-01, requiring **978307200 seconds added** to obtain Unix time. The Go importer validates first, commits one transaction, retains original IDs and merges without duplicating existing rows. Corrupt input remains untouched. Backup naming never replaces an earlier `.migrated` file.

Configuration remains `~/Library/Application Support/chaos/config.json`; an absent config imports from `../vibe-shot/config.json`. Original snake_case keys remain supported:

```text
provider api_key model base_url watch_dir output_dir copy_to_clipboard
language filename_template subfolder_rule notify_on_complete appearance
use_custom_prompt custom_prompt
```

New `provider_keys` keeps distinct credentials when the user switches providers, preventing accidental transmission of another provider's key. Settings writes are atomic, synced, and mode 0600. Malformed JSON and persistence failures are returned to the controller instead of silently replacing user settings.

Native preferences requiring separate migration are `autoStart`, `hasCompletedOnboarding` and `chaos.sessionNumber` from `com.chaos.app`. Device identity compatibility requires SHA-256 of the exact UTF-8 string `chaos.device.v1` followed by `IOPlatformUUID`, lowercase hex. Keychain fallback uses service `com.chaos.app.device`, account `device-seed`. Native migration and identity are owned by the application/platform packages.

## Audited defects and implementation responses

| Original concern | Migration response |
| --- | --- |
| Watcher marked a name seen before its bytes were stable; small initial writes were lost forever. | Baseline is captured synchronously; pending files stabilize before validation and only then become handled. A new file reusing a name is recognized by identity and metadata. |
| Watcher used a two-second timestamp approximation to ignore existing files. | Explicit initial directory snapshot excludes every existing file, even a freshly created screenshot. |
| Revert can immediately feed a restored screenshot back into the watcher. | Suppression can be registered before destination publication and follows that file identity while a fallback copy completes. Directory identity comparison handles symlinks and macOS `/tmp` versus `/private/tmp` aliases. |
| Simultaneous watcher, retries and batches interleaved the single progress state. | Core exposes preparation plans; the controller owns one serialized work queue and deduplication. |
| Clipboard failure after move recorded a failure with no destination. | Core returns the completed move independently; clipboard, sound and notifications are controller postprocessing concerns. |
| Health checks consumed a hosted free name. | Hosted health uses the non-consuming usage endpoint. Other providers use a small explicit completion test and parse its result. |
| Move collision check could race; a direct Go `os.Rename` replacement would overwrite destinations. | Fully written temporary file plus no-clobber hard-link publication; O_EXCL copy fallback for filesystems without hard links. Both retain the source until the destination is synced. |
| Source could be replaced while moving. | Identity, size and modification time are checked before publication and again before source unlink. A late replacement remains untouched and yields a recoverable `MoveError` with destination. |
| Cross-volume behavior needed to survive migration. | Temporary copy in destination directory works across volumes; no EXDEV-dependent rename assumption. |
| Plain byte copying loses Finder metadata and changes Date Created. | macOS extended attributes, creation date, file mode and modification time are copied, including Finder tags and image metadata, where the filesystem supports them. Private OS-managed security markers remain OS-managed. |
| Filesystem writes preceded detached history persistence. | Move hooks run before exact destination publication, allowing a controller journal; history writes are synchronous and errors are surfaced. Inserts are idempotent by ID for recovery. |
| Malformed legacy JSON could be archived as empty history; existing backups were removed. | Strict decode/validation, transactional merge, preserved corrupt source and collision-free backups. |
| Model output could exceed filesystem filename limits. | Sanitization caps UTF-8 basename bytes, leaving room for extension and collision suffixes. |
| Missing/invalid images became empty base64; HEIC fallback was mislabeled PNG. | Explicit readable regular-file validation, bounded decoding, required valid image, correct JPEG MIME and native macOS HEIC conversion. |
| JPEG EXIF orientation and transparent images needed deliberate handling. | EXIF rotation/reflection is applied before resizing; transparent pixels are composited on white before JPEG encoding. |
| Credentials persisted without explicit permissions and were reused across providers. | Atomic 0600 files and per-provider credential preservation; provider switching clears unassociated keys. |
| Any nonempty 2xx body counted as healthy. | Parsed completion success or structured valid usage response is required. |
| File extensions appeared as analytics categories. | Tokenization strips the extension and filters all supported image extension tokens. |
| “Today” data could be truncated by the 500-item display limit. | Full-history insights contain explicit today totals/average/p95 independent of the UI list cap. |
| Month counting included future-month rows; DST grouping depended on SQLite localtime support. | Calendar month equality and explicit Go timezone conversion, with DST boundary tests. |

## Hosted proxy contract and audit handoff

The migrated hosted package must preserve these externally consumed contracts:

- `POST /api/chat/completions`, Bearer `<public-app-token>:<device-hash>`, pinned model, `stream:false`, `n:1`, 16-message cap, 512-token ceiling and `X-Chaos-Trial-Remaining`.
- `GET /api/usage` returns `used`, `limit`, `remaining`, `globalSpent`, `globalLimit`.
- Admin-only `POST /api/usage?global=1|true` additionally requires `x-admin-token` and resets device/optional global counters.
- Redis keys remain `dev:<hash>` and `budget:YYYY-MM` (UTC); monthly expiration is 35 days.
- Existing environment names remain `APP_TOKEN`, `ADMIN_TOKEN`, `AGNES_API_KEY`, `AGNES_BASE_URL`, `MODEL`, `FREE_LIMIT`, `GLOBAL_MONTHLY_LIMIT`, `UPSTASH_REDIS_REST_URL`, `UPSTASH_REDIS_REST_TOKEN`.

The original proxy checked quotas before an upstream request and incremented afterwards, so concurrent calls could exceed either limit. The replacement must reserve quotas atomically, handle failures consistently, bound requests/responses and upstream time, and allowlist cost-bearing payload fields. The public client token and client-provided device ID remain a weak gate; the global cap is essential. Hosted implementation/tests are separate from `internal/core`.

## Evidence and validation

The baseline contained 94 Swift test functions, including one watcher test that only covered a missing directory and no hosted concurrency tests. Go tests exercise behavior rather than mirroring class names:

- Existing GRDB schema/metadata and UTC milliseconds; insert/update/delete; Swift reference epoch and enum decoding; migration merge/idempotency; preserved corrupt input and older backups.
- 513 persisted records with a 500-item display cap, full today totals, categories/types, timing, empty analytics, month boundaries and a New York DST streak.
- Twelve concurrent same-basename moves without overwrite or content loss; revert collision; same-name no-op; journal failure before publication; late source replacement; unsupported-hardlink publication path; macOS extended-attribute and exact creation-date preservation.
- Fresh baseline exclusion, partially written PNG detection, recreated same name, invalid/non-image rejection, pre-publication revert suppression through directory aliases, cancellation and watcher restart.
- PNG resizing, white transparency backing, JPEG orientation transformation, corrupt/missing image rejection, fake-provider request payloads and response variants, friendly 401/402/404/429/503 recovery, cancellation, and hosted health without consuming a completion.
- Temporary-directory end-to-end image preparation → local fake vision server → naming/move, verifying original file bytes are unchanged.

Commands run for the core package: `go test -race ./internal/core` and `go vet ./internal/core`. UI, native notification/clipboard interactions, real HEIC fixtures, packed `.app` behavior and deployed-hosted concurrency require their corresponding integration evidence; a passing core suite alone does not claim those checks.

Dependency research: Context7 retrieved current `modernc.org/sqlite` documentation from its official pkg.go.dev reference. Version discovery used the Go module proxy: SQLite v1.60.1 and `golang.org/x/image` v0.47.0. No live user config, history, Keychain entry, clipboard, provider account or hosted trial was changed by core tests.
