// Package browser — файловый браузер для открытия markdown-файлов
// из директории. Показывает сначала markdown и подкаталоги (как glow).
// Умеет создавать новые файлы и каталоги (клавиши n/N с вводом имени).
package browser

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"mviewer/internal/document"
	"mviewer/internal/ui"
)

// Model — состояние браузера.
type Model struct {
	dir     string
	entries []document.Entry
	cursor  int
	offset  int
	height  int
	err     error

	// OpenRequest — сигнал приложению открыть файл.
	openPath string

	// Режим ввода имени для создания: "" — выключен,
	// "file" / "dir" — создаём файл или каталог.
	inputMode string
	inputName []rune
	inputErr  string

	// deleteConfirm — имя элемента, ожидающего подтверждения удаления
	// ("" — подтверждения не требуется).
	deleteConfirm string
	// msg / msgErr — последнее сообщение (уведомление либо ошибка).
	msg    string
	msgErr bool
}

// New создаёт браузер для каталога.
func New(dir string) Model {
	m := Model{dir: dir}
	m.reload()
	return m
}

// Dir возвращает текущий каталог.
func (m *Model) Dir() string { return m.dir }

// OpenPath забирает запрошенный для открытия путь ("" если не запрашивался).
func (m *Model) OpenPath() string {
	p := m.openPath
	m.openPath = ""
	return p
}

// InputActive — идёт ли сейчас ввод имени нового файла/каталога.
// Пока активен, приложение обязано отдавать браузеру ВСЕ клавиши:
// буквы — часть имени, q/esc не должны закрывать приложение.
func (m *Model) InputActive() bool { return m.inputMode != "" }

// ModalActive — активен ли модальный режим браузера (ввод имени либо
// подтверждение удаления): приложение отдаёт браузеру все клавиши,
// глобальные обработчики (q — выход, ? — справка) не срабатывают.
func (m *Model) ModalActive() bool {
	return m.inputMode != "" || m.deleteConfirm != ""
}

// SetHeight устанавливает высоту списка.
func (m *Model) SetHeight(h int) {
	m.height = h
	m.clamp()
}

// reload перечитывает каталог.
func (m *Model) reload() {
	entries, err := document.ListDir(m.dir)
	if err != nil {
		m.err = err
		m.entries = nil
		return
	}
	m.err = nil
	m.entries = entries
	m.cursor, m.offset = 0, 0
	m.clamp()
}

// clamp удерживает курсор и прокрутку в границах.
func (m *Model) clamp() {
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.height > 0 {
		if m.cursor < m.offset {
			m.offset = m.cursor
		}
		if m.cursor >= m.offset+m.height {
			m.offset = m.cursor - m.height + 1
		}
		maxOff := len(m.entries) - m.height
		if maxOff < 0 {
			maxOff = 0
		}
		if m.offset > maxOff {
			m.offset = maxOff
		}
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// Selected возвращает выбранный элемент (nil, если список пуст).
func (m *Model) Selected() *document.Entry {
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return nil
	}
	return &m.entries[m.cursor]
}

// Update обрабатывает ввод в браузере.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.move(-1)
		case tea.MouseButtonWheelDown:
			m.move(1)
		case tea.MouseButtonLeft:
			// Первые 3 строки View — заголовок, подсказка и разделитель.
			const listOffset = 3
			if msg.Action == tea.MouseActionPress &&
				msg.Y >= listOffset && msg.Y-listOffset < m.height {
				m.cursor = m.offset + (msg.Y - listOffset)
				m.clamp()
			}
		}
		return nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return nil
}

// handleKey — навигация по файлам; в модальных режимах — ввод/подтверждение.
func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	if m.deleteConfirm != "" {
		return m.handleDeleteKey(msg)
	}
	if m.inputMode != "" {
		return m.handleInputKey(msg)
	}
	key := msg.String()
	m.msg = "" // любое действие снимает прошлое уведомление

	// Vim-раскладка: j/k и h/l дополнительно к стрелкам.
	switch key {
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "pgup":
		m.move(-m.pageSize())
	case "pgdown":
		m.move(m.pageSize())
	case "home", "g":
		m.cursor = 0
		m.clamp()
	case "end", "G":
		m.cursor = len(m.entries) - 1
		m.clamp()
	case "enter", "l", "L":
		// Открыть выбранный элемент: файл — сигнал открытия,
		// каталог — вход в него (l/L — vim-альтернатива enter).
		m.openSelected()
	case "left", "h", "backspace":
		m.goUp()
	case "r":
		m.reload()
	case "n":
		m.startInput("file")
	case "N":
		m.startInput("dir")
	case "d":
		m.startDelete()
	}
	return nil
}

