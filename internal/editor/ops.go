package editor

import "unicode"

// ops.go — операции vim над диапазонами и строками.
//
// Каждая операция сама решает, нужен ли снимок для undo (pushHistory),
// и сама выставляет курсор в результирующую позицию.

// ---- Операторы над мотионами и выделением ----

// runOperatorMotion применяет оператор (d/y/c) к диапазону мотиона.
// forceLinewise принудительно трактует диапазон как строки.
func (e *Editor) runOperatorMotion(op opKind, m motion, forceLinewise bool) bool {
	v := &e.vim
	v.operator = opNone
	v.pend = pendNone
	v.opCount, v.hasOpCount = 0, false
	v.takeCount() // счётчик израсходован мотионом
	if !m.ok {
		return false
	}

	linewise := m.linewise || forceLinewise
	if linewise {
		// Целые строки: от min до max строк включительно.
		start := e.buf.Row()
		end := m.to.Row
		if start > end {
			start, end = end, start
		}
		return e.opOnLineRange(op, start, end)
	}

	// Посимвольный диапазон.
	var start, end Pos
	if m.hasFrom {
		start, end = m.from, m.to
	} else {
		start, end = e.buf.Cursor(), m.to
		if cmpPos(start, end) > 0 {
			start, end = end, start
		}
	}
	if m.inclusive {
		end.Col++
	}
	return e.opOnCharRange(op, start, end)
}

// applyVisual применяет оператор к визуальному выделению.
func (e *Editor) applyVisual(op opKind) bool {
	v := &e.vim
	anchor, cur := v.anchor, e.buf.Cursor()

	if v.mode == ModeVisualLine {
		start, end := anchor.Row, cur.Row
		if start > end {
			start, end = end, start
		}
		v.mode = ModeNormal
		return e.opOnLineRange(op, start, end)
	}

	start, end := anchor, cur
	if cmpPos(start, end) > 0 {
		start, end = end, start
	}
	end.Col++ // выделение включает символ под курсором
	v.mode = ModeNormal
	return e.opOnCharRange(op, start, end)
}

// opOnLineRange выполняет операцию над диапазоном строк [from..to].
func (e *Editor) opOnLineRange(op opKind, from, to int) bool {
	if from < 0 {
		from = 0
	}
	if to >= e.buf.LinesCount() {
		to = e.buf.LinesCount() - 1
	}
	if from > to {
		return false
	}
	switch op {
	case opYank:
		e.yankRegister(from, to, true)
		e.moveTo(Pos{Row: from, Col: 0})
		e.buf.ClampNormal()
		return true
	case opDelete, opChange:
		e.pushHistory()
		e.yankRegister(from, to, true)
		e.buf.DeleteLines(from, to)
		e.buf.SetCursor(from, 0)
		e.buf.ClampNormal()
		if op == opChange {
			e.enterInsert()
		}
		return true
	}
	return false
}

// opOnCharRange выполняет операцию над посимвольным диапазоном [start, end).
func (e *Editor) opOnCharRange(op opKind, start, end Pos) bool {
	if cmpPos(start, end) >= 0 {
		return false // пустой диапазон
	}
	switch op {
	case opYank:
		e.vim.register = Register{Text: e.buf.GetRange(start, end), Linewise: false}
		e.moveTo(start)
		return true
	case opDelete, opChange:
		e.pushHistory()
		e.vim.register = Register{Text: e.buf.GetRange(start, end), Linewise: false}
		e.buf.ReplaceRange(start, end, "")
		e.moveTo(start)
		e.buf.ClampNormal()
		if op == opChange {
			e.enterInsert()
		}
		return true
	}
	return false
}

// yankRegister кладёт строки [from..to] в регистр.
func (e *Editor) yankRegister(from, to int, linewise bool) {
	var parts []string
	for i := from; i <= to && i < e.buf.LinesCount(); i++ {
		parts = append(parts, string(e.buf.Line(i)))
	}
	text := ""
	for i, p := range parts {
		if i > 0 {
			text += "\n"
		}
		text += p
	}
	e.vim.register = Register{Text: text, Linewise: linewise}
}

