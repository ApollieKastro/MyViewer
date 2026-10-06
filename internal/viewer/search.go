package viewer

import "strings"

// FindMatches возвращает индексы строк (0-based), содержащих query
// (регистронезависимо). Пустой запрос — nil.
func FindMatches(lines []string, query string) []int {
	if query == "" {
		return nil
	}
	q := strings.ToLower(query)
	var out []int
	for i, ln := range lines {
		if strings.Contains(strings.ToLower(ln), q) {
			out = append(out, i)
		}
	}
	return out
}

// NextMatch выбирает следующее совпадение от позиции from в направлении
// dir (+1 — вниз, -1 — вверх) с переходом через границы (wrap).
// inclusive=true — from включается в проверку (первый поиск),
// inclusive=false — строгое сравнение (шаги n/N).
// Возвращает индекс строки и true, если совпадение есть.
func NextMatch(matches []int, from, dir int, inclusive bool) (int, bool) {
	if len(matches) == 0 {
		return 0, false
	}
	if dir >= 0 {
		for _, mi := range matches {
			if (inclusive && mi >= from) || (!inclusive && mi > from) {
				return mi, true
			}
		}
		return matches[0], true // wrap к началу
	}
	for i := len(matches) - 1; i >= 0; i-- {
		if (inclusive && matches[i] <= from) || (!inclusive && matches[i] < from) {
			return matches[i], true
		}
	}
	return matches[len(matches)-1], true // wrap к концу
}

// Search возвращает индексы строк содержимого, содержащих query.
func (m *Model) Search(query string) []int { return FindMatches(m.lines, query) }
