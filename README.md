<p align="center">
  <img src="resources/icon.png" width="92" alt="Chaos app icon">
</p>

<h1 align="center">Chaos</h1>

<p align="center">
  <strong>Turn a desktop full of anonymous screenshots into a searchable archive.</strong>
</p>

<p align="center">
  A native macOS menu bar app that watches for screenshots, asks a vision model what they contain,<br>
  and files them under useful names while you keep working.
</p>

<p align="center">
  <img alt="macOS 15+" src="https://img.shields.io/badge/macOS-15%2B-111111?style=flat-square&amp;logo=apple&amp;logoColor=white">
  <img alt="Apple Silicon" src="https://img.shields.io/badge/Apple_Silicon-native-111111?style=flat-square&amp;logo=apple&amp;logoColor=white">
  <img alt="Go 1.27.2" src="https://img.shields.io/badge/Go-1.27.2-00ADD8?style=flat-square&amp;logo=go&amp;logoColor=white">
  <img alt="MyGo native UI" src="https://img.shields.io/badge/MyGo-native_UI-111111?style=flat-square">
</p>

<p align="center">
  <code>Screenshot 2026-05-30 at 14.23.45.png</code> → <code>terminal-git-log_143022.png</code>
</p>

<p align="center">
  <a href="docs/demo.mp4">
    <img src="docs/demo-preview.png" alt="Watch the 29-second Chaos demo">
  </a>
</p>

<p align="center">
  <strong><a href="docs/demo.mp4">Watch the 29-second demo →</a></strong>
</p>

The recording shows the previous Swift release; this branch implements the same
workflow with MyGo's native Go UI. Build from source to try the migrated app.

## Your screenshots should not become a second inbox

Screenshots are effortless to capture and surprisingly painful to retrieve. A week later, the image you need is buried among dozens of files named with timestamps and nothing else.

Chaos quietly fixes that as the files arrive:

- **Names screenshots by meaning** using your chosen vision model
- **Files them automatically** into an output folder, with optional daily or monthly subfolders
- **Accepts existing images** through drag and drop when your backlog needs attention
- **Keeps a searchable local history** of recent processing, including failures and retries
- **Stays out of the way** in the menu bar until you need it

## Install

### Download the app (no Terminal needed)

