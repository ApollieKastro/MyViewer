package editor

import "unicode"

// motion.go — вычисление конечных позиций для навигации vim.
//
// Все функции чистые: принимают буфер и стартовую позицию, возвращают
// целевую позицию. Никакого состояния — это делает логику тестируемой.

// Результат вычисления мотиона для оператора (d/y/c).
type motion struct {
	to        Pos  // конечная позиция
	from      Pos  // начало диапазона (только для текстовых объектов)
	hasFrom   bool // from заполнен — диапазон задан явно, а не от курсора
	inclusive bool // включать символ под to в диапазон (e, $, f, ...)
	linewise  bool // диапазон — целые строки (j, k, gg, G)
	ok        bool // мотион применим
}

// Классы символов (как в vim: пробельные, буквенно-цифровые, знаки).
const (
	classSpace = iota
	classWord
	classPunct
)

// classOf возвращает класс руны для привязки к словам.
// Классы как в vim: пробельные, буквенно-цифровые (включая кириллицу),
// знаки препинания.
func classOf(r rune) int {
	switch {
	case unicode.IsSpace(r):
		return classSpace
	case r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
		return classWord
	default:
		return classPunct
	}
}

// bigClassOf — класс для WORD-мотионов (W/B/E): разделитель только пробел.
func bigClassOf(r rune) int {
	if unicode.IsSpace(r) {
		return classSpace
	}
	return classWord
}

// ---- Базовые перемещения по позиции ----

// atDocStart / atDocEnd — границы документа.
func atDocStart(p Pos) bool { return p.Row <= 0 && p.Col <= 0 }

// stepRight — вправо на один символ (не выходит за конец документа).
func stepRight(b *Buffer, p Pos) Pos {
	if p.Row >= b.LinesCount()-1 && p.Col >= b.LineLen(p.Row)-1 {
		return p
	}
	if p.Col < b.LineLen(p.Row)-1 {
		p.Col++
		return p
	}
	// конец строки → начало следующей
	if p.Row < b.LinesCount()-1 {
		p.Row++
		p.Col = 0
	}
	return p
}

// stepLeft — влево на один символ (не выходит за начало документа).
func stepLeft(b *Buffer, p Pos) Pos {
	if p.Col > 0 {
		p.Col--
		return p
	}
	if p.Row > 0 {
		p.Row--
		p.Col = b.LineLen(p.Row) - 1
		if p.Col < 0 {
			p.Col = 0
		}
	}
	return p
}

// peek — руна под позицией (0 если позиция пуста).
func peek(b *Buffer, p Pos) rune {
	line := b.Line(p.Row)
	if p.Col < 0 || p.Col >= len(line) {
		return 0
	}
	return line[p.Col]
}

// lastPos — позиция последнего символа документа.
func lastPos(b *Buffer) Pos {
	row := b.LinesCount() - 1
	col := b.LineLen(row) - 1
	if col < 0 {
		col = 0
	}
	return Pos{Row: row, Col: col}
}

// ---- Мотионы ----

// motionLeft — h.
func motionLeft(b *Buffer, p Pos) motion {
	return motion{to: stepLeft(b, p), ok: true}
}

// motionRight — l.
func motionRight(b *Buffer, p Pos) motion {
	return motion{to: stepRight(b, p), ok: true}
}

// motionLineDown — j (диапазон линовый).
func motionLineDown(b *Buffer, p Pos, count int) motion {
	to := p
	for i := 0; i < count; i++ {
		if to.Row >= b.LinesCount()-1 {
			break
		}
		to.Row++
	}
	to.Col = clampColTo(b, to.Row, p.Col)
	return motion{to: to, linewise: true, ok: true}
}

// motionLineUp — k (диапазон линовый).
func motionLineUp(b *Buffer, p Pos, count int) motion {
	to := p
	for i := 0; i < count; i++ {
		if to.Row <= 0 {
			break
		}
		to.Row--
	}
	to.Col = clampColTo(b, to.Row, p.Col)
	return motion{to: to, linewise: true, ok: true}
}

// clampColTo зажимает колонку в длину строки row.
func clampColTo(b *Buffer, row, col int) int {
	if n := b.LineLen(row) - 1; col > n {
		if n < 0 {
			return 0
		}
		return n
	}
	if col < 0 {
		return 0
	}
	return col
}

