// Package render выполняет отрисовку markdown в ANSI-стилях через glamour.
//
// Ключевое решение для производительности: glamour вызывается один раз на
// комбинацию (исходник, ширина, тема). Результат кэшируется в виде среза
// строк; каждый кадр приложение лишь нарезает готовые строки по окну
// прокрутки, не трогая ANSI-разметку.
package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/muesli/termenv"
)

// Renderer рендерит markdown-документ в стилизованные строки терминала.
type Renderer struct {
	theme string
	// darkBackground — результат детекта фона терминала (для темы auto).
	// Определяется ОДИН РАЗ в main до запуска TUI: запрос к терминалу
	// внутри фонового рендера конфликтует с event-циклом Bubble Tea.
	darkBackground bool

	// Кэш последнего результата: покрывает реальный сценарий работы —
	// один открытый документ, ре-рендер только при ресайзе/смене темы.
	srcHash  uint64
	width    int
	cached   []string
	mode     renderMode
	key      string // имя файла (лексер) для code-режима, "" для остальных
	rendered bool
}

// renderMode — режим рендера, участвует в ключе кэша.
type renderMode uint8

const (
	modeMarkdown renderMode = iota // glamour-рендер markdown
	modeRaw                        // сырой текст/код без разметки
	modeCode                       // код с подсветкой синтаксиса (chroma)
)

// DetectBackground определяет, тёмный ли фон у терминала.
//
// Порядок: переменная окружения MVIEWER_BG (light|dark — принудительный
// override), затем нативный запрос OSC-11 через termenv. Вызывать ДО запуска
// tea.Program — после Bubble Tea захватывает stdin, и ответ терминала уйдёт
// ему как «мусорный ввод».
func DetectBackground() bool {
	if v := os.Getenv("MVIEWER_BG"); v != "" {
		return !strings.EqualFold(v, "light")
	}
	return termenv.HasDarkBackground()
}

// New создаёт рендерер с указанной темой. darkBackground используется,
// когда тема равна "auto".
func New(theme string, darkBackground bool) *Renderer {
	return &Renderer{theme: theme, darkBackground: darkBackground}
}

// SetTheme меняет тему (после чего следующий Render выполнит ре-рендер).
func (r *Renderer) SetTheme(theme string) {
	if r.theme != theme {
		r.theme = theme
		r.rendered = false
	}
}

// Theme возвращает текущую тему.
func (r *Renderer) Theme() string { return r.theme }

// Render возвращает отрендеренные строки markdown-документа для заданной
// ширины. Повторные вызовы с теми же аргументами отдают закэшированный срез.
func (r *Renderer) Render(src string, width int) ([]string, error) {
	return r.renderKeyed(src, width, modeMarkdown, "")
}

// RenderRaw возвращает строки обычного текста БЕЗ markdown-разметки:
// табы разворачиваются в пробелы, длинные строки переносятся по словам,
// JSON разворачивается с отступами. Используется для .txt/.log и прочих
// файлов, у которых нет лексера для подсветки.
func (r *Renderer) RenderRaw(src string, width int) ([]string, error) {
	return r.renderKeyed(src, width, modeRaw, "")
}

// RenderCode возвращает строки кода с подсветкой синтаксиса (chroma).
// filename нужен для выбора лексера; если лексер не найден или тема
// монокромная (ascii/notty), результат совпадает с RenderRaw.
func (r *Renderer) RenderCode(src string, width int, filename string) ([]string, error) {
	return r.renderKeyed(src, width, modeCode, filename)
}

// renderKeyed — общий вход с учётом режима и ключа (имя файла) в кэше.
func (r *Renderer) renderKeyed(src string, width int, mode renderMode, key string) ([]string, error) {
	if width < 1 {
		width = 1
	}
	h := hash(src)
	if r.rendered && h == r.srcHash && width == r.width &&
		mode == r.mode && key == r.key {
		return r.cached, nil
	}

	var lines []string
	var err error
	switch mode {
	case modeMarkdown:
		lines, err = r.renderMarkdown(src, width)
	case modeCode:
		lines, err = r.renderCode(src, key, width)
	default:
		lines, err = renderRawText(src, width)
	}
	if err != nil {
		return nil, err
	}
	r.srcHash, r.width, r.cached = h, width, lines
	r.mode, r.key, r.rendered = mode, key, true
	return lines, nil
}

// Invalidate сбрасывает кэш (используется при изменении документа).
func (r *Renderer) Invalidate() { r.rendered = false }