// ---- Навигация с состоянием (sticky column) ----

// moveTo ставит курсор, сбрасывая sticky-колонку.
func (e *Editor) moveTo(p Pos) {
	e.buf.SetCursor(p.Row, p.Col)
	e.buf.ClampNormal()
	e.vim.desiredCol = -1
	e.ensureVisible()
}

// moveHorizontal двигает курсор на colDelta символов.
// keepDesired — не сбрасывать sticky-колонку (для h/l её нет).
func (e *Editor) moveHorizontal(colDelta int, keepDesired bool) bool {
	v := &e.vim
	row, col := e.buf.Row(), e.buf.Col()
	col += colDelta
	if col < 0 {
		col = 0
	}
	if max := e.buf.LineLen(row) - 1; col > max {
		col = max
	}
	if col < 0 {
		col = 0
	}
	e.buf.SetCursor(row, col)
	if !keepDesired {
		v.desiredCol = -1
	}
	e.ensureVisible()
	return true
}

// moveVertical двигает курсор на строку delta со sticky-колонкой.
func (e *Editor) moveVertical(delta int, sticky bool) bool {
	v := &e.vim
	row, col := e.buf.Row(), e.buf.Col()
	if sticky {
		// Запоминаем целевую колонку при первом переходе (как vim).
		if v.desiredCol < 0 {
			v.desiredCol = col
		}
		col = v.desiredCol
	}
	row += delta
	if row < 0 {
		row = 0
	}
	if row >= e.buf.LinesCount() {
		row = e.buf.LinesCount() - 1
	}
	e.buf.SetCursor(row, col) // SetCursor зажмёт колонку в длину строки
	e.buf.ClampNormal()
	if !sticky {
		v.desiredCol = -1
	}
	e.ensureVisible()
	return true
}

// selectMotion устанавливает выделение от m.from до m.to (текстовые объекты).
func (e *Editor) selectMotion(m motion) {
	v := &e.vim
	if m.hasFrom {
		v.anchor = m.from
	} else {
		v.anchor = e.buf.Cursor()
	}
	e.moveTo(m.to)
}

// textObjectMotion вычисляет мотион текстового объекта (obj — 'i' или 'a').
func (e *Editor) textObjectMotion(obj rune, target rune) (motion, bool) {
	inner := obj == 'i'
	// Алиасы скобок как в vim: b → (), B → {}.
	switch target {
	case 'b':
		target = '('
	case 'B':
		target = '{'
	}
	m := objectRange(e.buf, e.buf.Cursor(), target, inner)
	return m, m.ok
}

// ---- Одиночные операции normal-режима ----

// deleteCharsForward удаляет n символов вперёд (x). Возвращает успех.
func (e *Editor) deleteCharsForward(n int) bool {
	row := e.buf.Row()
	col := e.buf.Col()
	length := e.buf.LineLen(row)
	if col >= length {
		return false
	}
	end := col + n
	if end > length {
		end = length
	}
	return e.opOnCharRange(opDelete, Pos{Row: row, Col: col}, Pos{Row: row, Col: end})
}

// deleteCharsBackward удаляет n символов назад (X).
func (e *Editor) deleteCharsBackward(n int) bool {
	row := e.buf.Row()
	col := e.buf.Col()
	if col == 0 {
		return false
	}
	start := col - n
	if start < 0 {
		start = 0
	}
	return e.opOnCharRange(opDelete, Pos{Row: row, Col: start}, Pos{Row: row, Col: col})
}

// deleteLines удаляет n строк с текущей (dd).
func (e *Editor) deleteLines(n int) bool {
	from := e.buf.Row()
	to := from + n - 1
	if to >= e.buf.LinesCount() {
		to = e.buf.LinesCount() - 1
	}
	return e.opOnLineRange(opDelete, from, to)
}

// yankLines копирует n строк с текущей (yy/Y).
func (e *Editor) yankLines(n int) bool {
	from := e.buf.Row()
	to := from + n - 1
	if to >= e.buf.LinesCount() {
		to = e.buf.LinesCount() - 1
	}
	e.yankRegister(from, to, true)
	e.moveTo(Pos{Row: from, Col: 0})
	return true
}

