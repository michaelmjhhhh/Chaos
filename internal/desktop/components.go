package desktop

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// The components below follow one rule for edges: a container that stands
// on its own (card, field, dialog, popover) has one hairline; anything inside
// it is separated by fills, dividers or space, never another closed border.

type variant int

const (
	primary variant = iota
	secondary
	ghost
	outline
	destructive
)

type btn struct {
	label    string
	icon     *ui.SVG
	trailing *ui.SVG
	kind     variant
	small    bool
	iconOnly bool
	disabled bool
	tip      string
}

var white = ui.Hex("#ffffff")

// button is the one push button: four sizes collapse into regular and
// small, colors ease between rest, hover and press, and a press settles the
// button 3% smaller for a tactile click.
func (d *Desktop) button(c *ui.Context, b btn) ui.Element {
	p := &d.pal
	h, pad, radius, size, glyph := float32(32), float32(12), float32(8), float32(13), float32(15)
	if b.small {
		h, pad, radius, size, glyph = 26, 9, 6, 12, 13
	}
	e := ui.ButtonBase(c).Height(h).Gap(6).Justify(ui.Center).Radius(radius).FontSize(size).FontWeight(500).Shrink(0)
	if b.iconOnly {
		e.Width(h)
	} else {
		e.PaddingX(pad)
	}
	hovered, pressed := e.Hovered() && !b.disabled, e.Pressed() && !b.disabled
	var bg, fg ui.Color
	switch b.kind {
	case primary:
		bg, fg = p.clay, white
		if pressed {
			bg = p.clayPressed
		} else if hovered {
			bg = p.clayHover
		}
		e.Shadow(0, 1, 2, 0, p.clayPressed.Alpha(.28)).InsetShadow(0, 1, 0, 0, white.Alpha(.14))
	case secondary:
		bg, fg = p.fill, p.text
		if pressed {
			bg = p.ink.Alpha(.14)
		} else if hovered {
			bg = p.ink.Alpha(.1)
		}
	case ghost:
		bg, fg = p.hover.Alpha(0), p.text2
		if pressed {
			bg, fg = p.pressed, p.text
		} else if hovered {
			bg, fg = p.hover, p.text
		}
	case outline:
		bg, fg = p.hover.Alpha(0), p.text
		edge := p.border
		if hovered || pressed {
			bg, edge = p.hover, p.borderStrong
		}
		e.Border(1, e.AnimateColor("edge", edge, fade))
	case destructive:
		bg, fg = p.danger.Alpha(0), p.danger
		if pressed {
			bg = p.danger.Alpha(.15)
		} else if hovered {
			bg = p.danger.Alpha(.09)
		}
	}
	e.Background(e.AnimateColor("bg", bg, fade)).TextColor(e.AnimateColor("fg", fg, fade))
	scale := float32(1)
	if pressed {
		scale = .97
	}
	e.AnimatePose(ui.Pose{Scale: scale}, press)
	if b.disabled {
		e.Disabled(true).Opacity(.42)
	}
	if b.iconOnly {
		e.Label(b.label)
		if b.tip == "" {
			b.tip = b.label
		}
	}
	if b.tip != "" {
		e.Tooltip(b.tip)
	}
	e.Children(func() {
		if b.icon != nil {
			ui.Icon(c, b.icon).Size(glyph, glyph)
		}
		if !b.iconOnly {
			ui.Text(c, b.label).SingleLine()
		}
		if b.trailing != nil {
			ui.Icon(c, b.trailing).Size(glyph-2, glyph-2).Opacity(.7)
		}
	})
	return e
}

// glide paints a highlight under host's children that springs toward
// target, the box of the chosen child in the last layout. Absolute children
// paint above flow children, so the highlight is drawn by the parent.
func (d *Desktop) glider(c *ui.Context, host ui.Element, key string, target ui.Rect, paint func(p *ui.Painter, r ui.Rect)) {
	hb := host.Bounds()
	if target.W <= 0 || hb.W <= 0 {
		// Nothing laid out yet: measure in the next frame.
		c.AnimationFrame()
		return
	}
	x := host.AnimateTo(key+"-x", target.X-hb.X, glide)
	y := host.AnimateTo(key+"-y", target.Y-hb.Y, glide)
	w := host.AnimateTo(key+"-w", target.W, glide)
	h := host.AnimateTo(key+"-h", target.H, glide)
	host.Draw(func(p *ui.Painter, r ui.Rect) { paint(p, ui.Rect{X: r.X + x, Y: r.Y + y, W: w, H: h}) })
}

