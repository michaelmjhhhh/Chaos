package desktop

import (
	"embed"
	"sync"

	"github.com/egoist/mygo/ui"
)

// Chaos ships its typefaces so it reads the same on every desktop: Inter for
// the interface, Newsreader for titles and figures, JetBrains Mono for
// filenames. All are under the SIL Open Font License; see fonts/LICENSE-*.
// Each weight is a static face, as MyGo matches registered faces by weight.
//
//go:embed fonts/*.ttf
var fontFiles embed.FS

const (
	sans  = "Inter"
	serif = "Newsreader"
	mono  = "JetBrains Mono"
)

var registerFonts = sync.OnceFunc(func() {
	for _, f := range []struct{ file, family string }{
		{"Inter-Regular.ttf", sans},
		{"Inter-Medium.ttf", sans},
		{"Inter-SemiBold.ttf", sans},
		{"Newsreader-Regular.ttf", serif},
		{"JetBrainsMono-Regular.ttf", mono},
	} {
		data, err := fontFiles.ReadFile("fonts/" + f.file)
		if err != nil {
			continue
		}
		// A face the text engine refuses leaves its family to the system's
		// fonts, which every lookup falls back to.
		_ = ui.RegisterFont(data, f.family)
	}
})
