<p align="center">
  <img src="resources/icon.png" width="92" alt="Chaos app icon">
</p>

<h1 align="center">Chaos</h1>

<p align="center">
  <strong>Turn a desktop full of anonymous screenshots into a searchable archive.</strong><br>
  A native macOS menu bar app that names and files your screenshots with a vision model.
</p>

<p align="center">
  <img alt="macOS 15+" src="https://img.shields.io/badge/macOS-15%2B-111111?style=flat-square&amp;logo=apple&amp;logoColor=white">
  <img alt="Apple Silicon" src="https://img.shields.io/badge/Apple_Silicon-native-111111?style=flat-square&amp;logo=apple&amp;logoColor=white">
  <img alt="Go 1.27.2" src="https://img.shields.io/badge/Go-1.27.2-00ADD8?style=flat-square&amp;logo=go&amp;logoColor=white">
  <img alt="MIT License" src="https://img.shields.io/badge/license-MIT-111111?style=flat-square">
</p>

<p align="center">
  <code>Screenshot 2026-05-30 at 14.23.45.png</code> → <code>terminal-git-log_143022.png</code>
</p>

<p align="center">
  <a href="docs/demo.mp4"><img src="docs/demo-preview.png" alt="Watch the 29-second Chaos demo"></a><br>
  <sub><a href="docs/demo.mp4">Watch the 29-second demo</a> · recorded on the previous Swift release; the workflow is unchanged.</sub>
</p>

---

## Contents

- [Features](#features)
- [Install](#install)
- [Quick start](#quick-start)
- [Providers](#providers)
- [How it works](#how-it-works)
- [Data and privacy](#data-and-privacy)
- [Development](#development)
- [License](#license)

## Features

- **Names by meaning.** A vision model describes each screenshot; Chaos turns that into a short, Spotlight-friendly filename.
- **Files automatically.** Renamed images land in your output folder, optionally grouped into daily or monthly subfolders.
- **Filename templates.** Combine `{slug}`, `{date}` and `{time}` into a consistent format.
- **Backlog intake.** Drop existing PNG, JPEG, HEIC or WebP files onto the dashboard to run them through the same flow.
- **Rename, undo and retry.** Correct a name, restore a file to where it came from, or reprocess failures — without ever overwriting a file.
- **Local history.** Full history is kept on disk; the latest 500 attempts are searchable in Pipeline.
- **Insights.** Activity calendar, streaks, categories, file types, latency and time saved.
- **Clipboard handoff.** Optionally copy the renamed image back to your clipboard.
- **Stays out of the way.** Lives in the menu bar and only touches new screenshots.

## Install

### Download

1. Download **[Chaos.dmg](https://github.com/michaelmjhhhh/Chaos/releases/latest/download/Chaos.dmg)** (or browse [all releases](https://github.com/michaelmjhhhh/Chaos/releases)).
2. Open the DMG and drag **Chaos** into **Applications**.
3. Open Chaos once. macOS will block it because the app is ad-hoc signed but not notarized.
4. Go to **System Settings → Privacy & Security**, click **Open Anyway** next to the Chaos message, then **Open**.

The **Open Anyway** step is only needed on first launch.

### Homebrew

```bash
brew tap michaelmjhhhh/chaos
brew install --cask chaos
```

> [!NOTE]
> Chaos requires an Apple Silicon Mac running macOS 15 or later.

## Quick start

1. Open **Settings** (`⌘,`).
2. Choose a [provider](#providers) and enter an API key if it needs one.
3. Pick the folder to watch and the folder where renamed images should go.
4. Click **Start Watching** on the dashboard.
5. Take a screenshot — Chaos names and files it.

## Providers

Chaos uses the OpenAI-compatible vision API, so any of these work:

| Provider | Default model | Base URL |
| --- | --- | --- |
| SiliconRouter | `gemini-3-flash-preview` | `https://api.siliconrouter.com/v1` |
| OpenAI | `gpt-4o-mini` | `https://api.openai.com/v1` |
| DeepSeek | `deepseek-v4-flash` | `https://api.deepseek.com` |
| OpenRouter | `openai/gpt-4o-mini` | `https://openrouter.ai/api/v1` |
| OpenAI-Compatible | `gpt-4o-mini` | your URL |
| Ollama | `qwen3-vl:2b` | `http://localhost:11434/v1` |

Each provider keeps its own API key; keys are never sent to another provider.

### Running locally with Ollama

Chaos does not bundle Ollama. Install and start it separately (`qwen3-vl` needs Ollama 0.12.7+), then:

```bash
ollama pull qwen3-vl:2b
```

Select **Ollama** in Settings. No API key is required.

## How it works

```text
New screenshot
      │
      ▼
Watcher ────────── waits until a new screenshot is fully written
      │
      ▼
Vision client ──── asks your model for a short description
      │
      ▼
Slug sanitizer ─── makes the description filesystem-safe
      │
      ▼
Journaled move ─── applies your template, files without overwriting, records history
```

Files already in the watched folder are ignored — only screenshots created after watching starts are processed. Images you drop onto the dashboard skip the screenshot-name check but follow the same pipeline.

## Data and privacy

Everything lives in `~/Library/Application Support/chaos/`:

| Path | Contents |
| --- | --- |
| `config.json` | Provider, API keys and filing settings (owner-only permissions) |
| `preferences.json` | Onboarding and launch preferences |
| `history.sqlite` | Processing history |
| `pending/` | File-move journal, reconciled on launch after a crash |

Images are sent only to the provider you configure. Chaos never overwrites an existing file and never removes a source that changed during processing.

<details>
<summary><strong>Upgrading from an older version</strong></summary>

- Existing Swift installs keep their configuration, SQLite history and bundle ID (`com.chaos.app`).
- Legacy `history.json` is imported with original timestamps and archived only after a successful import.
- Legacy UserDefaults (onboarding, session number, start-on-launch) are imported once into `preferences.json`.
- An existing `vibe-shot` CLI configuration is imported on first launch.
- The hosted free trial has been retired. Saved trial selections reset to SiliconRouter with an empty key — choose a provider and enter your own key, or use Ollama. Other keys and settings are preserved.

</details>

## Development

Chaos is written in Go with the [MyGo 0.4.6](https://github.com/egoist/mygo/tree/v0.4.6) native UI — no web frontend, JavaScript runtime, Swift or cgo. It needs Go 1.27.2 (fetched automatically by the Go toolchain).

```bash
make dev            # live development
make test           # unit, controller and native UI tests
make race           # race detector
make vet            # static analysis
make build          # desktop executable
./build-app.sh      # signed .build/Chaos.app
./package-dmg.sh    # .build/Chaos.dmg
```

Use an isolated profile for manual testing so your real settings and history stay untouched:

```bash
go run ./cmd/chaos --data-dir /tmp/chaos-preview
```

The source also builds for Linux and Windows, but macOS is the primary tested platform (HEIC conversion relies on macOS). More detail:

- [Framework audit](docs/migration/mygo-audit.md)
- [Feature parity](docs/migration/feature-parity.md)
- [Validation report](docs/migration/validation.md)

## License

[MIT](LICENSE). To report a security issue privately, see [SECURITY.md](SECURITY.md).
