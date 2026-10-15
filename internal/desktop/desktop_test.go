package desktop

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/michaelmjhhhh/Chaos/internal/app"
	"github.com/michaelmjhhhh/Chaos/internal/core"
)

type testController struct {
	snapshot app.Snapshot
	calls    []string
	saved    core.Config
}

func (f *testController) Snapshot() app.Snapshot { return f.snapshot }
func (f *testController) Start()                 { f.calls = append(f.calls, "start") }
func (f *testController) Stop()                  { f.calls = append(f.calls, "stop") }
func (f *testController) ProcessFiles(paths []string) {
	f.calls = append(f.calls, "process:"+strings.Join(paths, ","))
}
func (f *testController) Retry(id string)        { f.calls = append(f.calls, "retry:"+id) }
func (f *testController) RetryAll()              { f.calls = append(f.calls, "retry-all") }
func (f *testController) Rename(id, name string) { f.calls = append(f.calls, "rename:"+id+":"+name) }
func (f *testController) Revert(id string)       { f.calls = append(f.calls, "revert:"+id) }
func (f *testController) CheckHealth()           { f.calls = append(f.calls, "health") }
func (f *testController) CompleteOnboarding(start bool) {
	if start {
		f.calls = append(f.calls, "onboard-start")
	} else {
		f.calls = append(f.calls, "onboard-skip")
	}
}
func (f *testController) SaveConfig(cfg core.Config) {
	f.saved = cfg
	f.calls = append(f.calls, "save")
}
func (f *testController) DismissError()     { f.calls = append(f.calls, "dismiss") }
func (f *testController) ReplayOnboarding() { f.calls = append(f.calls, "replay") }
func (f *testController) SetAutoStart(v bool) {
	if v {
		f.calls = append(f.calls, "autostart-on")
	} else {
		f.calls = append(f.calls, "autostart-off")
	}
}

func fixture() (*Desktop, *testController) {
	now := time.Date(2026, 10, 13, 14, 0, 0, 0, time.Local)
	ctl := &testController{snapshot: app.Snapshot{OnboardingComplete: true, APIStatus: "OK", WatcherStatus: "Paused", Config: core.Config{Provider: "ollama"}}}
	ctl.snapshot.ResolvedWatchDir = filepath.FromSlash("/input")
	ctl.snapshot.ResolvedOutputDir = filepath.FromSlash("/out")
	ctl.snapshot.ConfigPath = filepath.FromSlash("/settings/config.json")
	ctl.snapshot.RecentFiles = []core.RecentFile{
		{ID: "one", OriginalName: "Screenshot.png", NewName: "meeting-notes.png", Path: "/out/meeting-notes.png", SourcePath: "/input/Screenshot.png", Timestamp: now, Duration: 1.2},
		{ID: "two", OriginalName: "Failed.jpg", SourcePath: "/input/Failed.jpg", Timestamp: now.Add(-time.Hour), IsError: true, ErrorMessage: "API timeout"},
		{ID: "three", OriginalName: "Old.png", NewName: "terminal-log.png", Path: "/out/terminal-log.png", Timestamp: now.AddDate(0, 0, -2), Duration: .5},
	}
	ctl.snapshot.Insights = core.Insights{TotalProcessed: 3, Successes: 2, Errors: 1, ThisMonth: 3, Daily: map[string]int{"2026-10-13": 2, "2026-10-11": 1}, TopCategories: []core.Labeled{{Label: "meeting", Count: 1}, {Label: "terminal", Count: 1}}, FileTypes: []core.Labeled{{Label: "png", Count: 2}}, AvgDuration: .85, P95Duration: 1.2, CurrentStreak: 1, LongestStreak: 1, Fastest: &core.DurationRecord{Name: "terminal-log.png", Duration: .5}, Slowest: &core.DurationRecord{Name: "meeting-notes.png", Duration: 1.2}}
	ctl.snapshot.Successes = 1
	ctl.snapshot.Errors = 1
	ctl.snapshot.AvgLatency = 1.2
	d := New(ctl, Actions{})
	d.now = func() time.Time { return now }
	return d, ctl
}
func click(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatalf("click %q: %v; texts=%q", label, err, tt.Texts())
	}
}
func expectCall(t *testing.T, c *testController, want string) {
	t.Helper()
	for _, v := range c.calls {
		if v == want {
			return
		}
	}
	t.Fatalf("missing %q in %q", want, c.calls)
}

