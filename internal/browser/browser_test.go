package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// runes отправляет введённые символы (как набор текста).
func runes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func keyOf(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

func TestCreateFileOpensIt(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	m.SetHeight(10)

	m.Update(runes("n"))
	if !m.InputActive() {
		t.Fatal("после n режим ввода должен включиться")
	}
	m.Update(runes("note.md"))
	m.Update(keyOf(tea.KeyEnter))

	if m.InputActive() {
		t.Fatal("после enter режим ввода должен выключиться")
	}
	path := filepath.Join(dir, "note.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("файл не создан: %v", err)
	}
	if got := m.OpenPath(); got != path {
		t.Errorf("OpenPath = %q, want %q (файл должен открыться)", got, path)
	}
}

func TestCreateFileQInNameDoesNotQuit(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	m.SetHeight(10)

	m.Update(runes("n"))
	m.Update(runes("q")) // q — часть имени, не выход
	m.Update(runes("uit.md"))
	m.Update(keyOf(tea.KeyEnter))

	if _, err := os.Stat(filepath.Join(dir, "quit.md")); err != nil {
		t.Fatalf("файл с q в имени не создан: %v", err)
	}
}

func TestCreateFileCancel(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	m.SetHeight(10)

	m.Update(runes("n"))
	m.Update(runes("cancel.md"))
	m.Update(keyOf(tea.KeyEsc))

	if m.InputActive() {
		t.Error("esc должен отменить ввод")
	}
	if _, err := os.Stat(filepath.Join(dir, "cancel.md")); err == nil {
		t.Error("после отмены файла быть не должно")
	}
	if p := m.OpenPath(); p != "" {
		t.Errorf("после отмены openPath должен быть пустым: %q", p)
	}
}

func TestCreateFileDuplicateRejected(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(existing, []byte("оригинал\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New(dir)
	m.SetHeight(10)
	m.Update(runes("n"))
	m.Update(runes("a.txt"))
	m.Update(keyOf(tea.KeyEnter))

	if !m.InputActive() {
		t.Error("при дубликате режим ввода должен остаться")
	}
	if !strings.Contains(m.inputErr, "существует") {
		t.Errorf("inputErr = %q, want «уже существует»", m.inputErr)
	}
	data, _ := os.ReadFile(existing)
	if string(data) != "оригинал\n" {
		t.Error("существующий файл перезаписан!")
	}
}

func TestCreateDirSelectsIt(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	m.SetHeight(10)

	m.Update(runes("N"))
	m.Update(runes("docs"))
	m.Update(keyOf(tea.KeyEnter))

	if _, err := os.Stat(filepath.Join(dir, "docs")); err != nil {
		t.Fatalf("каталог не создан: %v", err)
	}
	if p := m.OpenPath(); p != "" {
		t.Errorf("каталог не должен открываться как файл: %q", p)
	}
	sel := m.Selected()
	if sel == nil || sel.Name != "docs" {
		t.Errorf("курсор должен стоять на docs, got %+v", sel)
	}
}

func TestCreateNameValidation(t *testing.T) {
	m := New(t.TempDir())
	m.SetHeight(10)

	// Пустое имя.
	m.Update(runes("n"))
	m.Update(keyOf(tea.KeyEnter))
	if !strings.Contains(m.inputErr, "имя") {
		t.Errorf("пустое имя: %q", m.inputErr)
	}

	// Разделители пути запрещены.
	m.Update(runes("a/b"))
	if strings.Contains(string(m.inputName), "/") {
		t.Errorf("слэш попал в имя: %q", m.inputName)
	}
	if m.inputErr == "" {
		t.Error("должна быть ошибка о разделителях")
	}

	// backspace удаляет последний символ: имя "ab" → "a".
	m.Update(keyOf(tea.KeyBackspace))
	if string(m.inputName) != "a" {
		t.Errorf("после backspace имя = %q, want %q", m.inputName, "a")
	}
}

func TestOpenWithLandL(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("# a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.md"), []byte("# b\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// L открывает файл (как enter).
	m := New(dir)
	m.SetHeight(10)
	m.Update(runes("j")) // a.md после каталога sub
	m.Update(runes("L"))
	if p := m.OpenPath(); p != filepath.Join(dir, "a.md") {
		t.Errorf("L: OpenPath = %q, want a.md", p)
	}

	// l входит в каталог (как enter).
	m2 := New(dir)
	m2.SetHeight(10)
	m2.Update(runes("l")) // sub — первый (каталоги первыми)
	if m2.Dir() != sub {
		t.Errorf("l вошёл в %q, want %q", m2.Dir(), sub)
	}
	if p := m2.OpenPath(); p != "" {
		t.Errorf("каталог не должен открываться как файл: %q", p)
	}
}

func TestDeleteFileWithConfirmation(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "del.md")
	if err := os.WriteFile(file, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(dir)
	m.SetHeight(10)

	// Первое d — запрос подтверждения, ничего не удаляет.
	m.Update(runes("d"))
	if !m.ModalActive() {
		t.Fatal("после d включается подтверждение")
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("файл удалён без подтверждения: %v", err)
	}

	// esc — отмена.
	m.Update(keyOf(tea.KeyEsc))
	if m.ModalActive() {
		t.Error("esc должен снять подтверждение")
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("после отмены файл должен остаться: %v", err)
	}

	// Повторное d — подтверждение, файл удаляется.
	m.Update(runes("d"))
	m.Update(runes("d"))
	if _, err := os.Stat(file); err == nil {
		t.Error("файл не удалён после подтверждения")
	}
	if !strings.Contains(m.msg, "удалено") {
		t.Errorf("msg = %q, want «удалено»", m.msg)
	}
	if m.msgErr {
		t.Error("успешное удаление не должно быть ошибкой")
	}
	if m.ModalActive() {
		t.Error("после удаления подтверждение снимается")
	}
}

func TestDeleteConfirmSwallowsOtherKeys(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(dir)
	m.SetHeight(10)

	// Случайная клавиша (в т.ч. q) — отмена подтверждения, не удаление.
	m.Update(runes("d"))
	m.Update(runes("q"))
	if m.ModalActive() {
		t.Error("чужая клавиша должна отменить подтверждение")
	}
	if _, err := os.Stat(filepath.Join(dir, "a.md")); err != nil {
		t.Errorf("файл удалён без подтверждения: %v", err)
	}
	if m.msgErr {
		t.Error("отмена — не ошибка")
	}
}

func TestDeleteNonEmptyDirFails(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "inner.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(dir)
	m.SetHeight(10)

	m.Update(runes("d")) // подтверждение на sub (каталоги первыми)
	m.Update(runes("d")) // подтвердить

	if _, err := os.Stat(sub); err != nil {
		t.Fatalf("непустой каталог не должен удаляться: %v", err)
	}
	if !m.msgErr {
		t.Error("ожидали ошибку удаления непустого каталога")
	}
	if !strings.Contains(m.msg, "не удалось") {
		t.Errorf("msg = %q, want «не удалось удалить»", m.msg)
	}
}

func TestBrowseKeysWhileInputActive(t *testing.T) {
	m := New(t.TempDir())
	m.SetHeight(10)
	m.Update(runes("n"))
	// Стрелки/навигация во время ввода не двигают курсор и не ломают ввод.
	m.Update(keyOf(tea.KeyDown))
	m.Update(runes("file"))
	if got := string(m.inputName); got != "file" {
		t.Errorf("имя = %q, want %q", got, "file")
	}
}
