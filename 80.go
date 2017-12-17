package main

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type word_t struct {
	value []rune
	ty    byte
	len   int
	url   string
	toc   int16
	mark  int16
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

type words_t []*word_t

func (w *words_t) adjustableJoin(opt FormatOptions) *bytes.Buffer {
	words := *w
	if len(words) == 0 {
		return &bytes.Buffer{}
	}

	length := 0
	naturalEnd := words[len(words)-1].ty == runeNewline || words[len(words)-1].ty == runeEndOfBuffer
	wp := make(words_t, 0, len(words))
	wd, wl := make(words_t, 0, len(words)/2), make(words_t, 0, len(words)/2)
	var exEnding *word_t

	for i, word := range words {
		if !naturalEnd {
			// the leading spaces of 4, 8, 16 ... will be preserved, others will be discarded
			if word.len > 0 && i == 0 {
				if l, _ := word.surroundingSpaces(); l%4 != 0 {
					word.value = word.value[l:]
					word.len -= l
				}
			}

			// the trailing spaces will always be discarded
			if word.len > 0 && i == len(words)-1 {
				_, r := word.surroundingSpaces()
				word.value = word.value[:len(word.value)-r]
				word.len -= r
			}
		}

		if word.len > 0 {
			if word.ty == runeSpecial {
				exEnding = word
				continue
			}

			if word.url == "" && i < len(words)-1 {
				if word.ty == runeDelim {
					wd = append(wd, word)
				}
				if word.ty == runeLatin {
					wl = append(wl, word)
				}
			}

			length += word.len
			wp = append(wp, word)
		}
	}

	wd = append(wd, wl...)
	// adjust
	gap, fillstart := opt.width-length, 0
	if len(wp) <= 1 {
		return wp.join(opt)
	}

	if !naturalEnd && wp[len(wp)-1].isInMap(extendablePunc) {
		gap++
	}

	if naturalEnd || gap == 0 {
		return wp.join(opt)
	}

	if wp[0].startsWith("    ") {
		fillstart = 1
	}

	ln := len(wp) - 1 - fillstart
	lnh := len(wd)
	if ln == 0 {
		return wp.join(opt)
	}

	if lnh >= gap {
		// we have enough delimeters / latin characters, append spaces to them
		if gap == 1 {
			wd[lnh/2].value = append(wd[lnh/2].value, ' ')
		} else {
			dk := lnh / gap
			for i := 0; i < len(wd); i += dk {
				wd[i].value = append(wd[i].value, ' ')
				if gap--; gap <= 0 {
					break
				}
			}
		}
	} else if gap+1 <= ln {
		dk := ln / (gap + 1)
		for i := fillstart + dk; i < len(wp); i += dk {
			wp[i].value = append(wp[i].value, ' ')
			if gap--; gap <= 0 {
				break
			}
		}
	} else {
		for i := fillstart; i < len(wp)-1; i++ {
			r := ln - i + fillstart
			dk := gap / r
			if dk*r != gap {
				dk++
			}

			wp[i].value = appendSpacesRune(wp[i].value, dk, i == fillstart)

			if gap -= dk; gap <= 0 {
				break
			}
		}
	}

	if exEnding != nil {
		wp = append(wp, exEnding)
	}
	return wp.join(opt)
}

func (w *words_t) updateURL() {
	url := w.rawJoin()
	for _, word := range *w {
		word.url = url
	}
}

func (w words_t) dup() words_t {
	w2 := make(words_t, len(w))
	for i, word := range w {
		w2[i] = word.dup()
	}

	return w2
}

func CalcTag(value []rune) string {
	whole := &bytes.Buffer{}

	for _, r := range value {
		if runeWidth(r) == 1 {
			whole.WriteString("<dt>")
		} else {
			whole.WriteString("<dd>")
		}
		whole.WriteRune(r)
	}

	return whole.String()
}

type lastBuffer struct {
	bytes.Buffer
	last string
}

func (buf *lastBuffer) WriteString(s string) (n int, err error) {
	buf.last = s
	return buf.Buffer.WriteString(s)
}

func (w *words_t) join(opt FormatOptions) *bytes.Buffer {
	buf := &lastBuffer{}
	words := *w

	buf.WriteString("<div")
	if len(words) > 0 {
		if words[0].toc != 0 {
			buf.WriteString(" class=_toc id=toc" + strconv.Itoa(int(words[0].toc)))
		}
		if words[0].mark != 0 {
			buf.WriteString(" class=_mark id=mark" + strconv.Itoa(int(words[0].mark)))
		}
	}
	buf.WriteString(">")

	for i := 0; i < len(words); i++ {
		word := words[i]
		if word.url != "" {
			if word.ty == runeImage {
				buf.WriteString("<img class='_image' src='")
				buf.WriteString(word.url)
				buf.WriteString("' alt='")
				buf.WriteString(word.url)
				buf.WriteString("'>")
				break
			}

			if i == 0 || (words[i-1].url != word.url) {
				buf.WriteString("<a ")
				if word.url[0] != '#' {
					buf.WriteString(opt.linkTarget)
				}
				buf.WriteString(" href='")
				buf.WriteString(word.url)
				buf.WriteString("'>")
			}
		}

		if i == 0 && len(word.value) >= 4 && word.startsWith("====") {
			buf.Reset()
			buf.WriteString("<hr>")
			return &buf.Buffer
		}

		if buf.last == "</dl>" {
			buf.Truncate(buf.Len() - 5)
		} else {
			buf.WriteString("<dl>")
		}

		buf.WriteString(CalcTag(word.value))
		buf.WriteString("</dl>")

		if word.url != "" {
			if i == len(words)-1 || words[i+1].url != word.url {
				buf.WriteString("</a>")
			}
		}
	}

	buf.WriteString("</div>")
	return &buf.Buffer
}

func (w *words_t) rawJoin() string {
	buf := bytes.Buffer{}

	for _, word := range *w {
		if word.value != nil {
			for i := 0; i < len(word.value); i++ {
				buf.WriteRune(word.value[i])
			}
		}
	}

	return buf.String()
}

func (w *words_t) last() *word_t {
	if len(*w) == 0 {
		return nil
	}

	return (*w)[len(*w)-1]
}

type stream_t struct {
	buf       []byte
	idx       int
	beforeEnd bool
}

func (s *stream_t) nextRune() (rune, int) {
	return utf8.DecodeRune(s.buf[s.idx:])
}

func (s *stream_t) nextWord() *word_t {
	if s.beforeEnd {
		return nil
	}

	if s.idx >= len(s.buf) {
		s.beforeEnd = true
		return &word_t{ty: runeEndOfBuffer}
	}

	r, w := s.nextRune()
	s.idx += w

	t := runeType(r)
	if t == runeUnknown {
		if r != '\r' {
			//fmt.Println("unknown:", string(r), "=", r)
		}

		return s.nextWord()
		//}

		//panic()
	}

	icspace := func(r rune) string {
		switch r {
		case '\t':
			if tabWidth < 16 {
				return "                "[:tabWidth]
			}
			return strings.Repeat(" ", tabWidth)
		case fullSpace:
			return "  "
		default:
			return string(r)
		}
	}

	ret := &word_t{ty: t}

	switch t {
	case runeNewline:
		return ret // len = 0
	case runeSpace, runeDelim:
		sp := icspace(r)
		ret.value = []rune(sp)
		ret.len = stringWidth(sp)

		// continue to find delimeters and spaces
		for s.idx < len(s.buf) {
			r, w := s.nextRune()

			if t := runeType(r); t == runeDelim || t == runeSpace {
				ret.value = append(ret.value, []rune(icspace(r))...)
				ret.len += runeWidth(r)
			} else {
				break
			}

			s.idx += w
		}
	case runeLatin:
		ret.value = []rune{r}
		ret.len = runeWidth(r)

		// continue to find latin characters
		for s.idx < len(s.buf) {
			r, w := s.nextRune()

			if runeType(r) == runeLatin {
				ret.value = append(ret.value, r)
				ret.len += runeWidth(r)
			} else {
				break
			}

			s.idx += w
		}
	default:
		ret.value = []rune{r}
		ret.len = runeWidth(r)
	}

	return ret
}

type PrefixCallback struct {
	prefix   string
	callback func(in words_t) words_t
}

type FormatOptions struct {
	pc         []PrefixCallback
	width      int
	linkTarget string
	output     io.Writer
	skipTOC    bool
}

func Format80(buf []byte, opt FormatOptions) string {
	ws := stream_t{buf: buf}
	line, lines, length := words_t{}, []words_t{}, 0
	appendReset := func() {
		lines = append(lines, line)
		line = words_t{}
	}

	for t := ws.nextWord(); t != nil; t = ws.nextWord() {
		read := func(t *word_t) {
			adjusted := false
		AGAIN:
			if length+t.len > opt.width {
				length = 0
				if t.isInMap(canStayAtEnd) {
					t.ty = runeSpecial
					line = append(line, t)
					appendReset()
					return
				}

				if !adjusted && len(line) > 0 && line[len(line)-1].isInMap(cannotStayAtEnd) {
					last := line[len(line)-1]
					line = line[:len(line)-1]
					appendReset()
					length = last.len
					line = append(line, last)
					adjusted = true
					goto AGAIN
				}

				appendReset()
			}

			length += t.len
			line = append(line, t)

			if t.ty == runeNewline {
				length = 0
				lines = append(lines, line)
				line = words_t{}
			}
		}

		if words := t.split(opt.width-length, opt.width); words == nil {
			read(t)
		} else {
			for _, w := range words {
				read(w)
			}
		}
	}

	if len(line) > 0 {
		lines = append(lines, line)
	}

	urlWordList, inURLSpace := words_t{}, false
	toc := []words_t{}
NEXT_LINE:
	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if len(line) > 0 {
			if opt.pc != nil {
				for _, cb := range opt.pc {
					if line[0].startsWith(cb.prefix) {
						lines[i] = cb.callback(line)
						continue NEXT_LINE
					}
				}
			}

			if line[0].startsWith("--") && len(line) > 1 {
				line = line[1:]
				toc = append(toc, line)
				lines[i] = line
			}
		}

		for i := 0; i < len(line); {
			word := line[i]
			if inURLSpace {
				if word.isSpacesOnly() || word.ty == runeNewline || !word.isInMap(validURIChars) {
					inURLSpace = false
					urlWordList.updateURL()
				} else {
					urlWordList = append(urlWordList, word)
				}
			} else if word.isInMap(uriSchemes) && i < len(line)-1 && string(line[i+1].value) == "://" {
				inURLSpace = true
				urlWordList = []*word_t{word, line[i+1]}
				i += 2
				continue
			}
			i++
		}
	}

	if inURLSpace {
		urlWordList.updateURL()
	}

	lastURL := ""
	for i := 0; i < len(lines); {
		line := lines[i]
		if len(line) == 0 {
			i++
			continue
		}

		if url := line[0].url; url != "" &&
			(strings.HasSuffix(url, ".jpg") || strings.HasSuffix(url, ".png") ||
				strings.HasSuffix(url, ".gif") || strings.HasSuffix(url, ".webp")) {
			if url == lastURL {
				lines = append(lines[:i], lines[i+1:]...)
				continue
			}

			lines[i] = line[:1]
			line[0].ty = runeImage
			lastURL = url
		}

		i++
	}

	if tocnum := len(toc); tocnum > 0 && !opt.skipTOC {
		toclines := make([]words_t, tocnum+1)
		// digits := len(strconv.Itoa(tocnum))

		for i, t := range toc {
			num := strconv.Itoa(i + 1)
			toclines[i] = words_t{
				&word_t{ty: runeLatin, len: len(num), value: []rune(num)},
				&word_t{ty: runeDelim, len: 1, value: []rune{'.'}},
				&word_t{ty: runeSpace, len: 1, value: []rune{' '}},
			}

			toclines[i] = append(toclines[i], t.dup()...) // TODO: bad
			if toclines[i].last().ty != runeNewline {
				toclines[i] = append(toclines[i], &word_t{ty: runeNewline})
			}

			url := "#mark" + num
			for _, w := range toclines[i] {
				w.url = url
				w.toc = int16(i + 1)
			}

			url = "#toc" + num
			for _, w := range t {
				w.url = url
				w.mark = int16(i + 1)
			}
		}

		toclines[tocnum] = words_t{&word_t{ty: runeNewline}}
		lines = append(toclines, lines...)
	}

	if opt.output == nil {
		strlines := make([]string, len(lines))
		for i, line := range lines {
			if line != nil {
				strlines[i] = line.adjustableJoin(opt).String()
			}
		}
		return strings.Join(strlines, "\n")
	}

	for _, line := range lines {
		if line != nil {
			opt.output.Write(line.adjustableJoin(opt).Bytes())
		}
	}
	return ""
}
