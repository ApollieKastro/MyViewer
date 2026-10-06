// Package app — корневая модель TUI-приложения: режимы (браузер/просмотр/
// редактор/настройки/справка), глобальные клавиши и маршрутизация мыши.
package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mviewer/internal/browser"
	"mviewer/internal/config"
	"mviewer/internal/document"
	"mviewer/internal/editor"
	"mviewer/internal/render"
	"mviewer/internal/ui"
	"mviewer/internal/viewer"
)

// Mode — верхнеуровневый режим приложения.
type Mode uint8

const (
	// ModeBrowse — выбор файла в директории.
	ModeBrowse Mode = iota
	// ModeView — просмотр markdown.
	ModeView
	// ModeEdit — редактирование.
	ModeEdit
	// ModeSettings — модальное окно настроек.
	ModeSettings
	// ModeHelp — модальное окно справки.
	ModeHelp
	// ModeLinks — модальный список ссылок документа.
	ModeLinks
)

// statusMsg — временное сообщение в статусбаре.
type statusMsg struct {
	text string
	kind string // info | success | danger
	at   time.Time
}

// renderMsg — результат асинхронного рендеринга markdown.
type renderMsg struct {
	lines []string
	err   error
}

// externalDoneMsg — завершился внешний редактор (nvim/vim).
type externalDoneMsg struct {
	err error
}

// linkDoneMsg — завершилось открытие ссылки во внешней программе (xdg-open).
type linkDoneMsg struct {
	err    error
	target string
}

// Model — корневая модель приложения (интерфейс tea.Model).
type Model struct {
	cfg  *config.Config
	ren  *render.Renderer
	path string // исходный путь из CLI (для запуска без аргументов)

	doc    *document.Document
	browse browser.Model
	view   viewer.Model
	edit   *editor.Editor

	mode     Mode
	prevMode Mode // откуда пришли в help/settings/links
	stg      *ui.Settings

	// Список ссылок документа (ModeLinks) и каталог для разрешения
	// относительных путей.
	links    *ui.LinkList
	linksDir string

	// История переходов по ссылкам между файлами (ctrl+o / ctrl+i).
	history navigationHistory

	// Поиск в просмотре («/»): ввод запроса, найденные совпадения
	// (индексы строк рендера) и строка последнего перехода.
	searchActive  bool
	searchInput   string
	searchQuery   string
	searchMatches []int
	searchPos     int // строка последнего перехода (-1 — не было)

	width, height int
	bodyH         int

	msg      statusMsg
	quit     bool
	pendingQ bool // ожидание второго q при несохранённых изменениях

	// lastRenderWidth — ширина, для которой выполнен рендер.
	lastRenderWidth int
}

// New создаёт модель: определяет, что открыть по аргументу командной строки.
// darkBackground — фон терминала, детектированный до запуска TUI (для auto-темы).
func New(path string, cfg *config.Config, darkBackground bool) *Model {
	m := &Model{
		cfg:    cfg,
		ren:    render.New(cfg.Theme, darkBackground),
		path:   path,
		edit:   editor.New(),
		browse: browser.New("."),
	}
	m.view.SetOptions(cfg.VimMode, cfg.WheelLines, cfg.ScrollStep)
	m.edit.SetOptions(editor.Options{
		Vim:         cfg.VimMode,
		LineNumbers: cfg.ShowLineNumbers,
		WheelLines:  cfg.WheelLines,
	})
	m.mode = ModeBrowse

	// Определяем, что открыть.
	switch {
	case path == "":
		// Без аргумента: README → другой markdown → текст → браузер.
		if f := document.FindDocument("."); f != "" {
			m.openFile(f)
		} else {
			m.browse = browser.New(".")
			m.mode = ModeBrowse
		}
	default:
		info, err := statPath(path)
		switch {
		case err != nil:
			m.setMsg("не удалось открыть: "+err.Error(), "danger")
		case info:
			// Директория — браузер.
			m.browse = browser.New(path)
			m.mode = ModeBrowse
		default:
			m.openFile(path)
		}
	}
	return m
}

// statPath возвращает true, если путь — директория.
func statPath(path string) (isDir bool, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("нет такого пути: %w", err)
	}
	return info.IsDir(), nil
}

