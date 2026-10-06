// Package ui — общие элементы интерфейса: стили, статусбар, справка,
// модальное окно настроек. Слой не содержит бизнес-логики приложения.
package ui

import "github.com/charmbracelet/lipgloss"

// Цветовая палитра (адаптивная: светлая/тёмная терминальная тема).
var (
	// Accent — акцентный цвет интерфейса.
	Accent = lipgloss.AdaptiveColor{Light: "#1E66F5", Dark: "#89B4FA"}
	// Muted — приглушённый текст.
	Muted = lipgloss.AdaptiveColor{Light: "#9CA0B0", Dark: "#6C7086"}
	// Fg — основной текст.
	Fg = lipgloss.AdaptiveColor{Light: "#4C4F69", Dark: "#CDD6F4"}
	// Bg — фон панелей.
	Bg = lipgloss.AdaptiveColor{Light: "#EFF1F5", Dark: "#1E1E2E"}
	// PanelBg — фон модальных окон.
	PanelBg = lipgloss.AdaptiveColor{Light: "#E6E9EF", Dark: "#313244"}
	// Danger — предупреждения (несохранённые изменения).
	Danger = lipgloss.AdaptiveColor{Light: "#D20F39", Dark: "#F38BA8"}
	// Success — подтверждения (сохранено).
	Success = lipgloss.AdaptiveColor{Light: "#40A02B", Dark: "#A6E3A1"}
	// SelBg — фон выделения.
	SelBg = lipgloss.AdaptiveColor{Light: "#ACB0BE", Dark: "#45475A"}

	// SuccessText — текст успеха.
	SuccessText = lipgloss.NewStyle().Foreground(Success)
	// DangerText — текст предупреждения.
	DangerText = lipgloss.NewStyle().Foreground(Danger)
)

// Общие стили.
var (
	// StatusBar — нижняя строка состояния.
	StatusBar = lipgloss.NewStyle().
			Foreground(Fg).
			Background(Bg)

	// StatusAccent — акцентный сегмент статусбара.
	StatusAccent = lipgloss.NewStyle().
			Foreground(Bg).
			Background(Accent).
			Bold(true)

	// StatusDanger — сегмент предупреждения.
	StatusDanger = lipgloss.NewStyle().
			Foreground(Bg).
			Background(Danger).
			Bold(true)

	// StatusSuccess — сегмент успеха.
	StatusSuccess = lipgloss.NewStyle().
			Foreground(Bg).
			Background(Success).
			Bold(true)

	// Title — заголовок панели.
	Title = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	// MutedText — приглушённый текст.
	MutedText = lipgloss.NewStyle().Foreground(Muted)

	// Key — подсветка клавиш в справке.
	Key = lipgloss.NewStyle().
		Foreground(Fg).
		Background(PanelBg).
		Bold(true).
		Padding(0, 1)

	// Desc — описание клавиши.
	Desc = lipgloss.NewStyle().Foreground(Muted)

	// Panel — рамка модального окна.
	Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Accent).
		Background(PanelBg).
		Padding(0, 1)

	// FocusedItem — выбранный пункт настроек.
	FocusedItem = lipgloss.NewStyle().
			Foreground(Bg).
			Background(Accent).
			Bold(true)

	// Item — невыбранный пункт.
	Item = lipgloss.NewStyle().Foreground(Fg)
)
