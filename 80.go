package main

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var WIDTH = 40
var TARGETBLANK = "target='_blank'"

var (
	ulSuffix = []byte("</ul>")
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
	wh := make(words_t, 0, len(words)/2)

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
			if word.url == "" && (word.ty == runeDelim || word.ty == runeLatin) {
				wh = append(wh, word)
			}

			length += word.len
			wp = append(wp, word)
		}
	}

	// adjust
	gap, fillstart := opt.width-length, 0

	if naturalEnd || len(wp) <= 1 || gap == 0 {
		return wp.join(opt)
	}

	if wp[0].startsWith("    ") {
		fillstart = 1
	}

	ln := len(wp) - 1 - fillstart
	lnh := len(wh)
	if ln == 0 {
		return wp.join(opt)
	}

	if lnh >= gap {
		// we have enough delimeters / latin chacaters, append spaces to them
		dk := lnh / gap
		for i := 0; i < len(wh); i += dk {
			wh[i].value = append(wh[i].value, ' ')
			if gap--; gap <= 0 {
				break
			}
		}
	} else if gap <= ln {
		dk := ln / gap
		for i := fillstart; i < len(wp); i += dk {
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

	return wp.join(opt)
}

func (w *words_t) updateURL() {
	url := w.rawJoin()
	for _, word := range *w {
		word.url = url
	}
}

func (w *words_t) dup() words_t {
	w2 := make(words_t, len(*w))
	for i, word := range *w {
		w2[i] = word.dup()
	}

	return w2
}

func CalcTag(value []rune) (html string, openTag string, closeTag string) {
	whole, flag := &bytes.Buffer{}, false
	for _, r := range value {
		if runeWidth(r) == 1 {
			whole.WriteString("<li>")
			whole.WriteRune(r)
		} else {
			flag = true
			break
		}
	}

	if !flag {
		return "<ul>", whole.String(), "</ul>"
	}

	// cjk
	wt1, wt2 := []byte{'<', 'p', ' ', 0, '>'}, []byte{'<', 'p', ' ', 0, 0, '>'}
	calc := func(w int) []byte {
		if w <= 26 {
			wt1[3] = byte(w-1) + 'a'
			return wt1
		}

		b2 := w / 26
		wt2[3] = byte(b2) + 'a'
		wt2[4] = byte(w-b2*26) + 'a'
		return wt2
	}

	whole.Reset()
	cjk := strings.Split(string(value), " ")
	for i, part := range cjk {
		if part != "" {
			whole.Write(calc(stringWidth(part)))
			whole.WriteString(part)

			if i < len(cjk)-1 {
				whole.WriteString("</p>")
			}
		}

		if i < len(cjk)-1 {
			whole.WriteString("<ul><li> </ul>")
		}
	}

	return "", whole.String(), "</p>"
}

func (w *words_t) join(opt FormatOptions) *bytes.Buffer {
	buf := &bytes.Buffer{}
	words := *w

	for i := 0; i < len(words); {
		word := words[i]
		if word.isCJK() {
			j := 0
			for j = i; j < len(words); j++ {
				if !words[j].isCJK() {
					break
				}
			}

			if j > i {
				//   i            j
				//   |            |
				//  cjk cjk cjk latin latin ...
				for x := i + 1; x < j; x++ {
					words[i].value = append(words[i].value, words[x].value...)
				}

				words = append(words[:i+1], words[j:]...)
			}
		}

		i++
	}

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
			return buf
		}

		openTag, html, closeTag := CalcTag(word.value)
		if bytes.HasSuffix(buf.Bytes(), ulSuffix) && openTag == "<ul>" {
			buf.Truncate(buf.Len() - len(ulSuffix))
		} else {
			buf.WriteString(openTag)
		}

		buf.WriteString(html)
		buf.WriteString(closeTag)

		if word.url != "" {
			if i == len(words)-1 || words[i+1].url != word.url {
				buf.WriteString("</a>")
			}
		}
	}

	buf.WriteString("</div>")
	return buf
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
		if r == '\t' {
			if tabWidth < 16 {
				return "                "[:tabWidth]
			}
			return strings.Repeat(" ", tabWidth)
		}
		return string(r)
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
	callback func(in string) bool
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

	for t := ws.nextWord(); t != nil; t = ws.nextWord() {
		if length+t.len > opt.width {
			length = 0
			lines = append(lines, line)
			line = words_t{}
		}

		length += t.len
		line = append(line, t)

		if t.ty == runeNewline {
			length = 0
			lines = append(lines, line)
			line = words_t{}
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
					if line[0].startsWith(cb.prefix) && !cb.callback(line.rawJoin()) {
						lines[i] = nil
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

	if inURLSpace {
		urlWordList.updateURL()
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
