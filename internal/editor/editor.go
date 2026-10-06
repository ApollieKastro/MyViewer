package editor

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// editor.go — модель редактора: курсор, прокрутка, insert-ввод, отрисовка.

// SaveRequestMsg — сигнал приложению: пользователь запросил сохранение (Ctrl+S).
type SaveRequestMsg struct{}

// Options — параметры отрисовки и поведения редактора.
type Options struct {
	// Vim включает нормальный режим vim (иначе — простой редактор).
	Vim bool
	// LineNumbers показывает номера строк.
	LineNumbers bool
	// WheelLines — строк на поворот колеса мыши.
	WheelLines int
}

// Editor — встроенный текстовый редактор с vim-режимом.
type Editor struct {
	buf     *Buffer
	hist    *History
	vim     vimState
	opts    Options
	rowOff  int // верхняя видимая строка буфера
	colOff  int // левая видимая колонка
	width   int
	height  int
	exit    bool // запрос на выход из редактора (Esc в normal)
	changed bool // были ли изменения с последней синхронизации
}

// Стили курсора и выделения.
var (
	cursorStyle = lipgloss.NewStyle().Reverse(true)
	selectStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1E1E2E", Dark: "#CDD6F4"}).
			Background(lipgloss.AdaptiveColor{Light: "#A6ADC8", Dark: "#45475A"})
	gutterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#9399B2", Dark: "#6C7086"})
)

// New создаёт пустой редактор.
func New() *Editor {
	return &Editor{
		buf:  NewBuffer(""),
		hist: NewHistory(500),
		vim:  newVimState(),
		opts: Options{Vim: true, LineNumbers: true, WheelLines: 3},
	}
}

// Load заменяет содержимое редактора (новая история).
func (e *Editor) Load(text string) {
	e.buf.SetText(text)
	e.hist = NewHistory(500)
	e.vim = newVimState()
	e.vim.enabled = e.opts.Vim
	e.rowOff, e.colOff = 0, 0
	e.exit = false
	e.changed = false
	e.buf.ClampNormal()
}

// Modified сообщает, были ли изменения текста.
func (e *Editor) Modified() bool { return e.changed }

// ResetModified сбрасывает флаг изменений (после сохранения).
func (e *Editor) ResetModified() { e.changed = false }

// SetOptions применяет параметры из конфигурации.
func (e *Editor) SetOptions(o Options) {
	e.opts = o
	e.vim.enabled = o.Vim
	if !o.Vim {
		// При выключенном vim остаёмся в «печатном» режиме.
		e.vim.mode = ModeNormal
	}
}

// SetSize устанавливает размеры текстовой области.
func (e *Editor) SetSize(width, height int) {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	e.width, e.height = width, height
	e.ensureVisible()
}

// Lines возвращает содержимое построчно (для синхронизации с документом).
func (e *Editor) Lines() []string { return e.buf.Lines() }

// Content возвращает содержимое одной строкой.
func (e *Editor) Content() string { return e.buf.String() }

// Mode возвращает текущий подрежим.
func (e *Editor) Mode() Mode { return e.vim.mode }

// VimEnabled сообщает, включён ли vim-режим.
func (e *Editor) VimEnabled() bool { return e.opts.Vim }

// Cursor возвращает позицию курсора (0-based).
func (e *Editor) Cursor() (row, col int) { return e.buf.Row(), e.buf.Col() }

// Percent возвращает позицию курсора в процентах документа.
func (e *Editor) Percent() int {
	total := e.buf.LinesCount()
	if total <= 1 {
		return 100
	}
	p := e.buf.Row() * 100 / (total - 1)
	if p > 100 {
		p = 100
	}
	return p
}

// PendingLabel — метка незавершённой команды (d, 2, f...) для статусбара.
func (e *Editor) PendingLabel() string { return e.vim.PendingLabel() }

// RegisterText возвращает содержимое регистра (для статусбара/тестов).
func (e *Editor) RegisterText() string { return e.vim.register.Text }

// ConsumeExit забирает флаг выхода из редактора.
func (e *Editor) ConsumeExit() bool {
	ex := e.exit
	e.exit = false
	return ex
}

// ---- Навигация ----

// pageSize — число строк прокрутки на «страницу».
func (e *Editor) pageSize() int {
	h := e.height - 1
	if h < 1 {
		h = 1
	}
	return h
}

