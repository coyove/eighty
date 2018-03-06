package format80

import (
	"unicode"
)

type word_t struct {
	value []rune
	ty    byte
	len   int
	toc   int16
	mark  int16
	url   *string
}

func (w *word_t) dup() *word_t {
	w2 := *w
	return &w2
}

func (w *word_t) split(width1, width2 int) words_t {
	if w.len <= width1 || w.len <= width2 {
		return nil
	}

	words := make(words_t, 0, 2)

	var w2 *word_t
	width := width1
	for w.len > width {
		w2 = w.dup()
		var r rune
		var i, ln int
		for i, r = range w.value {
			if ln += runeWidth(r); ln > width {
				break
			}
		}

		w2.value = w.value[i:]
		w2.len = stringWidth(w2.value)

		w.value = w.value[:i]
		w.len = stringWidth(w.value)

		words = append(words, w)
		w = w2
		width = width2
	}

	words = append(words, w2)
	return words
}

func (w *word_t) isCJK() bool {
	if w.len == 0 || w.value == nil {
		return false
	}

	return unicode.Is(unicode.Scripts["Han"], w.value[0]) ||
		unicode.Is(unicode.Scripts["Hangul"], w.value[0]) ||
		unicode.Is(unicode.Scripts["Hiragana"], w.value[0]) ||
		unicode.Is(unicode.Scripts["Katakana"], w.value[0])
}

func (w *word_t) isSpacesOnly() bool {
	if w.len == 0 || w.value == nil {
		return false
	}

	for i := 0; i < len(w.value); i++ {
		if w.value[i] != ' ' {
			return false
		}
	}

	return true
}

func (w *word_t) isInMap(m map[string]bool) bool {
	if w.len == 0 || w.value == nil {
		return false
	}

	l, r := w.surroundingSpaces()
	if l == len(w.value) {
		return false
	}

	if m[string(w.value[l:len(w.value)-r])] {
		return true
	}

	for i := 0; i < len(w.value); i++ {
		if !m[string(w.value[i])] {
			return false
		}
	}

	return true
}

func (w *word_t) surroundingSpaces() (l int, r int) {
	if w.len == 0 || w.value == nil {
		return
	}

	for j := 0; j < len(w.value); j++ {
		if w.value[j] == ' ' {
			l++
		} else {
			break
		}
	}

	for j := len(w.value) - 1; j >= 0; j-- {
		if w.value[j] == ' ' {
			r++
		} else {
			break
		}
	}

	return
}

func (w *word_t) startsWith(prefix string) bool {
	if w.len == 0 || w.value == nil {
		return false
	}

	if len(w.value) < len(prefix) {
		return false
	}

	i := 0
	for _, p := range prefix {
		if p != w.value[i] {
			return false
		}

		i++
	}

	return true
}
