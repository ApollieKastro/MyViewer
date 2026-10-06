package editor

// Buffer — текстовый буфер редактора.
//
// Текст хранится как срез строк рун ([][]rune): это даёт O(1)-доступ к
// символу, корректную работу с многобайтовыми символами (кириллица,
// эмодзи) и предсказуемую память. Все позиции — rune-индексы.
type Buffer struct {
	lines [][]rune
	row   int // текущая строка курсора (0-based)
	col   int // текущая колонка курсора (rune index)
}

// Pos — позиция в буфере.
type Pos struct {
	Row int
	Col int
}

// NewBuffer создаёт буфер из текста. Гарантирует минимум одну строку.
func NewBuffer(text string) *Buffer {
	b := &Buffer{}
	b.SetText(text)
	b.row, b.col = 0, 0
	return b
}

// SetText заменяет содержимое буфера, сохраняя позицию в допустимых границах.
func (b *Buffer) SetText(text string) {
	lines := splitRunes(text)
	b.lines = lines
	b.clampCursor()
}

// splitRunes разбивает текст на строки рун, нормализуя переводы строк
// (CRLF и одиночный CR приводятся к LF).
func splitRunes(text string) [][]rune {
	if text == "" {
		return [][]rune{{}}
	}
	var lines [][]rune
	cur := make([]rune, 0, 64)
	for _, r := range text {
		switch r {
		case '\r':
			continue // отбрасываем CR (обработка CRLF)
		case '\n':
			lines = append(lines, cur)
			cur = make([]rune, 0, 64)
			continue
		}
		cur = append(cur, r)
	}
	lines = append(lines, cur)
	return lines
}

// ---- Доступ к содержимому ----

// LinesCount возвращает число строк.
func (b *Buffer) LinesCount() int { return len(b.lines) }

// Line возвращает строку по индексу (за границами — пустая строка).
func (b *Buffer) Line(i int) []rune {
	if i < 0 || i >= len(b.lines) {
		return nil
	}
	return b.lines[i]
}

// LineLen возвращает длину строки в рунах.
func (b *Buffer) LineLen(i int) int {
	if i < 0 || i >= len(b.lines) {
		return 0
	}
	return len(b.lines[i])
}

// String возвращает содержимое буфера как строку.
func (b *Buffer) String() string {
	var total int
	for _, l := range b.lines {
		total += len(l) + 1
	}
	out := make([]byte, 0, total)
	for i, l := range b.lines {
		if i > 0 {
			out = append(out, '\n')
		}
		out = append(out, string(l)...)
	}
	return string(out)
}

// Lines возвращает копию строк (для сохранения документа без побочных эффектов).
func (b *Buffer) Lines() []string {
	out := make([]string, len(b.lines))
	for i, l := range b.lines {
		out[i] = string(l)
	}
	return out
}

// CloneLines возвращает глубокую копию строк (для undo/redo и визуального режима).
func (b *Buffer) CloneLines() [][]rune {
	out := make([][]rune, len(b.lines))
	for i, l := range b.lines {
		c := make([]rune, len(l))
		copy(c, l)
		out[i] = c
	}
	return out
}

// ---- Курсор ----

// Cursor возвращает текущую позицию курсора.
func (b *Buffer) Cursor() Pos { return Pos{Row: b.row, Col: b.col} }

// Row возвращает текущую строку курсора.
func (b *Buffer) Row() int { return b.row }

// Col возвращает текущую колонку курсора.
func (b *Buffer) Col() int { return b.col }

// SetCursor ставит курсор в позицию, зажимая координаты в границы буфера.
func (b *Buffer) SetCursor(row, col int) {
	b.row, b.col = row, col
	b.clampCursor()
}

// clampCursor зажимает курсор в границы документа.
// В normal-режиме vim курсор не может стоять на col == len(line):
// это применяется отдельно через clampCursorNormal.
func (b *Buffer) clampCursor() {
	if b.row < 0 {
		b.row = 0
	}
	if b.row >= len(b.lines) {
		b.row = len(b.lines) - 1
	}
	if b.col < 0 {
		b.col = 0
	}
	if max := len(b.lines[b.row]); b.col > max {
		b.col = max
	}
}

// ClampNormal зажимает курсор по правилу normal-режима vim: не дальше
// последнего символа строки (а на пустой строке — на позиции 0).
func (b *Buffer) ClampNormal() {
	b.clampCursor()
	if max := len(b.lines[b.row]) - 1; b.col > max {
		b.col = max
	}
	if b.col < 0 {
		b.col = 0
	}
}

// ---- Мутации текста ----

// InsertRune вставляет руну в позицию курсора (курсор сдвигается вправо).
func (b *Buffer) InsertRune(r rune) {
	line := b.lines[b.row]
	nl := make([]rune, 0, len(line)+1)
	nl = append(nl, line[:b.col]...)
	nl = append(nl, r)
	nl = append(nl, line[b.col:]...)
	b.lines[b.row] = nl
	b.col++
}