// Init — стартовые команды (bubbletea интерфейс).
func (m *Model) Init() tea.Cmd {
	return nil
}

// ---- Команды ----

// fileLabel — короткая метка типа файла для статусбара: md для markdown,
// расширение файла (go/py/json) для остальных, txt — если расширения нет.
func fileLabel(doc *document.Document) string {
	if doc.Kind == document.KindMarkdown {
		return "md"
	}
	if ext := strings.TrimPrefix(filepath.Ext(doc.Path), "."); ext != "" {
		return ext
	}
	return "txt"
}

// renderContent рендерит документ по его типу: markdown — glamour,
// текст/код — подсветка синтаксиса (chroma; без лексера — сырой режим).
func renderContent(ren *render.Renderer, doc *document.Document, width int) ([]string, error) {
	if doc.Kind == document.KindMarkdown {
		return ren.Render(doc.Content(), width)
	}
	return ren.RenderCode(doc.Content(), width, filepath.Base(doc.Path))
}

// renderCmd — асинхронный рендер документа (не блокирует цикл событий).
// Содержимое, тип и имя файла копируются в замыкание: команда может
// выполниться после того, как пользователь уже открыл другой файл.
func (m *Model) renderCmd() tea.Cmd {
	src, kind, name := "", document.KindMarkdown, ""
	if m.doc != nil {
		src, kind, name = m.doc.Content(), m.doc.Kind, filepath.Base(m.doc.Path)
	}
	width := m.width
	ren := m.ren
	return func() tea.Msg {
		var (
			lines []string
			err   error
		)
		if kind == document.KindMarkdown {
			lines, err = ren.Render(src, width)
		} else {
			lines, err = ren.RenderCode(src, width, name)
		}
		return renderMsg{lines: lines, err: err}
	}
}

// saveCmd — команды после сохранения не требуются, всё синхронно.

// ---- Обновление ----

// Update — центральный диспетчер событий.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.onResize(msg)

	case renderMsg:
		if msg.err != nil {
			m.setMsg("ошибка рендера: "+msg.err.Error(), "danger")
			return m, nil
		}
		// Содержимое изменилось — старые совпадения невалидны.
		m.resetSearch()
		m.view.SetContent(msg.lines)
		return m, nil

	case editor.SaveRequestMsg:
		m.saveDocument()
		return m, nil

	case externalDoneMsg:
		if msg.err != nil {
			m.setMsg("редактор завершился с ошибкой: "+msg.err.Error(), "danger")
		}
		// Файл мог измениться извне — перечитываем и рендерим заново.
		return m, m.reloadDoc()

	case linkDoneMsg:
		if msg.err != nil {
			m.setMsg("не удалось открыть "+msg.target+": "+msg.err.Error(), "danger")
		} else {
			m.setMsg("открыто: "+msg.target, "success")
		}
		return m, nil

	case ui.CloseSettingsMsg:
		m.closeSettings()
		return m, nil

	case tea.MouseMsg:
		return m.onMouse(msg)

	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

// onResize пересчитывает размеры и запускает ре-рендер при изменении ширины.
func (m *Model) onResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	needRender := msg.Width != m.lastRenderWidth && m.lastRenderWidth != 0
	widthChanged := msg.Width != m.width
	m.width, m.height = msg.Width, msg.Height
	m.bodyH = msg.Height - 1
	if m.bodyH < 1 {
		m.bodyH = 1
	}
	m.view.SetSize(m.width, m.bodyH)
	m.edit.SetSize(m.width, m.bodyH)
	m.browse.SetHeight(m.bodyH)

	if m.doc != nil && (needRender || widthChanged) {
		m.lastRenderWidth = m.width
		return m, m.renderCmd()
	}
	m.lastRenderWidth = m.width
	return m, nil
}

// onMouse маршрутизирует мышиные события активному компоненту.
func (m *Model) onMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Клики/колесо в статусбаре игнорируем.
	if msg.Y >= m.bodyH && m.bodyH > 0 {
		return m, nil
	}
	// Сброс подтверждения выхода при любой активности.
	if m.pendingQ {
		m.pendingQ = false
	}
	switch m.mode {
	case ModeBrowse:
		return m, m.browse.Update(msg)
	case ModeView:
		return m, m.view.Update(msg)
	case ModeEdit:
		return m, m.edit.Update(msg)
	}
	return m, nil
}

