package editor

import "fmt"

// vim.go — машина состояний vim-режима и диспетчеризация normal/visual ввода.
//
// Разделение ответственности:
//   - motion.go   — чистые функции вычисления позиций;
//   - buffer.go   — хранение текста;
//   - ops.go      — операции над диапазонами (delete/yank/change/paste);
//   - vim.go      — счётчики, ожидания (d i", f x), регистры, режимы;
//   - editor.go   — модель: курсор, прокрутка, отрисовка, insert-ввод.

// Mode — подрежим редактора.
type Mode uint8

const (
	// ModeNormal — обычный режим vim.
	ModeNormal Mode = iota
	// ModeInsert — режим вставки.
	ModeInsert
	// ModeVisual — посимвольное выделение.
	ModeVisual
	// ModeVisualLine — выделение строк.
	ModeVisualLine
)

// String — человекочитаемое имя режима (для статусбара).
func (m Mode) String() string {
	switch m {
	case ModeInsert:
		return "INSERT"
	case ModeVisual:
		return "VISUAL"
	case ModeVisualLine:
		return "V-LINE"
	default:
		return "NORMAL"
	}
}

// IsVisual сообщает, активен ли визуальный режим.
func (m Mode) IsVisual() bool { return m == ModeVisual || m == ModeVisualLine }

// Register — буфер обмена vim (один неименованный регистр).
type Register struct {
	Text     string // содержимое
	Linewise bool   // true — целые строки (dd/yy), false — фрагмент текста
	Empty    bool   // регистр ещё не заполнялся
}

// opKind — ожидающий оператор.
type opKind uint8

const (
	opNone opKind = iota
	opDelete
	opYank
	opChange
)

// pendKind — что именно ожидает ввода.
type pendKind uint8

const (
	pendNone pendKind = iota
	// pendOperator — ждём мотион после d/y/c.
	pendOperator
	// pendG — ждём вторую g (команда gg).
	pendG
	// pendFind — ждём символ после f/F/t/T (pendChar хранит клавишу-команду).
	pendFind
	// pendReplace — ждём символ замены после r.
	pendReplace
	// pendTextObj — ждём цель текстового объекта после i/a (в операторе/visual).
	pendTextObj
)

// vimState — изменяемое состояние vim-диспетчера.
type vimState struct {
	enabled bool
	mode    Mode

	// Счётчики: текущий и множитель оператора (2d3w = 6).
	count      int
	hasCount   bool
	opCount    int
	hasOpCount bool

	// Ожидающее состояние.
	operator opKind
	pend     pendKind
	pendChar rune // f/F/t/T или 'i'/'a' для текстового объекта

	// Выделение визуального режима.
	anchor Pos

	// Регистр и sticky-колонка для j/k.
	register   Register
	desiredCol int // -1 — неактивно

	// histPushedInInsert — снимок для undo в этой insert-сессии уже сделан.
	histPushedInInsert bool
}

// newVimState создаёт состояние normal-режима.
func newVimState() vimState {
	return vimState{mode: ModeNormal, desiredCol: -1}
}

// reset сбрасывает временное состояние (esc).
func (v *vimState) reset() {
	v.count, v.hasCount = 0, false
	v.opCount, v.hasOpCount = 0, false
	v.operator = opNone
	v.pend = pendNone
	v.pendChar = 0
	v.desiredCol = -1
}

// takeCount забирает накопленный счётчик (0 — если не вводился).
func (v *vimState) takeCount() int {
	c := v.count
	v.count, v.hasCount = 0, false
	return c
}

// effectiveCount возвращает итоговый счётчик с учётом множителя оператора.
func (v *vimState) effectiveCount() int {
	mc := v.takeCount()
	if mc <= 0 {
		mc = 1
	}
	if v.hasOpCount {
		if v.opCount > 0 {
			mc *= v.opCount
		}
		v.opCount, v.hasOpCount = 0, false
	}
	return mc
}