// scrollBy прокручивает редактор на delta строк.
func (e *Editor) scrollBy(delta int) {
	e.rowOff += delta
	maxOff := e.buf.LinesCount() - 1
	if e.rowOff > maxOff {
		e.rowOff = maxOff
	}
	if e.rowOff < 0 {
		e.rowOff = 0
	}
}

// textWidth — ширина текстовой области с учётом номеров строк.
func (e *Editor) textWidth() int {
	w := e.width - e.gutterWidth()
	if w < 1 {
		return 1
	}
	return w
}

// gutterWidth — ширина колонки номеров строк.
func (e *Editor) gutterWidth() int {
	if !e.opts.LineNumbers {
		return 0
	}
	digits := len(strconv.Itoa(e.buf.LinesCount()))
	if digits < 2 {
		digits = 2
	}
	return digits + 1 // цифры + пробел-разделитель
}

// ensureVisible подгоняет окно прокрутки под позицию курсора.
func (e *Editor) ensureVisible() {
	if e.height <= 0 {
		return
	}
	row := e.buf.Row()
	if row < e.rowOff {
		e.rowOff = row
	}
	if row >= e.rowOff+e.height {
		e.rowOff = row - e.height + 1
	}
	if e.rowOff < 0 {
		e.rowOff = 0
	}

	col := e.buf.Col()
	textW := e.textWidth()
	if col < e.colOff {
		e.colOff = col
	}
	if col >= e.colOff+textW {
		e.colOff = col - textW + 1
	}
	if e.colOff < 0 {
		e.colOff = 0
	}
}

// enterInsert переводит в режим вставки.
func (e *Editor) enterInsert() {
	e.vim.mode = ModeInsert
	e.vim.histPushedInInsert = false
	e.vim.reset() // чистим счётчики/ожидания (mode меняем после reset!)
	e.vim.mode = ModeInsert
}

// ---- Обработка событий ----

// Update обрабатывает события ввода/размера. Возвращает tea.Cmd при необходимости.
func (e *Editor) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		e.SetSize(msg.Width, msg.Height)
		return nil

	case tea.MouseMsg:
		return e.handleMouse(msg)

	case tea.KeyMsg:
		return e.handleKey(msg)
	}
	return nil
}

// handleMouse — колесо и клики мышью.
func (e *Editor) handleMouse(msg tea.MouseMsg) tea.Cmd {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		e.scrollBy(-e.opts.WheelLines)
		return nil
	case tea.MouseButtonWheelDown:
		e.scrollBy(e.opts.WheelLines)
		return nil
	}
	// Левый клик — позиционировать курсор (по строке под указателем).
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
		y := msg.Y
		if y < 0 || y >= e.height {
			return nil
		}
		row := e.rowOff + y
		if row >= e.buf.LinesCount() {
			row = e.buf.LinesCount() - 1
		}
		gw := e.gutterWidth()
		col := e.colOff
		if msg.X > gw {
			col = e.colOff + (msg.X - gw)
		}
		e.buf.SetCursor(row, col)
		if e.vim.mode == ModeInsert {
			// в insert можно стать после последнего символа
			if l := e.buf.LineLen(row); e.buf.Col() > l {
				e.buf.SetCursor(row, l)
			}
		} else {
			e.buf.ClampNormal()
		}
		e.vim.desiredCol = -1
		e.ensureVisible()
	}
	return nil
}

// handleKey — клавиатурный ввод.
func (e *Editor) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	// Управляющие комбинации — независимо от режима.
	switch key {
	case "ctrl+s":
		return func() tea.Msg { return SaveRequestMsg{} }
	case "ctrl+d":
		e.scrollBy(e.pageSize() / 2)
		return nil
	case "ctrl+u":
		e.scrollBy(-e.pageSize() / 2)
		return nil
	case "ctrl+f":
		e.scrollBy(e.pageSize())
		return nil
	case "ctrl+b":
		e.scrollBy(-e.pageSize())
		return nil
	}

	if !e.opts.Vim {
		return e.handlePlainKey(msg, key)
	}

	switch e.vim.mode {
	case ModeInsert:
		e.handleInsertKey(msg, key)
	default:
		if e.vim.mode == ModeVisual || e.vim.mode == ModeVisualLine {
			e.handleVisualKey(key)
		} else {
			e.handleNormalKey(key)
		}
	}
	return nil
}