// onKey — глобальные клавиши и маршрутизация в режимы.
func (m *Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Ctrl+C — принудительный выход отовсюду.
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	// Любая клавиша снимает подтверждение выхода (кроме самой q).
	if m.pendingQ && key != "q" {
		m.pendingQ = false
	}

	// Модальные режимы браузера (ввод имени, подтверждение удаления)
	// поглощают ВСЕ клавиши: буквы — часть имени, q/esc/f1 не должны
	// закрывать приложение или открывать справку.
	if m.mode == ModeBrowse && m.browse.ModalActive() {
		cmd := m.browse.Update(msg)
		if p := m.browse.OpenPath(); p != "" {
			m.openFile(p)
		}
		return m, cmd
	}

	switch m.mode {
	case ModeSettings:
		return m, m.settings().Update(msg)
	case ModeLinks:
		if m.links == nil { // защита: список не создан
			m.mode = m.prevMode
			return m, nil
		}
		// Повторное o или esc — закрыть список ссылок.
		if key == "esc" || key == "o" {
			m.links = nil
			m.mode = m.prevMode
			return m, nil
		}
		cmd := m.links.Update(msg)
		if target, ok := m.links.TakeOpen(); ok {
			return m.openLinkTarget(target)
		}
		return m, cmd
	case ModeHelp:
		if key == "esc" || key == "?" || key == "q" {
			m.mode = m.prevMode
			return m, nil
		}
		return m, nil
	}

	// Глобальные клавиши (не в настройках/справке).
	if key == "f1" {
		return m.toggleHelp()
	}
	if key == "f2" {
		return m.toggleSettings()
	}
	if key == "ctrl+n" {
		// Переключение номеров строк (влияет на встроенный редактор).
		m.cfg.ShowLineNumbers = !m.cfg.ShowLineNumbers
		m.applyEditOptions()
		if err := m.cfg.Save(); err != nil {
			m.setMsg("номера строк вкл/выкл, но конфиг не сохранился: "+err.Error(), "danger")
			return m, nil
		}
		state := "выключены"
		if m.cfg.ShowLineNumbers {
			state = "включены"
		}
		m.setMsg("номера строк "+state, "success")
		return m, nil
	}

	switch m.mode {
	case ModeBrowse:
		return m.onBrowseKey(msg, key)
	case ModeView:
		return m.onViewKey(msg, key)
	case ModeEdit:
		return m.onEditKey(msg, key)
	}
	return m, nil
}

// onBrowseKey — клавиши файлового браузера.
func (m *Model) onBrowseKey(msg tea.KeyMsg, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "esc":
		return m, tea.Quit
	case "?":
		return m.toggleHelp()
	}
	cmd := m.browse.Update(msg)
	if p := m.browse.OpenPath(); p != "" {
		m.openFile(p)
	}
	return m, cmd
}

