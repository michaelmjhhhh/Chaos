package desktop

import "github.com/egoist/mygo/ui"

func (d *Desktop) theme(c *ui.Context) {
	t := *c.Theme()
	if d.config.Appearance == "dark" {
		t = *ui.DarkTheme()
	} else if d.config.Appearance == "light" {
		t = *ui.LightTheme()
	}
	t.FontSize = 13
	t.Radius = 7
	if t.Dark {
		t.Background = ui.Hex("#1a1918")
		t.Surface = ui.Hex("#262523")
		t.SurfaceHover = ui.Hex("#322f2b")
		t.SurfacePressed = ui.Hex("#3f3c36")
		t.Text = ui.Hex("#edeae3")
		t.TextMuted = ui.Hex("#aaa59b")
		t.Border = ui.Hex("#3a3833")
		t.Accent = ui.Hex("#d98e6b")
		t.AccentHover = ui.Hex("#e49c79")
		t.AccentPressed = ui.Hex("#f0a985")
		t.Success = ui.Hex("#5fbe79")
		t.Danger = ui.Hex("#e06b6b")
	} else {
		t.Background = ui.Hex("#faf9f5")
		t.Surface = ui.Hex("#ffffff")
		t.SurfaceHover = ui.Hex("#f5f0e8")
		t.SurfacePressed = ui.Hex("#ebe6df")
		t.Text = ui.Hex("#1f1e1b")
		t.TextMuted = ui.Hex("#6c6a64")
		t.Border = ui.Hex("#e2ddd4")
		t.Accent = ui.Hex("#b85e43")
		t.AccentHover = ui.Hex("#a9583e")
		t.AccentPressed = ui.Hex("#99482e")
		t.Success = ui.Hex("#397c49")
		t.Danger = ui.Hex("#c64545")
	}
	t.AccentText = ui.Hex("#ffffff")
	t.Focus = t.Accent
	t.Selection = t.Accent.Alpha(.22)
	// Respect system contrast preference even though Chaos owns its palette.
	if c.Preferences().HighContrast {
		t.TextMuted = t.Text
		t.Border = t.Text.Alpha(.65)
	}
	c.SetTheme(&t)
}
func heading(c *ui.Context, title, subtitle string) {
	ui.Text(c, title).Font("Georgia").FontSize(30)
	if subtitle != "" {
		ui.Text(c, subtitle).TextColor(c.Theme().TextMuted)
	}
}
func small(c *ui.Context, text string) { ui.Text(c, text).FontSize(11).TextColor(c.Theme().TextMuted) }
func section(c *ui.Context, title string, build func()) {
	ui.Column(c.Key(title)).Padding(20).Gap(12).Background(c.Theme().Surface).Radius(10).Border(1, c.Theme().Border).Children(func() {
		ui.Text(c, title).Font("Georgia").FontSize(20)
		build()
	})
}
func metric(c *ui.Context, value, label string) {
	ui.Column(c).Grow(1).Gap(4).Children(func() { ui.Text(c, value).Font("Georgia").FontSize(32); small(c, label) })
}