// motionWordForward — w / W: начало следующего слова.
func motionWordForward(b *Buffer, p Pos, big bool, count int) motion {
	res := p
	for i := 0; i < count; i++ {
		next := nextWordStart(b, res, big)
		if next == res {
			break
		}
		res = next
	}
	return motion{to: res, ok: true}
}

// nextWordStart — позиция начала следующего слова (алгоритм vim).
func nextWordStart(b *Buffer, p Pos, big bool) Pos {
	last := lastPos(b)
	if p == last {
		return p
	}
	cls := func(r rune) int {
		if big {
			return bigClassOf(r)
		}
		return classOf(r)
	}
	cur := cls(peek(b, p))
	p = stepRight(b, p)
	if cur != classSpace {
		// Пропускаем остаток текущего класса.
		for p != last && cls(peek(b, p)) == cur && cls(peek(b, p)) != classSpace {
			p = stepRight(b, p)
		}
		if cls(peek(b, p)) == cur {
			return p // застряли в конце (run занял хвост документа)
		}
	}
	// Пропускаем пробелы до начала следующего слова.
	for p != last && cls(peek(b, p)) == classSpace {
		p = stepRight(b, p)
	}
	if p == last && cls(peek(b, p)) == classSpace {
		return p
	}
	return p
}

// motionWordBackward — b / B: начало предыдущего слова.
func motionWordBackward(b *Buffer, p Pos, big bool, count int) motion {
	res := p
	for i := 0; i < count; i++ {
		prev := prevWordStart(b, res, big)
		if prev == res {
			break
		}
		res = prev
	}
	return motion{to: res, ok: true}
}

// prevWordStart — позиция начала предыдущего слова.
func prevWordStart(b *Buffer, p Pos, big bool) Pos {
	if atDocStart(p) {
		return p
	}
	cls := func(r rune) int {
		if big {
			return bigClassOf(r)
		}
		return classOf(r)
	}
	// Один шаг назад — на символ слева.
	q := stepLeft(b, p)
	if q == p {
		return p
	}
	// Пропускаем пробелы назад.
	for !atDocStart(q) && cls(peek(b, q)) == classSpace {
		nq := stepLeft(b, q)
		if nq == q {
			break
		}
		q = nq
	}
	if cls(peek(b, q)) == classSpace {
		return q // дошли до начала документа на пробелах
	}
	c := cls(peek(b, q))
	// Пропускаем символы класса назад — встанем на начало слова.
	for !atDocStart(q) {
		nq := stepLeft(b, q)
		if nq == q {
			break
		}
		if cls(peek(b, nq)) != c || cls(peek(b, nq)) == classSpace {
			break
		}
		q = nq
	}
	return q
}

// motionWordEnd — e / E: конец текущего или следующего слова.
func motionWordEnd(b *Buffer, p Pos, big bool, count int) motion {
	res := p
	for i := 0; i < count; i++ {
		next := nextWordEnd(b, res, big)
		if next == res {
			break
		}
		res = next
	}
	return motion{to: res, inclusive: true, ok: true}
}

// nextWordEnd — конец слова (последний символ run).
func nextWordEnd(b *Buffer, p Pos, big bool) Pos {
	last := lastPos(b)
	cls := func(r rune) int {
		if big {
			return bigClassOf(r)
		}
		return classOf(r)
	}
	q := stepRight(b, p)
	// Пропускаем пробелы.
	for q != last && cls(peek(b, q)) == classSpace {
		q = stepRight(b, q)
	}
	if cls(peek(b, q)) == classSpace {
		return last // хвост — пробелы до конца
	}
	c := cls(peek(b, q))
	// Идём до конца run: останавливаемся на последнем символе класса.
	for q != last && cls(peek(b, stepRight(b, q))) == c {
		q = stepRight(b, q)
	}
	return q
}

// motionLineStart — 0: начало строки.
func motionLineStart(b *Buffer, p Pos) motion {
	return motion{to: Pos{Row: p.Row, Col: 0}, ok: true}
}

// motionFirstNonBlank — ^: первый непробельный символ строки.
func motionFirstNonBlank(b *Buffer, p Pos) motion {
	line := b.Line(p.Row)
	col := 0
	for col < len(line) && (line[col] == ' ' || line[col] == '\t') {
		col++
	}
	if col >= len(line) && col > 0 {
		col = len(line) - 1
	}
	return motion{to: Pos{Row: p.Row, Col: col}, ok: true}
}

