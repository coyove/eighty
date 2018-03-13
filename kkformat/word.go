package kkformat

import (
	"fmt"
	"regexp"
	"unicode"
)

var _fmt_print_ = fmt.Println

type word_t struct {
	value []rune // word content
	len   uint32 // length
	ty    uint16
}

func (w *word_t) setIsNaturalStart() {
	w.ty = w.ty | 0x8000
}

func (w *word_t) isNaturalStart() bool {
	return (w.ty >> 15) == 1
}

func (w *word_t) setType(ty uint16) *word_t {
	w.ty = (w.ty & 0xc000) + (ty << 2 >> 2)
	return w
}

func (w *word_t) getType() uint16 {
	return w.ty << 2 >> 2
}

func (w *word_t) setIsCode() *word_t {
	w.ty = w.ty | 0x4000
	return w
}

func (w *word_t) isCode() bool {
	return (w.ty << 1 >> 15) == 1
}

func (w *word_t) setLen(l uint32) *word_t {
	w.len = l
	return w
}

func (w *word_t) incLen(l uint32) *word_t {
	w.len += l
	return w
}

func (w *word_t) getLen() uint32 {
	return uint32(w.len)
}

func (w *word_t) setValue(value []rune) *word_t {
	w.value = value
	return w
}

// dup is a shallow dup, "value" points to the same position
func (w *word_t) dup() *word_t {
	w2 := *w
	return &w2
}

// split splits the word into multiple parts or nil:
// first part (if exists) is "width1" long and rest parts (if exist) are all "width2" long
func (w *word_t) split(width1, width2 uint32) words_t {
	if w.getLen() <= width1 || w.getLen() <= width2 {
		return nil
	}

	words := make(words_t, 0, 2)

	var w2 *word_t
	width := width1
	for w.getLen() > width {
		w2 = w.dup()
		var r rune
		var i int
		var ln uint32

		for i, r = range w.value {
			if ln += RuneWidth(r); ln > width {
				break
			}
		}

		w2.value = w.value[i:]
		w2.setLen(StringWidth(w2.value))

		w.value = w.value[:i]
		w.setLen(StringWidth(w.value))

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

func (w *word_t) isInMap(re *regexp.Regexp) bool {
	if w.len == 0 || w.value == nil {
		return false
	}

	l, r := w.surroundingSpaces()
	if l == uint32(len(w.value)) {
		return false
	}

	s := string(w.value[l : uint32(len(w.value))-r])
	// fmt.Println(s, re.FindAllStringIndex(s, -1))
	// panic(1)
	return len(re.FindAllStringIndex(s, -1)) > 0
}

func (w *word_t) surroundingSpaces() (l uint32, r uint32) {
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
