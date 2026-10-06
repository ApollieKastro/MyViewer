package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectKind(t *testing.T) {
	cases := []struct {
		name string
		path string
		data string
		want FileKind
	}{
		{"markdown по расширению", "a.md", "# заголовок", KindMarkdown},
		{"markdown .markdown", "a.markdown", "текст", KindMarkdown},
		{"markdown .mdx", "a.mdx", "привет", KindMarkdown},
		{"markdown .rmd", "a.rmd", "привет", KindMarkdown},
		{"markdown .qmd", "a.qmd", "привет", KindMarkdown},
		{"текст по расширению", "a.txt", "просто текст", KindText},
		{"код", "main.go", "package main", KindText},
		{"без расширения — текст", "LICENSE", "MIT License", KindText},
		{"без расширения — заголовок md", "README", "# Проект\n\nописание", KindMarkdown},
		{"без расширения — H2 в начале", "CHANGELOG", "## 1.0\n\nновое", KindMarkdown},
		{"без расширения — BOM + заголовок", "README", "\xef\xbb\xbf# Проект", KindMarkdown},
		{"без расширения — shebang не md", "run", "#!/bin/sh\necho hi", KindText},
		{"без расширения — #без пробела не md", "conf", "#comment\nvalue", KindText},
		{"без расширения — комментарии на 6-й строке", "notes", "а\nб\nв\nг\nд\n# позно", KindText},
		{"расширение важнее комментариев", "app.conf", "# комментарий\nkey=val", KindText},
		{"неизвестное расширение — текст", "data.xyz", "привет", KindText},
		{"сигнатура бинарника важнее расширения", "x.md", "bin\x00data", KindBinary},
		{"PNG-сигнатура", "img.txt", "\x89PNG\r\n\x1a\n\x00\x00", KindBinary},
		{"ELF", "prog", "\x7fELF\x02\x01\x01\x00\x00\x00", KindBinary},
		{"лог с ANSI-кодами — текст", "ansi.log", "ok \x1b[32mgreen\x1b[0m", KindText},
		{"много управляющих символов", "weird.txt", "\x01\x02\x03\x04\x05\x06\x07\x08", KindBinary},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectKind(c.path, []byte(c.data))
			if got != c.want {
				t.Errorf("DetectKind(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func TestOpenRejectsBinary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blob.md") // расширение врёт — содержимое бинарное
	if err := os.WriteFile(path, []byte("MZ\x00\x03\x00\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Open(path)
	if err == nil {
		t.Fatal("Open бинарника должна возвращать ошибку")
	}
	if !strings.Contains(err.Error(), "не поддерживается") {
		t.Errorf("ошибка должна объяснять причину: %v", err)
	}
}

func TestOpenKinds(t *testing.T) {
	dir := t.TempDir()

	md := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(md, []byte("# hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := Open(md)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != KindMarkdown {
		t.Errorf("Kind = %v, want markdown", d.Kind)
	}

	txt := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(txt, []byte("* не markdown *\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err = Open(txt)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != KindText {
		t.Errorf("Kind = %v, want text", d.Kind)
	}
}

func TestFindDocument(t *testing.T) {
	dir := t.TempDir()

	// Пустой каталог — пусто.
	if got := FindDocument(dir); got != "" {
		t.Errorf("пустой каталог: %q", got)
	}

	// Только бинарные файлы — не берём.
	if err := os.WriteFile(filepath.Join(dir, "logo.png"), []byte("\x89PNG\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FindDocument(dir); got != "" {
		t.Errorf("бинарники не должны открываться: %q", got)
	}

	// README имеет приоритет над прочим markdown.
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("r"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := FindDocument(dir)
	if filepath.Base(got) != "README.md" {
		t.Errorf("приоритет README: %q", got)
	}

	// Без README — любой markdown.
	if err := os.Remove(filepath.Join(dir, "README.md")); err != nil {
		t.Fatal(err)
	}
	got = FindDocument(dir)
	if filepath.Base(got) != "a.md" {
		t.Errorf("без README берём markdown: %q", got)
	}

	// Без markdown — текстовый файл.
	if err := os.Remove(filepath.Join(dir, "a.md")); err != nil {
		t.Fatal(err)
	}
	got = FindDocument(dir)
	if filepath.Base(got) != "b.txt" {
		t.Errorf("без markdown берём текст: %q", got)
	}
}

func TestFileKindString(t *testing.T) {
	if KindMarkdown.String() != "md" || KindText.String() != "txt" || KindBinary.String() != "bin" {
		t.Errorf("метки: %q %q %q", KindMarkdown, KindText, KindBinary)
	}
}
