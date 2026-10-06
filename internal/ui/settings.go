package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mviewer/internal/config"
)

// Settings — модальное окно настроек.
//
// Работает с копией конфигурации: изменения применяются посекционно и
// сохраняются в файл при закрытии. App забирает итог через Result().
type Settings struct {
	cfg   *config.Config // копия
	index int            // выбранный пункт
	// touched — были ли изменения (для автосохранения при закрытии).
	touched bool
}

// Пункты меню настроек.
const (
	setVimMode = iota
	setTheme
	setLineNumbers
	setEditor
	setStartEdit
	setWheelLines
	setCount
)

// NewSettings создаёт окно настроек на основе конфигурации.
func NewSettings(cfg *config.Config) *Settings {
	cp := *cfg
	return &Settings{cfg: &cp}
}

// Result возвращает изменённую копию конфигурации и флаг изменений.
func (s *Settings) Result() (*config.Config, bool) {
	return s.cfg, s.touched
}

// Update обрабатывает навигацию и изменение значений.
func (s *Settings) Update(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	key := keyMsg.String()

	switch key {
	case "esc", "s", "enter":
		return func() tea.Msg { return CloseSettingsMsg{} }

	// Навигация по пунктам (стрелки или vim-клавиши).
	case "up", "k":
		if s.index > 0 {
			s.index--
		} else {
			s.index = setCount - 1
		}
	case "down", "j":
		if s.index < setCount-1 {
			s.index++
		} else {
			s.index = 0
		}

	// Изменение значения.
	case "left", "h":
		s.adjust(-1)
	case "right", "l":
		s.adjust(1)
	case " ", "tab":
		s.toggle()
	}
	return nil
}

// adjust двигает значение пункта на delta (стрелки ←/→).
func (s *Settings) adjust(delta int) {
	switch s.index {
	case setTheme:
		if delta < 0 {
			s.cfg.PrevTheme()
		} else {
			s.cfg.NextTheme()
		}
		s.touched = true
	case setWheelLines:
		s.cfg.WheelLines += delta
		if s.cfg.WheelLines < 1 {
			s.cfg.WheelLines = 1
		}
		if s.cfg.WheelLines > 20 {
			s.cfg.WheelLines = 20
		}
		s.touched = true
	case setEditor:
		s.cycleEditor(delta)
	}
}

// cycleEditor двигает выбор редактора по циклу (стрелки ←/→).
func (s *Settings) cycleEditor(delta int) {
	cycle := config.EditorCycle
	idx := 0
	for i, v := range cycle {
		if v == s.cfg.Editor {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(cycle)) % len(cycle)
	s.cfg.Editor = cycle[idx]
	s.touched = true
}

// toggle переключает булевы пункты (пробел).
func (s *Settings) toggle() {
	switch s.index {
	case setVimMode:
		s.cfg.VimMode = !s.cfg.VimMode
		s.touched = true
	case setLineNumbers:
		s.cfg.ShowLineNumbers = !s.cfg.ShowLineNumbers
		s.touched = true
	case setStartEdit:
		s.cfg.StartInEditMode = !s.cfg.StartInEditMode
		s.touched = true
	case setTheme:
		s.cfg.NextTheme()
		s.touched = true
	case setEditor:
		s.cycleEditor(1)
	}
}

// CloseSettingsMsg — команда закрытия окна (её ловит app).
type CloseSettingsMsg struct{}

// View рисует панель настроек, центрированную в окне width×height.
func (s *Settings) View(width, height int) string {
	var b strings.Builder

	b.WriteString(Title.Render("Настройки") + "\n")
	b.WriteString(MutedText.Render("↑↓/jk — выбор · ←→/hl — значение · пробел — переключить · esc — закрыть") + "\n")
	b.WriteString(strings.Repeat("─", 54) + "\n")

	rows := []struct {
		label string
		value func() string
	}{
		{"Vim mode", func() string { return onOff(s.cfg.VimMode) }},
		{"Тема markdown", func() string { return s.cfg.Theme }},
		{"Номера строк (ctrl+n)", func() string { return onOff(s.cfg.ShowLineNumbers) }},
		{"Редактор (e)", func() string { return editorLabel(s.cfg.Editor) }},
		{"Открывать в режиме правки", func() string { return onOff(s.cfg.StartInEditMode) }},
		{"Колесо мыши (строк)", func() string { return fmt.Sprintf("%d", s.cfg.WheelLines) }},
	}

	for i, row := range rows {
		value := row.value()
		label := row.label
		if i == s.index {
			// Выделенный пункт: строка целиком в стиле фокуса.
			line := fmt.Sprintf("  %s  %s", label, value)
			b.WriteString(FocusedItem.Render(padTo(line, 52)) + "\n")
		} else {
			b.WriteString(Item.Render(fmt.Sprintf("    %-30s %s", label, value)) + "\n")
		}
	}

	b.WriteString(strings.Repeat("─", 54) + "\n")
	b.WriteString(MutedText.Render("Изменения сохраняются в ~/.config/mviewer/config.json"))

	panel := Panel.Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}

// onOff — отображение булевого значения.
func onOff(v bool) string {
	if v {
		return SuccessText.Render("ON")
	}
	return MutedText.Render("OFF")
}

// editorLabel — человекочитаемое имя настройки редактора.
func editorLabel(v string) string {
	switch v {
	case config.EditorBuiltin:
		return "встроенный"
	case config.EditorNvim:
		return "neovim"
	case config.EditorVim:
		return "vim"
	default:
		return "авто (nvim→vim→встроенный)"
	}
}

// padTo дополняет строку пробелами до width (по видимым рунам).
func padTo(s string, width int) string {
	n := lipgloss.Width(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}