func TestDashboardCommandsAndTextFocus(t *testing.T) {
	d, ctl := fixture()
	picked := false
	d.actions.PickImages = func() { picked = true }
	tt := ui.NewTester(d.View, 1000, 760)
	click(t, tt, "Start Watching")
	expectCall(t, ctl, "start")
	click(t, tt, "Organize Existing Screenshots")
	if !picked {
		t.Fatal("organizer not invoked")
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyT)
	expectCall(t, ctl, "health")
	tt.Key(ui.Cmd, ui.KeyComma)
	if d.tab != "Settings" {
		t.Fatal("settings shortcut failed")
	}
	// No Space global start/stop while a settings field is being edited.
	before := len(ctl.calls)
	click(t, tt, "Provider")
	tt.Key(0, ui.KeyEscape)
	tt.Key(0, ui.KeySpace)
	for _, call := range ctl.calls[before:] {
		if call == "start" || call == "stop" {
			t.Fatal("Space toggled watcher in settings")
		}
	}
}
func TestPipelineSearchFiltersAndRetry(t *testing.T) {
	d, ctl := fixture()
	d.SelectTab("Pipeline")
	tt := ui.NewTester(d.View, 1000, 800)
	if !tt.HasText("meeting-notes.png") || !tt.HasText("terminal-log.png") {
		t.Fatalf("history missing: %q", tt.Texts())
	}
	click(t, tt, "Errors")
	if tt.HasText("meeting-notes.png") || !tt.HasText("API timeout") {
		t.Fatal("error filter did not filter records")
	}
	click(t, tt, "Retry all")
	expectCall(t, ctl, "retry-all")
	click(t, tt, "All")
	click(t, tt, "Search filings")
	tt.Type("TERMINAL")
	if !tt.HasText("terminal-log.png") || tt.HasText("meeting-notes.png") {
		t.Fatalf("case insensitive search failed: %q", tt.Texts())
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("timeout")
	if !tt.HasText("API timeout") {
		t.Fatal("error search failed")
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("zzzz")
	if !tt.HasText("No matches.") {
		t.Fatal("empty search state missing")
	}
}

func TestDashboardNetworkRecovery(t *testing.T) {
	d, ctl := fixture()
	s := ctl.snapshot
	s.Error = "Couldn't reach the naming service. Check your internet, then try again."
	s.RecoveryAction = "internet"
	d.SetSnapshot(s)
	tt := ui.NewTester(d.View, 1000, 800)
	if !tt.HasText(s.Error) {
		t.Fatal("network recovery message missing")
	}
	click(t, tt, "Retry failed images")
	expectCall(t, ctl, "retry-all")
}
func TestPipelineContextRenameAndRestore(t *testing.T) {
	d, ctl := fixture()
	d.SelectTab("Pipeline")
	tt := ui.NewTester(d.View, 1000, 800)
	if err := tt.RightClick("meeting-notes.png"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Rename…"); err != nil {
		t.Fatal(err)
	}
	if !d.renameOpen {
		t.Fatal("rename modal did not open")
	}
	click(t, tt, "New name")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("clearer-name")
	tt.Key(0, ui.KeyEnter)
	expectCall(t, ctl, "rename:one:clearer-name")
	if d.renameOpen {
		t.Fatal("rename modal remained open")
	}
	if err := tt.RightClick("meeting-notes.png"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Undo (restore original name)"); err != nil {
		t.Fatal(err)
	}
	expectCall(t, ctl, "revert:one")
}
func TestProviderSwitchAndProtectedKey(t *testing.T) {
	d, ctl := fixture()
	d.SelectTab("Settings")
	tt := ui.NewTester(d.View, 1000, 1800)
	click(t, tt, "Provider")
	click(t, tt, "OpenAI")
	if ctl.saved.Provider != "openai" {
		t.Fatalf("provider=%q", ctl.saved.Provider)
	}
	click(t, tt, "API Key")
	tt.Type("secret-test-key")
	if ctl.saved.APIKey != "secret-test-key" {
		t.Fatalf("key was not saved: %#v", ctl.saved)
	}
	if tt.HasText("secret-test-key") {
		t.Fatal("API key appears unmasked in render tree")
	}
	click(t, tt, "Advanced options ▸")
	if !tt.HasText("Model") || !tt.HasText("https://api.openai.com/v1") {
		t.Fatal("advanced preset missing")
	}
	click(t, tt, "Provider")
	click(t, tt, "OpenAI-Compatible")
	if !tt.HasText("Base URL") {
		t.Fatal("custom endpoint must be visible immediately")
	}
	click(t, tt, "Advanced options ▾")
	if d.advanced || tt.HasText("Base URL") {
		t.Fatal("custom provider advanced options did not collapse")
	}
	click(t, tt, "Advanced options ▸")
	if !tt.HasText("Base URL") {
		t.Fatal("custom endpoint could not be reopened")
	}
}
func TestWelcomeRequiresProviderSetup(t *testing.T) {
	d, _ := fixture()
	d.config = core.Config{}
	d.ShowWelcome()
	tt := ui.NewTester(d.View, 1000, 1200)
	click(t, tt, "Continue")
	if !tt.HasText("Turn on naming.") || !tt.HasText("API Key") {
		t.Fatal("welcome must show provider setup")
	}
	// A disabled button may reject clicks; either way it must not advance.
	_ = tt.Click("Continue")
	if d.welcomeStep != 1 {
		t.Fatal("remote provider advanced without a key")
	}
	click(t, tt, "API Key")
	tt.Type("test-key")
	click(t, tt, "Continue")
	if !tt.HasText("Your tidy folder.") {
		t.Fatal("configured provider could not advance")
	}
	click(t, tt, "Back")
	if !tt.HasText("Turn on naming.") {
		t.Fatal("back navigation skipped provider setup")
	}
}
func TestHelpSearchAndWelcome(t *testing.T) {
	d, ctl := fixture()
	d.ShowHelp()
	tt := ui.NewTester(d.View, 1000, 900)
	click(t, tt, "Search help")
	tt.Type("filename style")
	if !tt.HasText("Can I change the filename style?") || tt.HasText("How do I start naming screenshots?") {
		t.Fatal("help filtering failed")
	}
	click(t, tt, "Can I change the filename style?")
	if !tt.HasText("{slug}") {
		t.Fatal("FAQ did not expand")
	}
	click(t, tt, "Replay Welcome Guide")
	expectCall(t, ctl, "replay")
	if !tt.HasText("Chaos names your screenshots.") {
		t.Fatal("welcome missing")
	}
	click(t, tt, "Continue")
	if !tt.HasText("Turn on naming.") {
		t.Fatal("provider setup step missing")
	}
	click(t, tt, "Continue")
	if !tt.HasText("Your tidy folder.") {
		t.Fatal("folder step missing")
	}
	click(t, tt, "Continue")
	click(t, tt, "Start Watching")
	expectCall(t, ctl, "onboard-start")
}
func TestTrayNavigationAndReveal(t *testing.T) {
	d, ctl := fixture()
	var path string
	shown := false
	d.actions.RevealPath = func(p string) { path = p }
	d.actions.ShowWindow = func() { shown = true }
	tt := ui.NewTester(d.TrayView, 350, 650)
	click(t, tt, "meeting-notes.png")
	if path != "/out/meeting-notes.png" {
		t.Fatalf("revealed %q", path)
	}
	click(t, tt, "Start")
	expectCall(t, ctl, "start")
	click(t, tt, "Settings")
	if !shown || d.tab != "Settings" {
		t.Fatal("tray did not reveal settings")
	}
}
func TestNativeLayoutsAndScreenshots(t *testing.T) {
	for _, page := range []struct{ tab, control string }{
		{"Dashboard", "Start Watching"}, {"Pipeline", "Search filings"},
		{"Insights", "Images processed"}, {"Settings", "Provider"}, {"Help", "Search help"},
	} {
		for _, dark := range []bool{false, true} {
			name := strings.ToLower(page.tab) + "-light"
			if dark {
				name = strings.ToLower(page.tab) + "-dark"
			}
			t.Run(name, func(t *testing.T) {
				d, _ := fixture()
				// Empty historical buckets must not move the archive's start month.
				d.snapshot.Insights.Daily["2020-01-01"] = 0
				d.SelectTab(page.tab)
				tt := ui.NewTester(d.View, 1000, 800)
				tt.SetDark(dark)
				for _, size := range [][2]int{{1000, 800}, {620, 560}} {
					tt.SetSize(size[0], size[1])
					for _, label := range []string{"Dashboard", "Pipeline", "Insights", "Settings", "Help & Getting Started", "Paused", page.control} {
						assertVisible(t, tt, label, size[0], size[1])
					}
					if page.tab == "Insights" && !tt.HasText("LIFETIME · SINCE OCT 2026") {
						t.Fatal("Insights lost the archive start month")
					}
					assertThemePixels(t, tt, dark)
					saveUIArtifact(t, fmt.Sprintf("%s-%dx%d", name, size[0], size[1]), tt)
				}
				// Navigation must still accept clicks at the minimum window size.
				click(t, tt, "Settings")
				if d.tab != "Settings" {
					t.Fatal("minimum-size navigation did not select Settings")
				}
			})
		}
	}
}

func TestCustomPromptPreviewAndAppearance(t *testing.T) {
	d, ctl := fixture()
	d.SelectTab("Settings")
	tt := ui.NewTester(d.View, 1000, 2400)
	click(t, tt, "Use a custom naming prompt")
	if !ctl.saved.UseCustomPrompt || strings.TrimSpace(ctl.saved.CustomPrompt) == "" {
		t.Fatalf("custom prompt did not enable with default: %#v", ctl.saved)
	}
	click(t, tt, "Custom naming prompt")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Name files with a short descriptive Chinese title.")
	if ctl.saved.CustomPrompt != "Name files with a short descriptive Chinese title." {
		t.Fatal("custom prompt edits not saved")
	}
	click(t, tt, "Filename Template")
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("{date}_{slug}")
	if !tt.HasText("2026-06-27-meeting-notes.png") {
		t.Fatalf("preview not live: %q", tt.Texts())
	}
	click(t, tt, "No Subfolders")
	click(t, tt, "By Month")
	wantPreview := filepath.Join("2026-06", "2026-06-27-meeting-notes.png")
	if !tt.HasText(wantPreview) {
		t.Fatalf("subfolder preview missing native path %q; texts=%q", wantPreview, tt.Texts())
	}
	click(t, tt, "Dark")
	if ctl.saved.Appearance != "dark" {
		t.Fatal("appearance not saved")
	}
	click(t, tt, "Copy image to clipboard")
	if !ctl.saved.CopyToClipboard {
		t.Fatal("clipboard preference not saved")
	}
	click(t, tt, "Start watching on launch")
	expectCall(t, ctl, "autostart-on")
}

func TestSnapshotOrderingAndAnnouncements(t *testing.T) {
	d, _ := fixture()
	tt := ui.NewTester(d.View, 1000, 700)
	newest := d.snapshot
	newest.Revision = 9
	newest.CurrentFile = "named.png"
	newest.CurrentStage = "success"
	d.SetSnapshot(newest)
	tt.Frame()
	old := newest
	old.Revision = 8
	old.CurrentFile = "stale.png"
	d.SetSnapshot(old)
	tt.Frame()
	if !tt.HasText("named.png") || tt.HasText("stale.png") {
		t.Fatal("older snapshot replaced UI")
	}
	if len(tt.Announcements()) == 0 {
		t.Fatal("filing success not announced")
	}
}

func TestNativeTrayAndWelcomeScreenshots(t *testing.T) {
	d, _ := fixture()
	for i := 0; i < 2; i++ {
		f := d.snapshot.RecentFiles[0]
		f.ID = string(rune('a' + i))
		f.NewName = "another-image-with-a-longer-name.png"
		d.snapshot.RecentFiles = append(d.snapshot.RecentFiles, f)
	}
	tray := ui.NewTester(d.TrayView, 370, 540)
	assertVisible(t, tray, "Quit Chaos", 370, 540)
	assertVisible(t, tray, "Open Output", 370, 540)
	saveUIArtifact(t, "tray", tray)
	d.ShowWelcome()
	welcome := ui.NewTester(d.View, 1000, 780)
	for step := range 4 {
		label := "Continue"
		if step == 3 {
			label = "Start Watching"
		}
		assertVisible(t, welcome, "Skip", 1000, 780)
		assertVisible(t, welcome, label, 1000, 780)
		saveUIArtifact(t, fmt.Sprintf("welcome-%d", step), welcome)
		click(t, welcome, label)
		if step < 3 && d.welcomeStep != step+1 {
			t.Fatalf("welcome did not advance from step %d", step)
		}
	}
}

func TestWarningCanBeDismissedWithoutAnError(t *testing.T) {
	d, ctl := fixture()
	d.snapshot.Warning = "Clipboard was unavailable"
	tt := ui.NewTester(d.View, 1000, 900)
	if !tt.HasText(d.snapshot.Warning) {
		t.Fatal("warning missing")
	}
	click(t, tt, "Dismiss")
	expectCall(t, ctl, "dismiss")
	s := d.snapshot
	s.Warning = ""
	d.SetSnapshot(s)
	tt.Frame()
	if tt.HasText("Clipboard was unavailable") {
		t.Fatal("dismissed warning still rendered")
	}
}