// handleInsertKey — ввод в режиме вставки.
func (e *Editor) handleInsertKey(msg tea.KeyMsg, key string) {
	switch key {
	case "esc":
		// Выход в normal: курсор сдвигается влево (как в vim).
		e.vim.mode = ModeNormal
		e.vim.reset()
		if e.buf.Col() > 0 {
			e.buf.SetCursor(e.buf.Row(), e.buf.Col()-1)
		}
		e.buf.ClampNormal()
		e.ensureVisible()
		return
	case "enter":
		e.pushHistory()
		e.buf.Newline()
		e.ensureVisible()
		return
	case "backspace":
		if e.buf.Col() == 0 && e.buf.Row() == 0 {
			return
		}
		e.pushHistory()
		e.buf.Backspace()
		e.ensureVisible()
		return
	case "delete":
		if e.buf.Col() >= e.buf.LineLen(e.buf.Row()) && e.buf.Row() == e.buf.LinesCount()-1 {
			return
		}
		e.pushHistory()
		e.buf.DeleteForward()
		e.ensureVisible()
		return
	case "up":
		e.moveVertical(-1, true)
		return
	case "down":
		e.moveVertical(1, true)
		return
	case "left":
		if e.buf.Col() > 0 {
			e.buf.SetCursor(e.buf.Row(), e.buf.Col()-1)
		} else if e.buf.Row() > 0 {
			e.buf.SetCursor(e.buf.Row()-1, e.buf.LineLen(e.buf.Row()-1))
		}
		e.ensureVisible()
		return
	case "right":
		if e.buf.Col() < e.buf.LineLen(e.buf.Row()) {
			e.buf.SetCursor(e.buf.Row(), e.buf.Col()+1)
		} else if e.buf.Row() < e.buf.LinesCount()-1 {
			e.buf.SetCursor(e.buf.Row()+1, 0)
		}
		e.ensureVisible()
		return
	case "home":
		e.buf.SetCursor(e.buf.Row(), 0)
		e.ensureVisible()
		return
	case "end":
		e.buf.SetCursor(e.buf.Row(), e.buf.LineLen(e.buf.Row()))
		e.ensureVisible()
		return
	case "pgup":
		e.scrollBy(-e.pageSize())
		return
	case "pgdown":
		e.scrollBy(e.pageSize())
		return
	case "tab":
		e.pushHistory()
		e.buf.InsertText("\t")
		e.ensureVisible()
		return
	}

	// Обычные символы (включая пробел и многобайтовые руны).
	if text, ok := typedText(msg); ok && text != "" {
		e.pushHistory()
		e.buf.InsertText(text)
		e.ensureVisible()
	}
}

// typedText извлекает вводимый текст из события клавиши.
func typedText(msg tea.KeyMsg) (string, bool) {
	if msg.Type == tea.KeySpace {
		return " ", true
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
		// Игнорируем модификаторные комбинации (alt+руна — не вставка).
		if msg.Alt {
			return "", false
		}
		return string(msg.Runes), true
	}
	return "", false
}

// handlePlainKey — режим без vim: простая вставка и навигация,
// Esc выходит из редактора.
func (e *Editor) handlePlainKey(msg tea.KeyMsg, key string) tea.Cmd {
	switch key {
	case "esc":
		e.exit = true
		return nil
	case "enter":
		e.pushHistory()
		e.buf.Newline()
		e.ensureVisible()
		return nil
	case "backspace":
		if e.buf.Col() == 0 && e.buf.Row() == 0 {
			return nil
		}
		e.pushHistory()
		e.buf.Backspace()
		e.ensureVisible()
		return nil
	case "delete":
		e.pushHistory()
		e.buf.DeleteForward()
		e.ensureVisible()
		return nil
	case "up":
		e.moveVertical(-1, true)
		return nil
	case "down":
		e.moveVertical(1, true)
		return nil
	case "left":
		if e.buf.Col() > 0 {
			e.buf.SetCursor(e.buf.Row(), e.buf.Col()-1)
		}
		e.ensureVisible()
		return nil
	case "right":
		if e.buf.Col() < e.buf.LineLen(e.buf.Row()) {
			e.buf.SetCursor(e.buf.Row(), e.buf.Col()+1)
		}
		e.ensureVisible()
		return nil
	case "home":
		e.buf.SetCursor(e.buf.Row(), 0)
		e.ensureVisible()
		return nil
	case "end":
		e.buf.SetCursor(e.buf.Row(), e.buf.LineLen(e.buf.Row()))
		e.ensureVisible()
		return nil
	case "pgup":
		e.scrollBy(-e.pageSize())
		return nil
	case "pgdown":
		e.scrollBy(e.pageSize())
		return nil
	}
	if text, ok := typedText(msg); ok && text != "" {
		e.pushHistory()
		e.buf.InsertText(text)
		e.ensureVisible()
	}
	return nil
}

