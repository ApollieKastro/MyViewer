package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// HelpSection — раздел справки (заголовок + пары клавиша/описание).
type HelpSection struct {
	Title string
	Keys  []HelpKey
}

// HelpKey — одна строка справки.
type HelpKey struct {
	Key  string
	Desc string
}

// RenderHelp рисует модальное окно справки, центрированное в width×height.
func RenderHelp(width, height int, sections []HelpSection) string {
	var b strings.Builder
	b.WriteString(Title.Render("Справка") + "\n")
	b.WriteString(MutedText.Render("esc или ? — закрыть") + "\n\n")

	for _, sec := range sections {
		b.WriteString(lipgloss.NewStyle().Bold(true).Render(sec.Title) + "\n")
		for _, k := range sec.Keys {
			key := FormatKey(k.Key)
			// Выравнивание колонки клавиш.
			pad := 12 - lipgloss.Width(key)
			if pad < 1 {
				pad = 1
			}
			b.WriteString("  " + key + strings.Repeat(" ", pad) + Desc.Render(k.Desc) + "\n")
		}
		b.WriteString("\n")
	}

	panel := Panel.Render(strings.TrimRight(b.String(), "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}

// HelpForView — справка режима просмотра.
func HelpForView(vim bool) []HelpSection {
	nav := []HelpKey{
		{"↑/↓", "листать по строке"},
		{"PgUp/PgDn", "страница"},
		{"home/end", "начало/конец"},
		{"mouse", "колесо — прокрутка"},
	}
	if vim {
		nav = append(nav,
			HelpKey{"j/k", "вниз/вверх"},
			HelpKey{"g/G", "начало/конец документа"},
			HelpKey{"ctrl+d/u", "половина страницы"},
			HelpKey{"f/b", "страница вперёд/назад"},
		)
	}
	return []HelpSection{
		{Title: "Навигация", Keys: nav},
		{Title: "Действия", Keys: []HelpKey{
			{"e", "редактировать (nvim → vim → встроенный)"},
			{"o", "список ссылок документа (enter — открыть)"},
			{"ctrl+o / tab", "переходы по ссылкам: назад / вперёд (как в vim)"},
			{"/", "поиск по документу (enter — искать, esc — отмена)"},
			{"n / N", "следующее / предыдущее совпадение"},
			{"ctrl+n", "номера строк вкл/выкл"},
			{"s", "настройки"},
			{"?", "эта справка"},
			{"q", "выйти (повторное нажатие — при несохранённых)"},
		}},
	}
}

// HelpForLinks — справка списка ссылок документа.
func HelpForLinks() []HelpSection {
	return []HelpSection{
		{Title: "Ссылки", Keys: []HelpKey{
			{"↑/↓ или j/k", "выбор ссылки"},
			{"enter / o", "открыть ссылку"},
			{"esc / o", "закрыть список"},
		}},
	}
}

// HelpForEdit — справка режима редактирования.
func HelpForEdit(vim bool) []HelpSection {
	if !vim {
		return []HelpSection{
			{Title: "Редактирование", Keys: []HelpKey{
				{"esc", "вернуться к просмотру"},
				{"ctrl+s", "сохранить файл"},
				{"ctrl+n", "номера строк вкл/выкл"},
				{"↑/↓/←/→", "курсор"},
				{"home/end", "начало/конец строки"},
				{"mouse", "клик — курсор, колесо — прокрутка"},
			}},
		}
	}
	return []HelpSection{
		{Title: "Vim: режимы", Keys: []HelpKey{
			{"i/a", "вставка до/после курсора"},
			{"esc", "нормальный режим, затем — выход к просмотру"},
			{"v/V", "выделение: символов/строк"},
			{"o/O", "новая строка ниже/выше"},
		}},
		{Title: "Vim: навигация", Keys: []HelpKey{
			{"h j k l", "курсор"},
			{"w/b/e", "вперёд/назад/конец слова"},
			{"0 ^ $", "начало/первый символ/конец строки"},
			{"gg/G", "первая/последняя строка"},
			{"{n}G/%", "к строке / к проценту"},
			{"f/F/t/T{x}", "поиск символа в строке"},
			{"ctrl+d/u", "половина страницы"},
		}},
		{Title: "Vim: правка", Keys: []HelpKey{
			{"d/y/c + motion", "удалить/копировать/изменить"},
			{"dd/yy/cc", "операция над строками"},
			{"x/s", "символ / замена символа"},
			{"D/C/Y", "до конца строки"},
			{"p/P", "вставить после/перед"},
			{"r{char}/~", "заменить символ / регистр"},
			{"J", "объединить строки"},
			{"u/ctrl+r", "отменить/повторить"},
			{"{n}x, 2dw", "счётчики"},
		}},
		{Title: "Vim: объекты (после d/y/c или в visual)", Keys: []HelpKey{
			{"diw/daw", "слово / слово с пробелом"},
			{`di"/da(`, "внутри кавычек/скобок"},
		}},
		{Title: "Файл", Keys: []HelpKey{
			{"ctrl+s", "сохранить файл"},
			{"ctrl+n", "номера строк вкл/выкл"},
			{"esc", "выйти к просмотру"},
		}},
	}
}

// HelpForBrowse — справка файлового браузера.
func HelpForBrowse() []HelpSection {
	return []HelpSection{
		{Title: "Файлы", Keys: []HelpKey{
			{"↑/↓", "выбор файла"},
			{"enter / l / L", "открыть выбранный файл или каталог"},
			{"n", "новый файл (создать и открыть)"},
			{"N", "новый каталог"},
			{"d", "удалить (d — подтвердить, esc — отмена)"},
			{"←/h", "каталог выше"},
			{"r", "обновить список"},
			{"q", "выйти"},
			{"?", "эта справка"},
		}},
	}
}
