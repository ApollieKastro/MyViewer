#!/usr/bin/env bash
# Установщик mvi: установка, обновление и удаление в ~/.local/bin.
# Совместимость: рядом кладётся симлинк mviewer → mvi.
#
# Запуск интерактивно:   ./install.sh
# Запуск с аргументом:   ./install.sh install|update|remove
# Установка одной командой (из pipe):
#   curl -fsSL https://raw.githubusercontent.com/ApollieKastro/MyViewer/main/install.sh | bash
# Без Go на машине готовый бинарник скачивается из GitHub Releases.
set -euo pipefail

APP_NAME="mvi"
LEGACY_NAME="mviewer"          # старое имя команды (оставляем как симлинк)
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
BIN_PATH="$BIN_DIR/$APP_NAME"
LEGACY_PATH="$BIN_DIR/$LEGACY_NAME"
CONFIG_DIR="$HOME/.config/mviewer"   # каталог конфига не переименовываем
REPO_URL="${REPO_URL:-https://github.com/ApollieKastro/MyViewer.git}"
SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-.}")" && pwd)"
TMP_SRC_DIR=""                   # временный клон источников (если потребовался)
INTERACTIVE=0                    # 1 — stdin это терминал, read безопасен
[ -t 0 ] && INTERACTIVE=1

# ---- Цвета (отключаются, если вывод не в терминал) ----
if [ -t 1 ]; then
    C_RESET=$'\033[0m'; C_BOLD=$'\033[1m'
    C_GREEN=$'\033[32m'; C_RED=$'\033[31m'; C_YELLOW=$'\033[33m'
else
    C_RESET=""; C_BOLD=""; C_GREEN=""; C_RED=""; C_YELLOW=""
fi

ok()   { printf '%s✓%s %s\n' "$C_GREEN" "$C_RESET" "$*"; }
warn() { printf '%s!%s %s\n' "$C_YELLOW" "$C_RESET" "$*"; }
err()  { printf '%s✗%s %s\n' "$C_RED" "$C_RESET" "$*" >&2; }

# ---- Источники ----
# При запуске из pipe (curl | bash) скрипт лежит вне репозитория:
# клонируем исходники во временный каталог и собираем оттуда.
ensure_sources() {
    if [ -f "$SRC_DIR/go.mod" ]; then
        return 0   # запуск из клона репозитория — исходники на месте
    fi
    if ! command -v git >/dev/null 2>&1; then
        err "исходники не найдены рядом со скриптом, а git не установлен"
        err "установите git (https://git-scm.com/) или клонируйте репозиторий вручную"
        exit 1
    fi
    TMP_SRC_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mvi-install.XXXXXX")"
    trap 'rm -rf "$TMP_SRC_DIR"' EXIT
    echo "Клонирую исходники: $REPO_URL"
    if ! git clone --depth 1 "$REPO_URL" "$TMP_SRC_DIR/repo" >&2; then
        err "не удалось склонировать $REPO_URL"
        exit 1
    fi
    SRC_DIR="$TMP_SRC_DIR/repo"
}

# ---- Готовый бинарник из GitHub Releases ----
# Платформа по uname: linux/darwin + amd64/arm64.
detect_platform() {
    local os arch
    case "$(uname -s)" in
        Linux)  os="linux" ;;
        Darwin) os="darwin" ;;
        *) err "неподдерживаемая ОС: $(uname -s) (нужен Linux или macOS)"; exit 1 ;;
    esac
    case "$(uname -m)" in
        x86_64|amd64)  arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) err "неподдерживаемая архитектура: $(uname -m)"; exit 1 ;;
    esac
    PLATFORM="$os-$arch"
}

# Скачивает релиз mvi-<platform>.tar.gz во временный каталог.
# Используется, когда Go не установлен — сборка не нужна.
download_release() {
    detect_platform
    local url="https://github.com/ApollieKastro/MyViewer/releases/latest/download/mvi-${PLATFORM}.tar.gz"
    TMP_SRC_DIR="$(mktemp -d "${TMPDIR:-/tmp}/mvi-install.XXXXXX")"
    trap 'rm -rf "$TMP_SRC_DIR"' EXIT
    echo "Go не установлен — скачиваю готовый бинарник ($PLATFORM)..."
    if command -v curl >/dev/null 2>&1; then
        if ! curl -fsSL "$url" -o "$TMP_SRC_DIR/mvi.tar.gz"; then
            err "не удалось скачать $url"
            err "релиз для $PLATFORM ещё не опубликован — установите Go (https://go.dev/dl/)"
            exit 1
        fi
    elif command -v wget >/dev/null 2>&1; then
        if ! wget -qO "$TMP_SRC_DIR/mvi.tar.gz" "$url"; then
            err "не удалось скачать $url"
            err "релиз для $PLATFORM ещё не опубликован — установите Go (https://go.dev/dl/)"
            exit 1
        fi
    else
        err "нужен curl или wget для скачивания готового бинарника (либо установите Go)"
        exit 1
    fi
    if ! tar -xzf "$TMP_SRC_DIR/mvi.tar.gz" -C "$TMP_SRC_DIR"; then
        err "архив релиза повреждён — попробуйте позже"
        exit 1
    fi
    SRC_DIR="$TMP_SRC_DIR"
}

