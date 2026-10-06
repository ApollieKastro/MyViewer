package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"mviewer/internal/document"
)

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestLinkListNavigationAndOpen(t *testing.T) {
	l := NewLinkList([]document.Link{
		{Text: "первый", URL: "https://a.io"},
		{Text: "второй", URL: "https://b.io"},
		{Text: "третий", URL: "notes.md"},
	})

	// Начальное положение — первая ссылка; enter открывает её сразу.
	l.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if target, ok := l.TakeOpen(); !ok || target != "https://a.io" {
		t.Fatalf("TakeOpen = (%q, %v), want https://a.io", target, ok)
	}
	// Повторный забор — пусто (сообщение не «залипает»).
	if target, ok := l.TakeOpen(); ok || target != "" {
		t.Errorf("повторный TakeOpen = (%q, %v), want \"\", false", target, ok)
	}

	// j/j вниз, k вверх, границы.
	l.Update(keyRunes("j"))
	l.Update(keyRunes("j"))
	l.Update(keyRunes("j")) // за пределами — остаёмся на последней
	l.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if target, _ := l.TakeOpen(); target != "notes.md" {
		t.Errorf("после jjj want notes.md, got %q", target)
	}
	l.Update(keyRunes("k"))
	l.Update(keyRunes("k"))
	l.Update(keyRunes("k")) // за пределом — остаёмся на первой
	l.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if target, _ := l.TakeOpen(); target != "https://a.io" {
		t.Errorf("после kkk want a.io, got %q", target)
	}
}

func TestLinkListEmpty(t *testing.T) {
	l := NewLinkList(nil)
	l.Update(keyRunes("j"))
	l.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if target, ok := l.TakeOpen(); ok {
		t.Errorf("пустой список не должен открывать: %q", target)
	}
}
