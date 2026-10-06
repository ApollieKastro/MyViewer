package document

import (
	"bytes"
	"path/filepath"
	"strings"
)

// FileKind — тип открываемого файла, определяет способ просмотра.
type FileKind uint8

const (
	// KindText — обычный текст или код: просмотр без markdown-разметки
	// (сырой режим: отступы и спецсимволы сохраняются дословно).
	KindText FileKind = iota
	// KindMarkdown — markdown-документ: glamour-рендер.
	KindMarkdown
	// KindBinary — содержимое не является текстом; открытие запрещено.
	KindBinary
)

// String — короткая метка для статусбара.
func (k FileKind) String() string {
	switch k {
	case KindMarkdown:
		return "md"
	case KindBinary:
		return "bin"
	default:
		return "txt"
	}
}

// sampleSize — объём начала файла для определения бинарности.
const sampleSize = 8192

// DetectKind определяет тип файла по содержимому и расширению.
//
// Порядок: сначала содержимое (бинарность по сигнатурам и управляющим
// символам) — оно важнее расширения: бинарник с расширением .md остаётся
// бинарником. Затем расширение: markdown-расширения → KindMarkdown.
// Файлы БЕЗ расширения дополнительно проверяются по содержимому: первые
// непустые строки с markdown-заголовком (# + пробел) означают markdown
// (README, CHANGELOG без .md). Остальное — KindText.
func DetectKind(path string, data []byte) FileKind {
	if IsBinaryContent(data) {
		return KindBinary
	}
	if IsMarkdown(path) {
		return KindMarkdown
	}
	if filepath.Ext(path) == "" && looksLikeMarkdown(data) {
		return KindMarkdown
	}
	return KindText
}

// looksLikeMarkdown эвристически определяет markdown по содержимому файла
// БЕЗ расширения: первые непустые строки содержат заголовок вида
// "# Текст" / "## Текст" (решётки + обязательный пробел — "#!" и "#comment"
// не считаются). Проверяются только первые 5 непустых строк (~200 байт),
// чтобы случайные совпадения в середине конфига не дали ложного срабатывания.
func looksLikeMarkdown(data []byte) bool {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // UTF-8 BOM
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	checked := 0
	for _, line := range strings.Split(string(sample), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if isMarkdownHeading(line) {
			return true
		}
		checked++
		if checked >= 5 {
			break
		}
	}
	return false
}

// isMarkdownHeading — строка вида "# Заголовок": ведущая серия решёток,
// за которой обязателен пробел или таб.
func isMarkdownHeading(line string) bool {
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i == 0 || i == len(line) {
		return false
	}
	return line[i] == ' ' || line[i] == '\t'
}

// IsBinaryContent эвристически определяет, является ли данные текстом.
//
// Критерии: нулевой байт (0x00) в выборке — однозначно бинарно; либо
// доля управляющих символов (кроме \t \n \r и ESC — допустим в логах
// с ANSI-кодами) превышает 5% выборки.
func IsBinaryContent(data []byte) bool {
	sample := data
	if len(sample) > sampleSize {
		sample = sample[:sampleSize]
	}
	if len(sample) == 0 {
		return false
	}
	suspicious := 0
	for _, b := range sample {
		switch {
		case b == 0:
			return true
		case b == '\t' || b == '\n' || b == '\r' || b == 0x1b:
			// Допустимые управляющие символы текста.
		case b < 0x20 || b == 0x7f:
			suspicious++
		}
	}
	return suspicious*100 > len(sample)*5
}

// FindDocument ищет файл для быстрого открытия при запуске без аргумента:
// README* → другой markdown → текстовый файл → "" (нет файлов).
//
// Бинарные и незнакомые расширения не рассматриваются: запуск в случайном
// каталоге не должен открывать мусор.
func FindDocument(dir string) string {
	entries, err := ListDir(dir)
	if err != nil {
		return ""
	}
	var mdFallback, txtFallback string
	for _, e := range entries {
		if e.IsDir {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name))
		if IsMarkdown(e.Path) {
			if strings.HasPrefix(strings.ToUpper(e.Name), "README") {
				return e.Path // приоритет: привычный вход в проект
			}
			if mdFallback == "" {
				mdFallback = e.Path
			}
			continue
		}
		if txtFallback == "" && textOpenExts[ext] {
			txtFallback = e.Path
		}
	}
	if mdFallback != "" {
		return mdFallback
	}
	return txtFallback
}

// textOpenExts — расширения, пригодные для автозапуска без аргумента.
var textOpenExts = map[string]bool{
	".txt":  true,
	".log":  true,
	".text": true,
}