// raisedChip paints the chosen segment or page: a lifted card in light
// mode, a faint fill in dark mode.
func (d *Desktop) raisedChip(radius float32) func(p *ui.Painter, r ui.Rect) {
	pal := d.pal
	return func(p *ui.Painter, r ui.Rect) {
		if pal.dark {
			p.Fill(r, pal.thumb, radius)
			return
		}
		p.Shadow(r, radius, 0, 1, 2, 0, pal.shadow)
		p.Fill(r, pal.thumb, radius)
		p.Stroke(r, pal.border, radius, .5)
	}
}

type seg struct {
	label string
	icon  *ui.SVG
	count string
}

func (d *Desktop) segmented(c *ui.Context, key string, sel *int, items []seg) ui.Element {
	p := &d.pal
	s := ui.SegmentedBase(c.Key(key), sel, len(items))
	s.Track.Padding(2).Gap(2).Radius(9).Background(p.fill).Shrink(0)
	var chosen ui.Rect
	s.Track.Children(func() {
		for i, it := range items {
			e := s.Segment(i).Height(26).PaddingX(11).Gap(6).Radius(7).Justify(ui.Center).FontSize(12.5).FontWeight(500).Label(it.label)
			active := i == *sel
			fg := p.text2
			if active || e.Hovered() {
				fg = p.text
			}
			e.TextColor(e.AnimateColor("fg", fg, fade))
			if active {
				chosen = e.Bounds()
			}
			e.Children(func() {
				if it.icon != nil {
					ui.Icon(c, it.icon).Size(14, 14)
				}
				ui.Text(c, it.label).SingleLine()
				if it.count != "" {
					ui.Text(c, it.count).FontSize(11).TextColor(p.text3).FontFeatures("tnum")
				}
			})
		}
	})
	d.glider(c, s.Track, "thumb", chosen, d.raisedChip(7))
	return s.Track
}

type fieldOpts struct {
	label, placeholder string
	icon               *ui.SVG
	password, code     bool
	handle             *ui.Handle
	clearable          bool
	hint               string
	suffix             string
	autofocus          bool
}

// input is a single-line field: a hairline box that warms to clay with a
// soft halo while focused. It returns the editable element so callers can
// ask for Changed and Submitted.
func (d *Desktop) input(c *ui.Context, key string, value *string, o fieldOpts) ui.Element {
	p := &d.pal
	box := ui.Row(c.Key(key + "-box")).Height(32).PaddingX(10).Gap(8).Radius(8).Background(p.card).Cursor(ui.CursorText)
	focused := box.FocusWithin()
	edge := p.border
	switch {
	case focused:
		edge = p.clay.Alpha(.6)
	case box.Hovered():
		edge = p.borderStrong
	}
	box.Border(1, box.AnimateColor("edge", edge, fade))
	halo := p.clay.Alpha(0)
	if focused {
		halo = p.clay.Alpha(.14)
	}
	box.Shadow(0, 0, 0, 3, box.AnimateColor("halo", halo, fade))
	var in ui.Element
	box.Children(func() {
		if o.icon != nil {
			tint := p.text3
			if focused {
				tint = p.text2
			}
			ui.Icon(c, o.icon).Size(14, 14).TextColor(tint)
		}
		in = ui.TextInputBase(c.Key(key), value).Grow(1).MinWidth(0).FocusRing(false).Placeholder(o.placeholder)
		if o.label != "" {
			in.Label(o.label)
		}
		if o.code {
			in.Font(mono).FontSize(12.5)
		}
		if o.password {
			in.Password()
		}
		if o.handle != nil {
			in.Bind(o.handle)
		}
		if o.autofocus {
			in.AutoFocus()
		}
		if o.suffix != "" {
			ui.Text(c, o.suffix).Font(mono).FontSize(12.5).TextColor(p.text3).SingleLine()
		}
		if o.clearable && *value != "" {
			clear := ui.ButtonBase(c).Size(16, 16).Radius(8).Justify(ui.Center).Label("Clear").FocusRing(false)
			tone := p.ink.Alpha(.22)
			if clear.Hovered() {
				tone = p.ink.Alpha(.38)
			}
			clear.Background(clear.AnimateColor("bg", tone, fade)).TextColor(p.card)
			clear.Children(func() { ui.Icon(c, icX).Size(10, 10) })
			if clear.Clicked() {
				*value = ""
				if o.handle != nil {
					o.handle.Focus()
				}
			}
		} else if o.hint != "" && !focused {
			d.keys(c, o.hint)
		}
	})
	return in
}