// startDelete запрашивает подтверждение удаления выбранного элемента.
func (m *Model) startDelete() {
	if e := m.Selected(); e != nil {
		m.deleteConfirm = e.Name
		m.msg = ""
	}
}

// handleDeleteKey — клавиатура режима подтверждения удаления.
// Подтверждение: d/y/enter; отмена: esc/n; любая другая клавиша — отмена
// (случайное нажатие не должно ничего стирать).
func (m *Model) handleDeleteKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "d", "y", "enter":
		m.doDelete()
	default:
		m.deleteConfirm = ""
	}
	return nil
}

// doDelete удаляет подтверждённый элемент. Рекурсивного удаления каталогов
// нет: непустой каталог останется на месте, а ошибка попадёт в msg.
func (m *Model) doDelete() {
	name := m.deleteConfirm
	m.deleteConfirm = ""
	path := filepath.Join(m.dir, name)

	if err := os.Remove(path); err != nil {
		m.msg, m.msgErr = "не удалось удалить: "+err.Error(), true
		return
	}
	old := m.cursor
	m.msg, m.msgErr = "удалено: "+name, false
	m.reload()
	m.cursor = old // остаёмся на той же позиции списка
	m.clamp()
}

// handleInputKey — клавиатура режима создания (ввод имени).
func (m *Model) handleInputKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.inputMode, m.inputName, m.inputErr = "", nil, ""
	case "enter":
		m.submitInput()
	case "backspace":
		if n := len(m.inputName); n > 0 {
			m.inputName = m.inputName[:n-1]
			m.inputErr = ""
		}
	default:
		if msg.Type == tea.KeyRunes && !msg.Alt {
			bad := false
			for _, r := range msg.String() {
				if r == '/' || r == '\\' {
					bad = true
					continue
				}
				m.inputName = append(m.inputName, r)
			}
			if bad {
				m.inputErr = "разделители путей недопустимы"
			} else {
				m.inputErr = ""
			}
		}
	}
	return nil
}

// startInput включает режим ввода имени (file или dir).
func (m *Model) startInput(mode string) {
	m.inputMode = mode
	m.inputName = nil
	m.inputErr = ""
}