// onViewKey — клавиши режима просмотра.
func (m *Model) onViewKey(msg tea.KeyMsg, key string) (tea.Model, tea.Cmd) {
	// Ввод запроса поиска поглощает всю клавиатуру (кроме esc/enter,
	// они разбираются в searchKeyHandle).
	if m.searchActive {
		return m.searchKeyHandle(msg, key)
	}
	// Некоторые клиенты доставляют набранный текст одним пакетом рун
	// (например «/mviewer» за раз): разбираем как последовательность
	// одиночных нажатий, иначе команды вида «/» не распознаются.
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 {
		for _, r := range msg.Runes {
			m.onViewKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}, string(r))
		}
		return m, nil
	}
	switch key {
	case "/":
		// Новый поиск: чистим запрос и вводим строку в статусбаре.
		m.searchActive = true
		m.searchInput = ""
		m.setMsg("/… (enter — искать, esc — отмена)", "info")
		return m, nil
	case "n":
		m.searchJump(+1, false)
		return m, nil
	case "N":
		m.searchJump(-1, false)
		return m, nil
	case "q":
		if m.doc != nil && m.doc.Modified && !m.pendingQ {
			m.pendingQ = true
			m.setMsg("есть несохранённые изменения — ctrl+s сохранить, q — выйти", "danger")
			return m, nil
		}
		return m, tea.Quit
	case "?":
		return m.toggleHelp()
	case "s":
		return m.toggleSettings()
	case "e", "f4":
		return m.editExternal()
	case "h", "backspace":
		// К браузеру — в каталог текущего файла (в браузере h — вверх).
		dir := "."
		if m.doc != nil && m.doc.Path != "" {
			dir = filepath.Dir(m.doc.Path)
		}
		m.browse = browser.New(dir)
		m.browse.SetHeight(m.bodyH)
		m.mode = ModeBrowse
		m.setMsg("", "")
		return m, nil
	case "ctrl+s":
		m.saveDocument()
		return m, nil
	case "ctrl+o":
		// Назад по истории переходов по ссылкам (как в vim).
		return m.historyJump(false)
	case "tab", "ctrl+i":
		// Вперёд по истории (в терминале ctrl+i приходит как tab,
		// как и в vim — принимаем оба варианта).
		return m.historyJump(true)
	case "o":
		// Список ссылок документа.
		return m.openLinks()
	case "r":
		if m.doc != nil && m.doc.Path != "" {
			if _, err := document.Open(m.doc.Path); err == nil {
				m.openFile(m.doc.Path)
				m.setMsg("перезагружено", "success")
			}
		}
		return m, nil
	}
	return m, m.view.Update(msg)
}

// openLinks открывает модальный список ссылок документа (клавиша o).
func (m *Model) openLinks() (tea.Model, tea.Cmd) {
	if m.doc == nil {
		return m, nil
	}
	links := document.ExtractLinks(m.doc.Content())
	if len(links) == 0 {
		m.setMsg("в документе нет ссылок", "info")
		return m, nil
	}
	m.links = ui.NewLinkList(links)
	m.linksDir = filepath.Dir(m.doc.Path)
	m.prevMode = m.mode
	m.mode = ModeLinks
	m.setMsg("", "")
	return m, nil
}

// openLinkTarget открывает выбранную ссылку:
//   - с URI-схемой (http://, https://, mailto: …) — наружу через xdg-open;
//   - относительный путь — открывается в mvi (файл) либо браузер (каталог);
//   - якорь #… — пока не поддерживается, показывается сообщение.
func (m *Model) openLinkTarget(target string) (tea.Model, tea.Cmd) {
	if strings.HasPrefix(target, "#") {
		m.setMsg("переход по якорям пока не поддерживается", "info")
		return m, nil
	}

	// file:// — локальный файл или каталог: открываем в этом же
	// терминале (бесшовно), наружу через xdg-open не отдаём.
	if p, ok := document.FileURLPath(target); ok {
		target = p
	}

	// Внешняя ссылка — наружу. Управление терминалом отдаётся xdg-open
	// через tea.ExecProcess (как внешнему редактору) и возвращается после.
	if document.HasScheme(target) {
		m.links = nil
		m.mode = m.prevMode
		cmd := tea.ExecProcess(exec.Command("xdg-open", target), func(err error) tea.Msg {
			return linkDoneMsg{err: err, target: target}
		})
		return m, cmd
	}

	// Относительный путь — резолвим к каталогу документа.
	path := target
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.linksDir, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		m.setMsg("не найдено: "+target, "danger")
		return m, nil
	}
	m.links = nil
	m.mode = m.prevMode
	if info.IsDir() {
		m.browse = browser.New(path)
		m.browse.SetHeight(m.bodyH)
		m.mode = ModeBrowse
		return m, nil
	}
	// Переход между файлами: запоминаем текущий файл и позицию,
	// чтобы ctrl+o вернул сюда (история ссылок).
	m.pushHistory()
	m.openFile(path)
	return m, nil
}

// pushHistory запоминает текущую позицию перед переходом по ссылке.
func (m *Model) pushHistory() {
	if m.doc == nil || m.doc.Path == "" {
		return
	}
	m.history.push(historyEntry{path: m.doc.Path, offset: m.view.Offset()})
}