// textArea is input's multi-line sibling.
func (d *Desktop) textArea(c *ui.Context, key string, value *string, label string, height float32) ui.Element {
	p := &d.pal
	box := ui.Column(c.Key(key+"-box")).Height(height).Padding(9, 11).Radius(8).Background(p.card).Cursor(ui.CursorText)
	focused := box.FocusWithin()
	edge, halo := p.border, p.clay.Alpha(0)
	if focused {
		edge, halo = p.clay.Alpha(.6), p.clay.Alpha(.14)
	} else if box.Hovered() {
		edge = p.borderStrong
	}
	box.Border(1, box.AnimateColor("edge", edge, fade)).Shadow(0, 0, 0, 3, box.AnimateColor("halo", halo, fade))
	var in ui.Element
	box.Children(func() {
		in = ui.TextAreaBase(c.Key(key), value).Grow(1).FocusRing(false).Label(label).LineHeight(1.45)
	})
	return in
}

// dropdown is a select whose menu floats on a raised surface; the chosen
// option carries a check and the highlighted one a soft fill.
func (d *Desktop) dropdown(c *ui.Context, key, label string, value *string, options []string, disabled bool) ui.Element {
	p := &d.pal
	sel := ui.SelectBase(c.Key(key), value)
	tr := sel.Trigger.Height(32).PaddingX(10).Gap(8).Radius(8).MinWidth(168).Label(label).Background(p.card).FocusRing(false)
	open := sel.Open()
	edge, halo := p.border, p.clay.Alpha(0)
	switch {
	case open || tr.FocusVisible():
		edge, halo = p.clay.Alpha(.6), p.clay.Alpha(.14)
	case tr.Hovered() && !disabled:
		edge = p.borderStrong
	}
	tr.Border(1, tr.AnimateColor("edge", edge, fade)).Shadow(0, 0, 0, 3, tr.AnimateColor("halo", halo, fade))
	if disabled {
		tr.Disabled(true).Opacity(.5)
	}
	tr.Children(func() {
		ui.Text(c, *value).SingleLine().Grow(1)
		ui.Icon(c, icUpDown).Size(13, 13).TextColor(p.text3)
	})
	sel.Popup(func(panel ui.Element) {
		d.popover(panel).Margin(6, 0, 0, 0).Padding(4).Gap(1).MinWidth(200)
		panel.Transition(ui.ElementTransition{Duration: 140 * time.Millisecond, Enter: &ui.Motion{Y: -4, Scale: .98}, Exit: &ui.Motion{Scale: .98}})
		for _, opt := range options {
			item := sel.Item(opt).Height(28).PaddingX(8).Gap(10).Radius(6)
			if item.Highlighted() {
				item.Background(p.hover)
			}
			item.Children(func() {
				ui.Text(c, opt).SingleLine().Grow(1)
				if opt == *value {
					ui.Icon(c, icCheck).Size(13, 13).TextColor(p.clay)
				} else {
					ui.Box(c).Size(13, 13)
				}
			})
		}
	})
	return tr
}

