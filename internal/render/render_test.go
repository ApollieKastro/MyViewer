package render

import (
	"strings"
	"testing"
)

const sample = "# Заголовок\n\nПривет, **мир**! Это `код`.\n\n- один\n- два\n"

func TestRenderBasic(t *testing.T) {
	r := New("notty", true)
	lines, err := r.Render(sample, 60)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("пустой рендер")
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Заголовок") {
		t.Errorf("в рендере нет текста заголовка:\n%s", joined)
	}
	if !strings.Contains(joined, "привет") && !strings.Contains(joined, "Привет") {
		t.Errorf("в рендере нет текста:\n%s", joined)
	}
	// Нет завершающих пробелов и висячих пустых строк в конце.
	for i, l := range lines {
		if l != strings.TrimRight(l, " \t") {
			t.Errorf("строка %d заканчивается пробелом: %q", i, l)
		}
	}
}

func TestRenderCachedSameInput(t *testing.T) {
	r := New("notty", true)
	first, err := r.Render(sample, 60)
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.Render(sample, 60) // тот же ввод — кэш
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(first, "\n") != strings.Join(second, "\n") {
		t.Error("повторный рендер с теми же входными данными отличается")
	}
}

func TestRenderDependsOnWidth(t *testing.T) {
	r := New("notty", true)
	long := "# " + strings.Repeat("очень длинный текст ", 30)
	a, err := r.Render(long, 40)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Render(long, 120)
	if err != nil {
		t.Fatal(err)
	}
	maxWidth := func(lines []string) int {
		m := 0
		for _, l := range lines {
			if len(l) > m {
				m = len(l)
			}
		}
		return m
	}
	if maxWidth(a) >= maxWidth(b) {
		t.Errorf("узкий рендер (%d) не уже широкого (%d)", maxWidth(a), maxWidth(b))
	}
}

func TestSetThemeChangesOutput(t *testing.T) {
	r := New("notty", true)
	plain, err := r.Render(sample, 60)
	if err != nil {
		t.Fatal(err)
	}
	r.SetTheme("dracula")
	if r.Theme() != "dracula" {
		t.Errorf("Theme = %q", r.Theme())
	}
	styled, err := r.Render(sample, 60)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(plain, "\n") == strings.Join(styled, "\n") {
		t.Error("смена темы не изменила вывод (кэш не сброшен?)")
	}
}

func TestInvalidate(t *testing.T) {
	r := New("notty", true)
	a, err := r.Render(sample, 60)
	if err != nil {
		t.Fatal(err)
	}
	r.Invalidate()
	b, err := r.Render(sample, 60)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Error("после Invalidate результат должен совпадать (тот же вход)")
	}
}

// auto-тема обязана выбирать dark/light по детектированному фону, а не
// опрашивать терминал в фоне.
func TestAutoThemeFollowsDetectedBackground(t *testing.T) {
	darkAuto := New("auto", true)
	darkExplicit := New("dark", false)
	lightAuto := New("auto", false)
	lightExplicit := New("light", false)

	a, err := darkAuto.Render(sample, 60)
	if err != nil {
		t.Fatal(err)
	}
	b, err := darkExplicit.Render(sample, 60)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Error("auto с тёмным фоном должна совпадать с явной темой dark")
	}

	c, err := lightAuto.Render(sample, 60)
	if err != nil {
		t.Fatal(err)
	}
	d, err := lightExplicit.Render(sample, 60)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(c, "\n") != strings.Join(d, "\n") {
		t.Error("auto со светлым фоном должна совпадать с явной темой light")
	}
	if strings.Join(a, "\n") == strings.Join(c, "\n") {
		t.Error("dark и light должны давать разный вывод")
	}
}

func TestRenderRawKeepsTextVerbatim(t *testing.T) {
	r := New("dark", true)
	src := "Обычный текст *звёздочки* и # черта\n- пункт\n"
	lines, err := r.RenderRaw(src, 80)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Обычный текст *звёздочки* и # черта", "- пункт"}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("RenderRaw = %q, want %q (текст не должен искажаться)", lines, want)
	}
}