// historyJump — переход назад (forward=false) или вперёд (true)
// по истории переходов по ссылкам. Позиция прокрутки целевого файла
// восстанавливается; при недоступном файле история не меняется.
func (m *Model) historyJump(forward bool) (tea.Model, tea.Cmd) {
	var cur historyEntry
	if m.doc != nil && m.doc.Path != "" {
		cur = historyEntry{path: m.doc.Path, offset: m.view.Offset()}
	}

	var (
		e   historyEntry
		ok  bool
		err error
	)
	if forward {
		e, ok = m.history.forwardStep(cur)
	} else {
		e, ok = m.history.backward(cur)
	}
	if !ok {
		m.setMsg("нет истории переходов", "info")
		return m, nil
	}

	// Файл мог исчезнуть — тогда не прыгаем и возвращаем стеки как были.
	if _, serr := os.Stat(e.path); serr != nil {
		err = serr
	}
	if err == nil && !m.openFile(e.path) {
		err = fmt.Errorf("не удалось открыть %s", e.path)
	}
	if err != nil {
		// Возвращаем извлечённую позицию обратно, cur не переносим.
		if forward {
			m.history.forward = append(m.history.forward, e)
		} else {
			m.history.back = append(m.history.back, e)
		}
		m.setMsg("файл истории недоступен: "+filepath.Base(e.path), "danger")
		return m, nil
	}
	m.view.SetOffset(e.offset)
	return m, nil
}

// ---- Поиск в просмотре ----

// searchKeyHandle — клавиатура во время ввода запроса (/).
func (m *Model) searchKeyHandle(msg tea.KeyMsg, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.searchActive = false
		m.searchInput = ""
		m.setMsg("", "")
		return m, nil
	case "enter":
		m.searchActive = false
		m.runSearch()
		return m, nil
	case "backspace":
		// Удаляем последнюю руну (кириллица/эмодзи — не байт).
		if r := []rune(m.searchInput); len(r) > 0 {
			m.searchInput = string(r[:len(r)-1])
		}
		m.setMsg("/"+m.searchInput+"… (enter — искать, esc — отмена)", "info")
		return m, nil
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
		m.searchInput += string(msg.Runes)
		m.setMsg("/"+m.searchInput+"… (enter — искать, esc — отмена)", "info")
	}
	return m, nil
}

// runSearch выполняет первый поиск: собирает совпадения и прыгает
// к ближайшему от текущей позиции (включая текущую строку).
func (m *Model) runSearch() {
	q := strings.TrimSpace(m.searchInput)
	if q == "" {
		m.setMsg("пустой запрос", "info")
		return
	}
	m.searchQuery = q
	m.searchMatches = m.view.Search(q)
	m.searchPos = -1
	if len(m.searchMatches) == 0 {
		m.setMsg("нет совпадений: "+q, "info")
		return
	}
	m.searchJump(+1, true)
}

// searchJump переходит к следующему (dir=+1) или предыдущему (dir=-1)
// совпадению и центрирует его в экране. inclusive — включать позицию
// поиска в проверку (нужно только первому поиску).
func (m *Model) searchJump(dir int, inclusive bool) {
	if m.searchQuery == "" {
		m.setMsg("нет запроса — нажмите /", "info")
		return
	}
	if len(m.searchMatches) == 0 {
		m.searchMatches = m.view.Search(m.searchQuery)
		if len(m.searchMatches) == 0 {
			m.setMsg("нет совпадений: "+m.searchQuery, "info")
			return
		}
	}
	from := m.searchPos
	if from < 0 || inclusive {
		from = m.view.Offset()
	}
	mi, ok := viewer.NextMatch(m.searchMatches, from, dir, inclusive || m.searchPos < 0)
	if !ok {
		m.setMsg("нет совпадений: "+m.searchQuery, "info")
		return
	}
	m.searchPos = mi
	// Центрируем найденную строку в теле просмотра.
	off := mi - max(m.bodyH/2, 0)
	if off < 0 {
		off = 0
	}
	m.view.SetOffset(off)
	// Порядковый номер среди всех совпадений (1-based).
	num := 1
	for _, x := range m.searchMatches {
		if x >= mi {
			break
		}
		num++
	}
	m.setMsg(fmt.Sprintf("%s · совпадение %d/%d · строка %d",
		m.searchQuery, num, len(m.searchMatches), mi+1), "success")
}