// switchRow is a settings row that is itself the switch: the label, hint
// and track toggle together. The knob stretches while pressed and glides
// with a little spring; keyboard focus rings the track rather than the row.
func (d *Desktop) switchRow(c *ui.Context, title, hint string, value *bool, changed func()) {
	p := &d.pal
	row := ui.SwitchBase(c.Key("switch-"+title), value).Gap(16).Padding(12, 16).Label(title).FocusRing(false)
	on, pressed, ring := *value, row.Pressed(), row.FocusVisible()
	row.Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, title).FontWeight(500)
			if hint != "" {
				ui.Text(c, hint).FontSize(12).TextColor(p.text2)
			}
		})
		track := ui.Box(c).Size(32, 19).Radius(10).Shrink(0)
		bg := p.track
		if on {
			bg = p.clay
		}
		track.Background(track.AnimateColor("bg", bg, ui.Animation{Duration: 160 * time.Millisecond}))
		halo := p.clay.Alpha(0)
		if ring {
			halo = p.clay.Alpha(.3)
		}
		track.Shadow(0, 0, 0, 3, track.AnimateColor("ring", halo, fade))
		knobW := float32(15)
		if pressed {
			knobW = 19
		}
		w := track.AnimateTo("w", knobW, press)
		x := float32(2)
		if on {
			x = 32 - 2 - knobW
		}
		x = track.AnimateTo("x", x, glide)
		track.Children(func() {
			ui.Box(c).Absolute().Top(2).Left(x).Size(w, 15).Radius(7.5).Background(white).
				Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, .22)).Shadow(0, 0, 0, .5, ui.RGBA(0, 0, 0, .06))
		})
	})
	if row.Changed() {
		changed()
	}
}

func (d *Desktop) card(e ui.Element) ui.Element {
	p := &d.pal
	e.Background(p.card).Radius(12).Border(1, p.border)
	if p.dark {
		e.InsetShadow(0, 1, 0, 0, white.Alpha(.03))
	} else {
		e.Shadow(0, 1, 2, 0, p.shadow.Alpha(.04)).Shadow(0, 2, 8, 0, p.shadow.Alpha(.035))
	}
	return e
}

func (d *Desktop) popover(e ui.Element) ui.Element {
	p := &d.pal
	e.Background(p.raised).Radius(10)
	if p.dark {
		e.InsetShadow(0, 0, 0, 1, white.Alpha(.1)).Shadow(0, 8, 28, 0, ui.RGBA(0, 0, 0, .55)).Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, .35))
	} else {
		e.Border(1, p.border).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, .12)).Shadow(0, 2, 6, 0, ui.RGBA(0, 0, 0, .06))
	}
	return e
}

// group is a titled card of rows split by hairlines, as in System Settings.
func (d *Desktop) group(c *ui.Context, title, hint string, build func()) {
	p := &d.pal
	ui.Column(c.Key("group-" + title)).Gap(8).Children(func() {
		ui.Row(c).PaddingX(4).Gap(8).Children(func() {
			ui.Text(c, title).FontWeight(600).Role(ui.RoleHeading).Level(2)
			if hint != "" {
				ui.Text(c, hint).FontSize(12).TextColor(p.text3).SingleLine()
			}
		})
		d.card(ui.Column(c)).Gap(1).Dividers(1, p.border).Clip().Children(build)
	})
}

// settingRow lays a title and hint beside a control.
func (d *Desktop) settingRow(c *ui.Context, key, title, hint string, control func()) {
	p := &d.pal
	ui.Row(c.Key("row-"+key)).Gap(16).Padding(12, 16).Children(func() {
		ui.Column(c).Grow(1).Gap(2).MinWidth(0).Children(func() {
			ui.Text(c, title).FontWeight(500)
			if hint != "" {
				ui.Text(c, hint).FontSize(12).TextColor(p.text2)
			}
		})
		control()
	})
}

// stackRow holds a labeled control that needs the full width.
func (d *Desktop) stackRow(c *ui.Context, key string, build func()) {
	ui.Column(c.Key("stack-"+key)).Gap(8).Padding(14, 16).Children(build)
}

// field labels a control above it; a click on the label focuses it.
func (d *Desktop) field(c *ui.Context, label, hint string, build func()) {
	f := ui.Field(c, label, build)
	if hint != "" {
		f.Description(hint)
	}
}