// PendingLabel возвращает метку ожидающего ввода для статусбара
// (например, "d", "d2", "f", "r", "di").
func (v *vimState) PendingLabel() string {
	switch v.pend {
	case pendOperator:
		if v.hasOpCount {
			return fmt.Sprintf("%c%d", opChar(v.operator), v.opCount)
		}
		return string(opChar(v.operator))
	case pendG:
		return "g"
	case pendFind:
		return string(v.pendChar)
	case pendReplace:
		return "r"
	case pendTextObj:
		return string(v.pendChar)
	default:
		if v.hasCount {
			return fmt.Sprintf("%d", v.count)
		}
		return ""
	}
}

// opChar — символ оператора для метки.
func opChar(o opKind) rune {
	switch o {
	case opDelete:
		return 'd'
	case opYank:
		return 'y'
	case opChange:
		return 'c'
	default:
		return '?'
	}
}

// ---- Диспетчеризация ввода ----

// handleNormalKey обрабатывает клавишу в normal-режиме.
// Возвращает true, если клавиша потреблена диспетчером.
func (e *Editor) handleNormalKey(key string) bool {
	v := &e.vim

	// Ожидающие состояния имеют приоритет над обычной раскладкой.
	switch v.pend {
	case pendOperator:
		if e.handleMotionForOperator(key) {
			return true
		}
		// Неизвестный мотион — оператор отменяется, клавиша поглощается
		// (как в vim: dx ничего не делает).
		v.operator = opNone
		v.pend = pendNone
		v.opCount, v.hasOpCount = 0, false
		return true

	case pendG:
		v.pend = pendNone
		if key == "g" {
			count := v.takeCount()
			row := 0
			if count > 0 {
				row = count - 1
			}
			m := motionLine(e.buf, e.buf.Cursor(), row)
			if v.operator != opNone {
				op := v.operator
				v.operator = opNone
				e.runOperatorMotion(op, m, true)
				return true
			}
			e.moveTo(m.to)
			return true
		}
		// Не g — ожидание (и возможный оператор) отменено, клавиша
		// поглощается.
		v.operator = opNone
		v.opCount, v.hasOpCount = 0, false
		return true

	case pendFind:
		v.pend = pendNone
		if r, ok := singleRune(key); ok {
			e.applyFind(v.pendChar, r)
		}
		return true // прочий ввод при ожидании поглощаем

	case pendReplace:
		v.pend = pendNone
		if r, ok := singleRune(key); ok {
			e.replaceChar(r)
		}
		return true

	case pendTextObj:
		v.pend = pendNone
		if r, ok := singleRune(key); ok {
			m, okObj := e.textObjectMotion(v.pendChar, r)
			if v.operator != opNone {
				op := v.operator
				v.operator = opNone
				v.opCount, v.hasOpCount = 0, false
				if okObj {
					e.runOperatorMotion(op, m, false)
				}
				return true
			}
			if okObj {
				e.moveTo(m.to)
			}
			return true
		}
		return true
	}

	// Цифры — накопление счётчика (0 — команда, если счётчик не начат).
	if r, ok := singleRune(key); ok && r >= '0' && r <= '9' {
		if r == '0' && !v.hasCount {
			e.moveTo(motionLineStart(e.buf, e.buf.Cursor()).to)
			return true
		}
		v.count = v.count*10 + int(r-'0')
		v.hasCount = true
		if v.count > 9999 {
			v.count = 9999
		}
		return true
	}

	// Специальные клавиши.
	switch key {
	case "esc":
		// Чистый esc в normal — выход к просмотру; при незавершённой
		// команде (счётчик/ожидание) — только сброс.
		hadPending := v.pend != pendNone || v.hasCount
		v.reset()
		if !hadPending {
			e.exit = true
		}
		return true
	case "ctrl+c":
		v.reset()
		return true
	case "up":
		return e.moveVertical(-1, true)
	case "down":
		return e.moveVertical(1, true)
	case "left":
		e.moveTo(motionLeft(e.buf, e.buf.Cursor()).to)
		return true
	case "right", " ":
		e.moveTo(motionRight(e.buf, e.buf.Cursor()).to)
		return true
	case "enter":
		return e.moveVertical(1, true)
	case "backspace":
		e.moveTo(motionLeft(e.buf, e.buf.Cursor()).to)
		return true
	case "home":
		e.moveTo(motionFirstNonBlank(e.buf, e.buf.Cursor()).to)
		return true
	case "end":
		e.moveTo(motionLineEnd(e.buf, e.buf.Cursor()).to)
		return true
	case "pgdown":
		e.moveTo(motionLineDown(e.buf, e.buf.Cursor(), e.pageSize()).to)
		return true
	case "pgup":
		e.moveTo(motionLineUp(e.buf, e.buf.Cursor(), e.pageSize()).to)
		return true
	case "ctrl+r":
		return e.redo(or1(v.count))
	}

	if r, ok := singleRune(key); ok {
		return e.handleNormalRune(r)
	}
	return false
}