// changeLines удаляет n строк и переходит в insert (cc/S).
func (e *Editor) changeLines(n int) bool {
	return e.opOnLineRange(opChange, e.buf.Row(), e.buf.Row()+n-1)
}

// joinLines объединяет текущую строку с n следующими (J).
func (e *Editor) joinLines(n int) bool {
	if n < 1 {
		n = 1
	}
	row := e.buf.Row()
	if row+n >= e.buf.LinesCount() {
		n = e.buf.LinesCount() - 1 - row
	}
	if n < 1 {
		return false // объединять не с чем
	}
	e.pushHistory()

	// Объединяем по одной: trim хвостовых пробелов + один разделитель.
	joinCol := e.buf.LineLen(row)
	for i := 0; i < n; i++ {
		cur := e.buf.Line(row)
		next := e.buf.Line(row + 1)

		// Обрезаем хвостовые пробелы текущей.
		trimmed := len(cur)
		for trimmed > 0 && (cur[trimmed-1] == ' ' || cur[trimmed-1] == '\t') {
			trimmed--
		}
		// Обрезаем ведущие пробелы следующей.
		lead := 0
		for lead < len(next) && (next[lead] == ' ' || next[lead] == '\t') {
			lead++
		}
		needSep := trimmed > 0 && lead < len(next)

		merged := make([]rune, 0, trimmed+len(next)-lead+1)
		merged = append(merged, cur[:trimmed]...)
		if needSep {
			merged = append(merged, ' ')
		}
		merged = append(merged, next[lead:]...)

		e.buf.lines[row] = merged
		e.buf.DeleteLines(row+1, row+1)
	}
	// Курсор — на месте склейки (первый присоединённый символ).
	if joinCol > e.buf.LineLen(row) {
		joinCol = e.buf.LineLen(row)
	}
	if joinCol > 0 && e.buf.LineLen(row) > 0 {
		// vim ставит курсор на первый присоединённый символ.
		pos := joinCol
		if pos > e.buf.LineLen(row)-1 {
			pos = e.buf.LineLen(row) - 1
		}
		if pos < 0 {
			pos = 0
		}
		e.buf.SetCursor(row, pos)
	}
	e.buf.ClampNormal()
	e.ensureVisible()
	return true
}

// joinVisual объединяет все строки выделения (J в visual).
func (e *Editor) joinVisual() bool {
	v := &e.vim
	a, c := v.anchor.Row, e.buf.Row()
	if a > c {
		a, c = c, a
	}
	v.mode = ModeNormal
	return e.joinLines(c - a)
}

// swapCase меняет регистр n символов от курсора (~).
func (e *Editor) swapCase(n int) bool {
	row := e.buf.Row()
	col := e.buf.Col()
	length := e.buf.LineLen(row)
	if col >= length {
		return false
	}
	end := col + n
	if end > length {
		end = length
	}
	e.pushHistory()
	line := e.buf.Line(row)
	for i := col; i < end; i++ {
		r := line[i]
		switch {
		case isLowerRune(r):
			line[i] = toUpperRune(r)
		case isUpperRune(r):
			line[i] = toLowerRune(r)
		}
	}
	e.buf.lines[row] = line
	e.buf.SetCursor(row, end) // курсор после последнего изменённого
	e.buf.ClampNormal()
	e.ensureVisible()
	return true
}

// replaceChar заменяет символ под курсором на r (r{char}) n раз.
func (e *Editor) replaceChar(r rune) bool {
	row := e.buf.Row()
	col := e.buf.Col()
	length := e.buf.LineLen(row)
	if col >= length {
		return false
	}
	n := e.vim.effectiveCount()
	end := col + n
	if end > length {
		end = length
	}
	e.pushHistory()
	line := e.buf.Line(row)
	for i := col; i < end; i++ {
		line[i] = r
	}
	e.buf.lines[row] = line
	e.buf.SetCursor(row, col)
	return true
}

