package render

import (
	"strings"
	"testing"
)

const goSrc = "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n"

// stripANSI убирает управляющие последовательности для проверки текста.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != 0x1b {
			b.WriteByte(s[i])
			continue
		}
		// ESC [ параметры ; ... ; буква-завершение
		i++
		if i < len(s) && s[i] == '[' {
			i++
			for i < len(s) && !(s[i] >= 0x40 && s[i] <= 0x7e) {
				i++
			}
		}
	}
	return b.String()
}

func TestRenderCodeHighlights(t *testing.T) {
	r := New("dark", true)
	lines, err := r.RenderCode(goSrc, 80, "main.go")
	if err != nil {
		t.Fatalf("RenderCode: %v", err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "\x1b[") {
		t.Errorf("нет ANSI-подсветки:\n%q", joined)
	}
	if !strings.Contains(stripANSI(joined), "package main") {
		t.Errorf("текст исходника потерялся:\n%q", joined)
	}
	if !strings.Contains(stripANSI(joined), `fmt.Println("hi")`) {
		t.Errorf("тело функции потерялось:\n%q", joined)
	}
}

func TestRenderCodeUnknownLexerFallsBackToRaw(t *testing.T) {
	r := New("dark", true)
	src := "просто текст с *звёздочками*\n"
	lines, err := r.RenderCode(src, 80, "notes.unknownext")
	if err != nil {
		t.Fatalf("RenderCode: %v", err)
	}
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, "\x1b[") {
		t.Errorf("без лексера не должно быть ANSI: %q", joined)
	}
	if joined != strings.TrimSuffix(src, "\n") {
		t.Errorf("текст должен остаться дословно: %q", joined)
	}
}

func TestRenderCodeMonochromeThemeDisablesHighlight(t *testing.T) {
	for _, theme := range []string{"notty", "ascii"} {
		r := New(theme, true)
		lines, err := r.RenderCode(goSrc, 80, "main.go")
		if err != nil {
			t.Fatalf("RenderCode(%s): %v", theme, err)
		}
		joined := strings.Join(lines, "\n")
		if strings.Contains(joined, "\x1b[") {
			t.Errorf("%s: монокромная тема не должна подсвечивать: %q", theme, joined)
		}
		if !strings.Contains(joined, "package main") {
			t.Errorf("%s: текст потерялся: %q", theme, joined)
		}
	}
}

func TestRenderCodeCacheSeparatesFilenames(t *testing.T) {
	r := New("dark", true)
	a, err := r.RenderCode("x := 1\n", 60, "a.go")
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.RenderCode("x := 1\n", 60, "a.py")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(a, "\n") == strings.Join(b, "\n") {
		t.Error("кэш отдал результат чужого лексера (a.go vs a.py)")
	}
	// Повторный вызов того же файла — из кэша, результат тот же.
	a2, err := r.RenderCode("x := 1\n", 60, "a.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(a, "\n") != strings.Join(a2, "\n") {
		t.Error("кэш code-режима нестабилен")
	}
}

func TestCodeStyleMapping(t *testing.T) {
	cases := []struct {
		theme string
		want  string
		ok    bool
	}{
		{"dark", "monokai", true},
		{"light", "github", true},
		{"dracula", "dracula", true},
		{"tokyo-night", "tokyonight-night", true},
		{"notty", "", false},
		{"ascii", "", false},
	}
	for _, c := range cases {
		r := New(c.theme, true)
		got, ok := r.codeStyle()
		if got != c.want || ok != c.ok {
			t.Errorf("codeStyle(%q) = (%q, %v), want (%q, %v)", c.theme, got, ok, c.want, c.ok)
		}
	}
	// auto следует детектированному фону.
	if got, ok := New("auto", true).codeStyle(); !ok || got != "monokai" {
		t.Errorf("auto+dark = (%q, %v), want monokai", got, ok)
	}
	if got, ok := New("auto", false).codeStyle(); !ok || got != "github" {
		t.Errorf("auto+light = (%q, %v), want github", got, ok)
	}
}
