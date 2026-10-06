package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mviewer/internal/config"
)

// Регресс: клиенты могут доставлять набранный текст одним пакетом рун
// (например «/mviewer» за раз) — команды должны распознаваться так же,
// как последовательность одиночных нажатий.
func TestViewRuneBatchOpensSearch(t *testing.T) {
	m := New("", &config.Config{}, false)

	mm, _ := m.onViewKey(
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/mviewer")},
		"/mviewer",
	)
	got, ok := mm.(*Model)
	if !ok {
		t.Fatal("onViewKey должен возвращать *Model")
	}
	if !got.searchActive {
		t.Error("пакет с «/» должен открыть режим поиска")
	}
	if got.searchInput != "mviewer" {
		t.Errorf("searchInput = %q, want %q", got.searchInput, "mviewer")
	}

	// esc закрывает ввод, не потеряв остальной пакет.
	if _, _ = got.onViewKey(tea.KeyMsg{Type: tea.KeyEsc}, "esc"); got.searchActive {
		t.Error("esc должен закрыть режим поиска")
	}
}

// Одиночная «/» тоже открывает поиск; ввод пачки в активном поиске
// добавляется целиком.
func TestSearchInputRuneBatch(t *testing.T) {
	m := New("", &config.Config{}, false)

	if _, _ = m.onViewKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}, "/"); !m.searchActive {
		t.Fatal("одиночная «/» должна открыть поиск")
	}
	m.searchKeyHandle(
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abc")},
		"abc",
	)
	if m.searchInput != "abc" {
		t.Errorf("searchInput = %q, want %q", m.searchInput, "abc")
	}
}
