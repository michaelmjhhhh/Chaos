package desktop

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/core"
)

type helpTopic struct{ question, answer string }

var helpTopics = []helpTopic{
	{"How do I start naming screenshots?", "Press Start Watching on the dashboard, then take a screenshot (⌘⇧4). Chaos names it and moves it into your output folder. Images already in the watched folder are ignored; use Organize Existing Screenshots for your backlog."},
	{"Where do my renamed files go?", "Settings → Folders shows the output folder, usually chaos-output on your Desktop. Originals are moved there with a useful name. Pipeline → Undo filing restores the original path without overwriting existing files."},
	{"What is an API key and do I need one?", "An API key lets Chaos use your AI account. For remote providers, use Get an API key in Settings and paste your key. Ollama needs no key."},
	{"Which naming service should I choose?", "Ollama runs locally but must be installed separately; pull a vision model such as qwen3-vl:2b. Other providers use your account and credit."},
	{"The name doesn't match the picture. Why?", "AI can misread images. In Pipeline, select a filed image and choose Rename to correct its name, or Undo filing to restore its original location. Failed attempts have Retry. You can choose a different model in Settings → Advanced options."},
	{"Is my data private?", "Screenshots are sent to the provider you select to generate filenames. With local Ollama, processing stays on your Mac. Other providers have their own privacy policies. History and configuration stay on this Mac."},
	{"It says it can't connect. What now?", "Open Settings and Test Connection. Check your internet, API key, account credit and chosen model. For Ollama, start the local runtime and pull the model."},
	{"Can I change the filename style?", "Settings → Naming & filing supports {slug}, {date}, and {time}, plus day or month subfolders and a live preview. Choose English or Chinese names, or enable a custom naming prompt for your own instructions."},
}

var shortcuts = []struct{ keys, action string }{
	{"⌘O", "Organize existing screenshots"},
	{"Space", "Start or stop watching, on the dashboard"},
	{"⌘1 ⌘2 ⌘3", "Dashboard, Pipeline, Insights"},
	{"⌘F", "Search filings or help"},
	{"⌘,", "Settings"},
	{"⇧⌘T", "Test the connection"},
}

func (d *Desktop) help(c *ui.Context) {
	p := &d.pal
	d.page(c, "help", "", "Help & Getting Started", "Answers for a quieter screenshot workflow.", func() {
		d.button(c, btn{label: "Visit website", trailing: icArrowUR, kind: ghost}).OnClick(func() { d.link("https://github.com/michaelmjhhhh/Chaos") })
		d.button(c, btn{label: "Replay Welcome Guide", icon: icSparkle, kind: secondary}).OnClick(d.ShowWelcome)
	}, func() {
		d.input(c, "help-search", &d.helpQuery, fieldOpts{label: "Search help", placeholder: "Search help…", icon: icSearch, handle: &d.helpSearch, clearable: true, hint: "⌘F"})
		query := strings.ToLower(strings.TrimSpace(d.helpQuery))
		var shown []int
		for i, topic := range helpTopics {
			if query == "" || strings.Contains(strings.ToLower(topic.question+" "+topic.answer), query) {
				shown = append(shown, i)
			}
		}
		if len(shown) == 0 {
			d.emptyState(c, icSearch, "No results for “"+d.helpQuery+"”.", "Try fewer words, or browse every question by clearing the search.")
		} else {
			d.card(ui.Column(c.Key("faq"))).Gap(1).Dividers(1, p.border).Clip().Children(func() {
				for _, i := range shown {
					d.faq(c, i)
				}
			})
		}
		d.group(c, "Keyboard shortcuts", "", func() {
			for _, s := range shortcuts {
				ui.Row(c.Key(s.action)).Padding(10, 16).Gap(12).Children(func() {
					ui.Text(c, s.action).Grow(1)
					ui.Row(c).Gap(6).Children(func() {
						for _, combo := range strings.Fields(s.keys) {
							if combo == "Space" {
								ui.Text(c, combo).FontSize(11).FontWeight(500).TextColor(p.text2).Height(18).Padding(1, 6).Radius(4).
									Background(p.card).Border(1, p.border).Shadow(0, 1, 0, 0, p.border)
								continue
							}
							d.keys(c, combo)
						}
					})
				})
			}
		})
	})
}

// faq is one disclosure of the accordion: the chevron turns as the answer
// grows into view.
func (d *Desktop) faq(c *ui.Context, i int) {
	p := &d.pal
	topic := helpTopics[i]
	open := d.expanded[i]
	if open == nil {
		// The disclosure writes its toggle after the build, so it needs a
		// pointer that outlives the frame.
		open = new(bool)
		d.expanded[i] = open
	}
	ui.Column(c.Key(topic.question)).Children(func() {
		f := ui.CollapsibleBase(c.Key("trigger"), open)
		tr := f.Trigger.Padding(14, 16).Gap(12).Label(topic.question).FocusRing(false)
		bg := p.hover.Alpha(0)
		if tr.Hovered() || tr.FocusVisible() {
			bg = p.hover
		}
		tr.Background(tr.AnimateColor("bg", bg, fade))
		tr.Children(func() {
			ui.Text(c, topic.question).FontWeight(500).Grow(1)
			ui.Icon(c, icDown).Size(14, 14).TextColor(p.text3).Rotate(180 * f.Progress())
		})
		f.Panel(func() {
			ui.Text(c, topic.answer).Selectable().TextColor(p.text2).LineHeight(1.5).Padding(0, 44, 16, 16)
		})
	})
}

