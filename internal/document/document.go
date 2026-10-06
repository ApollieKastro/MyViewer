// Package document отвечает за представление markdown-документа:
// загрузку, сохранение и листинг директорий с markdown-файлами.
//
// Документ хранит содержимое как []string (построчно): это удобно для
// редактора и не требует аллокаций при отрисовке.
package document

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Расширения, которые считаются markdown-файлами.
var markdownExts = map[string]bool{
	".md":       true,
	".mdx":      true, // MDX (markdown + JSX)
	".rmd":      true, // R Markdown
	".qmd":      true, // Quarto
	".markdown": true,
	".mdown":    true,
	".mkd":      true,
	".mkdn":     true,
	".mdwn":     true,
	".mdtxt":    true,
	".mdtext":   true,
}

// Document — открытый markdown-файл в памяти.
type Document struct {
	// Path — абсолютный путь к файлу ("" для буфера без файла).
	Path string
	// Lines — содержимое документа построчно (без завершающих \n).
	Lines []string
	// Kind — тип файла: markdown (гламур-рендер) или текст (сырой режим).
	Kind FileKind
	// Modified — были ли несохранённые изменения.
	Modified bool
	// SavedAt — время последнего успешного сохранения.
	SavedAt time.Time
}

// New создаёт пустой документ (одна пустая строка — валидный текст).
func New(path string) *Document {
	return &Document{
		Path:  path,
		Lines: []string{""},
	}
}

// Open читает документ с диска. Бинарные файлы отклоняются с понятной
// ошибкой — в просмотр попадает только текст.
func Open(path string) (*Document, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("определение пути %q: %w", path, err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("чтение %q: %w", abs, err)
	}
	kind := DetectKind(abs, data)
	if kind == KindBinary {
		return nil, fmt.Errorf("формат не поддерживается: %s — бинарный файл", filepath.Base(abs))
	}
	return &Document{
		Path:    abs,
		Lines:   SplitLines(string(data)),
		Kind:    kind,
		SavedAt: time.Now(),
	}, nil
}

// SplitLines разбивает содержимое на строки, убирая \r\n и завершающий \n.
// Гарантирует, что результат непуст (минимум одна строка).
func SplitLines(s string) []string {
	if s == "" {
		return []string{""}
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// Content собирает содержимое обратно в строку для записи на диск.
func (d *Document) Content() string {
	return strings.Join(d.Lines, "\n") + "\n"
}

// Name возвращает имя файла без каталога (или имя каталога для безымянных).
func (d *Document) Name() string {
	if d.Path == "" {
		return "без имени"
	}
	return filepath.Base(d.Path)
}

// Dir возвращает каталог, содержащий документ ("" если путь не задан).
func (d *Document) Dir() string {
	if d.Path == "" {
		return ""
	}
	return filepath.Dir(d.Path)
}

// Save записывает документ по его пути.
func (d *Document) Save() error {
	if d.Path == "" {
		return fmt.Errorf("путь к файлу не задан")
	}
	return d.SaveAs(d.Path)
}

// SaveAs записывает документ по указанному пути и помечает его сохранённым.
func (d *Document) SaveAs(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("определение пути: %w", err)
	}
	info, statErr := os.Stat(abs)
	mode := os.FileMode(0o644)
	if statErr == nil && info != nil {
		mode = info.Mode().Perm() // сохраняем права существующего файла
	}
	tmp := abs + ".mviewer.tmp"
	if err := os.WriteFile(tmp, []byte(d.Content()), mode); err != nil {
		return fmt.Errorf("запись %q: %w", abs, err)
	}
	if err := os.Rename(tmp, abs); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("замена %q: %w", abs, err)
	}
	d.Path = abs
	d.Modified = false
	d.SavedAt = time.Now()
	return nil
}

// ReplaceLines заменяет содержимое (используется редактором) и помечает
// документ изменённым, если текст реально отличается.
func (d *Document) ReplaceLines(lines []string) {
	if len(lines) == 0 {
		lines = []string{""}
	}
	same := len(lines) == len(d.Lines)
	if same {
		for i := range lines {
			if lines[i] != d.Lines[i] {
				same = false
				break
			}
		}
	}
	d.Lines = lines
	d.Modified = !same
}

// Entry — элемент каталога для файлового браузера.
type Entry struct {
	Name    string
	Path    string
	IsDir   bool
	ModTime time.Time
	Size    int64
}

// IsMarkdown reports, является ли путь markdown-файлом.
func IsMarkdown(path string) bool {
	return markdownExts[strings.ToLower(filepath.Ext(path))]
}

// ListDir возвращает содержимое каталога: сначала markdown-файлы и
// подкаталоги (сортировка как в glow), затем остальные файлы.
func ListDir(dir string) ([]Entry, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("чтение каталога %q: %w", dir, err)
	}
	var docs, other []Entry
	for _, it := range items {
		name := it.Name()
		if strings.HasPrefix(name, ".") {
			continue // скрытые файлы не показываем
		}
		e := Entry{Name: name, Path: filepath.Join(dir, name), IsDir: it.IsDir()}
		if !it.IsDir() {
			if info, err := it.Info(); err == nil {
				e.ModTime = info.ModTime()
				e.Size = info.Size()
			}
		}
		if it.IsDir() || IsMarkdown(e.Path) {
			docs = append(docs, e)
		} else {
			other = append(other, e)
		}
	}
	sortEntries(docs)
	sortEntries(other)
	return append(docs, other...), nil
}

// sortEntries сортирует: каталоги первыми, затем по имени (без учёта регистра).
func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
}

// FindMarkdown возвращает первый markdown-файл в каталоге (приоритет README*),
// либо "" если таких нет. Используется для быстрого открытия при запуске без пути.
func FindMarkdown(dir string) string {
	entries, err := ListDir(dir)
	if err != nil {
		return ""
	}
	var fallback string
	for _, e := range entries {
		if e.IsDir {
			continue
		}
		if !IsMarkdown(e.Path) {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(e.Name), "README") {
			return e.Path
		}
		if fallback == "" {
			fallback = e.Path
		}
	}
	return fallback
}
