package desktop

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/core"
)

type helpTopic struct{ question, answer string }

var helpTopics = []helpTopic{
	{"How do I start naming screenshots?", "Press Start Watching on the dashboard, then take a screenshot (⌘⇧4). Chaos names it and moves it into your output folder. Images already in the watched folder are ignored; use Organize Existing Screenshots for your backlog."},
	{"Where do my renamed files go?", "Settings → Directories shows the output folder, usually chaos-output on your Desktop. Originals are moved there with a useful name. Pipeline → Undo filing restores the original path without overwriting existing files."},
	{"What is an API key and do I need one?", "An API key lets Chaos use your AI account. For remote providers, use Get an API key in Settings and paste your key. Ollama needs no key."},
	{"Which naming service should I choose?", "Ollama runs locally but must be installed separately; pull a vision model such as qwen3-vl:2b. Other providers use your account and credit."},
	{"The name doesn't match the picture. Why?", "AI can misread images. In Pipeline, select a filed image and choose Rename to correct its name, or Undo filing to restore its original location. Failed attempts have Retry. You can choose a different model in Settings → Advanced options."},
	{"Is my data private?", "Screenshots are sent to the provider you select to generate filenames. With local Ollama, processing stays on your Mac. Other providers have their own privacy policies. History and configuration stay on this Mac."},
	{"It says it can't connect. What now?", "Open Settings and Test Connection. Check your internet, API key, account credit and chosen model. For Ollama, start the local runtime and pull the model."},
	{"Can I change the filename style?", "Settings → Naming & Filing supports {slug}, {date}, and {time}, plus day or month subfolders and a live preview. Choose English or Chinese names, or enable a custom naming prompt for your own instructions."},
}

func (d *Desktop) help(c *ui.Context) {
	ui.Scroll(c.Key("help-scroll")).Grow(1).Padding(24).Gap(16).Children(func() {
		heading(c, "Help & Getting Started", "Answers for a quieter screenshot workflow.")
		ui.TextInput(c.Key("help-search"), &d.helpQuery).Label("Search help").Placeholder("Search help…")
		query := strings.ToLower(strings.TrimSpace(d.helpQuery))
		count := 0
		for i, topic := range helpTopics {
			if query != "" && !strings.Contains(strings.ToLower(topic.question+" "+topic.answer), query) {
				continue
			}
			count++
			ui.Column(c.Key(topic.question)).Gap(10).Padding(16).Background(c.Theme().Surface).Border(1, c.Theme().Border).Radius(8).Children(func() {
				label := topic.question
				if d.expanded[i] {
					label = "▾ " + label
				} else {
					label = "▸ " + label
				}
				ui.Button(c, label).Label(topic.question).FillWidth().OnClick(func() { d.expanded[i] = !d.expanded[i] })
				if d.expanded[i] {
					ui.Text(c, topic.answer).Selectable()
				}
			})
		}
		if count == 0 {
			ui.Text(c, "No results for “"+d.helpQuery+"”.").TextColor(c.Theme().TextMuted)
		}
		ui.Row(c).Gap(12).Children(func() {
			ui.Button(c, "Replay Welcome Guide").OnClick(d.ShowWelcome)
			ui.Button(c, "Visit website ↗").OnClick(func() { d.link("https://github.com/michaelmjhhhh/Chaos") })
		})
	})
}
func (d *Desktop) welcomeDialog(c *ui.Context) {
	ui.Modal(c.Key("welcome-modal"), &d.welcome, func() {
		titles := []string{"Chaos names your screenshots.", "Turn on naming.", "Your tidy folder.", "You're ready."}
		step := max(0, min(3, d.welcomeStep))
		heading(c, titles[step], "")
		switch step {
		case 0:
			ui.Text(c, "Take a screenshot. Chaos reads it, writes a useful name, and files it away so you can find it again.")
			small(c, "1. Watch your screenshot folder.")
			small(c, "2. Name each image with your chosen vision model.")
			small(c, "3. Keep a searchable local archive.")
		case 1:
			d.providerSettings(c, true)
		case 2:
			d.directoryRow(c, "Output", d.outputDir(), d.actions.PickOutputDirectory)
			d.directoryRow(c, "Watch", d.watchDir(), d.actions.PickWatchDirectory)
			ui.Text(c, "Images are moved with a clearer name. You can undo a filing from Pipeline.")
		case 3:
			ui.Text(c, "Start watching, then take a screenshot with ⌘⇧4. Or organize existing images from the dashboard.")
			small(c, "Your chosen naming provider receives the image. Local Ollama keeps it on this Mac.")
		}
		ui.Row(c).Gap(10).Children(func() {
			ui.Button(c, "Skip").OnClick(func() { d.finishWelcome(false) })
			ui.Spacer(c)
			small(c, []string{"Welcome", "Connect", "Folders", "Ready"}[step])
			if step > 0 {
				ui.Button(c, "Back").OnClick(func() {
					d.welcomeStep--
				})
			}
			label := "Continue"
			if step == 3 {
				label = "Start Watching"
			}
			provider := core.ProviderByID(d.config.Provider)
			disabled := step == 1 && (provider.Name == "" || (provider.RequiresKey && strings.TrimSpace(d.config.APIKey) == ""))
			ui.PrimaryButton(c, label).Disabled(disabled).OnClick(func() {
				if step == 3 {
					d.finishWelcome(true)
				} else {
					d.welcomeStep++
				}
			})
		})
	})
}
func (d *Desktop) finishWelcome(start bool) {
	d.welcome = false
	d.save()
	if d.controller != nil {
		d.controller.CompleteOnboarding(start)
	}
}
