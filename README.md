# Chaos

A native macOS app that watches for screenshots, uses AI vision to generate descriptive filenames, and automatically renames and organizes them.

## What it does

1. **Watches** a directory (default: `~/Desktop`) for new macOS screenshots
2. **Sends** each screenshot to a vision AI model for analysis
3. **Renames** the file with a concise, descriptive slug (e.g. `terminal-git-log_143022.png`)
4. **Moves** it to your output directory
5. **Optionally** copies the image to your clipboard

You can also drop an existing image onto the dashboard to run it through the
same filing workflow.

All from a native SwiftUI menu bar app with a live dashboard, durable processing
history, and settings — no CLI required.

## Requirements

- macOS 15 (Sequoia) or later
- Xcode 16+ or Swift 6.0+ toolchain
- An API key from a supported vision provider

## Supported Providers

| Provider | Default Model | Base URL |
|----------|--------------|----------|
| **SiliconRouter** | `gemini-3-flash-preview` | `https://api.siliconrouter.com/v1` |
| **OpenAI** | `gpt-4o-mini` | `https://api.openai.com/v1` |
| **DeepSeek** | `deepseek-v4-flash` | `https://api.deepseek.com` |
| **OpenRouter** | `openai/gpt-4o-mini` | `https://openrouter.ai/api/v1` |
| **OpenAI-Compatible** | `gpt-4o-mini` | *(you must provide one)* |

## Quick Start

```bash
# Build
swift build

# Build .app bundle
./build-app.sh

# Launch
open .build/Chaos.app
```

Then:
1. Open **Settings** (Cmd+,)
2. Choose a provider and enter your API key
3. Set your watch and output directories
4. Click **Start Watching** on the dashboard
5. Take a screenshot — it gets renamed and moved automatically

## Preview Install

The preview build is available through the project Homebrew tap:

```bash
brew tap michaelmjhhhh/chaos
brew install --cask chaos
```

The current preview is distributed for Apple Silicon Macs and is not yet
Developer ID notarized. macOS may require you to approve the app manually in
**System Settings → Privacy & Security** after the first launch.

## Shared Config

Chaos reads and writes its config file at:

```
~/Library/Application Support/chaos/config.json
```

On first launch, Chaos copies an existing `vibe-shot` CLI config forward when a
Chaos config does not yet exist. The legacy config is left untouched.

## Features

- **Dashboard** — live status, processing metrics (success rate, avg/p95 latency), recent activity
- **History** — searchable table of all processed files with error filtering
- **Durable History** — keeps the latest 500 successful and failed attempts across launches
- **Retry** — retry a failed image from its Pipeline context menu
- **Drop Intake** — process existing PNG, JPEG, HEIC, and WebP files from the Dashboard
- **Organization** — filename templates plus optional daily or monthly output folders
- **Menu Bar** — always-on popover with quick start/stop, recent files, and output folder access
- **Settings** — provider picker, API key, model, directories, language (English/Chinese), clipboard toggle
- **Safety Guards** — only processes files matching macOS screenshot patterns (prefix, PNG header, size, recency)
- **Sound Feedback** — plays the Glass sound on successful processing
- **Single Instance** — macOS handles this natively (no lock files needed)

## Project Structure

```
Chaos/
├── ChaosApp.swift          # App entry — WindowGroup + MenuBarExtra + Settings
├── AppState.swift             # @Observable central state and business logic
├── Models/
│   ├── AppConfig.swift        # Codable config (shared JSON schema with CLI)
│   ├── Provider.swift         # Provider enum with defaults
│   ├── ProcessingEvent.swift  # Status and stage enums
│   └── RecentFile.swift       # Processed file record
├── Services/
│   ├── ConfigService.swift    # Load/save config.json
│   ├── DirectoryWatcher.swift # DispatchSource file system monitor
│   ├── ScreenshotGuard.swift  # Eligibility checks
│   ├── VisionAPIClient.swift  # OpenAI-compatible vision API
│   ├── SlugSanitizer.swift    # Filename slug cleanup
│   ├── FileProcessor.swift    # Pipeline orchestrator
│   ├── FileRenamer.swift      # Move with collision avoidance
│   ├── ClipboardService.swift # NSPasteboard image copy
│   └── SoundService.swift     # Glass sound feedback
└── Views/
    ├── ContentView.swift      # Tab container (Dashboard / History)
    ├── DashboardView.swift    # Live metrics and status
    ├── HistoryView.swift      # Processed files table
    ├── SettingsView.swift     # Preferences (Cmd+,)
    ├── MenuBarView.swift      # Menu bar popover + icon
    └── Theme.swift            # Design tokens (colors, type, spacing)
```

## Screenshot Matching

Chaos only processes files that look like freshly taken macOS screenshots:

- Filename starts with `Screenshot`, `屏幕快照`, or `截屏`
- Extension is `.png`
- Regular file, modified after the watcher started
- Size between 20 KB and 25 MB
- Valid PNG magic header bytes

> [!NOTE]
> Existing files in the watch directory are ignored — only new screenshots trigger processing.

## Output Format

The default template is:

```
{slug}_{time}
```

Templates support `{slug}`, `{date}`, and `{time}`. Chaos preserves the source
image extension. If a collision occurs, a suffix is appended before the
extension, up to 100 attempts.