// submitInput создаёт файл или каталог по введённому имени.
// При успехе: файл — открывается сразу, каталог — выделяется в списке.
func (m *Model) submitInput() {
	name := strings.TrimSpace(string(m.inputName))
	switch {
	case name == "":
		m.inputErr = "введите имя"
		return
	case name == "." || name == "..":
		m.inputErr = "это имя использовать нельзя"
		return
	case strings.ContainsAny(name, `/\`):
		m.inputErr = "разделители путей недопустимы"
		return
	}
	path := filepath.Join(m.dir, name)
	if _, err := os.Lstat(path); err == nil {
		m.inputErr = "уже существует: " + name
		return
	}

	if m.inputMode == "dir" {
		if err := os.Mkdir(path, 0o755); err != nil {
			m.inputErr = err.Error()
			return
		}
		m.inputMode, m.inputName, m.inputErr = "", nil, ""
		m.reload()
		m.selectByName(name)
		return
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		m.inputErr = err.Error()
		return
	}
	_ = f.Close()
	m.inputMode, m.inputName, m.inputErr = "", nil, ""
	m.reload()
	m.selectByName(name)
	m.openPath = path // новый файл открывается сразу на редактирование
}

// selectByName ставит курсор на элемент с данным именем (после reload).
func (m *Model) selectByName(name string) {
	for i, e := range m.entries {
		if e.Name == name {
			m.cursor = i
			break
		}
	}
	m.clamp()
}

// move сдвигает курсор на delta.
func (m *Model) move(delta int) {
	m.cursor += delta
	m.clamp()
}

// pageSize — высота страницы списка.
func (m *Model) pageSize() int {
	h := m.height - 1
	if h < 1 {
		h = 1
	}
	return h
}

// openSelected открывает выбранный элемент: файл — сигнал открытия,
// каталог — переход внутрь.
func (m *Model) openSelected() {
	e := m.Selected()
	if e == nil {
		return
	}
	if e.IsDir {
		m.enter(e.Path)
		return
	}
	m.openPath = e.Path
}

// enter переходит в каталог.
func (m *Model) enter(dir string) {
	m.dir = dir
	m.reload()
}

// goUp поднимается в родительский каталог.
func (m *Model) goUp() {
	parent := filepath.Dir(m.dir)
	if parent == m.dir {
		return
	}
	old := filepath.Base(m.dir)
	m.dir = parent
	m.reload()
	// Ставим курсор на бывший каталог.
	for i, e := range m.entries {
		if e.IsDir && e.Name == old {
			m.cursor = i
			break
		}
	}
	m.clamp()
}

// View рисует список файлов.
func (m *Model) View(width int) string {
	if width < 10 {
		width = 10
	}
	var b strings.Builder

	b.WriteString(ui.Title.Render(m.dir) + "\n")

	// Вторая строка: подтверждение удаления / ввод имени / подсказка.
	switch {
	case m.deleteConfirm != "":
		b.WriteString(ui.MutedText.Render("  удалить ") +
			ui.FocusedItem.Render(m.deleteConfirm) +
			ui.MutedText.Render("?  d — да · esc — отмена") + "\n")
	case m.inputMode != "":
		label := "новый файл: "
		if m.inputMode == "dir" {
			label = "новый каталог: "
		}
		b.WriteString(ui.MutedText.Render("  "+label) +
			ui.FocusedItem.Render(string(m.inputName)) +
			ui.MutedText.Render("_  enter создать · esc отмена") + "\n")
	default:
		b.WriteString(ui.MutedText.Render(
			"  enter — открыть · n/N — создать · d — удалить · h/l — каталоги · r — обновить · q — выход") + "\n")
	}

	// Третья строка: ошибка ввода, сообщение браузера или пустая (разделитель).
	switch {
	case m.inputErr != "":
		b.WriteString(ui.DangerText.Render("  "+m.inputErr) + "\n")
	case m.msg != "":
		if m.msgErr {
			b.WriteString(ui.DangerText.Render("  "+m.msg) + "\n")
		} else {
			b.WriteString(ui.MutedText.Render("  "+m.msg) + "\n")
		}
	default:
		b.WriteString("\n")
	}

	if m.err != nil {
		b.WriteString(ui.DangerText.Render("  ошибка: " + m.err.Error()))
		return b.String()
	}
	if len(m.entries) == 0 {
		b.WriteString(ui.MutedText.Render("  (пусто)"))
		return b.String()
	}

	visible := m.height - 4 // заголовок + подсказка/ввод + ошибка/пустая
	if visible < 1 {
		visible = 1
	}
	end := m.offset + visible
	if end > len(m.entries) {
		end = len(m.entries)
	}

	for i := m.offset; i < end; i++ {
		e := m.entries[i]
		mark := "  "
		name := e.Name
		if e.IsDir {
			name += "/"
		}
		// Индикатор markdown.
		suffix := ""
		if !e.IsDir && document.IsMarkdown(e.Path) {
			suffix = "  " + ui.MutedText.Render("md")
		}
		if !e.IsDir && !e.ModTime.IsZero() {
			suffix += ui.MutedText.Render("  " + e.ModTime.Format("02 Jan 2006"))
		}

		line := mark + name + suffix
		if i == m.cursor {
			// Подсветка строки выделения.
			styled := ui.FocusedItem.Render(padVisible(" "+name, width-4)) + suffix
			b.WriteString(styled + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

// padVisible дополняет строку пробелами до width.
func padVisible(s string, width int) string {
	w := visibleWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// visibleWidth — видимая ширина строки (список без ANSI, считаем руны).
func visibleWidth(s string) int {
	return len([]rune(s))
}