// resetSearch сбрасывает поиск (контент изменился: новый файл, resize).
func (m *Model) resetSearch() {
	m.searchActive = false
	m.searchInput = ""
	m.searchQuery = ""
	m.searchMatches = nil
	m.searchPos = -1
}

// onEditKey — клавиши режима редактирования.
func (m *Model) onEditKey(msg tea.KeyMsg, key string) (tea.Model, tea.Cmd) {
	// F4 — переключение к просмотру.
	if key == "f4" {
		m.stopEdit()
		return m, nil
	}
	cmd := m.edit.Update(msg)
	// Esc в normal-режиме — возврат к просмотру.
	if m.edit.ConsumeExit() {
		stopCmd := m.stopEdit()
		return m, tea.Batch(cmd, stopCmd)
	}
	return m, cmd
}

// ---- Режимы ----

// applyEditOptions переносит актуальные настройки в редактор
// (вызывается при изменении конфига на лету: ctrl+n, закрытие настроек).
func (m *Model) applyEditOptions() {
	m.edit.SetOptions(editor.Options{
		Vim:         m.cfg.VimMode,
		LineNumbers: m.cfg.ShowLineNumbers,
		WheelLines:  m.cfg.WheelLines,
	})
}

// startEdit переходит в режим редактирования.
func (m *Model) startEdit() {
	if m.doc == nil {
		return
	}
	m.edit.Load(strings.Join(m.doc.Lines, "\n"))
	m.edit.SetSize(m.width, m.bodyH)
	m.applyEditOptions()
	m.mode = ModeEdit
	m.setMsg("", "")
}

// stopEdit выходит из редактора: синхронизирует документ и рендерит markdown.
func (m *Model) stopEdit() tea.Cmd {
	if m.doc != nil {
		m.doc.ReplaceLines(m.edit.Lines())
	}
	m.mode = ModeView
	m.lastRenderWidth = m.width
	// Асинхронный ре-рендер: текст мог измениться.
	return m.renderCmd()
}

// editExternal открывает файл по клавише e: сначала внешний редактор
// (neovim → vim, см. pickExternalEditor), при их отсутствии — встроенный.
//
// Внешний редактор работает с файлом на диске, поэтому несохранённые
// изменения сначала фиксируются на диск. Управление терминалом отдаётся
// редактору через tea.ExecProcess и возвращается после его выхода.
func (m *Model) editExternal() (tea.Model, tea.Cmd) {
	if m.doc == nil || m.doc.Path == "" {
		m.setMsg("нет открытого файла — выберите его в браузере", "danger")
		return m, nil
	}
	// Правки во встроенном редакторе могли не попасть на диск.
	if m.mode == ModeView && m.doc.Modified {
		if !m.saveDocument() {
			return m, nil // причина уже показана в статусбаре
		}
		m.setMsg("сохранено перед запуском редактора", "success")
	}

	bin := pickExternalEditor(m.cfg.Editor)
	if bin == "" {
		// Внешних редакторов нет — встроенный.
		m.startEdit()
		return m, nil
	}

	cmd := tea.ExecProcess(exec.Command(bin, m.doc.Path), func(err error) tea.Msg {
		return externalDoneMsg{err: err}
	})
	return m, cmd
}

// reloadDoc перечитывает документ с диска и запускает ре-рендер.
// Используется после выхода из внешнего редактора.
func (m *Model) reloadDoc() tea.Cmd {
	if m.doc == nil || m.doc.Path == "" {
		return nil
	}
	d, err := document.Open(m.doc.Path)
	if err != nil {
		m.setMsg("не удалось перечитать файл: "+err.Error(), "danger")
		return nil
	}
	m.doc = d
	m.pendingQ = false
	// Ре-рендер принудительно: ширина не изменилась, кэш сбрасываем
	// сменой lastRenderWidth — проще явно отрендерить.
	lines, rerr := renderContent(m.ren, m.doc, max(m.width, 40))
	if rerr != nil {
		m.setMsg("ошибка рендера: "+rerr.Error(), "danger")
		return nil
	}
	m.view.SetContent(lines)
	m.lastRenderWidth = max(m.width, 40)
	return nil
}

