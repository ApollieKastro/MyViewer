package app

import (
	"os/exec"
	"sync"

	"mviewer/internal/config"
)

// Кэш автопоиска: проверка PATH выполняется один раз на процесс
// (иначе каждый запуск редактора обошёл бы бы по LookPath).
var (
	editorOnce sync.Once
	editorPath string
)

// detectExternalEditor ищет neovim, затем vim. Возвращает путь к бинарнику
// или "", если ни одного внешнего редактора нет.
func detectExternalEditor() string {
	editorOnce.Do(func() {
		for _, name := range []string{"nvim", "vim"} {
			if p, err := exec.LookPath(name); err == nil {
				editorPath = p
				return
			}
		}
	})
	return editorPath
}

// pickExternalEditor выбирает редактор по настройке конфигурации.
// Возвращает путь к бинарнику; "" — использовать встроенный редактор.
func pickExternalEditor(setting string) string {
	switch setting {
	case config.EditorBuiltin:
		return ""
	case config.EditorNvim, config.EditorVim:
		// Принудительный выбор, но при отсутствии бинарника — автофолбэк,
		// чтобы клавиша e никогда не «умирала» молча.
		if p, err := exec.LookPath(setting); err == nil {
			return p
		}
		return detectExternalEditor()
	default: // config.EditorAuto
		return detectExternalEditor()
	}
}
