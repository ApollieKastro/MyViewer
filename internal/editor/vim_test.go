package editor

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// press симулирует последовательность нажатий клавиш.
func press(e *Editor, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		case "delete":
			msg = tea.KeyMsg{Type: tea.KeyDelete}
		case "space":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		case "ctrl+r":
			msg = tea.KeyMsg{Type: tea.KeyCtrlR}
		case "ctrl+s":
			msg = tea.KeyMsg{Type: tea.KeyCtrlS}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		e.Update(msg)
	}
}

// newTestEditor создаёт редактор с текстом и курсором в начале.
func newTestEditor(text string) *Editor {
	e := New()
	e.Load(text)
	e.SetSize(80, 24)
	return e
}

func assertText(t *testing.T, e *Editor, want string) {
	t.Helper()
	if got := e.Content(); got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func assertCursor(t *testing.T, e *Editor, row, col int) {
	t.Helper()
	r, c := e.Cursor()
	if r != row || c != col {
		t.Fatalf("cursor = (%d,%d), want (%d,%d)", r, c, row, col)
	}
}

// ---- Удаление строк ----

func TestDeleteLine(t *testing.T) {
	e := newTestEditor("one\ntwo\nthree")
	press(e, "d", "d")
	assertText(t, e, "two\nthree")
	assertCursor(t, e, 0, 0)
}

func TestDeleteLineWithCount(t *testing.T) {
	e := newTestEditor("one\ntwo\nthree\nfour")
	press(e, "2", "d", "d")
	assertText(t, e, "three\nfour")
}

func TestDeleteLineUndo(t *testing.T) {
	e := newTestEditor("one\ntwo")
	press(e, "d", "d")
	assertText(t, e, "two")
	press(e, "u")
	assertText(t, e, "one\ntwo")
	press(e, "ctrl+r")
	assertText(t, e, "two")
}

// ---- Операторы с мотионами ----

func TestDeleteWord(t *testing.T) {
	e := newTestEditor("foo bar")
	press(e, "d", "w")
	assertText(t, e, "bar")
	assertCursor(t, e, 0, 0)
}

func TestDeleteToEndOfLine(t *testing.T) {
	e := newTestEditor("abcdef")
	press(e, "l", "l", "D")
	assertText(t, e, "ab")
}

func TestDeleteInnerWord(t *testing.T) {
	e := newTestEditor("say hello now")
	// Курсор внутрь hello.
	press(e, "w")
	if _, c := e.Cursor(); c != 4 {
		t.Fatalf("w positioned at %d, want 4", c)
	}
	press(e, "l") // 'e' в hello
	press(e, "d", "i", "w")
	assertText(t, e, "say  now")
}

func TestChangeWordEntersInsert(t *testing.T) {
	e := newTestEditor("foo bar")
	press(e, "c", "w")
	if e.Mode() != ModeInsert {
		t.Fatalf("mode = %v, want INSERT", e.Mode())
	}
	press(e, "X")
	assertText(t, e, "Xbar")
	press(e, "esc")
	if e.Mode() != ModeNormal {
		t.Fatalf("mode after esc = %v, want NORMAL", e.Mode())
	}
}

func TestDeleteLinesYankPaste(t *testing.T) {
	e := newTestEditor("one\ntwo\nthree")
	press(e, "y", "y")
	press(e, "j", "p")
	assertText(t, e, "one\ntwo\none\nthree")
}

// ---- Одиночные операции ----

func TestXKey(t *testing.T) {
	e := newTestEditor("abc")
	press(e, "x")
	assertText(t, e, "bc")
	press(e, "2", "x")
	assertText(t, e, "")
}

func TestReplaceChar(t *testing.T) {
	e := newTestEditor("abc")
	press(e, "r", "Z")
	assertText(t, e, "Zbc")
	assertCursor(t, e, 0, 0)
}

func TestSwapCase(t *testing.T) {
	e := newTestEditor("abc")
	press(e, "~")
	assertText(t, e, "Abc")
}

func TestJoinLines(t *testing.T) {
	e := newTestEditor("one  \ntwo")
	press(e, "J")
	assertText(t, e, "one two")
}

func TestOpenLineBelow(t *testing.T) {
	e := newTestEditor("one")
	press(e, "o")
	if e.Mode() != ModeInsert {
		t.Fatalf("mode = %v, want INSERT", e.Mode())
	}
	press(e, "t", "w", "o")
	press(e, "esc")
	assertText(t, e, "one\ntwo")
	// Esc двигает курсор влево: с col=3 на col=2 (последний символ).
	assertCursor(t, e, 1, 2)
}

// ---- Вставка ----

func TestInsertAtStart(t *testing.T) {
	e := newTestEditor("abc")
	press(e, "i", "X", "Y")
	assertText(t, e, "XYabc")
	press(e, "esc")
	// Курсор в normal — на последнем вставленном символе (esc двигает влево).
	assertCursor(t, e, 0, 1)
}

func TestInsertAppendEnd(t *testing.T) {
	e := newTestEditor("abc")
	press(e, "A", "!")
	assertText(t, e, "abc!")
	press(e, "esc")
	assertCursor(t, e, 0, 3)
}

func TestInsertNewlineBackspace(t *testing.T) {
	e := newTestEditor("ab")
	press(e, "A", "enter")
	assertText(t, e, "ab\n")
	// Backspace в начале второй строки объединяет строки (как в vim).
	press(e, "backspace")
	assertText(t, e, "ab")
}

// ---- Визуальный режим ----

func TestVisualDelete(t *testing.T) {
	e := newTestEditor("abcdef")
	press(e, "v", "l", "l", "d")
	assertText(t, e, "def")
}

func TestVisualLineDelete(t *testing.T) {
	e := newTestEditor("one\ntwo\nthree")
	press(e, "j", "V", "d") // курсор на "two", выделить строку, удалить
	assertText(t, e, "one\nthree")
}

func TestVisualYankPaste(t *testing.T) {
	e := newTestEditor("abc\ntwo")
	press(e, "V", "y") // выделить строку и скопировать
	press(e, "G", "p") // в конец документа
	assertText(t, e, "abc\ntwo\nabc")
}

// ---- Навигация ----

func TestCountMotion(t *testing.T) {
	e := newTestEditor("1\n2\n3\n4\n5")
	press(e, "3", "j")
	assertCursor(t, e, 3, 0)
	press(e, "k", "k")
	assertCursor(t, e, 1, 0)
}

func TestGoToLine(t *testing.T) {
	e := newTestEditor("a\nb\nc\nd")
	press(e, "G")
	assertCursor(t, e, 3, 0)
	press(e, "g", "g")
	assertCursor(t, e, 0, 0)
}

func TestFindChar(t *testing.T) {
	e := newTestEditor("a=bcdef")
	press(e, "f", "=")
	assertCursor(t, e, 0, 1)
	// t ставит курсор перед целевым символом: 'e' на позиции 5 → курсор на 4.
	press(e, "t", "e")
	assertCursor(t, e, 0, 4)
}

// ---- Esc и выход ----

func TestEscInNormalRequestsExit(t *testing.T) {
	e := newTestEditor("text")
	press(e, "esc")
	if !e.ConsumeExit() {
		t.Fatal("esc in normal mode should request exit")
	}
	// Повторный вызов — флаг сброшен.
	if e.ConsumeExit() {
		t.Fatal("exit flag should be consumed")
	}
}

func TestEscInInsertGoesToNormal(t *testing.T) {
	e := newTestEditor("text")
	press(e, "i")
	press(e, "esc")
	if e.ConsumeExit() {
		t.Fatal("esc in insert must not exit editor")
	}
	if e.Mode() != ModeNormal {
		t.Fatalf("mode = %v, want NORMAL", e.Mode())
	}
}

// ---- Удаление слова через visual + объект ----

func TestVisualInnerWord(t *testing.T) {
	e := newTestEditor("say hello now")
	press(e, "w", "l") // внутри hello
	press(e, "v", "i", "w", "d")
	assertText(t, e, "say  now")
}
