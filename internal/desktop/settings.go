package desktop

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/core"
)

func (d *Desktop) settings(c *ui.Context) {
	p := &d.pal
	d.page(c, "settings", "", "Settings", "Connect a naming model and tune your filing workflow.", nil, func() {
		d.banners(c)
		d.group(c, "Naming service", "", func() { d.providerSettings(c, false) })
		d.group(c, "Folders", "", func() {
			d.directoryRow(c, "Watch", "New screenshots here are named and filed.", d.watchDir(), d.actions.PickWatchDirectory)
			d.directoryRow(c, "Output", "Named images are moved here.", d.snapshot.ResolvedOutputDir, d.actions.PickOutputDirectory)
		})
		d.namingSettings(c)
		d.group(c, "Behavior", "", func() {
			d.switchRow(c, "Copy image to clipboard", "Put each successfully filed image on the clipboard.", &d.config.CopyToClipboard, d.save)
			auto := d.snapshot.AutoStart
			d.switchRow(c, "Start watching on launch", "Chaos begins watching as soon as it opens.", &auto, func() {
				if d.controller != nil {
					d.controller.SetAutoStart(auto)
				}
			})
			d.switchRow(c, "Show a notification when done", "macOS may ask permission when you enable notifications.", &d.config.NotifyOnComplete, d.save)
		})
		d.group(c, "Appearance", "", func() {
			d.settingRow(c, "appearance", "Theme", "System follows your Mac.", func() {
				names := []string{"system", "light", "dark"}
				idx := 0
				for i, name := range names {
					if d.config.Appearance == name {
						idx = i
					}
				}
				items := []seg{{label: "System", icon: icMonitor}, {label: "Light", icon: icSun}, {label: "Dark", icon: icMoon}}
				if d.segmented(c, "appearance", &idx, items).Changed() {
					d.config.Appearance = names[idx]
					d.save()
				}
			})
		})
		d.group(c, "Configuration", "", func() {
			path := d.snapshot.ConfigPath
			ui.Row(c.Key("config")).Gap(12).Padding(12, 12, 12, 16).Children(func() {
				ui.Icon(c, icFile).Size(15, 15).TextColor(p.text2)
				ui.Text(c, d.shorten(path)).Font(mono).FontSize(12).Selectable().SingleLine().Grow(1).MinWidth(0)
				d.button(c, btn{label: "Reveal config", trailing: icArrowUR, kind: ghost, small: true}).OnClick(func() { d.reveal(path) })
			})
		})
	})
}

// providerSettings builds the naming-service rows; onboarding shows the
// essentials only, without the hosted custom endpoint or advanced options.
func (d *Desktop) providerSettings(c *ui.Context, onboarding bool) {
	p := &d.pal
	provider := core.ProviderByID(d.config.Provider)
	d.settingRow(c, "provider", "Provider", provider.Description, func() {
		choice := provider.Name
		providers := core.Providers()
		names := make([]string, 0, len(providers))
		for _, pr := range providers {
			if onboarding && pr.CustomURL {
				continue
			}
			names = append(names, pr.Name)
		}
		ui.Row(c).Gap(8).Shrink(0).Children(func() {
			kind, tone := "Remote", p.text2
			if provider.ID == "ollama" {
				kind, tone = "Local", p.success
			}
			d.pill(c, kind, tone, tone.Alpha(.1))
			if d.dropdown(c, "provider", "Provider", &choice, names, false).Changed() {
				for _, pr := range providers {
					if pr.Name == choice {
						d.config.SetProvider(pr.ID)
						d.advanced = pr.CustomURL
						d.save()
						provider = pr
						break
					}
				}
			}
		})
	})
	if provider.RequiresKey {
		d.stackRow(c, "api-key", func() {
			d.field(c, "API Key", "Stored on this Mac. Each provider keeps its own saved key.", func() {
				if d.input(c, "api-key", &d.config.APIKey, fieldOpts{placeholder: "Paste your API key", icon: icKey, password: true}).Changed() {
					d.save()
				}
			})
			if provider.SignupURL != "" {
				ui.Row(c).Children(func() {
					d.button(c, btn{label: "Get an API key", trailing: icArrowUR, kind: ghost, small: true}).Margin(0, 0, 0, -8).OnClick(func() { d.link(provider.SignupURL) })
				})
			}
		})
	}
	if !onboarding {
		d.advancedOptions(c, provider)
	}
	d.connection(c, provider)
}

