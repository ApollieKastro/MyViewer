package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mviewer/internal/document"
)

// LinkList — модальный список ссылок документа (аналог Settings:
// навигация j/k, открытие по enter, закрытие делает app по esc).
type LinkList struct {
	links  []document.Link
	cursor int
	offset int

	// openTarget — URL, запрошенный к открытию (забирается через TakeOpen).
	openTarget string
}

// NewLinkList создаёт список ссылок.
func NewLinkList(links []document.Link) *LinkList {
	return &LinkList{links: links}
}

// TakeOpen забирает запрошенную для открытия цель ("" — не запрашивалась).
func (l *LinkList) TakeOpen() (string, bool) {
	t := l.openTarget
	l.openTarget = ""
	return t, t != ""
}

// Update обрабатывает клавиши списка.
func (l *LinkList) Update(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	switch keyMsg.String() {
	case "up", "k":
		l.move(-1)
	case "down", "j":
		l.move(1)
	case "pgup":
		l.move(-l.pageSize())
	case "pgdown":
		l.move(l.pageSize())
	case "home", "g":
		l.cursor = 0
	case "end", "G":
		l.cursor = len(l.links) - 1
	case "enter", "o":
		if l.cursor >= 0 && l.cursor < len(l.links) {
			l.openTarget = l.links[l.cursor].URL
		}
	}
	return nil
}

// move сдвигает курсор с ограничением границ.
func (l *LinkList) move(delta int) {
	l.cursor += delta
	if l.cursor < 0 {
		l.cursor = 0
	}
	if l.cursor >= len(l.links) {
		l.cursor = len(l.links) - 1
	}
}

// pageSize — высота страницы списка.
func (l *LinkList) pageSize() int {
	h := 10
	if h < 1 {
		h = 1
	}
	return h
}

// View рисует панель ссылок, центрированную в окне width×height.
func (l *LinkList) View(width, height int) string {
	var b strings.Builder

	b.WriteString(Title.Render("Ссылки") + "\n")
	b.WriteString(MutedText.Render("↑↓/jk — выбор · enter — открыть · esc — закрыть") + "\n")
	b.WriteString(strings.Repeat("─", 54) + "\n")

	// Окно видимых строк с учётом высоты панели (шапка+разделители+подвал).
	visible := height - 8
	if visible < 3 {
		visible = 3
	}
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	if l.cursor >= l.offset+visible {
		l.offset = l.cursor - visible + 1
	}
	maxOff := len(l.links) - visible
	if maxOff < 0 {
		maxOff = 0
	}
	if l.offset > maxOff {
		l.offset = maxOff
	}
	if l.offset < 0 {
		l.offset = 0
	}

	end := l.offset + visible
	if end > len(l.links) {
		end = len(l.links)
	}
	for i := l.offset; i < end; i++ {
		lnk := l.links[i]
		label := lnk.Text
		if label == "" {
			label = lnk.URL
		}
		line := clip("  "+itoa(i+1)+". "+label, 52)
		if i == l.cursor {
			b.WriteString(FocusedItem.Render(padTo(line, 52)) + "\n")
		} else {
			b.WriteString(Item.Render(line) + "\n")
		}
	}

	b.WriteString(strings.Repeat("─", 54) + "\n")
	// Цель выделенной ссылки — отдельной строкой (куда откроется).
	if l.cursor >= 0 && l.cursor < len(l.links) {
		b.WriteString(Desc.Render(clip(l.links[l.cursor].URL, 52)))
	}

	panel := Panel.Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}

// clip обрезает строку до n рун (без учёта стилей — вызывать до рендера).
func clip(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n-1]) + "…"
}

// itoa — десятичное число в строку (без fmt в горячем пути отрисовки).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