func TestRenderRawTabsAndWrap(t *testing.T) {
	r := New("dark", true)

	// Табы разворачиваются в пробелы (шаг 4).
	lines, err := r.RenderRaw("a\tb", 40)
	if err != nil {
		t.Fatal(err)
	}
	if lines[0] != "a   b" {
		t.Errorf("табы: %q", lines[0])
	}

	// Длинная строка переносится по словам, без потери текста.
	long := strings.Repeat("слово ", 30)
	lines, err = r.RenderRaw(long, 40)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) < 3 {
		t.Fatalf("строка не перенеслась: %d строк", len(lines))
	}
	for _, l := range lines {
		if len([]rune(l)) > 40 {
			t.Errorf("строка шире окна: %q", l)
		}
	}
	// Текст не потерялся: те же слова в том же порядке.
	gotWords := strings.Fields(strings.Join(lines, " "))
	wantWords := strings.Fields(long)
	if len(gotWords) != len(wantWords) {
		t.Fatalf("потерялись слова: got %d, want %d", len(gotWords), len(wantWords))
	}
	for i := range gotWords {
		if gotWords[i] != wantWords[i] {
			t.Fatalf("слово %d: %q != %q", i, gotWords[i], wantWords[i])
		}
	}

	// \r\n нормализуется, хвостовые пустые строки отбрасываются.
	lines, err = r.RenderRaw("a\r\nb\r\n\r\n", 40)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(lines, "|") != "a|b" {
		t.Errorf("нормализация: %q", lines)
	}
}

func TestRenderRawPrettyJSON(t *testing.T) {
	r := New("dark", true)

	// Однострочный JSON разворачивается с отступами.
	lines, err := r.RenderRaw(`{"name":"mviewer","tags":["tui","vim"],"n":1}`, 80)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "\n  \"name\": \"mviewer\"") {
		t.Errorf("JSON не отформатирован:\n%s", joined)
	}
	if len(lines) < 4 {
		t.Errorf("ожидали разбитый JSON, got %d строк", len(lines))
	}

	// Невалидный JSON, похожий на объект, остаётся как есть.
	lines, err = r.RenderRaw("{\"broken\":", 80)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(lines, "\n") != "{\"broken\":" {
		t.Errorf("сломанный JSON должен остаться дословно: %q", lines)
	}

	// Не-JSON (обычный текст) не трогается.
	lines, err = r.RenderRaw("просто {скобки} в тексте\n", 80)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(lines, "\n") != "просто {скобки} в тексте" {
		t.Errorf("текст должен остаться дословно: %q", lines)
	}
}

func TestRenderCacheSeparatesModes(t *testing.T) {
	r := New("dark", true)
	src := "# hi\n"
	markdown, err := r.Render(src, 60)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := r.RenderRaw(src, 60) // тот же src, другой режим — кэш не должен соврать
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(markdown, "\n") == strings.Join(raw, "\n") {
		t.Error("кэш отдал markdown-результат для сырого режима")
	}
	// Повторный вызов сырого режима — снова из кэша.
	raw2, err := r.RenderRaw(src, 60)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(raw, "\n") != strings.Join(raw2, "\n") {
		t.Error("сырой кэш нестабилен")
	}
}

func TestPostprocess(t *testing.T) {
	// Хвостовой \n даёт пустую строку — она отрезается; внутренние пустые
	// строки сохраняются; завершающие пробелы убираются.
	in := "  строка  \n\n\nещё   \n"
	got := postprocess(in, 80)
	want := []string{"  строка", "", "", "ещё"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("postprocess = %q, want %q", got, want)
	}
	// Полностью пустой ввод — одна пустая строка.
	if g := postprocess("\n\n", 80); len(g) != 1 || g[0] != "" {
		t.Errorf("postprocess(пусто) = %q", g)
	}
}