// handleNormalRune обрабатывает однозначную клавишу normal-режима.
//
// Счётчик (count) забирается в ветках, где он использован, и сбрасывается:
// после обычной команды он не должен влиять на следующую. Операторы (d/y/c),
// ожидания (g, f, r) забирают счётчик сами при завершении.
func (e *Editor) handleNormalRune(r rune) bool {
	v := &e.vim
	count := 0
	if v.hasCount {
		count = v.count
	}

	switch r {
	// --- Навигация ---
	case 'h':
		n := or1(count)
		v.takeCount()
		return e.moveHorizontal(-n, true)
	case 'l':
		n := or1(count)
		v.takeCount()
		return e.moveHorizontal(n, true)
	case 'j':
		n := or1(count)
		v.takeCount()
		return e.moveVertical(n, true)
	case 'k':
		n := or1(count)
		v.takeCount()
		return e.moveVertical(-n, true)
	case 'w':
		n := or1(count)
		v.takeCount()
		e.moveTo(motionWordForward(e.buf, e.buf.Cursor(), false, n).to)
		return true
	case 'W':
		n := or1(count)
		v.takeCount()
		e.moveTo(motionWordForward(e.buf, e.buf.Cursor(), true, n).to)
		return true
	case 'b':
		n := or1(count)
		v.takeCount()
		e.moveTo(motionWordBackward(e.buf, e.buf.Cursor(), false, n).to)
		return true
	case 'B':
		n := or1(count)
		v.takeCount()
		e.moveTo(motionWordBackward(e.buf, e.buf.Cursor(), true, n).to)
		return true
	case 'e':
		n := or1(count)
		v.takeCount()
		e.moveTo(motionWordEnd(e.buf, e.buf.Cursor(), false, n).to)
		return true
	case 'E':
		n := or1(count)
		v.takeCount()
		e.moveTo(motionWordEnd(e.buf, e.buf.Cursor(), true, n).to)
		return true
	case '$':
		v.takeCount()
		e.moveTo(motionLineEnd(e.buf, e.buf.Cursor()).to)
		return true
	case '^':
		v.takeCount()
		e.moveTo(motionFirstNonBlank(e.buf, e.buf.Cursor()).to)
		return true
	case 'G':
		row := -1
		if v.hasCount {
			row = count - 1
		}
		v.takeCount()
		e.moveTo(motionLine(e.buf, e.buf.Cursor(), row).to)
		return true
	case '%':
		n := or1(count)
		v.takeCount()
		e.moveTo(motionPercent(e.buf, e.buf.Cursor(), n).to)
		return true
	case 'g':
		v.pend = pendG // счётчик сохраняется для {n}gg
		return true
	case 'f', 'F', 't', 'T':
		v.pend = pendFind
		v.pendChar = r // счётчик сохраняется для {n}f
		return true

	// --- Вход в режимы ---
	case 'i':
		v.takeCount()
		e.enterInsert()
		return true
	case 'a':
		v.takeCount()
		// a — вставка после курсора (в конце строки — без сдвига).
		if e.buf.Col() < e.buf.LineLen(e.buf.Row())-1 {
			e.buf.SetCursor(e.buf.Row(), e.buf.Col()+1)
		} else if e.buf.LineLen(e.buf.Row()) > 0 {
			e.buf.SetCursor(e.buf.Row(), e.buf.LineLen(e.buf.Row()))
		}
		e.enterInsert()
		return true
	case 'I':
		v.takeCount()
		e.moveTo(motionFirstNonBlank(e.buf, e.buf.Cursor()).to)
		e.enterInsert()
		return true
	case 'A':
		v.takeCount()
		e.buf.SetCursor(e.buf.Row(), e.buf.LineLen(e.buf.Row()))
		e.enterInsert()
		return true
	case 'o':
		v.takeCount()
		e.pushHistory()
		row := e.buf.Row()
		e.buf.InsertLinesAfter(row, [][]rune{{}})
		e.buf.SetCursor(row+1, 0)
		e.enterInsert()
		return true
	case 'O':
		v.takeCount()
		e.pushHistory()
		row := e.buf.Row()
		e.buf.InsertLinesAfter(row-1, [][]rune{{}})
		e.buf.SetCursor(row, 0)
		e.enterInsert()
		return true
	case 'v':
		if v.mode == ModeVisual {
			v.mode = ModeNormal
		} else {
			v.anchor = e.buf.Cursor()
			v.mode = ModeVisual
		}
		v.takeCount()
		return true
	case 'V':
		if v.mode == ModeVisualLine {
			v.mode = ModeNormal
		} else {
			v.anchor = Pos{Row: e.buf.Row(), Col: 0}
			v.mode = ModeVisualLine
		}
		v.takeCount()
		return true

	// --- Операторы и редактирование ---
	case 'd':
		return e.startOperator(opDelete)
	case 'y':
		return e.startOperator(opYank)
	case 'c':
		return e.startOperator(opChange)
	case 'x':
		n := or1(count)
		v.takeCount()
		return e.deleteCharsForward(n)
	case 'X':
		n := or1(count)
		v.takeCount()
		return e.deleteCharsBackward(n)
	case 'D':
		v.takeCount()
		return e.runOperatorMotion(opDelete, motionLineEnd(e.buf, e.buf.Cursor()), false)
	case 'C':
		v.takeCount()
		return e.runOperatorMotion(opChange, motionLineEnd(e.buf, e.buf.Cursor()), false)
	case 'Y':
		n := or1(count)
		v.takeCount()
		return e.yankLines(n)
	case 's':
		n := or1(count)
		v.takeCount()
		if e.deleteCharsForward(n) {
			e.enterInsert()
		}
		return true
	case 'S':
		n := or1(count)
		v.takeCount()
		return e.changeLines(n)
	case 'J':
		n := or1(count)
		v.takeCount()
		return e.joinLines(n)
	case 'r':
		v.pend = pendReplace // счётчик сохраняется для {n}r
		return true
	case '~':
		n := or1(count)
		v.takeCount()
		return e.swapCase(n)
	case 'p':
		v.takeCount()
		return e.paste(true)
	case 'P':
		v.takeCount()
		return e.paste(false)
	case 'u':
		n := or1(count)
		v.takeCount()
		return e.undo(n)
	}
	return false
}