1. Download the latest **[Chaos.dmg](https://github.com/michaelmjhhhh/Chaos/releases/latest/download/Chaos.dmg)** (or browse all [Releases](https://github.com/michaelmjhhhh/Chaos/releases/latest)).
2. Open the DMG and drag **Chaos** into your **Applications** folder.
3. The first time you open it, macOS shows a warning that it "could not verify"
   the developer. This is expected — Chaos is open source and ad-hoc signed, but
   not notarized by Apple. To open it once:
   - Double-click **Chaos** (it gets blocked the first time — that's fine).
   - Open **System Settings → Privacy & Security**, scroll to the bottom, and
     click **Open Anyway** next to the message about Chaos.
   - Click **Open** to confirm. You only do this once.

### Homebrew (for developers)

```bash
brew tap michaelmjhhhh/chaos
brew install --cask chaos
```

> [!NOTE]
> Chaos supports Apple Silicon Macs running macOS 15 or later. Because it isn't
> notarized by Apple, the one-time **Open Anyway** step above is needed on first
> launch — it is not required again after that.

## Start filing

1. Open **Settings** with `Cmd+,`.
2. Choose a provider and enter an API key if it requires one.
3. Pick the folder to watch and the folder where renamed images should land.
4. Select **Start Watching** on the dashboard.
5. Take a screenshot. Chaos names and files it automatically.

Need to clean up an existing image? Drop a PNG, JPEG, HEIC, or WebP onto the dashboard and it enters the same filing flow.

## Built for the way screenshots accumulate

| Capability | What it gives you |
| --- | --- |
| **AI-generated names** | Files you can recognize in Finder and find with Spotlight |
| **Filename templates** | A consistent format using `{slug}`, `{date}`, and `{time}` |
| **Automatic organization** | Optional day or month folders without manual sorting |
| **Local history** | Unlimited durable history, with the latest 500 attempts searchable in Pipeline |
| **Retry flow** | A quick way to reprocess failed images after fixing a provider or file issue |
| **Rename and undo** | Correct an AI name, or restore its original location without overwriting files |
| **Lifetime insights** | Activity calendar, streaks, categories, file types, performance and time saved |
| **Clipboard handoff** | An option to copy the renamed image back to your clipboard |
| **Screenshot guards** | Processing limited to new macOS screenshots when folder watching is active |
| **Editorial dashboard** | Live progress, current-day filed/error counts, latency, and folder controls |

## Bring your preferred model

Chaos speaks the OpenAI-compatible vision API format, so you can choose the service that fits your workflow.

| Provider | Default model | Base URL |
| --- | --- | --- |
| **Chaos (recommended)** | Managed trial model | Built-in hosted service; no setup required |
| **SiliconRouter** | `gemini-3-flash-preview` | `https://api.siliconrouter.com/v1` |
| **OpenAI** | `gpt-4o-mini` | `https://api.openai.com/v1` |
| **DeepSeek** | `deepseek-v4-flash` | `https://api.deepseek.com` |
| **OpenRouter** | `openai/gpt-4o-mini` | `https://openrouter.ai/api/v1` |
| **OpenAI-Compatible** | `gpt-4o-mini` | You provide the URL |
| **Ollama** | `qwen3-vl:2b` | `http://localhost:11434/v1` |

### Local Ollama

Chaos connects to Ollama as an optional local provider; it does not bundle or
manage the Ollama runtime. Install and start Ollama separately, then pull the
default lightweight vision model. `qwen3-vl` requires Ollama `0.12.7+`.

```bash
ollama pull qwen3-vl:2b
```

Select **Ollama** in Chaos Settings. No API key is required.

## How it works

```text
New screenshot
      │
      ▼
Folder watcher ─── waits for stable new screenshots
      │
      ▼
Go vision client ─ asks your configured model for a short description
      │
      ▼
Slug sanitizer ── makes the result filesystem-safe
      │
      ▼
Durable journal ─ applies your template, files without overwriting, and records history
```

Existing files in the watched directory are ignored. Chaos only processes screenshots created after the watcher starts. Images you explicitly drop onto the dashboard bypass the screenshot filename guard and enter the same naming pipeline.

## Build from source

```bash
go test ./...
./build-app.sh
open .build/Chaos.app
```

Requires macOS 15 or later and Go 1.27.2. Go downloads the pinned toolchain
automatically. The app uses [MyGo 0.4.6](https://github.com/egoist/mygo/tree/v0.4.6)
native Go UI: no web frontend, JavaScript runtime, Swift application code, or cgo.

```bash
make dev                       # MyGo native live development
make test                      # services, controller, hosted API and native UI tests
make race                      # concurrency checks
make vet                       # static analysis
./package-dmg.sh               # signed .build/Chaos.dmg
go run ./cmd/chaos --data-dir /tmp/chaos-preview  # isolated manual QA profile
```

The isolated profile does not import your settings, access your device identity,
or alter your existing history. The optional hosted proxy is also Go; its local
server and Vercel deployment instructions are in [hosted/README.md](hosted/README.md).
The Vercel project now builds from the repository root, not `hosted/`.

The source also builds for Linux and Windows. macOS remains the primary tested
desktop; HEIC conversion uses the built-in macOS image service. See the
[framework audit](docs/migration/mygo-audit.md),
[feature checklist](docs/migration/feature-parity.md), and
[validation report](docs/migration/validation.md) for compatibility and test details.

## Configuration

Chaos stores its configuration at:

```text
~/Library/Application Support/chaos/config.json
```

On first launch, Chaos imports an existing `vibe-shot` CLI configuration when one is available.

Existing Swift installations keep the same configuration, SQLite history, bundle
identifier and hosted-trial identity. Legacy `history.json` is imported with its
original timestamps and archived only after a successful transaction. UserDefaults
for onboarding, session number and start-on-launch are imported to `preferences.json`.

File changes are journaled under `pending/` and reconciled on launch after a crash.
API keys are stored in the local configuration with owner-only permissions; switching
providers keeps their keys separate. Processing is serialized and deduplicated.

## Security

Found a security issue? Please report it privately — see [SECURITY.md](SECURITY.md).

## License

Chaos is open source under the [MIT License](LICENSE).