// saveDocument сохраняет документ (Ctrl+S / подтверждение выхода).
// Возвращает успешность — вызывающие решают, продолжать ли операцию.
func (m *Model) saveDocument() bool {
	if m.doc == nil {
		return false
	}
	if m.mode == ModeEdit {
		m.doc.ReplaceLines(m.edit.Lines())
	}
	if m.doc.Path == "" {
		m.setMsg("файл не выбран", "danger")
		return false
	}
	if err := m.doc.Save(); err != nil {
		m.setMsg("ошибка сохранения: "+err.Error(), "danger")
		return false
	}
	m.edit.ResetModified()
	m.pendingQ = false
	m.setMsg("сохранено", "success")
	return true
}

// ---- Настройки и справка ----

// settings возвращает активное окно настроек (создаёт при необходимости).
func (m *Model) settings() *ui.Settings {
	if m.stg == nil {
		m.stg = ui.NewSettings(m.cfg)
	}
	return m.stg
}

// toggleSettings открывает/закрывает настройки.
func (m *Model) toggleSettings() (tea.Model, tea.Cmd) {
	if m.mode == ModeSettings {
		m.closeSettings()
		return m, nil
	}
	m.prevMode = m.mode
	m.stg = ui.NewSettings(m.cfg)
	m.mode = ModeSettings
	return m, nil
}

// closeSettings применяет и сохраняет настройки.
func (m *Model) closeSettings() {
	if m.stg == nil {
		m.mode = m.prevMode
		return
	}
	cfg, touched := m.stg.Result()
	m.stg = nil
	m.mode = m.prevMode
	if !touched {
		return
	}
	*m.cfg = *cfg
	if err := m.cfg.Save(); err != nil {
		m.setMsg("не удалось сохранить конфиг: "+err.Error(), "danger")
		return
	}
	// Применяем изменения.
	m.view.SetOptions(m.cfg.VimMode, m.cfg.WheelLines, m.cfg.ScrollStep)
	m.applyEditOptions()
	changedTheme := m.ren.Theme() != m.cfg.Theme
	m.ren.SetTheme(m.cfg.Theme)
	m.setMsg("настройки сохранены", "success")
	if changedTheme && m.doc != nil {
		m.mode = ModeView
		m.lastRenderWidth = m.width
		return
	}
	m.lastRenderWidth = m.width
}

// toggleHelp открывает/закрывает справку.
func (m *Model) toggleHelp() (tea.Model, tea.Cmd) {
	if m.mode == ModeHelp {
		m.mode = m.prevMode
		return m, nil
	}
	m.prevMode = m.mode
	m.mode = ModeHelp
	return m, nil
}

// ---- Открытие файлов ----

// openFile открывает markdown-файл: читает, рендерит, входит в режим просмотра.
// Возвращает успешность (для истории переходов: недоступный файл не прыгает).
func (m *Model) openFile(path string) bool {
	doc, err := document.Open(path)
	if err != nil {
		m.setMsg(err.Error(), "danger")
		return false
	}
	m.doc = doc
	m.pendingQ = false
	m.view.SetOptions(m.cfg.VimMode, m.cfg.WheelLines, m.cfg.ScrollStep)

	// Стартовый рендер — синхронный (вызывается до запуска цикла событий
	// либо в момент открытия: укладывается в несколько миллисекунд).
	lines, rerr := renderContent(m.ren, doc, max(m.width, 40))
	if rerr != nil {
		m.setMsg("ошибка рендера: "+rerr.Error(), "danger")
		lines = []string{"(ошибка отрисовки)"}
	}
	m.view.SetContent(lines)
	m.view.ScrollToTop()
	m.lastRenderWidth = max(m.width, 40)
	// Новый документ — поиск начинается заново.
	m.resetSearch()

	if m.cfg.StartInEditMode {
		m.startEdit()
	} else {
		m.mode = ModeView
	}
	m.setMsg("", "")
	return true
}

// ---- Сообщения ----

