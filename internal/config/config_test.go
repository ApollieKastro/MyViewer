package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	c := Default()
	if !c.VimMode {
		t.Error("VimMode должен быть true по умолчанию")
	}
	if c.Theme != ThemeAuto {
		t.Errorf("Theme = %q, want %q", c.Theme, ThemeAuto)
	}
	if c.Editor != EditorAuto {
		t.Errorf("Editor = %q, want %q", c.Editor, EditorAuto)
	}
	if c.WheelLines != 3 || c.ScrollStep != 1 {
		t.Errorf("WheelLines/ScrollStep = %d/%d, want 3/1", c.WheelLines, c.ScrollStep)
	}
}

func TestEditorNormalize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"editor":"emacs"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Editor != EditorAuto {
		t.Errorf("неизвестный редактор должен нормализоваться: %q", c.Editor)
	}
	// Все значения цикла валидны.
	for _, v := range EditorCycle {
		c.Editor = v
		c.normalize()
		if c.Editor != v {
			t.Errorf("normalize сломал валидное значение %q → %q", v, c.Editor)
		}
	}
}

func TestThemesUniqueAndKnown(t *testing.T) {
	seen := map[string]bool{}
	for _, th := range Themes() {
		if seen[th] {
			t.Errorf("тема %q встречается дважды", th)
		}
		seen[th] = true
		if !IsKnownTheme(th) {
			t.Errorf("IsKnownTheme(%q) = false", th)
		}
	}
	if IsKnownTheme("nope") {
		t.Error("IsKnownTheme(\"nope\") = true")
	}
}

func TestNextPrevThemeCycle(t *testing.T) {
	c := Default()
	c.Theme = ThemeAuto
	// Полный круг вперёд возвращает исходную тему.
	for range len(Themes()) {
		c.NextTheme()
	}
	if c.Theme != ThemeAuto {
		t.Errorf("после полного круга NextTheme = %q, want %q", c.Theme, ThemeAuto)
	}
	// Назад с первой темы — на последнюю.
	c.Theme = Themes()[0]
	c.PrevTheme()
	if c.Theme != Themes()[len(Themes())-1] {
		t.Errorf("PrevTheme с первой = %q, want %q", c.Theme, Themes()[len(Themes())-1])
	}
	// Неизвестная тема сбрасывается на первую.
	c.Theme = "unknown"
	c.NextTheme()
	if c.Theme != Themes()[0] {
		t.Errorf("NextTheme из неизвестной = %q, want %q", c.Theme, Themes()[0])
	}
}

func TestSetTheme(t *testing.T) {
	c := Default()
	if err := c.SetTheme(ThemeDracula); err != nil {
		t.Fatalf("SetTheme(dracula): %v", err)
	}
	if c.Theme != ThemeDracula {
		t.Errorf("Theme = %q, want %q", c.Theme, ThemeDracula)
	}
	if err := c.SetTheme("banana"); err == nil {
		t.Error("SetTheme(banana) должен возвращать ошибку")
	}
	if c.Theme != ThemeDracula {
		t.Errorf("после ошибки Theme = %q, want не меняться", c.Theme)
	}
}

func TestLoadFromAndNormalize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{"vim_mode":false,"theme":"banana","wheel_lines":0,"scroll_step":99}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if c.VimMode {
		t.Error("VimMode должен быть false")
	}
	if c.Theme != ThemeAuto {
		t.Errorf("битая тема должна нормализоваться: %q", c.Theme)
	}
	if c.WheelLines != 3 || c.ScrollStep != 1 {
		t.Errorf("normalize: %d/%d, want 3/1", c.WheelLines, c.ScrollStep)
	}
}

func TestLoadFromErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadFrom(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("отсутствующий файл должен давать ошибку")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{не json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFrom(bad); err == nil {
		t.Error("битый JSON должен давать ошибку")
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	// Подменяем каталог конфига, чтобы не трогать пользовательский файл.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	c := Default()
	c.VimMode = false
	c.Theme = ThemeTokyo
	c.WheelLines = 7
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(Path()); err != nil {
		t.Fatalf("файл конфига не создан: %v", err)
	}

	got, err := LoadFrom(Path())
	if err != nil {
		t.Fatalf("LoadFrom после Save: %v", err)
	}
	if got.VimMode || got.Theme != ThemeTokyo || got.WheelLines != 7 {
		t.Errorf("roundtrip: vim=%v theme=%q wheel=%d", got.VimMode, got.Theme, got.WheelLines)
	}
}
