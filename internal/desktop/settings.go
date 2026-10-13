package desktop

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/core"
)

func (d *Desktop) settings(c *ui.Context) {
	ui.Scroll(c.Key("settings-scroll")).Grow(1).Padding(24).Gap(20).Children(func() {
		heading(c, "Settings", "Connect a naming model and tune your filing workflow.")
		d.banners(c)
		d.providerSettings(c, false)
		section(c, "Directories", func() {
			d.directoryRow(c, "Watch", d.watchDir(), d.actions.PickWatchDirectory)
			d.directoryRow(c, "Output", d.outputDir(), d.actions.PickOutputDirectory)
		})
		d.namingSettings(c)
		section(c, "Behavior", func() {
			d.settingSwitch(c, "Copy image to clipboard", &d.config.CopyToClipboard, "Put each successfully filed image on the clipboard.")
			auto := d.snapshot.AutoStart
			switchRow(c, "Start watching on launch", &auto, func() {
				if d.controller != nil {
					d.controller.SetAutoStart(auto)
				}
			})

			d.settingSwitch(c, "Show a notification when done", &d.config.NotifyOnComplete, "macOS may ask permission when you enable notifications.")
		})
		section(c, "Appearance", func() {
			current := d.config.Appearance
			if current == "" {
				current = "system"
			}
			ui.Row(c).Gap(8).FocusGroup(ui.Horizontal).Children(func() {
				for _, name := range []string{"system", "light", "dark"} {
					label := strings.ToUpper(name[:1]) + name[1:]
					b := ui.Button(c.Key("appearance-"+name), label).Grow(1).OnClick(func() {
						d.config.Appearance = name
						d.save()
						if d.actions.ApplyAppearance != nil {
							d.actions.ApplyAppearance(name)
						}
					})
					if current == name {
						b.Background(c.Theme().Accent.Alpha(.15)).TextColor(c.Theme().Accent)
					}
				}
			})
			small(c, "System follows your Mac. Light and Dark keep Chaos in that appearance.")
		})
		section(c, "Configuration", func() {
			path := d.snapshot.ConfigPath
			if path == "" {
				path = filepath.Join(d.home, "Library", "Application Support", "chaos", "config.json")
			}
			ui.Text(c, d.shorten(path)).Font("monospace").Selectable()
			ui.Button(c, "Reveal config").OnClick(func() { d.reveal(path) })
		})
	})
}
func (d *Desktop) providerSettings(c *ui.Context, onboarding bool) {
	section(c, "Naming Service", func() {
		provider := core.ProviderByID(d.config.Provider)
		choice := provider.Name
		providers := core.Providers()
		names := make([]string, 0, len(providers))
		for _, p := range providers {
			if onboarding && p.CustomURL {
				continue
			}
			names = append(names, p.Name)
		}
		if ui.Select(c.Key("provider"), &choice, names).Label("Provider").Changed() {
			for _, p := range providers {
				if p.Name == choice {
					d.config.SetProvider(p.ID)
					d.save()
					provider = p
					break
				}
			}
		}
		ui.Text(c, provider.Description).TextColor(c.Theme().TextMuted)
		kind := "Remote"
		if provider.ID == "ollama" {
			kind = "Local"
		}
		if provider.ID == "chaos-hosted" {
			kind = "Built-in"
		}
		small(c, kind)
		if provider.RequiresKey {
			ui.Field(c, "API Key", func() {
				if ui.TextInput(c.Key("api-key"), &d.config.APIKey).Password().Placeholder("Paste your API key").Changed() {
					d.save()
				}
			}).Description("Stored on this Mac. Each provider keeps its own saved key.")
			if provider.SignupURL != "" {
				ui.Button(c, "Get an API key ↗").OnClick(func() { d.link(provider.SignupURL) })
			}
		}
		if !onboarding {
			if provider.CustomURL {
				d.advanced = true
			}
			label := "Advanced options ▸"
			if d.advanced {
				label = "Advanced options ▾"
			}
			ui.Button(c, label).OnClick(func() { d.advanced = !d.advanced })
			if d.advanced {
				ui.Field(c, "Model", func() {
					if ui.TextInput(c.Key("model"), &d.config.Model).Placeholder(provider.Model).Changed() {
						d.save()
					}
				}).Description("Leave blank for the default: " + provider.Model)
				if provider.CustomURL {
					ui.Field(c, "Base URL", func() {
						if ui.TextInput(c.Key("base-url"), &d.config.BaseURL).Placeholder("https://example.com/v1").Changed() {
							d.save()
						}
					}).Description("Required for a custom OpenAI-compatible provider.")
				} else {
					ui.Text(c, provider.BaseURL).Font("monospace").FontSize(11).Selectable()
				}
			}
		}
		ui.Row(c).Gap(12).Children(func() {
			label := "Test Connection"
			checking := strings.HasPrefix(d.snapshot.APIStatus, "Checking")
			if checking {
				label = "Testing Connection…"
			}
			ui.Button(c.Key("connection-test"), label).Disabled(checking).OnClick(func() {
				if d.controller != nil {
					d.controller.CheckHealth()
				}
			})
			switch d.snapshot.APIStatus {
			case "OK":
				ui.Text(c, "Connection successful.").TextColor(c.Theme().Success)
			case "FAIL":
				ui.Text(c, "Connection failed.").TextColor(c.Theme().Danger)
			}
		})
		if d.snapshot.HealthError != "" {
			ui.Text(c, d.snapshot.HealthError).TextColor(c.Theme().Danger).Selectable()
		}
		if provider.ID == "ollama" {
			small(c, "Private: naming runs on your Mac. Install Ollama separately, then run: ollama pull qwen3-vl:2b")
		} else {
			small(c, "Each image is sent to "+provider.Name+" to generate its name.")
		}
		if provider.ID == "chaos-hosted" {
			ui.Button(c, "Check free trial").OnClick(func() {
				if d.controller != nil {
					d.controller.CheckUsage()
				}
			})
			if usage := d.snapshot.Usage; usage != nil {
				text := fmt.Sprintf("You've used %d of %d free names — %d left.", usage.Used, usage.Limit, usage.Remaining)
				if usage.Remaining <= 0 {
					text = "You've used all your free names. Choose another provider and add your own key to continue."
				}
				ui.Text(c, text).Selectable()
			}
			if d.snapshot.UsageError != "" {
				ui.Text(c, d.snapshot.UsageError).TextColor(c.Theme().Danger)
			}
		}
	})
}
func (d *Desktop) namingSettings(c *ui.Context) {
	section(c, "Naming & Filing", func() {
		customActive := d.config.UseCustomPrompt && strings.TrimSpace(d.config.CustomPrompt) != ""
		lang := "English"
		if d.config.Language == "zh" {
			lang = "Chinese"
		}
		ui.Field(c, "Name language", func() {
			if ui.Select(c.Key("name-language"), &lang, []string{"English", "Chinese"}).Disabled(customActive).Changed() {
				d.config.Language = "en"
				if lang == "Chinese" {
					d.config.Language = "zh"
				}
				d.save()
			}
		}).Description("The language of filenames. A custom prompt overrides this choice.")
		ui.Field(c, "Filename Template", func() {
			if ui.TextInput(c.Key("filename-template"), &d.config.FilenameTemplate).Placeholder(core.DefaultTemplate).Changed() {
				d.save()
			}
		}).Description("{slug} = AI name · {date} = day · {time} = time")
		switchRow(c, "Use a custom naming prompt", &d.config.UseCustomPrompt, func() {
			if d.config.UseCustomPrompt && strings.TrimSpace(d.config.CustomPrompt) == "" {
				d.config.CustomPrompt = core.DefaultPrompt(d.config.Language)
			}
			d.save()
		})

		if d.config.UseCustomPrompt {
			if ui.TextArea(c.Key("custom-prompt"), &d.config.CustomPrompt).Height(130).Label("Custom naming prompt").Changed() {
				d.save()
			}
			small(c, "Your prompt replaces the default. Names remain capped and filesystem-safe.")
			ui.Button(c, "Reset prompt to default").OnClick(func() { d.config.CustomPrompt = core.DefaultPrompt(d.config.Language); d.save() })
		}
		group := "No Subfolders"
		if d.config.SubfolderRule == "day" {
			group = "By Day"
		}
		if d.config.SubfolderRule == "month" {
			group = "By Month"
		}
		ui.Field(c, "Group into subfolders", func() {
			if ui.Select(c.Key("subfolders"), &group, []string{"No Subfolders", "By Day", "By Month"}).Changed() {
				d.config.SubfolderRule = "none"
				if group == "By Day" {
					d.config.SubfolderRule = "day"
				}
				if group == "By Month" {
					d.config.SubfolderRule = "month"
				}
				d.save()
			}
		})
		small(c, "PREVIEW")
		sample := time.Date(2026, 6, 27, 14, 32, 0, 0, time.Local)
		name := core.RenderName(d.config.FilenameTemplate, "meeting-notes", sample) + ".png"
		folder := core.OutputDirectory("", d.config.SubfolderRule, sample)
		ui.Text(c, filepath.Join(folder, name)).Font("monospace").Padding(12).Background(c.Theme().Accent.Alpha(.07)).Radius(6).Selectable()
	})
}
func (d *Desktop) directoryRow(c *ui.Context, label, path string, choose func()) {
	ui.Column(c.Key("directory-" + label)).Gap(6).Children(func() {
		small(c, strings.ToUpper(label))
		ui.Row(c).Gap(12).Children(func() {
			ui.Text(c, d.shorten(path)).Font("monospace").FontSize(12).Grow(1).Selectable()
			ui.Button(c, "Choose "+label+"…").OnClick(func() {
				if choose != nil {
					choose()
				}
			})
		})
	})
}
func (d *Desktop) settingSwitch(c *ui.Context, label string, value *bool, hint string) {
	ui.Column(c.Key(label)).Gap(4).Children(func() { switchRow(c, label, value, d.save); small(c, hint) })
}

// The full label row is clickable, while the inner switch remains a native
// accessible switch. MyGo gives the innermost control its click, preventing
// a click on the switch from toggling the enclosing row a second time.
func switchRow(c *ui.Context, label string, value *bool, changed func()) {
	ui.Row(c.Key("switch-" + label)).Gap(12).Label(label).OnClick(func() { *value = !*value; changed() }).Children(func() {
		ui.Text(c, label).Grow(1)
		if ui.Switch(c, value).Label(label).Changed() {
			changed()
		}
	})
}