// setMsg ставит временное сообщение в статусбар ("" — очистить).
func (m *Model) setMsg(text, kind string) {
	if text == "" {
		m.msg = statusMsg{}
		return
	}
	m.msg = statusMsg{text: text, kind: kind, at: time.Now()}
}

// ---- Отрисовка ----

// View собирает экран: тело режима + строка состояния (или модалка).
func (m *Model) View() string {
	if m.quit {
		return ""
	}

	var body string
	switch m.mode {
	case ModeBrowse:
		body = m.browse.View(m.width)
	case ModeEdit:
		body = m.edit.View()
	case ModeSettings:
		body = m.settings().View(m.width, m.bodyH)
	case ModeLinks:
		body = m.links.View(m.width, m.bodyH)
	case ModeHelp:
		body = ui.RenderHelp(m.width, m.bodyH, m.helpSections())
	default: // ModeView
		if m.doc == nil {
			body = ui.MutedText.Render("  нет открытого файла — q для выхода")
		} else {
			body = m.view.View()
		}
	}
	// Гарантируем высоту тела (иначе статусбар «уплывёт»).
	body = fitHeight(body, m.bodyH)

	return body + "\n" + m.statusBar()
}

// fitHeight дополняет/обрезает строки до нужной высоты.
func fitHeight(s string, height int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		return strings.Join(lines[:height], "\n")
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// statusBar рисует строку состояния по текущему режиму.
func (m *Model) statusBar() string {
	d := ui.StatusData{Percent: -1, ShowHints: true}

	switch m.mode {
	case ModeBrowse:
		d.FileName = m.browse.Dir()
		d.Mode = "BROWSE"
		d.HintLeft = "enter открыть"
		d.HintRight = "s настройки  ? справка  q выход"

	case ModeSettings:
		d.FileName = "настройки"
		d.Mode = "SETTINGS"
		d.HintLeft = "esc закрыть"

	case ModeLinks:
		d.FileName = "ссылки документа"
		d.Mode = "LINKS"
		d.HintLeft = "enter открыть"
		d.HintRight = "esc закрыть"

	case ModeHelp:
		d.FileName = "справка"
		d.Mode = "HELP"
		d.HintLeft = "esc закрыть"

	case ModeEdit:
		if m.doc != nil {
			d.FileName = m.doc.Name()
			d.Modified = m.doc.Modified || m.edit.Modified()
			d.Percent = m.edit.Percent()
		}
		d.Mode = "EDIT"
		if m.cfg.VimMode {
			d.SubMode = m.edit.Mode().String()
			if p := m.edit.PendingLabel(); p != "" {
				d.SubMode += " · " + p
			}
		} else {
			d.SubMode = "plain"
		}
		d.HintLeft = "ctrl+s сохранить"
		d.HintRight = "f4 к просмотру  f1 справка"

	default: // ModeView
		if m.doc != nil {
			d.FileName = m.doc.Path
			if m.doc.Modified {
				d.Modified = true
			}
			d.Percent = m.view.Percent()
			// Метка типа: md — markdown, иначе расширение файла (go/py/txt).
			d.SubMode = fileLabel(m.doc)
		} else {
			d.FileName = "—"
		}
		d.Mode = "VIEW"
		d.HintLeft = "e редактировать"
		d.HintRight = "s настройки  ? справка  q выход"
	}

	// Сообщение с приоритетом.
	if m.msg.text != "" {
		// Сообщения об успехе гаснут через 3 секунды.
		if m.msg.kind == "success" && time.Since(m.msg.at) > 3*time.Second {
			m.msg = statusMsg{}
		} else {
			d.Message = m.msg.text
			d.MessageKind = m.msg.kind
		}
	}

	return ui.RenderStatusBar(m.width, d)
}

// helpSections собирает справку по текущему режиму.
func (m *Model) helpSections() []ui.HelpSection {
	switch m.prevMode {
	case ModeBrowse:
		return ui.HelpForBrowse()
	case ModeEdit:
		return ui.HelpForEdit(m.cfg.VimMode)
	case ModeLinks:
		return ui.HelpForLinks()
	default:
		return ui.HelpForView(m.cfg.VimMode)
	}
}
