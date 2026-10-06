// Package viewer реализует режим просмотра markdown.
//
// Компонент хранит готовые отрендеренные строки и окно прокрутки.
// Он не знает ни о markdown, ни о glamour: получает строки через SetContent,
// а сам отвечает только за навигацию (клавиатура, мышь) и нарезку окна.
package viewer

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Model — состояние режима просмотра.
type Model struct {
	lines  []string // отрендеренный документ
	offset int      // индекс первой видимой строки
	width  int
	height int // высота области просмотра (без статусбара)

	// mouseWheel — сколько строк крутит одно колесо (из конфига).
	mouseWheel int
	// scrollStep — шаг построчной навигации (стрелки/j/k).
	scrollStep int
	// vim — используются ли vim-клавиши в просмотре.
	vim bool
}

// New создаёт пустой просмотр.
func New() Model {
	return Model{
		lines:      []string{""},
		mouseWheel: 3,
		scrollStep: 1,
	}
}

// SetOptions обновляет параметры навигации из конфигурации.
func (m *Model) SetOptions(vim bool, wheel, step int) {
	m.vim = vim
	if wheel > 0 {
		m.mouseWheel = wheel
	}
	if step > 0 {
		m.scrollStep = step
	}
}

// SetContent устанавливает отрендеренные строки, сохраняя разумную позицию
// прокрутки (не даём улететь за конец при уменьшении документа).
func (m *Model) SetContent(lines []string) {
	if len(lines) == 0 {
		lines = []string{""}
	}
	m.lines = lines
	m.clamp()
}

// SetSize устанавливает размеры области просмотра.
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	m.clamp()
}

// LinesCount возвращает общее число строк содержимого.
func (m *Model) LinesCount() int { return len(m.lines) }

// Offset возвращает текущую строку прокрутки.
func (m *Model) Offset() int { return m.offset }

// SetOffset устанавливает позицию прокрутки с ограничением границ
// (используется для восстановления позиции при переходе назад/вперёд).
func (m *Model) SetOffset(off int) {
	m.offset = off
	m.clamp()
}

// Percent возвращает позицию прокрутки в процентах (0..100).
func (m *Model) Percent() int {
	if len(m.lines) <= m.height || m.height <= 0 {
		return 100
	}
	maxOff := len(m.lines) - m.height
	if maxOff <= 0 {
		return 100
	}
	p := m.offset * 100 / maxOff
	if p > 100 {
		p = 100
	}
	if p < 0 {
		p = 0
	}
	return p
}

// AtTop / AtBottom — состояние границ для подсветки в статусбаре.
func (m *Model) AtTop() bool    { return m.offset <= 0 }
func (m *Model) AtBottom() bool { return m.offset >= len(m.lines)-m.height }

// ScrollBy прокручивает содержимое на delta строк (отрицательное — вверх).
func (m *Model) ScrollBy(delta int) {
	m.offset += delta
	m.clamp()
}

// ScrollToTop / ScrollToBottom — переход в начало/конец документа.
func (m *Model) ScrollToTop()    { m.offset = 0 }
func (m *Model) ScrollToBottom() { m.offset = len(m.lines); m.clamp() }

// ScrollToPercent прокручивает к позиции 0..100.
func (m *Model) ScrollToPercent(p int) {
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	maxOff := len(m.lines) - m.height
	if maxOff < 0 {
		maxOff = 0
	}
	m.offset = maxOff * p / 100
	m.clamp()
}

// PageDown / PageUp — прокрутка на высоту экрана.
func (m *Model) PageDown() { m.ScrollBy(m.pageSize()) }
func (m *Model) PageUp()   { m.ScrollBy(-m.pageSize()) }

// HalfPageDown / HalfPageUp — прокрутка на половину экрана (vim ^D/^U).
func (m *Model) HalfPageDown() { m.ScrollBy(m.pageSize() / 2) }
func (m *Model) HalfPageUp()   { m.ScrollBy(-m.pageSize() / 2) }

// GotoLine переводит прокрутку так, чтобы строка line (1-based) была сверху.
func (m *Model) GotoLine(line int) {
	if line < 1 {
		line = 1
	}
	if line > len(m.lines) {
		line = len(m.lines)
	}
	m.offset = line - 1
	m.clamp()
}

// pageSize — размер прокрутки «страницей» (сохраняем контекст в 2 строки).
func (m *Model) pageSize() int {
	size := m.height - 2
	if size < 1 {
		size = 1
	}
	return size
}

// clamp удерживает offset в допустимых границах.
func (m *Model) clamp() {
	maxOff := len(m.lines) - m.height
	if maxOff < 0 {
		maxOff = 0
	}
	if m.offset > maxOff {
		m.offset = maxOff
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// Update обрабатывает события прокрутки. Глобальные клавиши (q, e, s, ?)
// перехватываются приложением до вызова этого метода.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return nil
}

// handleMouse — прокрутка колесом мыши.
func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.ScrollBy(-m.mouseWheel)
	case tea.MouseButtonWheelDown:
		m.ScrollBy(m.mouseWheel)
	}
	return nil
}

// handleKey — навигация клавиатурой.
func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	// Vim-раскладка просмотра (как в less/glow при включённом vim mode).
	if m.vim {
		switch msg.String() {
		case "j", "down":
			m.ScrollBy(m.scrollStep)
			return nil
		case "k", "up":
			m.ScrollBy(-m.scrollStep)
			return nil
		case "g":
			m.ScrollToTop()
			return nil
		case "G":
			m.ScrollToBottom()
			return nil
		case "ctrl+d":
			m.HalfPageDown()
			return nil
		case "ctrl+u":
			m.HalfPageUp()
			return nil
		case "f", "ctrl+f", "pagedown", " ":
			m.PageDown()
			return nil
		case "b", "ctrl+b", "pageup":
			m.PageUp()
			return nil
		}
		// vim-раскладка не перехватила — не возвращаемся, падаем в обычную ниже.
	}

	switch msg.String() {
	case "down", "j":
		m.ScrollBy(m.scrollStep)
	case "up", "k":
		m.ScrollBy(-m.scrollStep)
	case "pgdown", " ", "f":
		m.PageDown()
	case "pgup", "b":
		m.PageUp()
	case "ctrl+d":
		m.HalfPageDown()
	case "ctrl+u":
		m.HalfPageUp()
	case "home", "g":
		m.ScrollToTop()
	case "end", "G":
		m.ScrollToBottom()
	}
	return nil
}

// View отрисовывает видимое окно документа (только нарезка готовых строк —
// без каких-либо аллокаций ANSI-разметки).
func (m *Model) View() string {
	if m.height <= 0 {
		return ""
	}
	end := m.offset + m.height
	if end > len(m.lines) {
		end = len(m.lines)
	}
	if m.offset >= len(m.lines) {
		return ""
	}
	visible := m.lines[m.offset:end]

	var b strings.Builder
	b.Grow(m.width * len(visible))
	for i, line := range visible {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	return b.String()
}