// applyFind ищет символ в строке (f/F/t/T) с повтором count раз.
func (e *Editor) applyFind(cmd, target rune) bool {
	row := e.buf.Row()
	col := e.buf.Col()
	line := e.buf.Line(row)
	if len(line) == 0 {
		return false
	}
	n := e.vim.effectiveCount()

	pos := col
	switch cmd {
	case 'f', 't':
		for i := 0; i < n; i++ {
			found := -1
			for j := pos + 1; j < len(line); j++ {
				if line[j] == target {
					found = j
					break
				}
			}
			if found < 0 {
				return false
			}
			pos = found
		}
		if cmd == 't' {
			pos--
			if pos < 0 {
				pos = 0
			}
		}
	case 'F', 'T':
		for i := 0; i < n; i++ {
			found := -1
			for j := pos - 1; j >= 0; j-- {
				if line[j] == target {
					found = j
					break
				}
			}
			if found < 0 {
				return false
			}
			pos = found
		}
		if cmd == 'T' {
			pos++
			if pos >= len(line) {
				pos = len(line) - 1
			}
		}
	default:
		return false
	}
	e.buf.SetCursor(row, pos)
	e.ensureVisible()
	return true
}

// paste вставляет регистр после (after=true) или перед курсором.
func (e *Editor) paste(after bool) bool {
	v := &e.vim
	if v.register.Empty || v.register.Text == "" {
		return false
	}
	e.pushHistory()

	if v.register.Linewise {
		rows := splitRunes(v.register.Text)
		row := e.buf.Row()
		afterInt := row
		if !after {
			afterInt = row - 1
		}
		e.buf.InsertLinesAfter(afterInt, rows)
		e.buf.SetCursor(afterInt+1, 0)
		e.buf.ClampNormal()
		e.ensureVisible()
		return true
	}

	// Посимвольная вставка.
	if after {
		// После курсора: позиция col+1 (если строка пуста — в начало).
		if e.buf.LineLen(e.buf.Row()) == 0 {
			e.buf.SetCursor(e.buf.Row(), 0)
		} else {
			e.buf.SetCursor(e.buf.Row(), e.buf.Col()+1)
		}
	}
	e.buf.InsertText(v.register.Text)
	// InsertText ставит курсор после вставленного — в vim на последний символ.
	if e.buf.Col() > 0 {
		e.buf.SetCursor(e.buf.Row(), e.buf.Col()-1)
	}
	e.buf.ClampNormal()
	e.ensureVisible()
	return true
}

// undo откатывает n изменений. Возвращает успех.
func (e *Editor) undo(n int) bool {
	ok := false
	for i := 0; i < n; i++ {
		if !e.hist.Undo(e.buf) {
			break
		}
		ok = true
	}
	if ok {
		e.buf.ClampNormal()
		e.vim.histPushedInInsert = false
		e.ensureVisible()
	}
	return ok
}

// redo повторяет отмену (ctrl+r — обрабатывается отдельно в Update).
func (e *Editor) redo(n int) bool {
	ok := false
	for i := 0; i < n; i++ {
		if !e.hist.Redo(e.buf) {
			break
		}
		ok = true
	}
	if ok {
		e.buf.ClampNormal()
		e.vim.histPushedInInsert = false
		e.ensureVisible()
	}
	return ok
}

// ---- История ----

// pushHistory сохраняет снимок перед изменением.
// В insert-сессии снимок делается один раз — весь ввод до Esc откатывается
// одной командой u (как в vim).
func (e *Editor) pushHistory() {
	e.changed = true
	if e.vim.mode == ModeInsert && e.vim.histPushedInInsert {
		return
	}
	e.hist.Push(e.buf)
	if e.vim.mode == ModeInsert {
		e.vim.histPushedInInsert = true
	}
}

// ---- Unicode-регистр ----

func isUpperRune(r rune) bool { return unicode.IsUpper(r) }
func isLowerRune(r rune) bool { return unicode.IsLower(r) }
func toUpperRune(r rune) rune { return unicode.ToUpper(r) }
func toLowerRune(r rune) rune { return unicode.ToLower(r) }

// cmpPos сравнивает позиции: <0 — a раньше b.
func cmpPos(a, b Pos) int {
	if a.Row != b.Row {
		return a.Row - b.Row
	}
	return a.Col - b.Col
}
