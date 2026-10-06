package render

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// codeFormatter — truecolor-форматтер для современных терминалов
// (glamour тоже отдаёт 24-битные цвета, поэтому рассинхрона нет).
const codeFormatter = "terminal16m"

// renderCode подсвечивает исходный код через chroma, сохраняя текст
// дословно (как в сыром режиме): нормализация, JSON-отступы, табы и
// перенос длинных строк выполняются ДО подсветки — chroma добавляет
// только ANSI-цвета и не умеет переносить строки.
//
// Любая ошибка chroma (нет лексера, сбой токенизации) не валит рендер:
// возвращается обычный сырой текст. Монокромные темы (ascii, notty)
// подсветку отключают — цвета там всё равно не будет.
func (r *Renderer) renderCode(src, filename string, width int) ([]string, error) {
	plain, err := renderRawText(src, width)
	if err != nil {
		return nil, err
	}

	styleName, ok := r.codeStyle()
	if !ok {
		return plain, nil
	}
	lexer := lexers.Match(filename)
	if lexer == nil {
		return plain, nil
	}
	style := styles.Get(styleName)
	formatter := formatters.Get(codeFormatter)
	if style == nil || formatter == nil {
		return plain, nil
	}

	lexer = chroma.Coalesce(lexer) // объединяет смежные токены одного типа
	it, err := lexer.Tokenise(nil, strings.Join(plain, "\n"))
	if err != nil {
		return plain, nil // фолбэк: текст важнее подсветки
	}
	var buf bytes.Buffer
	buf.Grow(len(src) * 2)
	if err := formatter.Format(&buf, style, it); err != nil {
		return plain, nil
	}

	out := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return []string{""}, nil
	}
	return out, nil
}

// codeStyle возвращает chroma-стиль для текущей темы mviewer.
// Второе значение false — подсветка отключена (монокромные темы).
func (r *Renderer) codeStyle() (string, bool) {
	switch r.theme {
	case "ascii", "notty":
		return "", false // безцветные темы: подсветка не нужна
	case "dracula":
		return "dracula", true
	case "tokyo-night":
		return "tokyonight-night", true
	case "pink":
		return "dracula", true // ближайший родственник по палитре
	case "light":
		return "github", true
	case "dark":
		return "monokai", true
	default: // auto и неизвестные значения
		if r.darkBackground {
			return "monokai", true
		}
		return "github", true
	}
}
