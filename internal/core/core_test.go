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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
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
	return c.Resolve(filepath.Dir(out), "device")
}

func TestConfigLegacyMigrationAndCredentialIsolation(t *testing.T) {
	dir := t.TempDir()
	path, legacy := filepath.Join(dir, "chaos", "config.json"), filepath.Join(dir, "vibe-shot", "config.json")
	writeTestFile(t, legacy, []byte(`{"provider":"openai","api_key":"old-key","use_custom_prompt":true,"custom_prompt":"  Custom  ","copy_to_clipboard":true}`))
	cfg, err := LoadConfig(path, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Resolve(dir, "hash").CustomPrompt != "Custom" {
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
	if info.Mode().Perm() != 0600 {
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
	if len(Providers()) != 7 {
		t.Fatal("provider missing")
	}
	for _, p := range Providers() {
		cfg := Config{Provider: p.ID, APIKey: "key", BaseURL: "https://custom.example/v1"}
		r := cfg.Resolve("/home/example", "abc")
		if r.Provider.ID != p.ID || r.Model != p.Model {
			t.Fatalf("bad defaults %s", p.ID)
		}
		if p.ID == "chaos-hosted" && r.APIKey != HostedAppToken+":abc" {
			t.Fatal("hosted device auth changed")
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
func TestNamingSafetyAndTokens(t *testing.T) {
	at := time.Date(2026, 2, 1, 12, 4, 5, 0, time.UTC)
	if got := RenderName("", "terminal-log", at); got != "terminal-log-120405" {
		t.Fatal(got)
	}
	if got := RenderName("{date}_{slug}_{time}", "中文测试", at); got != "2026-02-01-中文测试-120405" {
		t.Fatal(got)
	}
	if got := OutputDirectory("/out", "month", at); got != "/out/2026-02" {
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
func TestClientPayloadParsingAndHostedHealth(t *testing.T) {
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
		if r.URL.Path == "/api/usage" {
			fmt.Fprint(w, `{"used":3,"limit":20,"remaining":17}`)
			return
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
	cfg.Provider = ProviderByID("chaos-hosted")
	if err = client.CheckHealth(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(paths, []string{"/api/chat/completions", "/api/usage"}) {
		t.Fatal("health consumed naming quota", paths)
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
			friendly := Friendly(err, "chaos-hosted")
			if friendly.Message == "" || friendly.Action == "" {
				t.Fatal("missing recovery")
			}
			if status == 402 && !strings.Contains(friendly.Message, "free") {
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
func TestHostedMissingIdentityValidation(t *testing.T) {
	if err := (Config{Provider: "chaos-hosted"}).Resolve("/home", "").Validate(); err == nil {
		t.Fatal("hosted empty identity accepted")
	}
}