// InsertText вставляет многострочный текст в позицию курсора.
// Курсор оказывается в конце вставленного фрагмента.
func (b *Buffer) InsertText(text string) {
	rows := splitRunes(text)
	if len(rows) == 1 {
		line := b.lines[b.row]
		nl := make([]rune, 0, len(line)+len(rows[0]))
		nl = append(nl, line[:b.col]...)
		nl = append(nl, rows[0]...)
		nl = append(nl, line[b.col:]...)
		b.lines[b.row] = nl
		b.col += len(rows[0])
		return
	}
	head := append([]rune{}, b.lines[b.row][:b.col]...)
	tail := append([]rune{}, b.lines[b.row][b.col:]...)

	last := rows[len(rows)-1]
	merged := append(append(append([]rune{}, head...), rows[0]...), rows[1]...)

	// Собляем: head + первая строка, средние, последняя + tail.
	out := make([][]rune, 0, len(b.lines)+len(rows)-1)
	out = append(out, b.lines[:b.row]...)
	out = append(out, merged)
	for _, r := range rows[1 : len(rows)-1] {
		out = append(out, append([]rune{}, r...))
	}
	out = append(out, append(append([]rune{}, last...), tail...))
	b.lines = out

	b.row += len(rows) - 1
	b.col = len(last)
	b.clampCursor()
}

// Newline разрывает строку в позиции курсора (аналог Enter).
func (b *Buffer) Newline() {
	line := b.lines[b.row]
	head := append([]rune{}, line[:b.col]...)
	tail := append([]rune{}, line[b.col:]...)

	b.lines[b.row] = head
	rest := make([][]rune, 0, len(b.lines)+1)
	rest = append(rest, b.lines[:b.row+1]...)
	rest = append(rest, tail)
	rest = append(rest, b.lines[b.row+1:]...)
	b.lines = rest

	b.row++
	b.col = 0
}

// Backspace удаляет символ слева от курсора.
func (b *Buffer) Backspace() {
	if b.col > 0 {
		line := b.lines[b.row]
		nl := make([]rune, 0, len(line)-1)
		nl = append(nl, line[:b.col-1]...)
		nl = append(nl, line[b.col:]...)
		b.lines[b.row] = nl
		b.col--
		return
	}
	if b.row == 0 {
		return
	}
	// Сливаем с предыдущей строкой.
	prev := b.lines[b.row-1]
	cur := b.lines[b.row]
	merged := make([]rune, 0, len(prev)+len(cur))
	merged = append(merged, prev...)
	merged = append(merged, cur...)

	out := make([][]rune, 0, len(b.lines)-1)
	out = append(out, b.lines[:b.row-1]...)
	out = append(out, merged)
	out = append(out, b.lines[b.row+1:]...)
	b.lines = out

	b.row--
	b.col = len(prev)
}

// DeleteForward удаляет символ справа от курсора (vim x).
func (b *Buffer) DeleteForward() {
	line := b.lines[b.row]
	if b.col >= len(line) {
		return
	}
	nl := make([]rune, 0, len(line)-1)
	nl = append(nl, line[:b.col]...)
	nl = append(nl, line[b.col+1:]...)
	b.lines[b.row] = nl
	b.clampCursor()
}

// DeleteBackwardChar удаляет символ под курсором вперёд (Delete в insert).
func (b *Buffer) DeleteForwardChar() { b.DeleteForward() }

// ---- Операции над строками и диапазонами ----

// DeleteLines удаляет строки [from, to] включительно.
func (b *Buffer) DeleteLines(from, to int) {
	if from < 0 {
		from = 0
	}
	if to >= len(b.lines) {
		to = len(b.lines) - 1
	}
	if from > to || len(b.lines) == 1 {
		// Последнюю строку не удаляем полностью — делаем пустой.
		if len(b.lines) == 1 {
			b.lines[0] = nil
			b.row, b.col = 0, 0
			return
		}
		return
	}
	out := make([][]rune, 0, len(b.lines)-(to-from+1))
	out = append(out, b.lines[:from]...)
	out = append(out, b.lines[to+1:]...)
	b.lines = out
	b.clampCursor()
}

// InsertLinesAfter вставляет строки после указанной (-1 — в начало).
func (b *Buffer) InsertLinesAfter(after int, rows [][]rune) {
	if after < -1 || after >= len(b.lines) {
		after = len(b.lines) - 1
	}
	// Не мутируем входные срезы — копируем.
	copies := make([][]rune, len(rows))
	for i, r := range rows {
		copies[i] = append([]rune{}, r...)
	}
	out := make([][]rune, 0, len(b.lines)+len(copies))
	out = append(out, b.lines[:after+1]...)
	out = append(out, copies...)
	out = append(out, b.lines[after+1:]...)
	b.lines = out
	b.clampCursor()
}

