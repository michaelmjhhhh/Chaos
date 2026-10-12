//go:build darwin

package core

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHEICDecodeAndVisionPreparation(t *testing.T) {
	dir := t.TempDir()
	// Generate all input locally. This exercises macOS's real HEIC codec rather
	// than renaming a PNG or mocking the conversion command.
	synthetic := image.NewNRGBA(image.Rect(0, 0, 1600, 800))
	for y := 0; y < 800; y++ {
		for x := 0; x < 1600; x++ {
			pixel := color.NRGBA{R: 255, A: 255}
			if x >= 800 {
				pixel = color.NRGBA{B: 255, A: 255}
			}
			synthetic.SetNRGBA(x, y, pixel)
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, synthetic); err != nil {
		t.Fatal(err)
	}
	pngPath, heicPath := filepath.Join(dir, "synthetic.png"), filepath.Join(dir, "synthetic.HEIC")
	writeTestFile(t, pngPath, encoded.Bytes())
	command := exec.Command("/usr/bin/sips", "--setProperty", "format", "heic", pngPath, "--out", heicPath)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create synthetic HEIC with macOS codec: %v: %s", err, output)
	}
	before, err := os.ReadFile(heicPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) < 12 || string(before[4:8]) != "ftyp" {
		t.Fatal("sips did not create an ISO media HEIC container")
	}
	conversionDir := filepath.Join(dir, "conversions")
	if err := os.Mkdir(conversionDir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", conversionDir)
	decoded, err := DecodeImage(context.Background(), heicPath)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Size() != image.Pt(1600, 800) {
		t.Fatalf("native HEIC decode lost full resolution: %v", decoded.Bounds())
	}
	assertRedBlueImage(t, decoded)
	prepared, err := PrepareImage(context.Background(), heicPath)
	if err != nil {
		t.Fatal(err)
	}
	vision := decodeVisionJPEG(t, prepared, image.Pt(1280, 640))
	assertRedBlueImage(t, vision)
	assertImageBytesUnchanged(t, heicPath, before)
	assertNoHEICConversions(t, conversionDir)

	t.Run("corrupt HEIC preserves source", func(t *testing.T) {
		path := filepath.Join(dir, "truncated.heic")
		writeTestFile(t, path, before[:32])
		assertRejectedImagePreservesSource(t, path)
		assertNoHEICConversions(t, conversionDir)
	})
	t.Run("canceled conversion preserves source", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := DecodeImage(ctx, heicPath); !errors.Is(err, context.Canceled) {
			t.Fatalf("native decode did not report cancellation: %v", err)
		}
		if _, err := PrepareImage(ctx, heicPath); !errors.Is(err, context.Canceled) {
			t.Fatalf("vision preparation did not report cancellation: %v", err)
		}
		assertImageBytesUnchanged(t, heicPath, before)
		assertNoHEICConversions(t, conversionDir)
	})
}

func assertRedBlueImage(t *testing.T, decoded image.Image) {
	t.Helper()
	bounds := decoded.Bounds()
	r, g, b, _ := decoded.At(bounds.Dx()/4, bounds.Dy()/2).RGBA()
	if r < 50000 || g > 15000 || b > 15000 {
		t.Fatalf("HEIC red content changed: %d %d %d", r, g, b)
	}
	r, g, b, _ = decoded.At(bounds.Dx()*3/4, bounds.Dy()/2).RGBA()
	if b < 50000 || g > 15000 || r > 15000 {
		t.Fatalf("HEIC blue content changed: %d %d %d", r, g, b)
	}
}

func assertNoHEICConversions(t *testing.T, dir string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "chaos-image-*"))
	if err != nil || len(paths) != 0 {
		t.Fatalf("HEIC conversion left temporary files: %v, %v", paths, err)
	}
}