// keys draws a shortcut as keycaps, as "⌘ ⇧ 4".
func (d *Desktop) keys(c *ui.Context, combo string) {
	p := &d.pal
	ui.Row(c).Gap(3).Shrink(0).Label(combo).Children(func() {
		for _, k := range []rune(combo) {
			if k == ' ' {
				continue
			}
			ui.Text(c, string(k)).FontSize(11).FontWeight(500).TextColor(p.text2).TextAlign(ui.Center).
				MinWidth(18).Height(18).Padding(1, 4).Radius(4).Background(p.card).Border(1, p.border).
				Shadow(0, 1, 0, 0, p.border)
		}
	})
}

func (d *Desktop) pill(c *ui.Context, text string, fg, bg ui.Color) ui.Element {
	return ui.Row(c).Padding(2, 8).Radius(999).Background(bg).TextColor(fg).FontSize(11).FontWeight(500).Shrink(0).Gap(5).
		Children(func() { ui.Text(c, text).SingleLine() })
}

// pulse is a status dot; live dots breathe a soft ring outward, painted
// without rebuilding the view, at a modest rate since Chaos may watch all day.
func (d *Desktop) pulse(c *ui.Context, color ui.Color, live bool, label string) ui.Element {
	still := c.Preferences().ReduceMotion
	return ui.Box(c).Size(8, 8).Shrink(0).Label(label).Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		if live && !still {
			t := float32(p.Now().UnixMilli()%2000) / 2000
			p.FillPath((&ui.Path{}).Circle(cx, cy, 3.5+5*t), color.Alpha(.32*(1-t)))
			p.After(40 * time.Millisecond)
		}
		p.FillPath((&ui.Path{}).Circle(cx, cy, 3.5), color)
	})
}

// alert is an inline banner: a tinted hairline box with an icon, the
// message, and its actions. It slides in from above and folds away.
func (d *Desktop) alert(c *ui.Context, key string, tone ui.Color, ic *ui.SVG, text string, actions func()) {
	p := &d.pal
	ui.Row(c.Key("alert-"+key)).Padding(10, 12).Gap(10).Radius(10).AlignItems(ui.Start).
		Background(tone.Alpha(.07)).Border(1, tone.Alpha(.22)).
		Transition(ui.ElementTransition{Enter: &ui.Motion{Y: -6}, Exit: &ui.Motion{Collapse: true}}).
		Children(func() {
			ui.Icon(c, ic).Size(16, 16).TextColor(tone).Margin(1, 0, 0, 0)
			ui.Text(c, text).Selectable().Grow(1).MinWidth(0).TextColor(p.text).LineHeight(1.4)
			if actions != nil {
				ui.Row(c).Gap(4).Shrink(0).Margin(-4, -4, -4, 0).Children(actions)
			}
		})
}

// emptyState centers a quiet message in a page or card.
func (d *Desktop) emptyState(c *ui.Context, ic *ui.SVG, title, body string) {
	p := &d.pal
	ui.Column(c).Gap(10).Padding(40, 24).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Size(44, 44).Radius(12).Justify(ui.Center).Background(p.fill).Children(func() {
			ui.Icon(c, ic).Size(20, 20).TextColor(p.text2)
		})
		ui.Text(c, title).Font(serif).FontSize(22).TextAlign(ui.Center)
		if body != "" {
			ui.Text(c, body).TextColor(p.text2).TextAlign(ui.Center).MaxWidth(340)
		}
	})
}

func (d *Desktop) caption(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(11.5).FontWeight(500).TextColor(d.pal.text3)
}

func (d *Desktop) muted(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(12).TextColor(d.pal.text2)
}

// thumb shows a filed image, or a soft placeholder while it loads.
func (d *Desktop) thumb(c *ui.Context, path string, w, h, radius float32) {
	p := &d.pal
	if bitmap := d.thumbnail(c, path); bitmap != nil {
		ui.Image(c, bitmap).Size(w, h).Fit(ui.Cover).Radius(radius).Shrink(0).
			Transition(ui.ElementTransition{Enter: &ui.Motion{Scale: .96}})
		return
	}
	ui.Row(c).Size(w, h).Radius(radius).Justify(ui.Center).Shrink(0).Background(p.fill).Children(func() {
		ui.Icon(c, icImage).Size(min(w, h)*.36, min(w, h)*.36).TextColor(p.text3)
	})
}