// motionLineEnd — $: конец строки (не включая перевод строки).
func motionLineEnd(b *Buffer, p Pos) motion {
	col := b.LineLen(p.Row) - 1
	if col < 0 {
		col = 0
	}
	return motion{to: Pos{Row: p.Row, Col: col}, inclusive: true, ok: true}
}

// motionLine — переход к строке row (0-based) для g g / G / {n}G / {n}%.
// Отрицательный row означает последнюю строку документа.
func motionLine(b *Buffer, p Pos, row int) motion {
	if row < 0 {
		row = b.LinesCount() - 1
	}
	if row >= b.LinesCount() {
		row = b.LinesCount() - 1
	}
	if row < 0 {
		row = 0
	}
	col := 0
	// Встанем на первый непробельный (как vim при G).
	line := b.Line(row)
	for col < len(line) && (line[col] == ' ' || line[col] == '\t') {
		col++
	}
	if col >= len(line) && len(line) > 0 {
		col = len(line) - 1
	}
	return motion{to: Pos{Row: row, Col: col}, linewise: true, ok: true}
}

// ---- Текстовые объекты (iw, aw, i", a(, ...) ----

// objectRange вычисляет границы текстового объекта под курсором.
// inner — внутренняя часть (i) или с учётом обрамления (a).
// Если символ-обрамляющий не найден в строке, возвращается ok=false.
func objectRange(b *Buffer, p Pos, target rune, inner bool) motion {
	line := b.Line(p.Row)
	if len(line) == 0 {
		return motion{ok: false}
	}
	col := clampNormalCol(line, p.Col)

	// Кавычки и обратные кавычки — ищем пару в строке.
	if target == '"' || target == '\'' || target == '`' {
		return quoteObject(line, p.Row, col, target, inner)
	}
	// Скобки — ищем пару с балансом вложенности.
	if target == '(' || target == ')' {
		return bracketObject(line, p.Row, col, '(', ')', inner)
	}
	if target == '[' || target == ']' {
		return bracketObject(line, p.Row, col, '[', ']', inner)
	}
	if target == '{' || target == '}' {
		return bracketObject(line, p.Row, col, '{', '}', inner)
	}
	if target == '<' || target == '>' {
		return bracketObject(line, p.Row, col, '<', '>', inner)
	}
	// Слово.
	return wordObject(line, p.Row, col, inner)
}

// clampNormalCol зажимает колонку в границы строки.
func clampNormalCol(line []rune, col int) int {
	if col < 0 {
		return 0
	}
	if col > len(line)-1 {
		if len(line) == 0 {
			return 0
		}
		return len(line) - 1
	}
	return col
}

// wordObject — iw/aw: слово под курсором (или пробелы после него для aw).
func wordObject(line []rune, row, col int, inner bool) motion {
	if len(line) == 0 {
		return motion{ok: false}
	}
	col = clampNormalCol(line, col)

	// Если курсор на пробеле: для aw — пробелы плюс следующее слово,
	// для iw — только соседние пробелы.
	if classOf(line[col]) == classSpace {
		start := col
		for start > 0 && classOf(line[start-1]) == classSpace {
			start--
		}
		end := col
		for end < len(line) && classOf(line[end]) == classSpace {
			end++
		}
		if !inner {
			// aw: захватываем следующее слово после пробелов.
			if end < len(line) && classOf(line[end]) != classSpace {
				c := classOf(line[end])
				for end < len(line) && classOf(line[end]) == c {
					end++
				}
			}
		}
		if start >= end {
			return motion{ok: false}
		}
		return motion{
			from: Pos{Row: row, Col: start}, to: Pos{Row: row, Col: end - 1},
			hasFrom: true, inclusive: true, ok: true,
		}
	}

	c := classOf(line[col])
	start := col
	for start > 0 && classOf(line[start-1]) == c {
		start--
	}
	end := col + 1
	for end < len(line) && classOf(line[end]) == c {
		end++
	}
	if inner {
		return motion{
			from: Pos{Row: row, Col: start}, to: Pos{Row: row, Col: end - 1},
			hasFrom: true, inclusive: true, ok: true,
		}
	}
	// aw: слово + пробелы после него (не захватываем пустой хвост строки).
	awEnd := end
	for awEnd < len(line) && classOf(line[awEnd]) == classSpace {
		awEnd++
	}
	if awEnd > end {
		return motion{
			from: Pos{Row: row, Col: start}, to: Pos{Row: row, Col: awEnd - 1},
			hasFrom: true, inclusive: true, ok: true,
		}
	}
	return motion{
		from: Pos{Row: row, Col: start}, to: Pos{Row: row, Col: end - 1},
		hasFrom: true, inclusive: true, ok: true,
	}
}