// ReplaceRange заменяет диапазон [start, end) текстом.
// end.Row == -1 означает конец документа. Курсор ставится в начало
// вставленного текста.
func (b *Buffer) ReplaceRange(start, end Pos, text string) {
	if end.Row < 0 || end.Row >= len(b.lines) {
		end = Pos{Row: len(b.lines) - 1, Col: len(b.lines[len(b.lines)-1])}
	}
	if start.Row < 0 {
		start = Pos{}
	}
	// Вырезаем кусок.
	var removed string
	switch {
	case start.Row == end.Row:
		line := b.lines[start.Row]
		c0, c1 := clampCols(line, start.Col, end.Col)
		removed = string(line[c0:c1])
		nl := make([]rune, 0, len(line)-(c1-c0))
		nl = append(nl, line[:c0]...)
		nl = append(nl, line[c1:]...)
		b.lines[start.Row] = nl
	default:
		first := b.lines[start.Row]
		last := b.lines[end.Row]
		c0, _ := clampCols(first, start.Col, start.Col)
		c1, _ := clampCols(last, end.Col, end.Col)

		var sb []byte
		sb = append(sb, string(first[c0:])...)
		sb = append(sb, '\n')
		for r := start.Row + 1; r < end.Row; r++ {
			sb = append(sb, string(b.lines[r])...)
			sb = append(sb, '\n')
		}
		sb = append(sb, string(last[:c1])...)
		removed = string(sb)

		merged := make([]rune, 0, c0+(len(last)-c1))
		merged = append(merged, first[:c0]...)
		merged = append(merged, last[c1:]...)

		out := make([][]rune, 0, len(b.lines)-(end.Row-start.Row))
		out = append(out, b.lines[:start.Row]...)
		out = append(out, merged)
		out = append(out, b.lines[end.Row+1:]...)
		b.lines = out
	}

	_ = removed // возвращает значение вызывающему через GetRange при необходимости
	b.row, b.col = start.Row, start.Col
	if text != "" {
		b.InsertText(text)
	}
	b.clampCursor()
}

// GetRange возвращает текст в диапазоне [start, end).
func (b *Buffer) GetRange(start, end Pos) string {
	if end.Row < 0 || end.Row >= len(b.lines) {
		end = Pos{Row: len(b.lines) - 1, Col: len(b.lines[len(b.lines)-1])}
	}
	if start.Row < 0 {
		start = Pos{}
	}
	if start.Row == end.Row {
		line := b.lines[start.Row]
		c0, c1 := clampCols(line, start.Col, end.Col)
		return string(line[c0:c1])
	}
	var sb []byte
	first := b.lines[start.Row]
	c0, _ := clampCols(first, start.Col, start.Col)
	sb = append(sb, string(first[c0:])...)
	sb = append(sb, '\n')
	for r := start.Row + 1; r < end.Row; r++ {
		sb = append(sb, string(b.lines[r])...)
		sb = append(sb, '\n')
	}
	last := b.lines[end.Row]
	c1, _ := clampCols(last, end.Col, end.Col)
	sb = append(sb, string(last[:c1])...)
	return string(sb)
}

// clampCols нормализует пару колонок так, чтобы c0 <= c1 и обе были в границах.
func clampCols(line []rune, a, b int) (int, int) {
	if a < 0 {
		a = 0
	}
	if b < 0 {
		b = 0
	}
	if a > len(line) {
		a = len(line)
	}
	if b > len(line) {
		b = len(line)
	}
	if a > b {
		a, b = b, a
	}
	return a, b
}

// ---- Undo/Redo ----

// snapshot — снимок состояния буфера для истории отмен.
type snapshot struct {
	lines    [][]rune
	row, col int
}

// History — стек undo/redo с ограничением размера.
type History struct {
	undo []snapshot
	redo []snapshot
	max  int
}

// NewHistory создаёт историю с ограничением depth снимков.
func NewHistory(depth int) *History {
	if depth <= 0 {
		depth = 500
	}
	return &History{max: depth}
}

// Push сохраняет текущее состояние в стек undo и очищает redo.
func (h *History) Push(b *Buffer) {
	h.undo = append(h.undo, snapshot{lines: b.CloneLines(), row: b.row, col: b.col})
	if len(h.undo) > h.max {
		h.undo = h.undo[len(h.undo)-h.max:]
	}
	h.redo = h.redo[:0]
}

// CanUndo / CanRedo — доступность операций.
func (h *History) CanUndo() bool { return len(h.undo) > 0 }
func (h *History) CanRedo() bool { return len(h.redo) > 0 }

// Undo откатывает буфер на предыдущее состояние.
func (h *History) Undo(b *Buffer) bool {
	if len(h.undo) == 0 {
		return false
	}
	h.redo = append(h.redo, snapshot{lines: b.CloneLines(), row: b.row, col: b.col})
	s := h.undo[len(h.undo)-1]
	h.undo = h.undo[:len(h.undo)-1]
	b.lines, b.row, b.col = s.lines, s.row, s.col
	b.clampCursor()
	return true
}

// Redo повторяет отменённое изменение.
func (h *History) Redo(b *Buffer) bool {
	if len(h.redo) == 0 {
		return false
	}
	h.undo = append(h.undo, snapshot{lines: b.CloneLines(), row: b.row, col: b.col})
	s := h.redo[len(h.redo)-1]
	h.redo = h.redo[:len(h.redo)-1]
	b.lines, b.row, b.col = s.lines, s.row, s.col
	b.clampCursor()
	return true
}
