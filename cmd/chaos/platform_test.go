package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func thumbnailFixture(t *testing.T, width, height int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.SetNRGBA(0, 0, color.NRGBA{R: 80, G: 40, B: 20, A: 128})
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(path, data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestThumbnailFitsPreviewWithoutUpscaling(t *testing.T) {
	for _, tc := range []struct {
		name                string
		width, height, w, h int
	}{
		{"landscape", 1280, 640, 640, 320},
		{"portrait", 320, 1280, 160, 640},
		{"small", 20, 10, 20, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := thumbnail(context.Background(), thumbnailFixture(t, tc.width, tc.height))
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("preview is not PNG: %v", err)
			}
			if got := img.Bounds(); got.Dx() != tc.w || got.Dy() != tc.h {
				t.Fatalf("preview bounds %v; want %dx%d", got, tc.w, tc.h)
			}
			if tc.name == "small" {
				if got := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA); got.A != 128 {
					t.Fatalf("PNG preview lost transparency: %v", got)
				}
			}
		})
	}
}

func TestThumbnailHonorsCancellation(t *testing.T) {
	path := thumbnailFixture(t, 20, 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	data, err := thumbnail(ctx, path)
	if !errors.Is(err, context.Canceled) || len(data) != 0 {
		t.Fatalf("cancelled thumbnail returned %d bytes, error %v", len(data), err)
	}
}
