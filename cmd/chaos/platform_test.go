package main

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestThumbnailPreservesAspectRatioAndTransparency(t *testing.T) {
	for _, dimensions := range [][2]int{{1280, 320}, {32, 8}} {
		source := image.NewNRGBA(image.Rect(0, 0, dimensions[0], dimensions[1]))
		ink := color.NRGBA{R: 200, G: 40, B: 80, A: 128}
		draw.Draw(source, source.Bounds(), image.NewUniform(ink), image.Point{}, draw.Src)
		path := filepath.Join(t.TempDir(), "source.png")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		encodeErr := png.Encode(f, source)
		closeErr := f.Close()
		if encodeErr != nil || closeErr != nil {
			t.Fatalf("write fixture: %v, %v", encodeErr, closeErr)
		}
		got, err := thumbnail(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		wantWidth := min(dimensions[0], 640)
		if got.Bounds().Dx() != wantWidth || got.Bounds().Dy() != wantWidth/4 {
			t.Fatalf("thumbnail dimensions %v", got.Bounds())
		}
		if pixel := color.NRGBAModel.Convert(got.At(0, 0)); pixel != ink {
			t.Fatalf("thumbnail changed transparent pixel: got %v want %v", pixel, ink)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := thumbnail(ctx, path); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled thumbnail: %v", err)
		}
	}
}
