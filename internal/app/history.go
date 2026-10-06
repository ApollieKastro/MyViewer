package app

// historyEntry — сохранённая позиция перехода: файл и строка прокрутки.
type historyEntry struct {
	path   string
	offset int
}

// navigationHistory — история переходов между файлами по ссылкам
// (аналог jump list в vim): стек «назад» и стек «вперёд».
//
// При новом переходе стек «вперёд» очищается — история ветвится,
// как в браузере. Откат назад переносит текущую позицию в «вперёд».
type navigationHistory struct {
	back    []historyEntry
	forward []historyEntry
}

// push запоминает текущую позицию перед новым переходом.
func (h *navigationHistory) push(e historyEntry) {
	h.back = append(h.back, e)
	h.forward = nil
}

// backward возвращает позицию для перехода назад и запоминает текущую.
func (h *navigationHistory) backward(cur historyEntry) (historyEntry, bool) {
	if len(h.back) == 0 {
		return historyEntry{}, false
	}
	e := h.back[len(h.back)-1]
	h.back = h.back[:len(h.back)-1]
	h.forward = append(h.forward, cur)
	return e, true
}

// forward возвращает позицию для перехода вперёд и запоминает текущую.
func (h *navigationHistory) forwardStep(cur historyEntry) (historyEntry, bool) {
	if len(h.forward) == 0 {
		return historyEntry{}, false
	}
	e := h.forward[len(h.forward)-1]
	h.forward = h.forward[:len(h.forward)-1]
	h.back = append(h.back, cur)
	return e, true
}