// ---- Отрисовка ----

// View рисует видимое окно редактора с номерами строк, курсором и выделением.
func (e *Editor) View() string {
	if e.height <= 0 {
		return ""
	}
	gw := e.gutterWidth()
	textW := e.textWidth()

	var b strings.Builder
	for i := 0; i < e.height; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		row := e.rowOff + i
		if row >= e.buf.LinesCount() {
			continue // за концом документа — пустая строка
		}
		e.renderLine(&b, row, gw, textW)
	}
	return b.String()
}

// renderLine рисует одну строку буфера.
func (e *Editor) renderLine(b *strings.Builder, row, gutter, textW int) {
	// Номер строки.
	if gutter > 0 {
		num := strconv.Itoa(row + 1)
		pad := gutter - 1 - len(num)
		if pad < 0 {
			pad = 0
		}
		b.WriteString(gutterStyle.Render(strings.Repeat(" ", pad) + num + " "))
	}

	line := e.buf.Line(row)
	start := e.colOff
	if start > len(line) {
		start = len(line)
	}
	end := start + textW
	if end > len(line) {
		end = len(line)
	}

	// Границы выделения и курсора для этой строки.
	selA, selB, hasSel := e.selectionBounds(row)
	curRow, curCol := e.buf.Row(), e.buf.Col()
	onThisLine := curRow == row

	// Собираем сегменты одинакового стиля.
	var seg strings.Builder
	var segStyle segKind = segPlain
	flush := func() {
		if seg.Len() == 0 {
			return
		}
		switch segStyle {
		case segCursorSel, segCursor:
			b.WriteString(cursorStyle.Render(seg.String()))
		case segSelect:
			b.WriteString(selectStyle.Render(seg.String()))
		default:
			b.WriteString(seg.String())
		}
		seg.Reset()
	}

	cursorPlaced := false
	for c := start; c < end; c++ {
		isSel := hasSel && c >= selA && c <= selB
		isCur := onThisLine && c == curCol
		kind := segPlain
		switch {
		case isCur && isSel:
			kind = segCursorSel
		case isCur:
			kind = segCursor
		case isSel:
			kind = segSelect
		}
		if kind != segStyle {
			flush()
			segStyle = kind
		}
		seg.WriteRune(line[c])
		if isCur {
			cursorPlaced = true
		}
	}
	flush()

	// Курсор за концом строки (insert-режим) — рисуем блок после текста.
	if onThisLine && (curCol >= len(line) || curCol >= end) && curCol >= start {
		if !cursorPlaced {
			b.WriteString(cursorStyle.Render(" "))
		}
	}
}

// segKind — стиль сегмента строки.
type segKind uint8

const (
	segPlain segKind = iota
	segSelect
	segCursor
	segCursorSel
)

// selectionBounds возвращает границы выделения (в колонках) для строки.
func (e *Editor) selectionBounds(row int) (int, int, bool) {
	v := &e.vim
	if !v.mode.IsVisual() {
		return 0, 0, false
	}
	if v.mode == ModeVisualLine {
		a, b := v.anchor.Row, e.buf.Row()
		if a > b {
			a, b = b, a
		}
		if row < a || row > b {
			return 0, 0, false
		}
		return 0, e.buf.LineLen(row) - 1, true
	}
	a, b := v.anchor, e.buf.Cursor()
	if cmpPos(a, b) > 0 {
		a, b = b, a
	}
	if row < a.Row || row > b.Row {
		return 0, 0, false
	}
	from, to := 0, e.buf.LineLen(row)-1
	if row == a.Row {
		from = a.Col
	}
	if row == b.Row {
		to = b.Col
	}
	if from > to {
		return 0, 0, false
	}
	return from, to, true
}