# ---- Получение бинарника ----
# Порядок: go build из исходников → бинарник рядом со скриптом → GitHub Release.
obtain_binary() {
    if command -v go >/dev/null 2>&1; then
        ensure_sources
        echo "Сборка проекта..."
        (cd "$SRC_DIR" && go build -o "$APP_NAME" .)
    elif [ -x "$SRC_DIR/$APP_NAME" ]; then
        warn "go не найден — использую готовый бинарник $SRC_DIR/$APP_NAME"
    else
        download_release
    fi
    [ -x "$SRC_DIR/$APP_NAME" ] || { err "не удалось получить бинарник $APP_NAME"; exit 1; }
}

# Проверка, что BIN_DIR виден из PATH.
check_path() {
    case ":$PATH:" in
        *":$BIN_DIR:"*) return 0 ;;
    esac
    warn "$BIN_DIR не найден в PATH — команда $APP_NAME может не запускаться"
    warn "добавьте в ~/.bashrc:  export PATH=\"\$BIN_DIR:\$PATH\""
    return 0
}

# ---- 1. Установка ----
do_install() {
    if [ -x "$BIN_PATH" ]; then
        warn "$APP_NAME уже установлен: $("$BIN_PATH" --version 2>/dev/null || echo '?')"
        # В pipe (stdin не терминал) вопрос задавать нельзя — перезаписываем.
        if [ "$INTERACTIVE" -eq 1 ]; then
            read -r -p "Перезаписать? [y/N] " answer || answer="y"
        else
            answer="y"
        fi
        case "$answer" in
            [yY]|[yY][eE][sS]|да|Да) ;;
            *) echo "Отменено."; return 0 ;;
        esac
    elif [ -e "$LEGACY_PATH" ]; then
        ok "найдена старая установка $LEGACY_NAME — обновляю до $APP_NAME"
    fi

    obtain_binary
    mkdir -p "$BIN_DIR"
    cp -f "$SRC_DIR/$APP_NAME" "$BIN_PATH"
    chmod 755 "$BIN_PATH"
    # Совместимость: mviewer продолжает работать (симлинк на mvi).
    ln -sf "$APP_NAME" "$LEGACY_PATH"
    ok "установлено: $BIN_PATH ($("$BIN_PATH" --version 2>/dev/null))"
    check_path
    echo "Запуск:  $APP_NAME [опции] [файл.md | директория]"
}

# ---- 2. Обновление ----
do_update() {
    if [ ! -x "$BIN_PATH" ]; then
        if [ -e "$LEGACY_PATH" ]; then
            do_install # миграция со старого имени
            return
        fi
        err "$APP_NAME не установлен — сначала выберите «Установить»"
        exit 1
    fi
    local old
    old="$("$BIN_PATH" --version 2>/dev/null || echo '?')"

    obtain_binary
    cp -f "$SRC_DIR/$APP_NAME" "$BIN_PATH"
    chmod 755 "$BIN_PATH"
    ln -sf "$APP_NAME" "$LEGACY_PATH"
    local new
    new="$("$BIN_PATH" --version 2>/dev/null || echo '?')"
    ok "обновлено: $old → $new"
    check_path
}

# ---- 3. Удаление ----
do_remove() {
    local removed=""
    if [ -x "$BIN_PATH" ]; then
        rm -f "$BIN_PATH"
        removed="$BIN_PATH"
    fi
    if [ -e "$LEGACY_PATH" ] || [ -L "$LEGACY_PATH" ]; then
        rm -f "$LEGACY_PATH"
        removed="${removed:+$removed, }$LEGACY_PATH"
    fi
    if [ -z "$removed" ]; then
        warn "$APP_NAME не установлен ($BIN_PATH отсутствует)"
        return 0
    fi
    ok "удалено: $removed"

    if [ -d "$CONFIG_DIR" ]; then
        # В pipe (stdin не терминал) настройки не трогаем.
        if [ "$INTERACTIVE" -eq 1 ]; then
            read -r -p "Удалить также настройки ($CONFIG_DIR)? [y/N] " answer || answer=""
        else
            answer=""
        fi
        case "$answer" in
            [yY]|[yY][eE][sS]|да|Да)
                rm -rf "$CONFIG_DIR"
                ok "настройки удалены"
                ;;
            *) echo "Настройки сохранены." ;;
        esac
    fi
}

# ---- Интерактивное меню ----
menu() {
    echo "${C_BOLD}Установщик $APP_NAME${C_RESET}"
    echo
    local installed="не установлен"
    if [ -x "$BIN_PATH" ]; then
        installed="установлен: $("$BIN_PATH" --version 2>/dev/null || echo '?') → $BIN_PATH"
    fi
    echo "Текущее состояние: $installed"
    echo
    echo "  1) Установить"
    echo "  2) Обновить"
    echo "  3) Удалить"
    echo "  0) Выход"
    echo
    read -r -p "Выберите пункт: " choice || choice=0
    case "$choice" in
        1) do_install ;;
        2) do_update ;;
        3) do_remove ;;
        0) exit 0 ;;
        *) err "неизвестный пункт: $choice"; exit 1 ;;
    esac
}

# ---- Точка входа ----
case "${1:-}" in
    install|установить)  do_install ;;
    update|обновить)     do_update ;;
    remove|uninstall|удалить) do_remove ;;
    -h|--help|help)
        echo "Использование: ./install.sh [install|update|remove]"
        echo "Без аргументов — интерактивное меню (из pipe — сразу установка)."
        echo "Каталог установки: $BIN_DIR (переопределяется переменной BIN_DIR)"
        ;;
    "")
        # Из pipe (curl | bash) меню не показываем — stdin это сам скрипт.
        if [ "$INTERACTIVE" -eq 1 ]; then menu; else do_install; fi
        ;;
    *) err "неизвестная команда: $1 (см. --help)"; exit 1 ;;
esac
