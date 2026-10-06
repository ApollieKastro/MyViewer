package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StatusData — данные для строки состояния.
type StatusData struct {
	// FileName — отображаемое имя файла (или каталога).
	FileName string
	// Modified — есть несохранённые изменения.
	Modified bool
	// Percent — позиция в документе (0..100), -1 — не показывать.
	Percent int
	// Mode — верхнеуровневый режим: BROWSE / VIEW / EDIT.
	Mode string
	// SubMode — подрежим (NORMAL/INSERT/...) или метка ожидания (d2, f).
	SubMode string
	// ShowHints — показывать подсказки клавиш справа.
	ShowHints bool
	// HintLeft/HintRight — подсказки для режима.
	HintLeft, HintRight string
	// Message — временное сообщение (перекрывает позицию).
	Message string
	// MessageKind: "info" | "success" | "danger".
	MessageKind string
}

// RenderStatusBar рисует нижнюю строку состояния нужной ширины.
func RenderStatusBar(width int, d StatusData) string {
	if width < 10 {
		width = 10
	}

	// Левая часть: имя файла и позиция.
	left := " " + truncate(d.FileName, width/2)
	if d.Modified {
		left += " " + StatusDanger.Render("[+]")
	}
	if d.Percent >= 0 && d.Message == "" {
		left += MutedText.Render(fmt.Sprintf("  %d%%", d.Percent))
	}

	// Центр: сообщение.
	center := ""
	if d.Message != "" {
		style := StatusAccent
		switch d.MessageKind {
		case "success":
			style = StatusSuccess
		case "danger":
			style = StatusDanger
		}
		center = " " + style.Render(" "+d.Message+" ") + " "
	}

	// Правая часть: режим и подсказки.
	right := StatusAccent.Render(" " + d.Mode + " ")
	if d.SubMode != "" {
		right += StatusBar.Render(" " + d.SubMode + " ")
	}
	if d.ShowHints {
		hints := ""
		if d.HintLeft != "" {
			hints += MutedText.Render(" " + d.HintLeft + " ")
		}
		if d.HintRight != "" {
			hints += MutedText.Render(" " + d.HintRight + " ")
		}
		right += hints
	}

	// Подгоняем под ширину: left + gap + center + right == width.
	// При нехватке места сначала убираем подсказки, затем сообщение.
	gap := width - lipgloss.Width(left) - lipgloss.Width(center) - lipgloss.Width(right)
	if gap < 1 && d.ShowHints {
		d.ShowHints = false
		right = StatusAccent.Render(" " + d.Mode + " ")
		if d.SubMode != "" {
			right += StatusBar.Render(" " + d.SubMode + " ")
		}
		gap = width - lipgloss.Width(left) - lipgloss.Width(center) - lipgloss.Width(right)
	}
	if gap < 1 && center != "" {
		// Сообщение важнее: ужимаем его под доступное место, а не удаляем
		// (оно может предупреждать о несохранённых изменениях).
		// -1 — запас на пробел разрыва, иначе gap останется 0.
		center = shrinkMessage(d, width-lipgloss.Width(left)-lipgloss.Width(right)-1)
		gap = width - lipgloss.Width(left) - lipgloss.Width(center) - lipgloss.Width(right)
	}
	if gap < 1 {
		return StatusBar.Render(truncateRendered(left+right, width))
	}

	line := left + strings.Repeat(" ", gap) + center + right
	return StatusBar.Render(line)
}

// shrinkMessage ужимает центральное сообщение под доступную ширину
// (с учётом обрамляющих пробелов и стиля).
func shrinkMessage(d StatusData, avail int) string {
	if avail < 6 {
		return ""
	}
	style := StatusAccent
	switch d.MessageKind {
	case "success":
		style = StatusSuccess
	case "danger":
		style = StatusDanger
	}
	return " " + style.Render(" "+truncate(d.Message, avail-4)+" ") + " "
}

// truncate обрезает строку до width рун с многоточием.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= width {
		return s
	}
	if width <= 3 {
		return string(rs[:width])
	}
	return string(rs[:width-3]) + "..."
}

// truncateRendered обрезает строку с ANSI-кодами по видимой ширине.
func truncateRendered(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	// Простой путь: обрезаем по рунам, игнорируя позицию кодов (сегменты
	// статусбара невелики — допустимая точность для крайнего случая).
	plain := strings.Map(func(r rune) rune {
		if r == 0x1b {
			return -1
		}
		return r
	}, s)
	return truncate(plain, width)
}

// FormatKey возвращает оформленную клавишу.
func FormatKey(key string) string {
	return Key.Render(key)
}

// FormatHint собирает подсказку вида "key action  key action".
func FormatHint(pairs ...string) string {
	var b strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			b.WriteString(MutedText.Render("  "))
		}
		b.WriteString(Key.Render(pairs[i]))
		b.WriteString(MutedText.Render(" " + pairs[i+1]))
	}
	return b.String()
}
