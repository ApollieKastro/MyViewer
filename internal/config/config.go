// Package config хранит настройки приложения и их персистентность.
//
// Конфигурация — value-объект: создаётся через Default(), загружается из
// JSON-файла (~/.config/mviewer/config.json) и сохраняется обратно при
// подтверждении настроек. Отсутствующий или битый файл не считается ошибкой:
// приложение работает на дефолтах.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Темы рендеринга markdown (стандартные стили glamour).
const (
	ThemeAuto    = "auto"
	ThemeDark    = "dark"
	ThemeLight   = "light"
	ThemeDracula = "dracula"
	ThemeTokyo   = "tokyo-night"
	ThemePink    = "pink"
	ThemeASCII   = "ascii"
	ThemeNoTTY   = "notty"
)

// Значения настройки Editor — какой редактор открывать по клавише e.
const (
	// EditorAuto — neovim, затем vim, затем встроенный (по наличию в PATH).
	EditorAuto = "auto"
	// EditorBuiltin — всегда встроенный редактор mviewer.
	EditorBuiltin = "builtin"
	// EditorNvim / EditorVim — принудительный выбор; если бинарника нет,
	// срабатывает автофолбэк (auto).
	EditorNvim = "nvim"
	EditorVim  = "vim"
)

// EditorCycle — порядок переключения в настройках (пункт «Редактор»).
var EditorCycle = []string{EditorAuto, EditorBuiltin, EditorNvim, EditorVim}

// Themes возвращает список доступных тем (для цикла в настройках).
func Themes() []string {
	return []string{
		ThemeAuto, ThemeDark, ThemeLight, ThemeDracula,
		ThemeTokyo, ThemePink, ThemeASCII, ThemeNoTTY,
	}
}

// Config — полный набор настроек приложения.
//
// Все поля имеют json-тегы, чтобы формат файла был стабильным и человекочитаемым.
type Config struct {
	// VimMode включает vim-клавиатуру: в просмотре — навигацию (j/k/g/G),
	// в редакторе — нормальный режим с операторами и мотионами.
	VimMode bool `json:"vim_mode"`

	// Theme — стиль glamour для отрисовки markdown.
	Theme string `json:"theme"`

	// ShowLineNumbers показывает нумерацию строк в редакторе.
	ShowLineNumbers bool `json:"show_line_numbers"`

	// StartInEditMode открывает документ сразу в режиме редактирования.
	StartInEditMode bool `json:"start_in_edit_mode"`

	// WheelLines — число строк, прокручиваемых одним поворотом колеса мыши.
	WheelLines int `json:"wheel_lines"`

	// ScrollStep — шаг прокрутки стрелками/клавишами построчной навигации.
	ScrollStep int `json:"scroll_step"`

	// SoftWrap включает перенос длинных строк в редакторе по ширине окна.
	SoftWrap bool `json:"soft_wrap"`

	// Editor выбирает редактор для клавиши e: auto (neovim → vim →
	// встроенный), builtin, nvim, vim.
	Editor string `json:"editor"`
}

// Default возвращает конфигурацию по умолчанию.
func Default() *Config {
	return &Config{
		VimMode:         true,
		Theme:           ThemeAuto,
		ShowLineNumbers: true,
		StartInEditMode: false,
		WheelLines:      3,
		ScrollStep:      1,
		SoftWrap:        false,
		Editor:          EditorAuto,
	}
}

// Path возвращает путь к файлу конфигурации пользователя.
func Path() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "mviewer", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "mviewer-config.json"
	}
	return filepath.Join(home, ".config", "mviewer", "config.json")
}

// Load читает конфигурацию из файла. Если файла нет или он повреждён,
// возвращаются дефолтные значения (ошибка конфига не должна блокировать запуск).
func Load() *Config {
	cfg := Default()
	data, err := os.ReadFile(Path())
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return Default()
	}
	cfg.normalize()
	return cfg
}

// LoadFrom читает конфигурацию из произвольного пути (используется в тестах).
func LoadFrom(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("разбор конфига: %w", err)
	}
	cfg.normalize()
	return cfg, nil
}

// Save записывает конфигурацию в файл, создавая родительские каталоги.
func (c *Config) Save() error {
	c.normalize()
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога конфига: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование конфига: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("запись конфига: %w", err)
	}
	return nil
}

// SetTheme устанавливает тему, если она входит в список допустимых.
func (c *Config) SetTheme(theme string) error {
	for _, t := range Themes() {
		if t == theme {
			c.Theme = theme
			return nil
		}
	}
	return fmt.Errorf("неизвестная тема %q", theme)
}

// NextTheme переключает тему на следующую в цикле. Возвращает новую тему.
func (c *Config) NextTheme() string {
	themes := Themes()
	for i, t := range themes {
		if t == c.Theme {
			c.Theme = themes[(i+1)%len(themes)]
			return c.Theme
		}
	}
	c.Theme = themes[0]
	return c.Theme
}

// PrevTheme переключает тему на предыдущую в цикле. Возвращает новую тему.
func (c *Config) PrevTheme() string {
	themes := Themes()
	for i, t := range themes {
		if t == c.Theme {
			c.Theme = themes[(i-1+len(themes))%len(themes)]
			return c.Theme
		}
	}
	c.Theme = themes[0]
	return c.Theme
}

// IsKnownTheme проверяет, поддерживается ли тема.
func IsKnownTheme(theme string) bool {
	for _, t := range Themes() {
		if t == theme {
			return true
		}
	}
	return false
}

// normalize приводит значения к допустимым диапазонам.
func (c *Config) normalize() {
	if !IsKnownTheme(c.Theme) {
		c.Theme = ThemeAuto
	}
	if c.WheelLines < 1 || c.WheelLines > 20 {
		c.WheelLines = 3
	}
	if c.ScrollStep < 1 || c.ScrollStep > 10 {
		c.ScrollStep = 1
	}
	switch c.Editor {
	case EditorAuto, EditorBuiltin, EditorNvim, EditorVim:
	default:
		c.Editor = EditorAuto
	}
}

// ErrNoConfig сигнализирует об отсутствии файла конфигурации.
var ErrNoConfig = errors.New("файл конфигурации не найден")
