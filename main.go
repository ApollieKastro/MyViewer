// mvi — быстрый терминальный просмотрщик и редактор markdown.
//
// Запуск: mvi [опции] [файл].
// Режимы: выбор файла (браузер), красивый просмотр (glamour), редактирование
// со встроенным vim-движком, настройки и справка. Поддерживаются мышь
// (колесо, клики) и полная клавиатурная навигация.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"mviewer/internal/app"
	"mviewer/internal/config"
	"mviewer/internal/render"
)

// version — версия приложения.
const version = "1.1.0"

// progName — имя команды (используется в usage и сообщениях).
const progName = "mvi"

func main() {
	var (
		showHelp    bool
		showVersion bool
		style       string
	)

	flag.BoolVar(&showHelp, "h", false, "показать справку")
	flag.BoolVar(&showHelp, "help", false, "показать справку")
	flag.BoolVar(&showVersion, "v", false, "показать версию")
	flag.BoolVar(&showVersion, "version", false, "показать версию")
	flag.StringVar(&style, "style", "", "тема рендера (auto, dark, light, dracula, tokyo-night, pink, ascii, notty)")
	flag.Usage = usage
	flag.Parse()

	if showVersion {
		fmt.Printf("%s %s\n", progName, version)
		return
	}
	if showHelp {
		usage()
		return
	}

	// Конфигурация: файл пользователя + переопределение из --style.
	cfg := config.Load()
	if style != "" {
		if err := cfg.SetTheme(style); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
			os.Exit(2)
		}
	}

	// Фон терминала — до запуска TUI: после Bubble Tea захватывает stdin,
	// и запрос OSC-11 уйдёт event-циклу как «мусорный ввод».
	darkBackground := false
	if cfg.Theme == "auto" || cfg.Theme == "" {
		darkBackground = render.DetectBackground()
	}

	path := flag.Arg(0)

	// Запуск TUI: альт-экран (не засорять скролл терминала) + мышь с
	// нажатиями/кликами (cell motion — колесо, клики, без лишних событий).
	program := tea.NewProgram(
		app.New(path, cfg, darkBackground),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
		os.Exit(1)
	}
}

// usage печатает справку по использованию.
func usage() {
	fmt.Fprintf(os.Stdout, `%s %s — просмотрщик и редактор markdown в терминале

Использование:
  %s [опции] [файл]

  файл — markdown, текст или код. Без аргумента открывается README.md
  текущего каталога (иначе — первый markdown, затем .txt/.log, затем
  файловый браузер). Бинарные файлы не открываются.

Опции:
  -h, --help        показать эту справку
  -v, --version     показать версию
      --style тема  тема рендера: auto, dark, light, dracula,
                    tokyo-night, pink, ascii, notty

Клавиши (основные):
  q           выход          e       редактировать (nvim → vim → встроенный)
  ?           справка        s       настройки
  мышь        колесо/клик    ctrl+s  сохранить (в редакторе)
  ctrl+n      номера строк вкл/выкл
  n/N         новый файл/каталог (в браузере)

Vim mode (включён по умолчанию, настройки — s):
  просмотр:   j/k, g/G, ctrl+d/u, f/b
  редактор:   нормальный режим vim: hjkl, w/b/e, d/y/c, p, u,
              визуальный режим, счётчики, текстовые объекты

Настройки хранятся в ~/.config/mviewer/config.json
`, progName, version, progName)
}
