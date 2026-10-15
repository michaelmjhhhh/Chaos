package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os/exec"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/michaelmjhhhh/Chaos/internal/core"
	xdraw "golang.org/x/image/draw"
)

func readImage(path string) (image.Image, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return core.DecodeImage(ctx, path)
}

func copyImage(path string) error {
	img, err := readImage(path)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err = png.Encode(&out, img); err != nil {
		return err
	}
	return mygo.Clipboard.WriteImage(out.Bytes())
}

func thumbnail(ctx context.Context, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	img, err := core.DecodeImage(ctx, path)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	scale := min(1.0, 640/float64(max(b.Dx(), b.Dy())))
	out := image.NewNRGBA(image.Rect(0, 0, max(1, int(float64(b.Dx())*scale)), max(1, int(float64(b.Dy())*scale))))
	xdraw.ApproxBiLinear.Scale(out, out.Bounds(), img, b, draw.Src, nil)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var data bytes.Buffer
	if err = png.Encode(&data, out); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

func notify(title, body string) error {
	return mygo.NewNotification(mygo.NotificationOptions{Title: title, Body: body, Silent: true, Group: "chaos-filings"}).Show()
}

func playSound() error {
	if runtime.GOOS != "darwin" {
		mygo.Shell.Beep()
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "/usr/bin/afplay", "/System/Library/Sounds/Glass.aiff").Run()
}

func trayIcon(watching bool) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	ink := color.NRGBA{A: 255}
	if runtime.GOOS != "darwin" {
		ink = color.NRGBA{R: 225, G: 104, B: 87, A: 255}
	}
	for _, r := range []image.Rectangle{image.Rect(6, 5, 26, 8), image.Rect(4, 8, 7, 24), image.Rect(6, 24, 26, 27), image.Rect(23, 8, 26, 11), image.Rect(23, 21, 26, 24)} {
		draw.Draw(img, r, image.NewUniform(ink), image.Point{}, draw.Src)
	}
	if watching {
		draw.Draw(img, image.Rect(12, 13, 21, 21), image.NewUniform(ink), image.Point{}, draw.Src)
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}
