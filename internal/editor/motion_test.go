package editor

import "testing"

// ---- Мотионы движения по словам ----

func TestWordForward(t *testing.T) {
	b := NewBuffer("foo bar baz")
	// Ожидаемые позиции начала слов: 0, 4, 8.
	want := []int{4, 8, 10}
	p := Pos{}
	for _, w := range want {
		m := motionWordForward(b, p, false, 1)
		if m.to.Col != w {
			t.Errorf("w from %v = %d, want %d", p, m.to.Col, w)
		}
		p = m.to
	}
}

func TestWordForwardWithPunctuation(t *testing.T) {
	b := NewBuffer("foo, bar")
	// От начала: foo (0), затем ',' (3), затем bar (5).
	p := Pos{}
	m := motionWordForward(b, p, false, 1)
	if m.to.Col != 3 {
		t.Fatalf("w: expected punctuation at 3, got %d", m.to.Col)
	}
	m = motionWordForward(b, m.to, false, 1)
	if m.to.Col != 5 {
		t.Fatalf("w after punct: expected 5, got %d", m.to.Col)
	}
}

func TestWordForwardCyrillic(t *testing.T) {
	b := NewBuffer("привет мир")
	m := motionWordForward(b, Pos{}, false, 1)
	if m.to.Col != 7 {
		t.Fatalf("w (cyrillic): got %d, want 7", m.to.Col)
	}
}

func TestWordBackward(t *testing.T) {
	b := NewBuffer("foo bar baz")
	// От позиции 8 (baz) назад: 4 (bar), затем 0 (foo).
	m := motionWordBackward(b, Pos{Col: 8}, false, 1)
	if m.to.Col != 4 {
		t.Fatalf("b: got %d, want 4", m.to.Col)
	}
	m = motionWordBackward(b, m.to, false, 1)
	if m.to.Col != 0 {
		t.Fatalf("b again: got %d, want 0", m.to.Col)
	}
}

func TestWordEnd(t *testing.T) {
	b := NewBuffer("foo bar")
	// e с начала: конец foo = 2.
	m := motionWordEnd(b, Pos{}, false, 1)
	if m.to.Col != 2 || !m.inclusive {
		t.Fatalf("e: col=%d inclusive=%v, want 2/true", m.to.Col, m.inclusive)
	}
	// Второй e: конец bar = 6.
	m = motionWordEnd(b, m.to, false, 1)
	if m.to.Col != 6 {
		t.Fatalf("e again: got %d, want 6", m.to.Col)
	}
}

func TestLineEndIsInclusive(t *testing.T) {
	b := NewBuffer("abc")
	m := motionLineEnd(b, Pos{})
	if m.to.Col != 2 || !m.inclusive {
		t.Fatalf("$: col=%d inclusive=%v, want 2/true", m.to.Col, m.inclusive)
	}
}

// ---- Текстовые объекты ----

func TestInnerWord(t *testing.T) {
	b := NewBuffer("say hello now")
	// Курсор внутри hello (pos 6).
	m := objectRange(b, Pos{Col: 6}, 'w', true)
	if !m.ok {
		t.Fatal("iw should be ok")
	}
	if m.from.Col != 4 || m.to.Col != 8 {
		t.Fatalf("iw range = [%d,%d], want [4,8]", m.from.Col, m.to.Col)
	}
	got := b.GetRange(m.from, advance(m.to))
	if got != "hello" {
		t.Fatalf("iw text = %q, want %q", got, "hello")
	}
}

func TestInnerQuote(t *testing.T) {
	b := NewBuffer(`say "hello" now`)
	// Курсор внутри кавычек (pos 6 — 'e').
	m := objectRange(b, Pos{Col: 6}, '"', true)
	if !m.ok {
		t.Fatal("i\" should be ok")
	}
	got := b.GetRange(m.from, advance(m.to))
	if got != "hello" {
		t.Fatalf("i\" text = %q, want %q", got, "hello")
	}
	// a" включает кавычки.
	m = objectRange(b, Pos{Col: 6}, '"', false)
	got = b.GetRange(m.from, advance(m.to))
	if got != `"hello"` {
		t.Fatalf("a\" text = %q, want %q", got, `"hello"`)
	}
}

func TestInnerParen(t *testing.T) {
	b := NewBuffer("call(a, b)")
	// Курсор на 'a' (pos 5).
	m := objectRange(b, Pos{Col: 5}, '(', true)
	if !m.ok {
		t.Fatal("i( should be ok")
	}
	got := b.GetRange(m.from, advance(m.to))
	if got != "a, b" {
		t.Fatalf("i( text = %q, want %q", got, "a, b")
	}
	// Курсор на закрывающей скобке (pos 9).
	m = objectRange(b, Pos{Col: 9}, '(', true)
	got = b.GetRange(m.from, advance(m.to))
	if got != "a, b" {
		t.Fatalf("i( from close: %q, want %q", got, "a, b")
	}
}

func TestNestedParens(t *testing.T) {
	b := NewBuffer("f(a(b)c)")
	// Курсор на 'b' (pos 4) — внутренняя пара.
	m := objectRange(b, Pos{Col: 4}, '(', true)
	got := b.GetRange(m.from, advance(m.to))
	if got != "b" {
		t.Fatalf("nested i( = %q, want %q", got, "b")
	}
	// Курсор на 'a' (pos 2) — внешняя пара.
	m = objectRange(b, Pos{Col: 2}, '(', true)
	got = b.GetRange(m.from, advance(m.to))
	if got != "a(b)c" {
		t.Fatalf("outer i( = %q, want %q", got, "a(b)c")
	}
}

// advance возвращает позицию следующей руны (конец диапазона включительно).
func advance(p Pos) Pos {
	return Pos{Row: p.Row, Col: p.Col + 1}
}