// renderMarkdown выполняет фактический рендер markdown через glamour.
func (r *Renderer) renderMarkdown(src string, width int) ([]string, error) {
	opts := []glamour.TermRendererOption{
		glamour.WithEmoji(),
		// WordWrap чуть уже ширины: glamour добавляет собственные отступы,
		// а запас предотвращает переполнение из-за двойных отступов.
		glamour.WithWordWrap(width),
	}
	if r.theme == "auto" || r.theme == "" {
		// Явный выбор стиля по заранее детектированному фону: надёжнее
		// glamour.WithAutoStyle(), который опрашивает терминал в фоне.
		if r.darkBackground {
			opts = append(opts, glamour.WithStandardStyle("dark"))
		} else {
			opts = append(opts, glamour.WithStandardStyle("light"))
		}
	} else {
		opts = append(opts, glamour.WithStandardStyle(r.theme))
	}

	tr, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return nil, fmt.Errorf("создание рендерера glamour: %w", err)
	}
	defer tr.Close() //nolint:errcheck

	out, err := tr.Render(src)
	if err != nil {
		return nil, fmt.Errorf("рендер markdown: %w", err)
	}
	return postprocess(out, width), nil
}

// renderRawText готовит обычный текст к показу: \r\n → \n, табы → пробелы,
// длинные строки переносятся по словам, хвостовые пустые строки отбрасываются.
//
// В отличие от markdown-режима содержимое не искажается: звёздочки, решётки
// и тире остаются как в файле. JSON дополнительно разворачивается
// с отступами (файлы-однострочники иначе нечитаемы).
func renderRawText(src string, width int) ([]string, error) {
	s := strings.ReplaceAll(src, "\r\n", "\n")
	s = prettyJSON(s)
	raw := strings.Split(s, "\n")
	for len(raw) > 0 && strings.TrimSpace(raw[len(raw)-1]) == "" {
		raw = raw[:len(raw)-1]
	}
	if len(raw) == 0 {
		return []string{""}, nil
	}
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = expandTabs(line, 4)
		out = append(out, wrapLine(line, width)...)
	}
	return out, nil
}

// prettyJSON возвращает JSON с отступами (2 пробела), если src — валидный
// JSON, начинающийся с объекта или массива; иначе — исходник без изменений.
// Попытка дешёвая: парсинг выполняется только для файлов, похожих на JSON,
// и результат входит в ключ кэша (повторных парсингов нет).
func prettyJSON(src string) string {
	trimmed := strings.TrimLeft(src, " \t\n")
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return src
	}
	var buf bytes.Buffer
	buf.Grow(len(src) + len(src)/4)
	if err := json.Indent(&buf, []byte(src), "", "  "); err != nil {
		return src
	}
	return buf.String()
}

// expandTabs разворачивает табуляцию в пробелы с шагом tabWidth.
func expandTabs(line string, tabWidth int) string {
	if !strings.Contains(line, "\t") {
		return line
	}
	var b strings.Builder
	b.Grow(len(line) + tabWidth)
	col := 0
	for _, r := range line {
		if r == '\t' {
			pad := tabWidth - col%tabWidth
			b.WriteString(strings.Repeat(" ", pad))
			col += pad
			continue
		}
		b.WriteRune(r)
		col++
	}
	return b.String()
}

// wrapLine переносит строку по словам до width рун. Слово шире окна
// режется жёстко. Разрыв происходит на месте пробела (он не дублируется).
func wrapLine(line string, width int) []string {
	rs := []rune(line)
	if width < 8 || len(rs) <= width {
		return []string{line}
	}
	var out []string
	for len(rs) > width {
		cut := -1
		for i := width; i > 0; i-- {
			if rs[i] == ' ' {
				cut = i
				break
			}
		}
		if cut <= 0 {
			cut = width // одно слово длиннее окна
		}
		out = append(out, string(rs[:cut]))
		rs = rs[cut:]
		if len(rs) > 0 && rs[0] == ' ' {
			rs = rs[1:] // пробел в точке переноса не сохраняем
		}
	}
	out = append(out, string(rs))
	return out
}

// postprocess нормализует вывод glamour: убирает хвостовые пробелы
// (экономим CPU на выравнивании и делаем прокрутку точной) и пустые
// строки в конце документа.
func postprocess(s string, width int) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	raw := strings.Split(s, "\n")

	// Обрезаем хвостовые пустые строки.
	for len(raw) > 0 && strings.TrimSpace(raw[len(raw)-1]) == "" {
		raw = raw[:len(raw)-1]
	}
	// Обрезаем хвостовые пробелы каждой строки (после ANSI-кода это безопасно:
	// пробелы всегда идут до управляющих последовательностей сброса).
	for i, line := range raw {
		raw[i] = trimRightSpaces(line)
	}
	if len(raw) == 0 {
		return []string{""}
	}
	return raw
}

// trimRightSpaces убирает завершающие пробелы/табы, не разрушая ANSI-последовательности.
func trimRightSpaces(line string) string {
	i := len(line)
	for i > 0 {
		c := line[i-1]
		if c == ' ' || c == '\t' {
			i--
			continue
		}
		break
	}
	return line[:i]
}

// hash — быстрый FNV-1a по содержимому (для инвалидации кэша).
func hash(s string) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	h := uint64(offset64)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}
