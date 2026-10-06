package viewer

import "testing"

func TestFindMatches(t *testing.T) {
	lines := []string{
		"# Заголовок",
		"TODO: дописать",
		"ничего",
		"todo снова, но в конце",
	}
	got := FindMatches(lines, "todo")
	want := []int{1, 3}
	if len(got) != len(want) || (len(got) > 0 && (got[0] != want[0] || got[1] != want[1])) {
		t.Errorf("FindMatches(todo) = %v, want %v", got, want)
	}
	if g := FindMatches(lines, ""); g != nil {
		t.Errorf("пустой запрос должен давать nil, got %v", g)
	}
	if g := FindMatches(lines, "нет такого"); g != nil {
		t.Errorf("нет совпадений → nil, got %v", g)
	}
}

func TestNextMatchForwardWrap(t *testing.T) {
	matches := []int{5, 10, 20}

	// Первый поиск (inclusive) — с текущей позиции включительно.
	if got, ok := NextMatch(matches, 5, +1, true); !ok || got != 5 {
		t.Errorf("inclusive от 5 = %v,%v; want 5,true", got, ok)
	}
	// Шаг вниз — строго после позиции.
	if got, ok := NextMatch(matches, 5, +1, false); !ok || got != 10 {
		t.Errorf("step от 5 = %v,%v; want 10,true", got, ok)
	}
	// Wrap: после последнего — к первому.
	if got, ok := NextMatch(matches, 20, +1, false); !ok || got != 5 {
		t.Errorf("wrap вниз = %v,%v; want 5,true", got, ok)
	}
}

func TestNextMatchBackwardWrap(t *testing.T) {
	matches := []int{5, 10, 20}

	if got, ok := NextMatch(matches, 10, -1, false); !ok || got != 5 {
		t.Errorf("step вверх = %v,%v; want 5,true", got, ok)
	}
	// Wrap: выше первого — к последнему.
	if got, ok := NextMatch(matches, 5, -1, false); !ok || got != 20 {
		t.Errorf("wrap вверх = %v,%v; want 20,true", got, ok)
	}
	if _, ok := NextMatch(nil, 0, +1, true); ok {
		t.Error("пустой список совпадений → false")
	}
}
