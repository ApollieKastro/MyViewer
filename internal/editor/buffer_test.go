package editor

import "testing"

// ---- Базовые операции буфера ----

func TestSplitLinesVariants(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{""}},
		{"one", []string{"one"}},
		{"one\ntwo", []string{"one", "two"}},
		// Завершающий \n создаёт пустую последнюю строку — как в vim.
		{"one\ntwo\n", []string{"one", "two", ""}},
		{"one\r\ntwo", []string{"one", "two"}},
		{"привет\nмир", []string{"привет", "мир"}},
	}
	for _, c := range cases {
		b := NewBuffer(c.in)
		got := b.Lines()
		if len(got) != len(c.want) {
			t.Fatalf("SplitLines(%q) = %v, want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("SplitLines(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestInsertAndNewline(t *testing.T) {
	b := NewBuffer("hello")
	b.SetCursor(0, 5)
	b.InsertRune('!')
	if got := b.String(); got != "hello!" {
		t.Fatalf("InsertRune: got %q", got)
	}
	b.Newline()
	if got := b.String(); got != "hello!\n" {
		t.Fatalf("Newline: got %q", got)
	}
	if b.Row() != 1 || b.Col() != 0 {
		t.Fatalf("cursor after newline = (%d,%d), want (1,0)", b.Row(), b.Col())
	}
}

func TestBackspaceJoinsLines(t *testing.T) {
	b := NewBuffer("abc\ndef")
	b.SetCursor(1, 0)
	b.Backspace()
	if got := b.String(); got != "abcdef" {
		t.Fatalf("Backspace join: got %q", got)
	}
	if b.Row() != 0 || b.Col() != 3 {
		t.Fatalf("cursor = (%d,%d), want (0,3)", b.Row(), b.Col())
	}
}

func TestMultibyteCursor(t *testing.T) {
	b := NewBuffer("привет")
	b.SetCursor(0, 0)
	b.InsertRune('я')
	if got := b.String(); got != "япривет" {
		t.Fatalf("multibyte insert: got %q", got)
	}
	// Курсор должен указывать на руну, а не на байт.
	if b.Col() != 1 {
		t.Fatalf("col = %d, want 1 (rune index)", b.Col())
	}
	b.Backspace()
	if got := b.String(); got != "привет" {
		t.Fatalf("multibyte backspace: got %q", got)
	}
}

func TestReplaceRangeMultiline(t *testing.T) {
	b := NewBuffer("one\ntwo\nthree")
	// Заменяем "ne\ntw" на "X".
	b.ReplaceRange(Pos{0, 1}, Pos{1, 2}, "X")
	if got := b.String(); got != "oXo\nthree" {
		t.Fatalf("ReplaceRange: got %q", got)
	}
}

func TestGetRange(t *testing.T) {
	b := NewBuffer("alpha\nbeta\ngamma")
	// col 3 в "alpha" — это 'h' (a=0,l=1,p=2,h=3).
	got := b.GetRange(Pos{0, 3}, Pos{2, 2})
	if got != "ha\nbeta\nga" {
		t.Fatalf("GetRange = %q", got)
	}
}

func TestUndoRedo(t *testing.T) {
	b := NewBuffer("start")
	h := NewHistory(10)

	h.Push(b)
	b.SetCursor(0, 5)
	b.InsertRune('!')
	if b.String() != "start!" {
		t.Fatalf("setup: %q", b.String())
	}

	if !h.Undo(b) {
		t.Fatal("Undo should succeed")
	}
	if b.String() != "start" {
		t.Fatalf("after undo: %q", b.String())
	}
	if !h.Redo(b) {
		t.Fatal("Redo should succeed")
	}
	if b.String() != "start!" {
		t.Fatalf("after redo: %q", b.String())
	}
}

func TestDeleteLinesKeepsOneLine(t *testing.T) {
	b := NewBuffer("only")
	b.DeleteLines(0, 0)
	if b.LinesCount() != 1 || b.String() != "" {
		t.Fatalf("deleting only line: %q (lines=%d)", b.String(), b.LinesCount())
	}
}
