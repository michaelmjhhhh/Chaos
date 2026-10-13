package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// Synthetic fixtures created for this repository, with no third-party image
// content. Encode a 64x32 NRGBA PNG: x<32 is opaque red, x>=32 is transparent.
// Then cwebp 1.6.0: `cwebp -lossless synthetic.png -o lossless.webp` and
// `cwebp -q 90 synthetic.png -o lossy.webp`. Keeping these tiny encoded fixtures
// inline avoids requiring a WebP encoder on development machines or CI.
const syntheticLosslessWebP = "UklGRiYAAABXRUJQVlA4TBkAAAAvP8AHEA8Q8x/zHwyxYDJ/6RB6I/ofVfANAA=="
const syntheticLossyWebP = "UklGRoYAAABXRUJQVlA4WAoAAAAQAAAAPwAAHwAAQUxQSBQAAAABDzD/ERFCLJjMXzqE3oj+RxV8A1ZQOCBMAAAA8AMAnQEqQAAgAD4xFolDIiEhFgQAIAMEsYBmu0A/AD8AADKMNAliwAD+96KX/9eiUqpf3sP/+Fmfwsz+Fmf/Cc2fv90UD4A2eDkAAA=="

func TestWebPDecodeAndVisionPreparation(t *testing.T) {
	for _, tc := range []struct{ name, encoded string }{
		{"lossless", syntheticLosslessWebP},
		{"lossy with alpha", syntheticLossyWebP},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := base64.StdEncoding.DecodeString(tc.encoded)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "synthetic.WEBP")
			writeTestFile(t, path, data)
			decoded, err := DecodeImage(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Bounds().Size() != image.Pt(64, 32) {
				t.Fatalf("wrong WebP dimensions: %v", decoded.Bounds())
			}
			r, g, b, a := decoded.At(16, 16).RGBA()
			if r < 58000 || g > 6000 || b > 6000 || a != 65535 {
				t.Fatalf("opaque red WebP pixel changed: %d %d %d %d", r, g, b, a)
			}
			if tc.name == "lossless" && (r != 65535 || g != 0 || b != 0) {
				t.Fatal("lossless WebP changed pixel values")
			}
			_, _, _, a = decoded.At(48, 16).RGBA()
			if a != 0 {
				t.Fatal("WebP alpha was lost")
			}
			prepared, err := PrepareImage(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			vision := decodeVisionJPEG(t, prepared, image.Pt(64, 32))
			r, g, b, _ = vision.At(48, 16).RGBA()
			if r < 60000 || g < 60000 || b < 60000 {
				t.Fatal("transparent WebP was not composited on white for vision")
			}
			r, g, b, _ = vision.At(16, 16).RGBA()
			if r < 58000 || g > 6000 || b > 6000 {
				t.Fatal("WebP content was lost during JPEG preparation")
			}
			assertImageBytesUnchanged(t, path, data)
		})
	}
}

func TestCorruptWebPDoesNotReachProviderOrMoveSource(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(syntheticLosslessWebP)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "truncated.webp")
	writeTestFile(t, path, data[:20])
	assertRejectedImagePreservesSource(t, path)
}

func decodeVisionJPEG(t *testing.T, prepared PreparedImage, size image.Point) image.Image {
	t.Helper()
	if prepared.MIMEType != "image/jpeg" {
		t.Fatalf("wrong vision content type: %q", prepared.MIMEType)
	}
	data, err := base64.StdEncoding.DecodeString(prepared.Base64)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("vision payload is not a valid JPEG: %v", err)
	}
	if decoded.Bounds().Size() != size {
		t.Fatalf("wrong vision dimensions: %v, want %v", decoded.Bounds(), size)
	}
	return decoded
}

func assertImageBytesUnchanged(t *testing.T, path string, before []byte) {
	t.Helper()
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("source bytes changed or disappeared: %v", err)
	}
}

type rejectImageHTTP struct{ calls atomic.Int32 }

func (c *rejectImageHTTP) Do(*http.Request) (*http.Response, error) {
	c.calls.Add(1)
	return nil, errors.New("invalid image must not reach provider")
}

func assertRejectedImagePreservesSource(t *testing.T, path string) {
	t.Helper()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeImage(context.Background(), path); err == nil {
		t.Fatal("invalid native image decode succeeded")
	}
	if _, err := PrepareImage(context.Background(), path); err == nil {
		t.Fatal("invalid image produced a vision payload")
	}
	transport := new(rejectImageHTTP)
	processor := Processor{Client: &Client{HTTP: transport}}
	out := filepath.Join(filepath.Dir(path), "out")
	if _, err := processor.Process(context.Background(), path, testConfig("http://unit.invalid/v1", out), nil); err == nil {
		t.Fatal("invalid image was processed")
	}
	if transport.calls.Load() != 0 {
		t.Fatal("invalid image was sent to a provider")
	}
	assertImageBytesUnchanged(t, path, before)
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid image created an output directory: %v", err)
	}
}
