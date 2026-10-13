package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func sampleImage(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, b.Bytes())
}
func testConfig(base, out string) ResolvedConfig {
	c := Config{Provider: "openai-compatible", APIKey: "unit-key", BaseURL: base, WatchDir: filepath.Dir(out), OutputDir: out, Model: "test-model"}
	return c.Resolve(filepath.Dir(out))
}

func TestConfigLegacyMigrationAndCredentialIsolation(t *testing.T) {
	dir := t.TempDir()
	path, legacy := filepath.Join(dir, "chaos", "config.json"), filepath.Join(dir, "vibe-shot", "config.json")
	writeTestFile(t, legacy, []byte(`{"provider":"openai","api_key":"old-key","use_custom_prompt":true,"custom_prompt":"  Custom  ","copy_to_clipboard":true}`))
	cfg, err := LoadConfig(path, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Resolve(dir).CustomPrompt != "Custom" {
		t.Fatal("custom prompt not retained")
	}
	cfg.SetProvider("openrouter")
	if cfg.APIKey != "" {
		t.Fatal("credential crossed provider boundary")
	}
	cfg.APIKey = "router-key"
	cfg.SetProvider("openai")
	if cfg.APIKey != "old-key" {
		t.Fatal("provider key not restored")
	}
	if err = SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig(path, legacy)
	if err != nil || !reflect.DeepEqual(loaded, cfg) {
		t.Fatalf("round trip: %#v %v", loaded, err)
	}
	info, _ := os.Stat(path)
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("permissions %v", info.Mode())
	}
	writeTestFile(t, path, []byte("corrupt"))
	if _, err = LoadConfig(path, legacy); err == nil {
		t.Fatal("corruption silently accepted")
	}
	content, _ := os.ReadFile(path)
	if string(content) != "corrupt" {
		t.Fatal("corrupt original replaced")
	}
}
func TestProviderResolution(t *testing.T) {
	if len(Providers()) != 6 {
		t.Fatal("provider missing")
	}
	for _, p := range Providers() {
		cfg := Config{Provider: p.ID, APIKey: "key", BaseURL: "https://custom.example/v1"}
		r := cfg.Resolve("/home/example")
		if r.Provider.ID != p.ID || r.Model != p.Model {
			t.Fatalf("bad defaults %s", p.ID)
		}
		if p.ID == "ollama" && r.APIKey != "" {
			t.Fatal("ollama received secret")
		}
		if !p.CustomURL && r.BaseURL != p.BaseURL {
			t.Fatal("preset custom URL leaked")
		}
	}
	c := Config{Provider: "openai", Model: "override", BaseURL: "ignored"}
	c.SetProvider("openai")
	if c.Model != "override" {
		t.Fatal("same-provider settings reset")
	}
	c.SetProvider("ollama")
	if c.Model != "" || c.BaseURL != "" {
		t.Fatal("overrides not cleared")
	}
}