// startOperator начинает ожидание мотиона после d/y/c (или выполняет dd/yy/cc).
func (e *Editor) startOperator(op opKind) bool {
	v := &e.vim
	if v.operator != opNone {
		// Повторный оператор: dd, yy, cc.
		n := v.effectiveCount()
		v.operator = opNone
		v.pend = pendNone
		v.opCount, v.hasOpCount = 0, false
		return e.repeatOperator(op, n)
	}
	v.operator = op
	v.pend = pendOperator
	v.opCount = v.takeCount()
	v.hasOpCount = v.opCount > 0
	return true
}

// repeatOperator выполняет линовую версию оператора (dd/yy/cc) с
// заранее вычисленным счётчиком.
func (e *Editor) repeatOperator(op opKind, n int) bool {
	switch op {
	case opDelete:
		return e.deleteLines(n)
	case opYank:
		return e.yankLines(n)
	case opChange:
		return e.changeLines(n)
	}
	return false
}

// handleMotionForOperator обрабатывает клавишу как мотион оператора.
func (e *Editor) handleMotionForOperator(key string) bool {
	v := &e.vim
	if v.operator == opNone {
		return false
	}
	op := v.operator

	switch key {
	case "esc", "ctrl+c":
		v.operator, v.pend = opNone, pendNone
		v.opCount, v.hasOpCount = 0, false
		return true
	case "up":
		return e.runOperatorMotion(op, motionLineUp(e.buf, e.buf.Cursor(), or1(v.count)), true)
	case "down":
		return e.runOperatorMotion(op, motionLineDown(e.buf, e.buf.Cursor(), or1(v.count)), true)
	case "left":
		return e.runOperatorMotion(op, motionLeft(e.buf, e.buf.Cursor()), false)
	case "right":
		return e.runOperatorMotion(op, motionRight(e.buf, e.buf.Cursor()), false)
	case "home":
		return e.runOperatorMotion(op, motionLineStart(e.buf, e.buf.Cursor()), false)
	case "end":
		return e.runOperatorMotion(op, motionLineEnd(e.buf, e.buf.Cursor()), false)
	case "enter":
		return e.runOperatorMotion(op, motionLineDown(e.buf, e.buf.Cursor(), or1(v.count)), true)
	}

	if r, ok := singleRune(key); ok {
		switch r {
		case 'h':
			return e.runOperatorMotion(op, motionLeft(e.buf, e.buf.Cursor()), false)
		case 'l':
			return e.runOperatorMotion(op, motionRight(e.buf, e.buf.Cursor()), false)
		case 'j':
			return e.runOperatorMotion(op, motionLineDown(e.buf, e.buf.Cursor(), or1(v.count)), true)
		case 'k':
			return e.runOperatorMotion(op, motionLineUp(e.buf, e.buf.Cursor(), or1(v.count)), true)
		case 'w':
			m := motionWordForward(e.buf, e.buf.Cursor(), false, v.effectiveCount())
			return e.runOperatorMotion(op, m, false)
		case 'W':
			m := motionWordForward(e.buf, e.buf.Cursor(), true, v.effectiveCount())
			return e.runOperatorMotion(op, m, false)
		case 'b':
			m := motionWordBackward(e.buf, e.buf.Cursor(), false, v.effectiveCount())
			return e.runOperatorMotion(op, m, false)
		case 'B':
			m := motionWordBackward(e.buf, e.buf.Cursor(), true, v.effectiveCount())
			return e.runOperatorMotion(op, m, false)
		case 'e':
			m := motionWordEnd(e.buf, e.buf.Cursor(), false, v.effectiveCount())
			return e.runOperatorMotion(op, m, false)
		case 'E':
			m := motionWordEnd(e.buf, e.buf.Cursor(), true, v.effectiveCount())
			return e.runOperatorMotion(op, m, false)
		case '0':
			return e.runOperatorMotion(op, motionLineStart(e.buf, e.buf.Cursor()), false)
		case '^':
			return e.runOperatorMotion(op, motionFirstNonBlank(e.buf, e.buf.Cursor()), false)
		case '$':
			return e.runOperatorMotion(op, motionLineEnd(e.buf, e.buf.Cursor()), false)
		case 'g':
			v.pend = pendG // ждём вторую g; gg выполнится в handleNormalKey
			return true
		case 'G':
			row := -1
			if v.hasCount {
				row = v.count - 1
				v.takeCount()
			}
			return e.runOperatorMotion(op, motionLine(e.buf, e.buf.Cursor(), row), true)
		case '%':
			m := motionPercent(e.buf, e.buf.Cursor(), or1(v.count))
			return e.runOperatorMotion(op, m, false)
		case 'i':
			v.pend = pendTextObj
			v.pendChar = 'i'
			return true
		case 'a':
			v.pend = pendTextObj
			v.pendChar = 'a'
			return true
		case 'f', 'F', 't', 'T':
			v.pend = pendFind
			v.pendChar = r
			return true
		case 'd', 'y', 'c':
			// Повтор оператора: dd, yy, cc. Счётчик забираем до сброса.
			if r == opChar(op) {
				n := v.effectiveCount()
				v.operator = opNone
				v.pend = pendNone
				v.opCount, v.hasOpCount = 0, false
				return e.repeatOperator(op, n)
			}
		}
	}
	// Неизвестный мотион — отмена оператора.
	v.operator, v.pend = opNone, pendNone
	v.opCount, v.hasOpCount = 0, false
	return false
}

