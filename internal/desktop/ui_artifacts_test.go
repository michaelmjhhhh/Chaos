package desktop

import (
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func assertVisible(t *testing.T, tt *ui.Tester, label string, width, height int) {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok || r.W <= 0 || r.H <= 0 || r.X < 0 || r.Y < 0 || r.X+r.W > float32(width) || r.Y+r.H > float32(height) {
		t.Fatalf("%q is missing or clipped in %dx%d: %#v found=%v", label, width, height, r, ok)
	}
}

func assertThemePixels(t *testing.T, tt *ui.Tester, dark bool) {
	t.Helper()
	// The left edge is the sidebar, the darkest step of the surface ladder.
	want := color.RGBA{R: 243, G: 242, B: 236, A: 255}
	if dark {
		want = color.RGBA{R: 29, G: 28, B: 26, A: 255}
	}
	img := tt.Image()
	if got := img.RGBAAt(0, img.Bounds().Dy()/2); got != want {
		t.Fatalf("system appearance not reflected in background: got %v want %v", got, want)
	}
	// Verify the renderer drew content, rather than merely allocating a canvas.
	var content int
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.RGBAAt(x, y) != want {
				content++
			}
		}
	}
	if content < 1000 {
		t.Fatalf("rendered only %d non-background pixels", content)
	}
}

// Each caller creates its destination, so any screenshot test can run alone.
func saveUIArtifact(t *testing.T, name string, tt *ui.Tester) {
	t.Helper()
	output := os.Getenv("CHAOS_UI_ARTIFACTS")
	if output == "" {
		return
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(output, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	encodeErr := png.Encode(f, tt.Image())
	closeErr := f.Close()
	if encodeErr != nil {
		t.Fatal(encodeErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
}
