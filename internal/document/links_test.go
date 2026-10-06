package document

import (
	"strings"
	"testing"
)

func TestExtractLinksInline(t *testing.T) {
	src := "Вот [сайт](https://example.com) и [другой файл](notes.md).\n"
	links := ExtractLinks(src)
	if len(links) != 2 {
		t.Fatalf("найдено %d ссылок, want 2: %+v", len(links), links)
	}
	if links[0].Text != "сайт" || links[0].URL != "https://example.com" {
		t.Errorf("links[0] = %+v", links[0])
	}
	if links[1].Text != "другой файл" || links[1].URL != "notes.md" {
		t.Errorf("links[1] = %+v", links[1])
	}
}

func TestExtractLinksSkipsImages(t *testing.T) {
	src := "![картинка](img.png) и [ссылка](https://x.io)\n"
	links := ExtractLinks(src)
	if len(links) != 1 || links[0].URL != "https://x.io" {
		t.Errorf("картинка не должна считаться ссылкой: %+v", links)
	}
}

func TestExtractLinksBareURL(t *testing.T) {
	src := "Смотри https://example.com/path и <https://bold.io>,\n"
	links := ExtractLinks(src)
	if len(links) != 2 {
		t.Fatalf("golые/autolink: %+v", links)
	}
	if links[0].URL != "https://example.com/path" {
		t.Errorf("links[0] = %+v", links[0])
	}
	if links[1].URL != "https://bold.io" {
		t.Errorf("links[1] = %+v (autolink должен дать чистый URL)", links[1])
	}
}

func TestExtractLinksNoDuplicatesInsideInline(t *testing.T) {
	// URL внутри inline-ссылки не должен находиться второй раз «голым».
	src := "[ссылка](https://example.com/a)\n"
	links := ExtractLinks(src)
	if len(links) != 1 {
		t.Errorf("дубликат: %+v", links)
	}
}

func TestExtractLinksOrderAndEscapes(t *testing.T) {
	src := "1 [a](https://a.io) 2 \\[не ссылка] 3 [b](https://b.io)\n"
	links := ExtractLinks(src)
	if len(links) != 2 {
		t.Fatalf("esc: %+v", links)
	}
	if links[0].URL != "https://a.io" || links[1].URL != "https://b.io" {
		t.Errorf("порядок/esc: %+v", links)
	}
}

func TestExtractLinksNestedText(t *testing.T) {
	src := "см. [раздел [вложенный]](https://deep.io) тут\n"
	links := ExtractLinks(src)
	if len(links) != 1 || links[0].URL != "https://deep.io" {
		t.Errorf("вложенные скобки в тексте: %+v", links)
	}
	if !strings.Contains(links[0].Text, "вложенный") {
		t.Errorf("текст ссылки = %q", links[0].Text)
	}
}

func TestExtractLinksEmpty(t *testing.T) {
	if links := ExtractLinks("просто текст без ссылок\n"); len(links) != 0 {
		t.Errorf("ожидали 0 ссылок: %+v", links)
	}
	// Пустой URL — не ссылка.
	if links := ExtractLinks("[текст]()\n"); len(links) != 0 {
		t.Errorf("пустой URL: %+v", links)
	}
}

func TestHasScheme(t *testing.T) {
	cases := map[string]bool{
		"https://x.io": true,
		"mailto:a@b.c": true,
		"notes.md":     false,
		"#anchor":      false,
		"./dir":        false,
	}
	for url, want := range cases {
		if got := HasScheme(url); got != want {
			t.Errorf("HasScheme(%q) = %v, want %v", url, got, want)
		}
	}
}

func TestFileURLPath(t *testing.T) {
	good := map[string]string{
		"file:///tmp/notes.md":      "/tmp/notes.md",
		"FILE:///tmp/a.md":          "/tmp/a.md", // схема регистронезависима
		"file://localhost/tmp/b.md": "/tmp/b.md",
		"file:///tmp/my%20notes.md": "/tmp/my notes.md",
		"file:///tmp/doc.md#раздел": "/tmp/doc.md", // fragment отбрасывается
	}
	for raw, want := range good {
		got, ok := FileURLPath(raw)
		if !ok || got != want {
			t.Errorf("FileURLPath(%q) = %q, %v; want %q, true", raw, got, ok, want)
		}
	}

	bad := []string{
		"notes.md",                    // не file://
		"https://example.com/file.md", // другая схема
		"file://server/share/doc.md",  // чужой host
		"file://",                     // пустой путь
		"file:///%zz",                 // битый percent-код
	}
	for _, raw := range bad {
		if got, ok := FileURLPath(raw); ok {
			t.Errorf("FileURLPath(%q) = %q, true; want false", raw, got)
		}
	}
}
