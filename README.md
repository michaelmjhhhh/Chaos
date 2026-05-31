# Chaos

**Turn a desktop full of anonymous screenshots into a searchable archive.**

A native macOS menu bar app that watches for screenshots, asks a vision model what they contain, and files them under useful names while you keep working.

`Screenshot 2026-05-30 at 14.23.45.png` → `terminal-git-log_143022.png`

[![Watch the 29-second Chaos demo](docs/demo-preview.png)](docs/demo.mp4)

**[Watch the 29-second demo →](docs/demo.mp4)**

## 📸 Your screenshots should not become a second inbox

Screenshots are effortless to capture and surprisingly painful to retrieve. A week later, the image you need is buried among dozens of files named with timestamps and nothing else.

Chaos quietly fixes that as the files arrive:

- **Names screenshots by meaning** using your chosen vision model
- **Files them automatically** into an output folder, with optional daily or monthly subfolders
- **Accepts existing images** through drag and drop when your backlog needs attention
- **Keeps a searchable local history** of recent processing, including failures and retries
- **Stays out of the way** in the menu bar until you need it

## 📦 Install

```bash
brew tap michaelmjhhhh/chaos
brew install --cask chaos
```

> [!NOTE]
> Chaos currently supports Apple Silicon Macs running macOS 15 or later. The app is not yet notarized, so macOS will ask you to approve it in **System Settings → Privacy & Security** after the first launch.

## ✨ Start filing

1. Open **Settings** with `Cmd+,`.
2. Choose a provider and enter your API key.
3. Pick the folder to watch and the folder where renamed images should land.
4. Select **Start Watching** on the dashboard.
5. Take a screenshot. Chaos names and files it automatically.

Need to clean up an existing image? Drop a PNG, JPEG, HEIC, or WebP onto the dashboard and it enters the same filing flow.

## 🧭 Built for the way screenshots accumulate

|  | Capability | What it gives you |
| --- | --- | --- |
| 🪄 | **AI-generated names** | Files you can recognize in Finder and find with Spotlight |
| 🧩 | **Filename templates** | A consistent format using `{slug}`, `{date}`, and `{time}` |
| 📁 | **Automatic organization** | Optional day or month folders without manual sorting |
| 🕘 | **Local history** | The latest 500 attempts, searchable across launches |
| ↻ | **Retry flow** | A quick way to reprocess failed images after fixing a provider or file issue |
| 📋 | **Clipboard handoff** | An option to copy the renamed image back to your clipboard |
| 🛡️ | **Screenshot guards** | Processing limited to new macOS screenshots when folder watching is active |
| 📊 | **Editorial dashboard** | Live progress, recent filings, latency, throughput, and success rate |

## 🧠 Bring your preferred model

Chaos speaks the OpenAI-compatible vision API format, so you can choose the service that fits your workflow.

| Provider | Default model | Base URL |
| --- | --- | --- |
| **SiliconRouter** | `gemini-3-flash-preview` | `https://api.siliconrouter.com/v1` |
| **OpenAI** | `gpt-4o-mini` | `https://api.openai.com/v1` |
| **DeepSeek** | `deepseek-v4-flash` | `https://api.deepseek.com` |
| **OpenRouter** | `openai/gpt-4o-mini` | `https://openrouter.ai/api/v1` |
| **OpenAI-Compatible** | `gpt-4o-mini` | You provide the URL |

## ⚙️ How it works

```text
New screenshot
      │
      ▼
ScreenshotGuard ── rejects unrelated files
      │
      ▼
VisionAPIClient ── asks your configured model for a short description
      │
      ▼
SlugSanitizer ─── makes the result filesystem-safe
      │
      ▼
FileRenamer ───── applies your template, avoids collisions, and files the image
```

Existing files in the watched directory are ignored. Chaos only processes screenshots created after the watcher starts. Images you explicitly drop onto the dashboard bypass the screenshot filename guard and enter the same naming pipeline.

## 🛠️ Build from source

```bash
swift build
./build-app.sh
open .build/Chaos.app
```

Requires macOS 15 or later and Swift 6.0 or later.

## 📁 Configuration

Chaos stores its configuration at:

```text
~/Library/Application Support/chaos/config.json
```

On first launch, Chaos imports an existing `vibe-shot` CLI configuration when one is available.
