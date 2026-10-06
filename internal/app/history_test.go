package app

import "testing"

func TestNavigationHistoryBackAndForward(t *testing.T) {
	var h navigationHistory

	// Пустая история — переходов нет.
	if _, ok := h.backward(historyEntry{path: "x"}); ok {
		t.Error("backward на пустой истории должен возвращать false")
	}
	if _, ok := h.forwardStep(historyEntry{path: "x"}); ok {
		t.Error("forward на пустой истории должен возвращать false")
	}

	// Переход A → B: позиция A уходит в back.
	h.push(historyEntry{path: "a.md", offset: 42})

	// ctrl+o из B — назад в A, позиция B уходит в forward.
	bCur := historyEntry{path: "b.md", offset: 0}
	back, ok := h.backward(bCur)
	if !ok || back.path != "a.md" || back.offset != 42 {
		t.Fatalf("backward = %+v, %v; want a.md@42", back, ok)
	}

	// ctrl+i из A — вперёд в B.
	fwd, ok := h.forwardStep(historyEntry{path: "a.md", offset: 42})
	if !ok || fwd.path != "b.md" {
		t.Fatalf("forward = %+v, %v; want b.md", fwd, ok)
	}

	// Снова назад — и новый переход очищает «вперёд».
	if _, ok := h.backward(bCur); !ok {
		t.Fatal("backward после forward")
	}
	h.push(historyEntry{path: "c.md", offset: 0})
	if _, ok := h.forwardStep(bCur); ok {
		t.Error("новый переход должен очистить стек вперёд")
	}
}

func TestNavigationHistoryStackOrder(t *testing.T) {
	var h navigationHistory
	h.push(historyEntry{path: "1.md", offset: 1})
	h.push(historyEntry{path: "2.md", offset: 2})

	// Стоим в 3.md: назад к 2.md, затем к 1.md.
	first, ok := h.backward(historyEntry{path: "3.md", offset: 3})
	if !ok || first.path != "2.md" {
		t.Fatalf("первый backward = %+v, want 2.md", first)
	}
	second, ok := h.backward(historyEntry{path: "2.md", offset: 2})
	if !ok || second.path != "1.md" {
		t.Fatalf("второй backward = %+v, want 1.md", second)
	}
	if _, ok := h.backward(historyEntry{path: "1.md"}); ok {
		t.Error("третий backward — история кончилась")
	}

	// Вперёд — в обратном порядке: из 1.md → 2.md → 3.md.
	one, ok := h.forwardStep(historyEntry{path: "1.md", offset: 1})
	if !ok || one.path != "2.md" {
		t.Fatalf("первый forward = %+v, want 2.md", one)
	}
	two, ok := h.forwardStep(historyEntry{path: "2.md", offset: 2})
	if !ok || two.path != "3.md" {
		t.Fatalf("второй forward = %+v, want 3.md", two)
	}
	if _, ok := h.forwardStep(historyEntry{path: "3.md"}); ok {
		t.Error("третий forward — «вперёд» кончилось")
	}
}