func (d *Desktop) advancedOptions(c *ui.Context, provider core.Provider) {
	p := &d.pal
	ui.Column(c.Key("advanced")).Children(func() {
		adv := ui.CollapsibleBase(c.Key("advanced-toggle"), &d.advanced)
		tr := adv.Trigger.Padding(12, 16).Gap(8).Label("Advanced options").FocusRing(false)
		fg := p.text2
		if tr.Hovered() || d.advanced {
			fg = p.text
		}
		tr.TextColor(tr.AnimateColor("fg", fg, fade))
		if tr.FocusVisible() {
			tr.Background(p.hover)
		}
		tr.Children(func() {
			ui.Icon(c, icRight).Size(13, 13).Rotate(90 * adv.Progress())
			ui.Text(c, "Advanced options").FontWeight(500).Grow(1)
			if !d.advanced {
				d.caption(c, "Model, endpoint")
			}
		})
		adv.Panel(func() {
			ui.Column(c).Gap(14).Padding(0, 16, 14, 37).Children(func() {
				d.field(c, "Model", "Leave blank for the default: "+provider.Model, func() {
					if d.input(c, "model", &d.config.Model, fieldOpts{placeholder: provider.Model, code: true}).Changed() {
						d.save()
					}
				})
				if provider.CustomURL {
					d.field(c, "Base URL", "Required for a custom OpenAI-compatible provider.", func() {
						if d.input(c, "base-url", &d.config.BaseURL, fieldOpts{placeholder: "https://example.com/v1", code: true}).Changed() {
							d.save()
						}
					})
				} else {
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, "Endpoint").FontWeight(500)
						ui.Text(c, provider.BaseURL).Font(mono).FontSize(12).TextColor(p.text2).Selectable()
					})
				}
			})
		})
	})
}

// connection tests the provider; the button's icon turns while a check
// runs and the result settles into a colored pill beside it.
func (d *Desktop) connection(c *ui.Context, provider core.Provider) {
	p := &d.pal
	hint := "Each image is sent to " + provider.Name + " to generate its name."
	if provider.ID == "ollama" {
		hint = "Private: naming runs on your Mac. Install Ollama separately, then run: ollama pull qwen3-vl:2b"
	}
	d.settingRow(c, "connection", "Connection", hint, func() {
		ui.Row(c).Gap(10).Shrink(0).Children(func() {
			switch d.snapshot.APIStatus {
			case "OK":
				ui.Row(c.Key("health")).Gap(6).TextColor(p.success).Transition(ui.ElementTransition{Enter: &ui.Motion{X: 6}}).Children(func() {
					ui.Icon(c, icCheck).Size(13, 13)
					ui.Text(c, "Connection successful.").FontSize(12).FontWeight(500)
				})
			case "FAIL":
				ui.Row(c.Key("health")).Gap(6).TextColor(p.danger).Transition(ui.ElementTransition{Enter: &ui.Motion{X: 6}}).Children(func() {
					ui.Icon(c, icAlert).Size(13, 13)
					ui.Text(c, "Connection failed.").FontSize(12).FontWeight(500)
				})
			}
			checking := strings.HasPrefix(d.snapshot.APIStatus, "Checking")
			label := "Test Connection"
			if checking {
				label = "Testing Connection…"
			}
			b := ui.ButtonBase(c.Key("connection-test")).Height(32).PaddingX(12).Gap(6).Radius(8).FontWeight(500).Background(p.fill).Disabled(checking)
			if b.Hovered() && !checking {
				b.Background(p.ink.Alpha(.1))
			}
			if checking {
				b.Opacity(.7)
			}
			b.OnClick(func() {
				if d.controller != nil {
					d.controller.CheckHealth()
				}
			}).Children(func() {
				spin := ui.Icon(c, icRetry).Size(14, 14)
				if checking {
					spin.Rotate(spin.Loop("spin", 900*time.Millisecond, ui.Linear) * 360)
				}
				ui.Text(c, label).SingleLine()
			})
		})
	})
	if d.snapshot.HealthError != "" {
		ui.Column(c.Key("health-error")).Padding(0, 16, 14, 16).Children(func() {
			d.alert(c, "health", p.danger, icAlert, d.snapshot.HealthError, nil)
		})
	}
}