// ---- Визуальный режим ----

// handleVisualKey обрабатывает клавишу в visual/visual-line режиме.
func (e *Editor) handleVisualKey(key string) bool {
	v := &e.vim

	// Ожидающие состояния (gg, текстовый объект) — первыми.
	if v.pend == pendG {
		v.pend = pendNone
		if key == "g" {
			v.takeCount()
			e.moveTo(Pos{Row: 0, Col: 0})
			return true
		}
	}
	if v.pend == pendTextObj {
		v.pend = pendNone
		if r, ok := singleRune(key); ok {
			if m, okObj := e.textObjectMotion(v.pendChar, r); okObj {
				e.selectMotion(m)
			}
		}
		return true
	}

	switch key {
	case "esc", "ctrl+c":
		v.mode = ModeNormal
		v.count, v.hasCount = 0, false
		v.desiredCol = -1
		return true
	case "v":
		if v.mode == ModeVisualLine {
			v.mode = ModeVisual
		} else {
			v.mode = ModeNormal
		}
		return true
	case "V":
		if v.mode == ModeVisual {
			v.mode = ModeVisualLine
			v.anchor = Pos{Row: e.buf.Row(), Col: 0}
		} else {
			v.mode = ModeNormal
		}
		return true
	case "up":
		return e.moveVertical(-1, false)
	case "down":
		return e.moveVertical(1, false)
	case "left":
		return e.moveHorizontal(-1, false)
	case "right":
		return e.moveHorizontal(1, false)
	}

	// Цифры — счётчик.
	if r, ok := singleRune(key); ok && r >= '0' && r <= '9' {
		if r == '0' && !v.hasCount {
			e.moveTo(motionLineStart(e.buf, e.buf.Cursor()).to)
			return true
		}
		v.count = v.count*10 + int(r-'0')
		v.hasCount = true
		return true
	}

	if r, ok := singleRune(key); ok {
		switch r {
		// Операторы применяются к выделению.
		case 'd', 'x':
			e.applyVisual(opDelete)
			return true
		case 'y':
			e.applyVisual(opYank)
			return true
		case 'c', 's':
			e.applyVisual(opChange)
			return true
		case 'J':
			e.joinVisual()
			return true

		// Текстовые объекты в visual: vi", va( и т.п.
		case 'i', 'a':
			v.pend = pendTextObj
			v.pendChar = r
			return true

		// Навигация.
		case 'h':
			return e.moveHorizontal(-1, false)
		case 'l':
			return e.moveHorizontal(1, false)
		case 'j':
			return e.moveVertical(1, false)
		case 'k':
			return e.moveVertical(-1, false)
		case 'w':
			e.moveTo(motionWordForward(e.buf, e.buf.Cursor(), false, or1(v.count)).to)
			v.takeCount()
			return true
		case 'W':
			e.moveTo(motionWordForward(e.buf, e.buf.Cursor(), true, or1(v.count)).to)
			v.takeCount()
			return true
		case 'b':
			e.moveTo(motionWordBackward(e.buf, e.buf.Cursor(), false, or1(v.count)).to)
			v.takeCount()
			return true
		case 'B':
			e.moveTo(motionWordBackward(e.buf, e.buf.Cursor(), true, or1(v.count)).to)
			v.takeCount()
			return true
		case 'e':
			e.moveTo(motionWordEnd(e.buf, e.buf.Cursor(), false, or1(v.count)).to)
			v.takeCount()
			return true
		case '0':
			e.moveTo(motionLineStart(e.buf, e.buf.Cursor()).to)
			return true
		case '$':
			e.moveTo(motionLineEnd(e.buf, e.buf.Cursor()).to)
			return true
		case 'G':
			row := -1
			if v.hasCount {
				row = v.count - 1
			}
			v.takeCount()
			e.moveTo(motionLine(e.buf, e.buf.Cursor(), row).to)
			return true
		case 'g':
			v.pend = pendG
			return true
		}
	}
	return false
}

// ---- Вспомогательные ----

// singleRune возвращает руну, если клавиша состоит ровно из одной руны.
func singleRune(key string) (rune, bool) {
	rs := []rune(key)
	if len(rs) != 1 {
		return 0, false
	}
	return rs[0], true
}

// or1 заменяет 0 на 1 (счётчик не вводился).
func or1(n int) int {
	if n <= 0 {
		return 1
	}
	return n
}

// motionPercent — {n}%: позиция на n% документа.
func motionPercent(b *Buffer, p Pos, percent int) motion {
	if percent <= 0 {
		percent = 50
	}
	if percent > 100 {
		percent = 100
	}
	row := (b.LinesCount() - 1) * percent / 100
	return motionLine(b, p, row)
}
