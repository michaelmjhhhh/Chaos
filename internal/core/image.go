package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

type PreparedImage struct{ Base64, MIMEType string }
type ImagePreparer func(context.Context, string) (PreparedImage, error)

const MaxImageBytes int64 = 100 << 20
const maxImagePixels int64 = 40_000_000

// PrepareImage validates, orients, downsizes and JPEG-encodes an image. Native
// macOS HEIC support is delegated to its installed ImageIO-backed sips utility.
// No shell is used and no provider request occurs if decoding fails.
func PrepareImage(ctx context.Context, path string) (PreparedImage, error) {
	if !AcceptsImage(path) {
		return PreparedImage{}, errors.New("unsupported image type")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return PreparedImage{}, err
	}
	if !info.Mode().IsRegular() {
		return PreparedImage{}, errors.New("source must be a regular image file")
	}
	if info.Size() <= 0 || info.Size() > MaxImageBytes {
		return PreparedImage{}, errors.New("image must be between 1 byte and 100 MiB")
	}
	original := path
	if strings.EqualFold(filepath.Ext(path), ".heic") {
		if runtime.GOOS != "darwin" {
			return PreparedImage{}, errors.New("HEIC conversion requires macOS")
		}
		dir, err := os.MkdirTemp("", "chaos-image-*")
		if err != nil {
			return PreparedImage{}, err
		}
		defer os.RemoveAll(dir)
		path = filepath.Join(dir, "prepared.jpg")
		command := exec.CommandContext(ctx, "/usr/bin/sips", "--resampleHeightWidthMax", "1280", "--setProperty", "format", "jpeg", original, "--out", path)
		if err = command.Run(); err != nil {
			return PreparedImage{}, fmt.Errorf("decode HEIC image: %w", err)
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return PreparedImage{}, err
	}
	data, err := boundedRead(f, MaxImageBytes)
	f.Close()
	if err != nil {
		return PreparedImage{}, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return PreparedImage{}, fmt.Errorf("decode image: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
		return PreparedImage{}, errors.New("image dimensions exceed the 40 megapixel limit")
	}
	if err = ctx.Err(); err != nil {
		return PreparedImage{}, err
	}
	source, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return PreparedImage{}, fmt.Errorf("decode image: %w", err)
	}
	if format == "jpeg" {
		source = orientImage(source, jpegOrientation(data))
	}
	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width > 1280 || height > 1280 {
		if width >= height {
			height = max(1, height*1280/width)
			width = 1280
		} else {
			width = max(1, width*1280/height)
			height = 1280
		}
	}
	// A white backing keeps transparent screenshots readable after JPEG encoding.
	target := image.NewRGBA(image.Rect(0, 0, width, height))
	stddraw.Draw(target, target.Bounds(), image.NewUniform(color.White), image.Point{}, stddraw.Src)
	draw.CatmullRom.Scale(target, target.Bounds(), source, bounds, stddraw.Over, nil)
	var out bytes.Buffer
	if err = jpeg.Encode(&out, target, &jpeg.Options{Quality: 70}); err != nil {
		return PreparedImage{}, err
	}
	if err = ctx.Err(); err != nil {
		return PreparedImage{}, err
	}
	return PreparedImage{base64.StdEncoding.EncodeToString(out.Bytes()), "image/jpeg"}, nil
}

// jpegOrientation reads the TIFF orientation tag without trusting offsets.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xff || data[1] != 0xd8 {
		return 1
	}
	for at := 2; at+4 <= len(data); {
		if data[at] != 0xff {
			return 1
		}
		marker := data[at+1]
		if marker == 0xda || marker == 0xd9 {
			return 1
		}
		if marker == 0xff {
			at++
			continue
		}
		size := int(binary.BigEndian.Uint16(data[at+2 : at+4]))
		if size < 2 || at+2+size > len(data) {
			return 1
		}
		segment := data[at+4 : at+2+size]
		at += 2 + size
		if marker != 0xe1 || len(segment) < 14 || string(segment[:6]) != "Exif\x00\x00" {
			continue
		}
		tiff := segment[6:]
		var order binary.ByteOrder
		switch string(tiff[:2]) {
		case "II":
			order = binary.LittleEndian
		case "MM":
			order = binary.BigEndian
		default:
			return 1
		}
		offset := uint64(order.Uint32(tiff[4:8]))
		if offset+2 > uint64(len(tiff)) {
			return 1
		}
		count := int(order.Uint16(tiff[offset : offset+2]))
		for i := 0; i < count; i++ {
			p := offset + 2 + uint64(i)*12
			if p+12 > uint64(len(tiff)) {
				return 1
			}
			entry := tiff[p : p+12]
			if order.Uint16(entry[:2]) == 0x112 && order.Uint16(entry[2:4]) == 3 && order.Uint32(entry[4:8]) == 1 {
				v := int(order.Uint16(entry[8:10]))
				if v >= 1 && v <= 8 {
					return v
				}
			}
		}
		return 1
	}
	return 1
}
func orientImage(source image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return source
	}
	b := source.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := x, y
			switch orientation {
			case 2:
				dx = w - 1 - x
			case 3:
				dx = w - 1 - x
				dy = h - 1 - y
			case 4:
				dy = h - 1 - y
			case 5:
				dx = y
				dy = x
			case 6:
				dx = h - 1 - y
				dy = x
			case 7:
				dx = h - 1 - y
				dy = w - 1 - x
			case 8:
				dx = y
				dy = w - 1 - x
			}
			dst.Set(dx, dy, source.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