func (d *Desktop) namingSettings(c *ui.Context) {
	p := &d.pal
	d.group(c, "Naming & filing", "", func() {
		customActive := d.config.UseCustomPrompt && strings.TrimSpace(d.config.CustomPrompt) != ""
		lang := "English"
		if d.config.Language == "zh" {
			lang = "Chinese"
		}
		d.settingRow(c, "language", "Name language", "The language of filenames. A custom prompt overrides this choice.", func() {
			if d.dropdown(c, "name-language", "Name language", &lang, []string{"English", "Chinese"}, customActive).Changed() {
				d.config.Language = "en"
				if lang == "Chinese" {
					d.config.Language = "zh"
				}
				d.save()
			}
		})
		d.stackRow(c, "template", func() {
			d.field(c, "Filename Template", "", func() {
				if d.input(c, "filename-template", &d.config.FilenameTemplate, fieldOpts{placeholder: core.DefaultTemplate, code: true}).Changed() {
					d.save()
				}
			})
			// Tokens insert themselves at the end of the template.
			ui.Row(c).Gap(6).Wrap().Children(func() {
				for _, tok := range []struct{ token, meaning string }{{"{slug}", "AI name"}, {"{date}", "day"}, {"{time}", "time"}} {
					chip := ui.ButtonBase(c.Key("token-" + tok.token)).Height(24).PaddingX(8).Gap(6).Radius(6).Tooltip("Add " + tok.token + " to the template")
					bg := p.fill
					if chip.Hovered() {
						bg = p.claySoft
					}
					chip.Background(chip.AnimateColor("bg", bg, fade)).OnClick(func() {
						if d.config.FilenameTemplate == "" {
							d.config.FilenameTemplate = core.DefaultTemplate
						}
						d.config.FilenameTemplate += tok.token
						d.save()
					})
					chip.Children(func() {
						ui.Text(c, tok.token).Font(mono).FontSize(11.5).TextColor(p.clayText)
						ui.Text(c, tok.meaning).FontSize(11.5).TextColor(p.text2)
					})
				}
			})
		})
		group := "No Subfolders"
		if d.config.SubfolderRule == "day" {
			group = "By Day"
		}
		if d.config.SubfolderRule == "month" {
			group = "By Month"
		}
		d.settingRow(c, "subfolders", "Group into subfolders", "Keep the output folder tidy by day or month.", func() {
			if d.dropdown(c, "subfolders", "Group into subfolders", &group, []string{"No Subfolders", "By Day", "By Month"}, false).Changed() {
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
		d.stackRow(c, "preview", func() {
			d.caption(c, "Preview")
			sample := time.Date(2026, 6, 27, 14, 32, 0, 0, time.Local)
			name := core.RenderName(d.config.FilenameTemplate, "meeting-notes", sample) + ".png"
			folder := core.OutputDirectory("", d.config.SubfolderRule, sample)
			ui.Row(c).Gap(10).Padding(10, 12).Radius(8).Background(p.fill).Children(func() {
				ui.Icon(c, icImage).Size(14, 14).TextColor(p.text3)
				ui.Text(c, filepath.Join(folder, name)).Font(mono).FontSize(12.5).Selectable().Grow(1).MinWidth(0).ContentTransition(ui.ContentTransition{})
			})
		})
		d.switchRow(c, "Use a custom naming prompt", "Replace the built-in instructions with your own.", &d.config.UseCustomPrompt, func() {
			if d.config.UseCustomPrompt && strings.TrimSpace(d.config.CustomPrompt) == "" {
				d.config.CustomPrompt = core.DefaultPrompt(d.config.Language)
			}
			d.save()
		})
		if d.config.UseCustomPrompt {
			ui.Column(c.Key("custom-prompt-row")).Gap(8).Padding(4, 16, 14, 16).Transition(ui.ElementTransition{Enter: &ui.Motion{Collapse: true}, Exit: &ui.Motion{Collapse: true}}).Children(func() {
				if d.textArea(c, "custom-prompt", &d.config.CustomPrompt, "Custom naming prompt", 140).Changed() {
					d.save()
				}
				ui.Row(c).Gap(8).Children(func() {
					d.caption(c, "Your prompt replaces the default. Names remain capped and filesystem-safe.").Grow(1)
					d.button(c, btn{label: "Reset prompt to default", icon: icUndo, kind: ghost, small: true}).OnClick(func() {
						d.config.CustomPrompt = core.DefaultPrompt(d.config.Language)
						d.save()
					})
				})
			})
		}
	})
}

// directoryRow shows a folder as a monospaced path that opens on click,
// with a quiet button to choose another.
func (d *Desktop) directoryRow(c *ui.Context, label, hint, path string, choose func()) {
	p := &d.pal
	ui.Row(c.Key("directory-"+label)).Gap(12).Padding(10, 12, 10, 16).Children(func() {
		ui.Row(c).Size(30, 30).Radius(8).Justify(ui.Center).Background(p.fill).Shrink(0).Children(func() {
			ui.Icon(c, icFolder).Size(15, 15).TextColor(p.text2)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				ui.Text(c, label).FontWeight(500)
				d.caption(c, hint).SingleLine()
			})
			ui.Text(c, d.shorten(path)).Font(mono).FontSize(12).TextColor(p.text2).SingleLine().Selectable()
		})
		d.button(c, btn{label: "Open", kind: ghost, small: true, iconOnly: true, icon: icArrowUR, tip: "Open in Finder"}).OnClick(func() { d.open(path) })
		d.button(c, btn{label: "Choose " + label + "…", kind: secondary, small: true}).OnClick(func() {
			if choose != nil {
				choose()
			}
		})
	})
}