var welcomeSteps = []struct{ title, name string }{
	{"Chaos names your screenshots.", "Welcome"},
	{"Turn on naming.", "Connect"},
	{"Your tidy folder.", "Folders"},
	{"You're ready.", "Ready"},
}

func (d *Desktop) welcomeDialog(c *ui.Context) {
	p := &d.pal
	d.dialog(c, "welcome-modal", &d.welcome, 540, func() {
		step := max(0, min(len(welcomeSteps)-1, d.welcomeStep))
		// Progress: the current step stretches into a pill.
		ui.Row(c.Key("welcome-steps")).Gap(6).Label(fmt.Sprintf("Step %d of %d", step+1, len(welcomeSteps))).Children(func() {
			for i := range welcomeSteps {
				bar := ui.Box(c.Key(i)).Height(4).Radius(2)
				w := float32(8)
				if i == step {
					w = 24
				}
				tone := p.track
				if i <= step {
					tone = p.clay
				}
				bar.Width(bar.AnimateTo("w", w, glide)).Background(bar.AnimateColor("tone", tone, fade))
			}
			ui.Spacer(c)
			d.caption(c, welcomeSteps[step].name)
		})
		ui.Column(c.Key(fmt.Sprint("welcome-", step))).Gap(16).Transition(ui.ElementTransition{Animation: &glide, Enter: &ui.Motion{X: 14}}).Children(func() {
			ui.Text(c, welcomeSteps[step].title).Font(serif).FontSize(28).Role(ui.RoleHeading).Level(1)
			switch step {
			case 0:
				ui.Text(c, "Take a screenshot. Chaos reads it, writes a useful name, and files it away so you can find it again.").TextColor(p.text2).LineHeight(1.45)
				ui.Column(c).Gap(10).Children(func() {
					for i, line := range []string{"Watch your screenshot folder.", "Name each image with your chosen vision model.", "Keep a searchable local archive."} {
						ui.Row(c.Key(line)).Gap(12).Children(func() {
							ui.Text(c, fmt.Sprint(i+1)).FontSize(11.5).FontWeight(600).TextColor(p.clayText).TextAlign(ui.Center).
								Size(22, 22).Padding(3, 0).Radius(11).Background(p.claySoft)
							ui.Text(c, line)
						})
					}
				})
			case 1:
				d.inset(c, func() { d.providerSettings(c, true) })
			case 2:
				d.inset(c, func() {
					d.directoryRow(c, "Output", "Named images go here.", d.snapshot.ResolvedOutputDir, d.actions.PickOutputDirectory)
					d.directoryRow(c, "Watch", "Chaos looks for new screenshots here.", d.watchDir(), d.actions.PickWatchDirectory)
				})
				ui.Text(c, "Images are moved with a clearer name. You can undo a filing from Pipeline.").TextColor(p.text2)
			case 3:
				ui.Row(c).Gap(10).Children(func() {
					ui.Text(c, "Start watching, then take a screenshot with").TextColor(p.text2)
					d.keys(c, "⌘⇧4")
				})
				ui.Text(c, "Or organize existing images from the dashboard.").TextColor(p.text2)
				ui.Row(c).Gap(10).Padding(10, 12).Radius(10).Background(p.fill).Children(func() {
					ui.Icon(c, icLock).Size(14, 14).TextColor(p.text2)
					ui.Text(c, "Your chosen naming provider receives the image. Local Ollama keeps it on this Mac.").FontSize(12).TextColor(p.text2).Grow(1)
				})
			}
		})
		ui.Row(c).Gap(8).Padding(4, 0, 0, 0).Children(func() {
			d.button(c.Key("skip"), btn{label: "Skip", kind: ghost}).OnClick(func() { d.finishWelcome(false) })
			ui.Spacer(c)
			if step > 0 {
				d.button(c.Key("back"), btn{label: "Back", kind: secondary}).OnClick(func() { d.welcomeStep-- })
			}
			provider := core.ProviderByID(d.config.Provider)
			disabled := step == 1 && provider.RequiresKey && strings.TrimSpace(d.config.APIKey) == ""
			next := btn{label: "Continue", trailing: icArrowR, kind: primary, disabled: disabled}
			if step == len(welcomeSteps)-1 {
				next = btn{label: "Start Watching", icon: icPlay, kind: primary}
			}
			d.button(c.Key("next"), next).OnClick(func() {
				if step == len(welcomeSteps)-1 {
					d.finishWelcome(true)
				} else {
					d.welcomeStep++
				}
			})
		})
	})
}

// inset groups rows inside a dialog on a faint fill, keeping the dialog's
// own edge the only closed border.
func (d *Desktop) inset(c *ui.Context, build func()) {
	ui.Column(c).Radius(12).Background(d.pal.ink.Alpha(.03)).Gap(1).Dividers(1, d.pal.border).Clip().Children(build)
}

func (d *Desktop) finishWelcome(start bool) {
	d.welcome = false
	d.save()
	if d.controller != nil {
		d.controller.CompleteOnboarding(start)
	}
}
