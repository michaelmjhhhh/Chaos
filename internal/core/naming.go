package core

import (
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b[:])
	return strings.ToUpper(s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:])
}

// Sanitize retains the original English/CJK alphabet and caps UTF-8 bytes to
// leave room for collision suffixes and extensions on common filesystems.
func Sanitize(raw string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		allowed := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r >= 0x4e00 && r <= 0x9fff || r >= 0x3400 && r <= 0x4dbf || r >= 0x20000 && r <= 0x2a6df
		if allowed {
			if b.Len()+len(string(r)) > 220 {
				break
			}
			b.WriteRune(r)
			hyphen = false
		} else if !hyphen && b.Len() > 0 {
			if b.Len()+1 > 220 {
				break
			}
			b.WriteByte('-')
			hyphen = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "untitled-shot"
	}
	return s
}
func RenderName(template, slug string, at time.Time) string {
	if strings.TrimSpace(template) == "" {
		template = DefaultTemplate
	}
	return Sanitize(strings.NewReplacer("{slug}", slug, "{date}", at.Format("2006-01-02"), "{time}", at.Format("150405")).Replace(template))
}
func OutputDirectory(base, rule string, at time.Time) string {
	switch rule {
	case "day":
		return filepath.Join(base, at.Format("2006-01-02"))
	case "month":
		return filepath.Join(base, at.Format("2006-01"))
	default:
		return base
	}
}
func AcceptsImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".heic", ".webp":
		return true
	}
	return false
}
func DefaultPrompt(language string) string {
	if language == "zh" {
		return "You are a file-naming assistant. Analyze the image and provide a concise, slug-style filename in Simplified Chinese (Chinese characters, numbers, hyphens only). No explanation, no prefix."
	}
	return "You are a file-naming assistant. Analyze the image and provide a concise, slug-style English filename (lowercase, numbers, hyphens only). No explanation, no prefix."
}

type RecentFile struct {
	ID           string    `json:"id"`
	OriginalName string    `json:"originalName"`
	NewName      string    `json:"newName"`
	Path         string    `json:"path"`
	SourcePath   string    `json:"sourcePath"`
	Timestamp    time.Time `json:"timestamp"`
	Duration     float64   `json:"duration"`
	IsError      bool      `json:"isError"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
}

func (f RecentFile) SearchKey() string {
	s := f.NewName + " " + f.OriginalName
	if f.IsError {
		s += " " + f.ErrorMessage
	}
	return strings.ToLower(s)
}

type Labeled struct {
	Label string
	Count int
}
type DurationRecord struct {
	Name     string
	Duration float64
}

var stopWords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields("the and for with from this that into onto your you are but not all any was were has have had out its his her their them they she him who how why what when where which ourselves png jpg jpeg heic webp") {
		m[w] = true
	}
	return m
}()

func TopNouns(names []string, limit int) []Labeled {
	counts := map[string]int{}
	for _, name := range names {
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		for _, t := range strings.FieldsFunc(strings.ToLower(stem), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }) {
			if len([]rune(t)) < 3 || stopWords[t] {
				continue
			}
			if strings.IndexFunc(t, unicode.IsLetter) >= 0 {
				counts[t]++
			}
		}
	}
	return ranked(counts, limit)
}
func ranked(m map[string]int, limit int) []Labeled {
	v := make([]Labeled, 0, len(m))
	for k, n := range m {
		v = append(v, Labeled{k, n})
	}
	sort.Slice(v, func(i, j int) bool {
		if v[i].Count == v[j].Count {
			return v[i].Label < v[j].Label
		}
		return v[i].Count > v[j].Count
	})
	if limit < 0 {
		limit = 0
	}
	if len(v) > limit {
		v = v[:limit]
	}
	return v
}