// quoteObject — i"/a" и аналоги: содержимое между парой одинаковых кавычек.
//
// Алгоритм: собираем позиции всех кавычек данного типа в строке и выбираем
// пару, охватывающую курсор (открывающая — чётная по счёту, закрывающая —
// нечётная). Незакрытая пара считается ошибкой.
func quoteObject(line []rune, row, col int, q rune, inner bool) motion {
	if len(line) == 0 {
		return motion{ok: false}
	}
	col = clampNormalCol(line, col)

	// Позиции всех кавычек.
	var pos []int
	for i, r := range line {
		if r == q {
			pos = append(pos, i)
		}
	}
	if len(pos) < 2 {
		return motion{ok: false}
	}
	// Ищем пару: последнюю кавычку слева от курсора (включая позицию курсора).
	idx := -1
	for i, p := range pos {
		if p <= col {
			idx = i
		}
	}
	if idx < 0 {
		return motion{ok: false} // курсор левее всех кавычек
	}
	var open, close int
	if idx%2 == 0 {
		// Курсор на открывающей (или внутри) — пара следующая.
		if idx+1 >= len(pos) {
			return motion{ok: false} // незакрытая пара
		}
		open, close = pos[idx], pos[idx+1]
	} else {
		// Курсор на закрывающей — пара предыдущая.
		open, close = pos[idx-1], pos[idx]
	}
	if inner {
		if close-1 < open+1 {
			return motion{ok: false} // пустые кавычки: i" ничего не берёт
		}
		return motion{
			from: Pos{Row: row, Col: open + 1}, to: Pos{Row: row, Col: close - 1},
			hasFrom: true, inclusive: true, ok: true,
		}
	}
	return motion{
		from: Pos{Row: row, Col: open}, to: Pos{Row: row, Col: close},
		hasFrom: true, inclusive: true, ok: true,
	}
}

// bracketObject — скобочные объекты с балансом вложенности ((...), [...], {...}).
//
// Сначала ищем открывающую скобку слева от курсора (с балансом), затем от
// неё — парную закрывающую вперёд. Если курсор стоит на закрывающей, поиск
// начинается от неё.
func bracketObject(line []rune, row, col int, open, close rune, inner bool) motion {
	if len(line) == 0 {
		return motion{ok: false}
	}
	col = clampNormalCol(line, col)

	// Нормализуем пару: open — всегда открывающая.
	if open == ')' || open == ']' || open == '}' || open == '>' {
		open, close = close, open
	}

	// Ищем открывающую скобку слева (включая позицию курсора):
	// идём назад, считая скобки; глубина 0 на открывающей — наша пара.
	openIdx := -1
	depth := 0
	if line[col] == open {
		openIdx = col
	} else {
		for i := col; i >= 0; i-- {
			switch line[i] {
			case close:
				depth++
			case open:
				depth--
				if depth <= 0 {
					openIdx = i
					i = -1 // выходим из цикла
				}
			}
		}
	}
	if openIdx < 0 {
		return motion{ok: false} // курсор не внутри пары
	}

	// Ищем парную закрывающую вперёд от открывающей.
	depth = 0
	for i := openIdx; i < len(line); i++ {
		switch line[i] {
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				if inner {
					if i-1 < openIdx+1 {
						return motion{ok: false} // ()
					}
					return motion{
						from: Pos{Row: row, Col: openIdx + 1}, to: Pos{Row: row, Col: i - 1},
						hasFrom: true, inclusive: true, ok: true,
					}
				}
				return motion{
					from: Pos{Row: row, Col: openIdx}, to: Pos{Row: row, Col: i},
					hasFrom: true, inclusive: true, ok: true,
				}
			}
		}
	}
	return motion{ok: false} // незакрытая пара
}
