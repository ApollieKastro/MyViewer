package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitLines(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{""}},
		{"one", []string{"one"}},
		{"one\ntwo", []string{"one", "two"}},
		// Завершающий \n не создаёт пустую строку — документ хранит строки
		// без завершающего перевода.
		{"one\ntwo\n", []string{"one", "two"}},
		{"one\r\ntwo", []string{"one", "two"}},
		{"\n", []string{""}},
	}
	for _, c := range cases {
		got := SplitLines(c.in)
		if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("SplitLines(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsMarkdown(t *testing.T) {
	yes := []string{"a.md", "b.MD", "c.markdown", "d.mdown", "dir/x.mkd"}
	no := []string{"a.txt", "a.go", "README", "md", "a.md.txt"}
	for _, p := range yes {
		if !IsMarkdown(p) {
			t.Errorf("IsMarkdown(%q) = false", p)
		}
	}
	for _, p := range no {
		if IsMarkdown(p) {
			t.Errorf("IsMarkdown(%q) = true", p)
		}
	}
}

func TestOpenSaveRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(path, []byte("привет\nмир\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(d.Lines) != 2 || d.Lines[0] != "привет" {
		t.Fatalf("Lines = %q", d.Lines)
	}
	if d.Modified {
		t.Error("только что открытый документ не должен быть изменён")
	}
	if d.Name() != "doc.md" {
		t.Errorf("Name = %q", d.Name())
	}

	d.ReplaceLines([]string{"пока", "мир", "!"})
	if !d.Modified {
		t.Error("после ReplaceLines Modified должен быть true")
	}
	if err := d.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if d.Modified {
		t.Error("после Save Modified должен быть false")
	}

	// Временный файл атомарного сохранения не должен остаться.
	if _, err := os.Stat(path + ".mviewer.tmp"); !os.IsNotExist(err) {
		t.Error("остался временный файл .mviewer.tmp")
	}

	back, err := Open(path)
	if err != nil {
		t.Fatalf("перечитывание: %v", err)
	}
	if strings.Join(back.Lines, "\n") != "пока\nмир\n!" {
		t.Errorf("после roundtrip Lines = %q", back.Lines)
	}
}

func TestReplaceLinesSameText(t *testing.T) {
	d := New("x.md")
	d.Lines = []string{"a", "b"}
	d.ReplaceLines([]string{"a", "b"})
	if d.Modified {
		t.Error("тот же текст не должен помечать документ изменённым")
	}
	d.ReplaceLines([]string{"a", "c"})
	if !d.Modified {
		t.Error("другой текст должен помечать документ изменённым")
	}
	d.ReplaceLines(nil)
	if len(d.Lines) != 1 || d.Lines[0] != "" {
		t.Errorf("пустой ввод должен давать одну пустую строку: %q", d.Lines)
	}
}

func TestContentAlwaysEndsWithNewline(t *testing.T) {
	d := New("x.md")
	d.Lines = []string{"a", "b"}
	if got := d.Content(); got != "a\nb\n" {
		t.Errorf("Content = %q, want %q", got, "a\nb\n")
	}
}

func TestListDirOrderAndHidden(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.txt", "a.md", "Z.md", ".secret", "c.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	entries, err := ListDir(dir)
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}

	var names []string
	for _, e := range entries {
		names = append(names, e.Name)
	}
	want := []string{"sub", "a.md", "Z.md", "b.txt", "c.go"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("порядок = %q, want %q", names, want)
	}
	for _, e := range entries {
		if e.Name == ".secret" {
			t.Error("скрытые файлы не должны отображаться")
		}
	}
}

func TestFindMarkdown(t *testing.T) {
	dir := t.TempDir()
	// README имеет приоритет.
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte("r"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := FindMarkdown(dir)
	if filepath.Base(got) != "readme.md" {
		t.Errorf("FindMarkdown = %q, want readme.md", got)
	}

	// Без README — первый markdown.
	if err := os.Remove(filepath.Join(dir, "readme.md")); err != nil {
		t.Fatal(err)
	}
	got = FindMarkdown(dir)
	if filepath.Base(got) != "notes.md" {
		t.Errorf("FindMarkdown = %q, want notes.md", got)
	}

	// Без markdown вообще — пусто.
	empty := t.TempDir()
	if got := FindMarkdown(empty); got != "" {
		t.Errorf("FindMarkdown в пустом каталоге = %q, want \"\"", got)
	}
	// Несуществующий каталог — тоже пусто, без паники.
	if got := FindMarkdown(filepath.Join(dir, "нет-такого")); got != "" {
		t.Errorf("FindMarkdown для отсутствующего = %q", got)
	}
}
