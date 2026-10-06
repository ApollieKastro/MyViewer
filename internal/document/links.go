package document

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

// Link — ссылка, найденная в тексте документа.
type Link struct {
	Text string // отображаемый текст ("" если в источнике не указан)
	URL  string // адрес: http(s), mailto: либо относительный путь
}

// bareURLRe — голые URL с известными схемами (вне inline-ссылок).
var bareURLRe = regexp.MustCompile(`(?:https?://|mailto:)[^\s<>()\[\]"']+`)

// schemeRe — строка начинается с URI-схемы (http://, mailto: …).
var schemeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*:`)

// HasScheme reports, является ли цель внешней ссылкой (есть URI-схема).
func HasScheme(url string) bool { return schemeRe.MatchString(url) }

// FileURLPath преобразует file:// URL в локальный путь: percent-кодировка
// декодируется, fragment (#…) отбрасывается. Возвращает false, если это
// не локальный file:// URL (чужой host, битый URL, пустой путь) —
// такие цели не следует открывать «внутри» приложения.
func FileURLPath(raw string) (string, bool) {
	if !strings.HasPrefix(strings.ToLower(raw), "file://") {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" {
		return "", false
	}
	// Локальны только пустой host (file:///…) и localhost.
	if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		return "", false
	}
	if u.Path == "" {
		return "", false
	}
	return u.Path, true
}

// ExtractLinks извлекает ссылки в документе в порядке появления.
//
// Понимаются inline-ссылки [текст](url) (одна вложенность в тексте,
// экранирование \[ допускается) и голые URL вида http://…/mailto:…
// (в том числе autolink <http://…>). Картинки ![alt](src) — не ссылки.
// URL, уже покрытый inline-ссылкой, повторно не добавляется.
func ExtractLinks(src string) []Link {
	type located struct {
		pos  int
		link Link
	}
	var found []located
	var covered [][2]int // диапазоны inline-ссылок [начало, конец)

	// Inline-ссылки: [text](url).
	for i := 0; i < len(src); i++ {
		if src[i] != '[' || (i > 0 && src[i-1] == '!') {
			continue
		}
		textStart, textEnd, urlStart, urlEnd, ok := findInline(src, i)
		if !ok {
			continue
		}
		url := src[urlStart:urlEnd]
		if strings.TrimSpace(url) == "" {
			continue
		}
		text := strings.TrimSpace(src[textStart:textEnd])
		found = append(found, located{pos: i, link: Link{Text: text, URL: url}})
		covered = append(covered, [2]int{i, urlEnd})
	}

	// Голые URL вне уже найденных inline-ссылок.
	for _, loc := range bareURLRe.FindAllStringIndex(src, -1) {
		if overlaps(covered, loc[0], loc[1]) {
			continue
		}
		found = append(found, located{
			pos:  loc[0],
			link: Link{URL: src[loc[0]:loc[1]]},
		})
	}

	sort.SliceStable(found, func(i, j int) bool { return found[i].pos < found[j].pos })
	links := make([]Link, 0, len(found))
	for _, f := range found {
		links = append(links, f.link)
	}
	return links
}

// findInline ищет окончание inline-ссылки, начиная с позиции '['.
// Возвращает границы текста и URL (без скобок).
func findInline(src string, start int) (textStart, textEnd, urlStart, urlEnd int, ok bool) {
	depth := 0
	for j := start + 1; j < len(src); j++ {
		c := src[j]
		if c == '\\' {
			j++ // экранирование
			continue
		}
		switch c {
		case '\n':
			return 0, 0, 0, 0, false // ссылка не выходит за строку
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
				continue
			}
			if j+1 >= len(src) || src[j+1] != '(' {
				return 0, 0, 0, 0, false
			}
			for k := j + 2; k < len(src); k++ {
				if src[k] == '\\' {
					k++
					continue
				}
				if src[k] == ')' {
					return start + 1, j, j + 2, k, true
				}
				if src[k] == '\n' {
					return 0, 0, 0, 0, false
				}
			}
			return 0, 0, 0, 0, false
		}
	}
	return 0, 0, 0, 0, false
}

// overlaps — пересекается ли [a,b) с каким-либо занятым диапазоном.
func overlaps(covered [][2]int, a, b int) bool {
	for _, r := range covered {
		if a < r[1] && r[0] < b {
			return true
		}
	}
	return false
}
