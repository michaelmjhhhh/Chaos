# Chaos

A native macOS menu bar app that watches for screenshots, sends them to a vision AI model, and renames them with descriptive filenames — automatically.

`Screenshot 2026-05-30 at 14.23.45.png` → `terminal-git-log_143022.png`

<video src="docs/demo.mp4" autoplay loop muted playsinline></video>

## Install

```bash
brew tap michaelmjhhhh/chaos
brew install --cask chaos
```

Apple Silicon only. Not yet notarized — macOS will ask you to approve it in **System Settings → Privacy & Security** after first launch.

### Build from source

```bash
swift build
./build-app.sh
open .build/Chaos.app
```

Requires macOS 15+ and Swift 6.0+.

## Setup

1. Open **Settings** (Cmd+,)
2. Pick a provider and enter your API key
3. Set your watch and output directories
4. Hit **Start Watching** on the dashboard
5. Take a screenshot — it gets renamed and filed automatically

## Providers

| Provider | Default Model | Base URL |
|----------|--------------|----------|
| **SiliconRouter** | `gemini-3-flash-preview` | `https://api.siliconrouter.com/v1` |
| **OpenAI** | `gpt-4o-mini` | `https://api.openai.com/v1` |
| **DeepSeek** | `deepseek-v4-flash` | `https://api.deepseek.com` |
| **OpenRouter** | `openai/gpt-4o-mini` | `https://openrouter.ai/api/v1` |
| **OpenAI-Compatible** | `gpt-4o-mini` | *(you provide the URL)* |

## Features

- **Menu bar** — always-on popover with start/stop, recent files, and output folder access
- **Dashboard** — live status, success rate, avg/p95 latency, recent activity
- **History** — searchable table of all processed files with error filtering; persists the latest 500 across launches
- **Drop intake** — drag an existing PNG, JPEG, HEIC, or WebP onto the dashboard to process it
- **Organization** — filename templates (`{slug}_{time}`) plus optional daily or monthly output folders
- **Clipboard** — optionally copies the renamed image to your clipboard
- **Safety guards** — only touches files matching macOS screenshot patterns (prefix, PNG header, size, recency)
- **Sound feedback** — plays the Glass sound on success
- **Language** — English or Chinese filename generation

## How it works

1. **DirectoryWatcher** monitors your watch folder via `DispatchSource` file system events
2. **ScreenshotGuard** validates the file — right prefix (`Screenshot`, `屏幕快照`, `截屏`), PNG header, 20 KB–25 MB, created after the watcher started
3. **VisionAPIClient** sends the image to your configured model
4. **SlugSanitizer** cleans the response into a filesystem-safe slug
5. **FileRenamer** moves it to your output directory with collision avoidance

Existing files in the watch directory are ignored — only new screenshots trigger processing.

## Config

```
~/Library/Application Support/chaos/config.json
```

On first launch, Chaos migrates an existing `vibe-shot` CLI config if one exists.

## Project structure

```
Chaos/
├── ChaosApp.swift             # App entry — WindowGroup + MenuBarExtra + Settings
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
    ├── PipelineView.swift     # Processed files table
    ├── SettingsView.swift     # Preferences (Cmd+,)
    ├── MenuBarView.swift      # Menu bar popover + icon
    └── Theme.swift            # Design tokens (colors, type, spacing)
```