func TestRetiredProviderMigration(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprint("legacy=", legacy), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.json")
			source, legacyPath := path, ""
			if legacy {
				legacyPath = filepath.Join(dir, "legacy.json")
				source = legacyPath
			}
			original := []byte(`{"provider":"chaos-hosted","api_key":"retired-key","model":"agnes-2.0-flash","base_url":"https://retired.invalid/api","provider_keys":{"chaos-hosted":"retired-key","openai":"saved-key"},"watch_dir":"/screenshots","language":"zh"}`)
			writeTestFile(t, source, original)
			cfg, err := LoadConfig(path, legacyPath)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Provider != "siliconrouter" || cfg.APIKey != "" || cfg.Model != "" || cfg.BaseURL != "" {
				t.Fatalf("retired provider settings carried over: %+v", cfg)
			}
			if cfg.WatchDir != "/screenshots" || cfg.Language != "zh" || cfg.ProviderKeys["openai"] != "saved-key" {
				t.Fatal("unrelated settings lost")
			}
			if _, ok := cfg.ProviderKeys["chaos-hosted"]; ok {
				t.Fatal("retired credentials retained")
			}
			if err := cfg.Resolve(dir).Validate(); err == nil {
				t.Fatal("retired provider should require setup")
			}
			preserved, _ := os.ReadFile(source)
			if !bytes.Equal(preserved, original) {
				t.Fatal("loading rewrote source config")
			}
			if err := SaveConfig(path, cfg); err != nil {
				t.Fatal(err)
			}
			reloaded, err := LoadConfig(path, "")
			if err != nil || !reflect.DeepEqual(cfg, reloaded) {
				t.Fatal("migration did not survive save/reload", err)
			}
			cfg.SetProvider("openai")
			if cfg.APIKey != "saved-key" {
				t.Fatal("saved provider credential not restored")
			}
		})
	}
	for _, provider := range Providers() {
		if provider.ID == "chaos-hosted" || strings.Contains(provider.Model, "agnes") || strings.Contains(provider.BaseURL, "vercel") {
			t.Fatal("retired service is still selectable")
		}
	}
	// In-memory configurations must be safe even before they have been saved.
	cfg := Config{Provider: "chaos-hosted", APIKey: "retired-key", Model: "agnes-2.0-flash", ProviderKeys: map[string]string{"openai": "saved-key"}}
	resolved := cfg.Resolve(t.TempDir())
	if resolved.Model != ProviderByID("siliconrouter").Model || resolved.APIKey != "" {
		t.Fatal("resolution carried retired model or credentials to new provider")
	}
	cfg.SetProvider("openai")
	if cfg.APIKey != "saved-key" || cfg.ProviderKeys["siliconrouter"] != "" {
		t.Fatal("retired credential crossed provider boundary")
	}
}
func TestNamingSafetyAndTokens(t *testing.T) {
	at := time.Date(2026, 2, 1, 12, 4, 5, 0, time.UTC)
	if got := RenderName("", "terminal-log", at); got != "terminal-log-120405" {
		t.Fatal(got)
	}
	if got := RenderName("{date}_{slug}_{time}", "中文测试", at); got != "2026-02-01-中文测试-120405" {
		t.Fatal(got)
	}
	if got := OutputDirectory("/out", "month", at); got != filepath.Join("/out", "2026-02") {
		t.Fatal(got)
	}
	for _, input := range []string{"../escape", " ", strings.Repeat("文", 300), "a/b\\c\x00d"} {
		got := Sanitize(input)
		if got == "" || len(got) > 220 || strings.ContainsAny(got, "/\\.\x00") {
			t.Fatalf("unsafe slug %q", got)
		}
	}
	if Sanitize("___") != "untitled-shot" {
		t.Fatal("fallback changed")
	}
	top := TopNouns([]string{"terminal-log.jpg", "terminal-the-123.jpeg", "browser-doc.webp"}, 6)
	if top[0].Label != "terminal" || top[0].Count != 2 {
		t.Fatal(top)
	}
	for _, item := range top {
		if item.Label == "jpg" || item.Label == "jpeg" || item.Label == "webp" {
			t.Fatal("extension counted as noun")
		}
	}
}
func TestNoClobberConcurrentMovesAndRevert(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	var wg sync.WaitGroup
	results := make(chan string, 12)
	failures := make(chan error, 12)
	for i := 0; i < 12; i++ {
		source := filepath.Join(dir, fmt.Sprintf("source-%d.JPG", i))
		writeTestFile(t, source, []byte(fmt.Sprintf("image-%d", i)))
		wg.Add(1)
		go func() {
			defer wg.Done()
			dst, err := MoveFile(source, out, "same")
			if err != nil {
				failures <- err
			} else {
				results <- dst
			}
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	contents := map[string]bool{}
	for dst := range results {
		if seen[dst] {
			t.Fatal("destination overwritten")
		}
		seen[dst] = true
		b, err := os.ReadFile(dst)
		if err != nil {
			t.Fatal(err)
		}
		contents[string(b)] = true
	}
	if len(seen) != 12 || len(contents) != 12 {
		t.Fatalf("lost files: %v", seen)
	}
	source := filepath.Join(dir, "original.png")
	writeTestFile(t, source, []byte("source"))
	dst, err := MoveFile(source, out, "revert")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, source, []byte("occupant"))
	restored, err := RevertFile(dst, source)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(restored) != "original-restored.png" {
		t.Fatal(restored)
	}
	b, _ := os.ReadFile(source)
	if string(b) != "occupant" {
		t.Fatal("revert overwrote occupant")
	}
	same, err := RenameFile(restored, "original-restored")
	if err != nil || same != restored {
		t.Fatal("same filename should be no-op")
	}
}
func TestMoveHookPrecedesPublicationAndCanReject(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "a.png")
	writeTestFile(t, source, []byte("original"))
	wantErr := errors.New("journal unavailable")
	_, err := MoveFileWithHook(source, filepath.Join(dir, "out"), "new", func(dst string) error {
		if _, err := os.Lstat(dst); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("destination published before journal")
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(source)
	if string(b) != "original" {
		t.Fatal("source altered after rejected journal")
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "out"))
	if len(entries) != 0 {
		t.Fatal("temporary files leaked")
	}
}
func TestImagePreparationAndOrientation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.png")
	sampleImage(t, path, 2400, 1200)
	prepared, err := PrepareImage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(prepared.Base64)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width != 1280 || cfg.Height != 640 || prepared.MIMEType != "image/jpeg" {
		t.Fatalf("bad resized image %+v %v", cfg, err)
	}
	decoded, _ := jpeg.Decode(bytes.NewReader(raw))
	r, g, b, _ := decoded.At(100, 100).RGBA()
	if r < 60000 || g < 60000 || b < 60000 {
		t.Fatal("transparent image lost white backing")
	}
	writeTestFile(t, path, []byte("not an image"))
	if _, err = PrepareImage(context.Background(), path); err == nil {
		t.Fatal("invalid image reached API")
	}
	if _, err = PrepareImage(context.Background(), path+"missing.png"); err == nil {
		t.Fatal("missing file became empty data")
	}
	image := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	image.Set(0, 0, color.NRGBA{R: 255, A: 255})
	oriented := orientImage(image, 6)
	if oriented.Bounds().Dx() != 1 || oriented.Bounds().Dy() != 2 {
		t.Fatal("EXIF rotation wrong")
	}
	r, _, _, _ = oriented.At(0, 0).RGBA()
	if r == 0 {
		t.Fatal("EXIF pixel mapping wrong")
	}
}
func TestClientPayloadParsingAndHealth(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		paths = append(paths, r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer unit-key" {
			t.Error("missing credentials")
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"  中文名字  "}}]}`)
	}))
	defer server.Close()
	cfg := testConfig(server.URL+"/api", filepath.Join(t.TempDir(), "out"))
	cfg.CustomPrompt = "Use my naming rules"
	cfg.Language = "zh"
	client := &Client{HTTP: server.Client()}
	slug, err := client.GenerateSlug(context.Background(), PreparedImage{"aW1hZ2U=", "image/jpeg"}, cfg)
	if err != nil || slug != "中文名字" {
		t.Fatal(slug, err)
	}
	messages := body["messages"].([]any)
	if messages[0].(map[string]any)["content"] != cfg.CustomPrompt {
		t.Fatal("custom prompt lost")
	}
	parts := messages[1].(map[string]any)["content"].([]any)
	if parts[0].(map[string]any)["text"] != "Return only the filename slug." {
		t.Fatal("custom prompt language overridden")
	}
	if body["max_tokens"] != float64(32) || body["stream"] != false {
		t.Fatal("request limits changed")
	}
	if err = client.CheckHealth(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(paths, []string{"/api/chat/completions", "/api/chat/completions"}) {
		t.Fatal("unexpected provider endpoints", paths)
	}
	for _, input := range []string{`{"choices":[{"message":{"content":[{"type":"text","text":"array-name"}]}}]}`, `{"candidates":[{"content":{"parts":[{"text":"gemini-name"}]}}]}`} {
		if value, err := ParseSlug([]byte(input)); err != nil || value == "" {
			t.Fatal(value, err)
		}
	}
	for _, input := range []string{`{}`, `{"error":{"message":"failure"}}`, `{"choices":[{"message":{"content":" "}}]}`, `broken`} {
		if _, err := ParseSlug([]byte(input)); err == nil {
			t.Fatal("bad response accepted", input)
		}
	}
}
func TestFriendlyDistinguishesFilesystemAndNetworkErrors(t *testing.T) {
	secretURL := "https://account:private-password@provider.invalid/v1?api_key=private-key"
	for _, tc := range []struct {
		name, provider, action, detail string
		err                            error
	}{
		{"file permission", "openai", "retry", "permission denied", &os.PathError{Op: "open", Path: "/images/source.png", Err: os.ErrPermission}},
		{"missing file with Ollama", "ollama", "retry", "file does not exist", &os.PathError{Op: "stat", Path: "/images/missing.png", Err: os.ErrNotExist}},
		{"file publication", "openai", "retry", "permission denied", &os.LinkError{Op: "link", Old: "/images/source.png", New: "/images/out.png", Err: syscall.EACCES}},
		{"network connection", "openai", "internet", "internet", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}},
		{"local model connection", "ollama", "settings", "Ollama", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}},
		{"network timeout", "openai", "retry", "too long", &net.DNSError{Err: "timeout", IsTimeout: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Request wrappers must not leak credentials, including when they
			// wrap a local filesystem error (for example reading a certificate).
			err := &url.Error{Op: "Post", URL: secretURL, Err: tc.err}
			friendly := Friendly(err, tc.provider)
			if friendly.Action != tc.action || !strings.Contains(friendly.Message, tc.detail) {
				t.Fatalf("wrong recovery: %+v", friendly)
			}
			if strings.Contains(friendly.Message, "private-") || strings.Contains(friendly.Message, "provider.invalid") {
				t.Fatal("request credentials leaked into error message")
			}
			var pathError *os.PathError
			if errors.As(tc.err, &pathError) && (!strings.Contains(friendly.Message, pathError.Path) || !strings.Contains(friendly.Message, pathError.Op)) {
				t.Fatal("filesystem operation/path detail was lost")
			}
		})
	}
}

func TestProviderErrorsAndCancellation(t *testing.T) {
	for _, status := range []int{401, 402, 404, 429, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"error":{"message":"upstream issue"}}`)
			}))
			defer server.Close()
			cfg := testConfig(server.URL, filepath.Join(t.TempDir(), "out"))
			_, err := (&Client{}).GenerateSlug(context.Background(), PreparedImage{"eA==", "image/png"}, cfg)
			var httpErr *HTTPError
			if !errors.As(err, &httpErr) || httpErr.Status != status {
				t.Fatal(err)
			}
			friendly := Friendly(err, "openai")
			if friendly.Message == "" || friendly.Action == "" {
				t.Fatal("missing recovery")
			}
			if status == 402 && !strings.Contains(friendly.Message, "credit") {
				t.Fatal(friendly)
			}
		})
	}
	for _, base := range []string{"", "ftp://example.test", "https:///missing-host", "https://user:password@example.test", "https://example.test?token=x"} {
		if _, err := endpoint(base, "chat/completions"); err == nil {
			t.Fatal("invalid endpoint", base)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "a.png")
	writeTestFile(t, path, []byte("x"))
	if err := WaitUntilStable(ctx, path, 12, time.Millisecond); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestProcessorEndToEndPreservesOriginalBytes(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "manual.png")
	sampleImage(t, source, 32, 32)
	before, _ := os.ReadFile(source)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"choices":[{"message":{"content":"terminal-git-log"}}]}`)
	}))
	defer server.Close()
	cfg := testConfig(server.URL, filepath.Join(dir, "out"))
	processor := Processor{Client: &Client{HTTP: server.Client()}}
	var stages []string
	record, err := processor.Process(context.Background(), source, cfg, func(stage string) { stages = append(stages, stage) })
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(record.Path)
	if !bytes.Equal(before, after) {
		t.Fatal("image bytes changed on disk")
	}
	if record.SourcePath != source || record.ID == "" || record.NewName == "" || record.Duration <= 0 {
		t.Fatal(record)
	}
	if !reflect.DeepEqual(stages, []string{"analyzing", "renaming"}) {
		t.Fatal(stages)
	}
	if _, err = os.Stat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("source not moved")
	}
}

func TestMoveNeverDeletesReplacedSource(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	writeTestFile(t, source, []byte("original"))
	destination, err := MoveFileWithHook(source, filepath.Join(dir, "out"), "file", func(string) error {
		if err := os.Remove(source); err != nil {
			return err
		}
		return os.WriteFile(source, []byte("replacement"), 0600)
	})
	var moveErr *MoveError
	if !errors.As(err, &moveErr) || destination == "" {
		t.Fatalf("expected recoverable partial move: %s %v", destination, err)
	}
	sourceBytes, _ := os.ReadFile(source)
	destinationBytes, _ := os.ReadFile(destination)
	if string(sourceBytes) != "replacement" || string(destinationBytes) != "original" {
		t.Fatal("source replacement was lost")
	}
}
func TestNoHardlinkFallbackDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	temp, target := filepath.Join(dir, "temp"), filepath.Join(dir, "final")
	writeTestFile(t, temp, []byte("source"))
	if err := publishWithoutLinks(temp, target); err != nil {
		t.Fatal(err)
	}
	if err := publishWithoutLinks(temp, target); !errors.Is(err, os.ErrExist) {
		t.Fatal("fallback overwrote existing file", err)
	}
	contents, _ := os.ReadFile(target)
	if string(contents) != "source" {
		t.Fatal(string(contents))
	}
}

func TestProcessorPreservesSourceReplacedDuringAnalysis(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	sampleImage(t, source, 32, 32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := os.Remove(source); err != nil {
			t.Error(err)
		}
		if err := os.WriteFile(source, []byte("replacement"), 0600); err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"old-image-name"}}]}`)
	}))
	defer server.Close()
	processor := Processor{Client: &Client{HTTP: server.Client()}}
	_, err := processor.Process(context.Background(), source, testConfig(server.URL, filepath.Join(dir, "out")), nil)
	if err == nil {
		t.Fatal("replacement was filed under old analysis")
	}
	b, _ := os.ReadFile(source)
	if string(b) != "replacement" {
		t.Fatal("replacement was moved or deleted")
	}
}
func TestProcessorRejectsChangesDuringImagePreparation(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	sampleImage(t, source, 32, 32)
	called := false
	processor := Processor{PrepareImage: func(context.Context, string) (PreparedImage, error) {
		called = true
		data, _ := os.ReadFile(source)
		info, _ := os.Stat(source)
		data[len(data)-1] ^= 0xff
		if err := os.WriteFile(source, data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(source, info.ModTime(), info.ModTime()); err != nil {
			t.Fatal(err)
		}
		return PreparedImage{"eA==", "image/png"}, nil
	}}
	_, err := processor.Prepare(context.Background(), source, testConfig("http://127.0.0.1:1", filepath.Join(dir, "out")))
	if !called || err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("in-place change not caught before API: %v", err)
	}
}
func TestDecodeImageAppliesRealJPEGEXIFOrientation(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 8))
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, img, nil); err != nil {
		t.Fatal(err)
	}
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	original := encoded.Bytes()
	data := append([]byte{}, original[:2]...)
	data = append(data, 0xff, 0xe1, 0, byte(len(exif)+2))
	data = append(data, exif...)
	data = append(data, original[2:]...)
	path := filepath.Join(t.TempDir(), "portrait.jpg")
	writeTestFile(t, path, data)
	decoded, err := DecodeImage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 8 || decoded.Bounds().Dy() != 16 {
		t.Fatalf("EXIF orientation ignored: %v", decoded.Bounds())
	}
}
