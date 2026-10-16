package desktop

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// palette holds Chaos's design tokens: a warm-neutral surface ladder
// (sidebar → canvas → card → raised), an ink alpha scale for hairlines and
// fills, and a single clay accent. Elevation comes from surface steps and
// hairlines rather than heavy borders.
type palette struct {
	dark bool

	sidebar, canvas, card, raised ui.Color

	// ink is the text color at full strength; translucent steps of it make
	// borders and fills that sit naturally on every surface.
	ink                   ui.Color
	border, borderStrong  ui.Color
	hover, pressed, fill  ui.Color
	text, text2, text3    ui.Color
	clay, clayHover       ui.Color
	clayPressed, claySoft ui.Color
	clayText              ui.Color
	success, danger, warn ui.Color
	tooltip, tooltipText  ui.Color
	shadow                ui.Color
	thumb, track          ui.Color
}

var (
	lightPalette = palette{
		sidebar: ui.Hex("#f3f2ec"), canvas: ui.Hex("#faf9f5"), card: ui.Hex("#ffffff"), raised: ui.Hex("#ffffff"),
		ink:  ui.Hex("#1f1e1b"),
		text: ui.Hex("#1f1e1b"), text2: ui.Hex("#67655f"), text3: ui.Hex("#98968f"),
		clay: ui.Hex("#c6613f"), clayHover: ui.Hex("#b5532f"), clayPressed: ui.Hex("#a3492a"), clayText: ui.Hex("#b4552f"),
		success: ui.Hex("#3f7f4c"), danger: ui.Hex("#c2453b"), warn: ui.Hex("#a86a14"),
		tooltip: ui.Hex("#2b2a27"), tooltipText: ui.Hex("#f4f2ec"),
		shadow: ui.RGBA(31, 30, 27, 0.08), thumb: ui.Hex("#ffffff"),
	}
	darkPalette = palette{
		dark:    true,
		sidebar: ui.Hex("#1d1c1a"), canvas: ui.Hex("#262522"), card: ui.Hex("#2e2d2a"), raised: ui.Hex("#353430"),
		ink:  ui.Hex("#f3f0e8"),
		text: ui.Hex("#ece9e1"), text2: ui.Hex("#a7a49b"), text3: ui.Hex("#76736c"),
		clay: ui.Hex("#d97757"), clayHover: ui.Hex("#e08264"), clayPressed: ui.Hex("#e8906f"), clayText: ui.Hex("#e48a6b"),
		success: ui.Hex("#6dbb82"), danger: ui.Hex("#e5736a"), warn: ui.Hex("#deaa55"),
		tooltip: ui.Hex("#f0eee6"), tooltipText: ui.Hex("#1f1e1b"),
		shadow: ui.RGBA(0, 0, 0, 0.42), thumb: ui.Hex("#45433f"),
	}
)

func (p *palette) derive(highContrast bool) {
	if p.dark {
		p.border, p.borderStrong = p.ink.Alpha(.09), p.ink.Alpha(.17)
		p.hover, p.pressed, p.fill = p.ink.Alpha(.06), p.ink.Alpha(.1), p.ink.Alpha(.07)
		p.claySoft = p.clay.Alpha(.16)
		p.track = p.ink.Alpha(.2)
	} else {
		p.border, p.borderStrong = p.ink.Alpha(.1), p.ink.Alpha(.2)
		p.hover, p.pressed, p.fill = p.ink.Alpha(.05), p.ink.Alpha(.09), p.ink.Alpha(.055)
		p.claySoft = p.clay.Alpha(.1)
		p.track = p.ink.Alpha(.16)
	}
	// Respect the system contrast preference even though Chaos owns its palette.
	if highContrast {
		p.text2, p.text3 = p.text, p.text
		p.border, p.borderStrong = p.ink.Alpha(.55), p.ink.Alpha(.75)
	}
}

// Motion tokens: quick color fades for hover, a short press, and springs
// for things that travel.
var (
	fade  = ui.Animation{Duration: 120 * time.Millisecond}
	press = ui.Animation{Duration: 90 * time.Millisecond}
	glide = ui.Animation{Spring: ui.SpringOptions{Duration: 320 * time.Millisecond, Bounce: 0.12}}
)

func (d *Desktop) theme(c *ui.Context) {
	dark := c.Theme().Dark
	switch d.config.Appearance {
	case "dark":
		dark = true
	case "light":
		dark = false
	}
	if dark {
		d.pal = darkPalette
	} else {
		d.pal = lightPalette
	}
	d.pal.derive(c.Preferences().HighContrast)
	p := &d.pal
	t := *ui.LightTheme()
	if dark {
		t = *ui.DarkTheme()
	}
	t.FontSize, t.Radius, t.Spacing, t.Font = 13, 8, 4, sans
	t.Background, t.Surface = p.canvas, p.card
	t.SurfaceHover, t.SurfacePressed = p.card.Mix(p.ink, .04), p.card.Mix(p.ink, .08)
	t.Text, t.TextMuted, t.Border = p.text, p.text2, p.border
	t.Accent, t.AccentHover, t.AccentPressed, t.AccentText = p.clay, p.clayHover, p.clayPressed, ui.Hex("#ffffff")
	t.Success, t.Danger, t.Warning = p.success, p.danger, p.warn
	t.Focus, t.Selection = p.clay, p.clay.Alpha(.22)
	t.Inverse, t.InverseText = p.tooltip, p.tooltipText
	t.Scrollbar, t.ScrollbarWidth = p.ink.Alpha(.22), 6
	if c.Preferences().HighContrast {
		t.TextMuted = t.Text
	}
	c.SetTheme(&t)
}
