package kkformat

import (
	"bytes"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"
)

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
		return (&word_t{}).setType(runeEndOfBuffer)
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

	ret := (&word_t{}).setType(t)

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

	continue_next := func() {
		for s.idx < len(s.buf) {
			r, w := s.nextRune()

			if runeType(r) == t {
				ret.value = append(ret.value, r)
				ret.len += RuneWidth(r)
			} else {
				break
			}

			s.idx += w
		}
	}

	// switch t {
	// case runeNewline:
	// 	return ret // len = 0
	// case runeSpace, runeHalfDelim:
	// 	sp := icspace(r)
	// 	ret.value = []rune(sp)
	// 	ret.len = StringWidth(sp)
	// 	continue_next()
	// case runeLatin:
	// 	ret.value = []rune{r}
	// 	ret.len = RuneWidth(r)

	// 	// continue to find latin characters
	// 	for s.idx < len(s.buf) {
	// 		r, w := s.nextRune()

	// 		if runeType(r) == runeLatin {
	// 			ret.value = append(ret.value, r)
	// 			ret.setLen(ret.getLen() + RuneWidth(r))
	// 		} else {
	// 			break
	// 		}

	// 		s.idx += w
	// 	}
	// default:
	// 	ret.value = []rune{r}
	// 	ret.setLen(RuneWidth(r))
	// }
	switch t {
	case runeNewline:
		return ret // len = 0
	case runeSpace:
		sp := icspace(r)
		ret.value = []rune(sp)
		ret.len = StringWidth(sp)
		continue_next()
	case runeHalfDelim, runeLatin:
		ret.value = []rune{r}
		ret.len = RuneWidth(r)
		continue_next()
	default:
		ret.value = []rune{r}
		ret.setLen(RuneWidth(r))
	}

	return ret
}

// Formatter struct
type Formatter struct {
	Source     []byte // source buffer of the input
	Columns    uint32 // columns of the output
	LinkTarget string // link target, e.g.: target=_blank
	SkipToC    bool   // skip the generation of ToC
	ID         int64

	urls []string
	tmp  *bytes.Buffer
	len  int64
	w    io.Writer
	wp   words_t // for a single line, wp holds the content whose spaces have been processed
	wd   words_t // for a single line, wd holds the delimeters in it
	wl   words_t // for a single line, wl holds the latin characters, it will be appended to wd eventually
}

func (o *Formatter) resetPDL() {
	o.wp = o.wp[:0]
	o.wd = o.wd[:0]
	o.wl = o.wl[:0]
}

func (o *Formatter) write(ss ...string) {
	for _, s := range ss {
		buf := *(*[]byte)(unsafe.Pointer(&s))
		o.tmp.Write(buf)
		o.len += int64(len(buf))
	}
}

func (o *Formatter) flush() {
	o.w.Write(o.tmp.Bytes())
	o.tmp.Reset()
}

// WriteTo renders the content to "w"
func (o *Formatter) WriteTo(w io.Writer) (int64, error) {
	o.w = w
	o.tmp = &bytes.Buffer{}
	o.wp, o.wd, o.wl = make(words_t, 0, 32), make(words_t, 0, 32), make(words_t, 0, 32)
	ws := stream_t{buf: o.Source}

	line, lines, length := make(words_t, 0, 10), []words_t{}, uint32(0)
	nobrk := false

	appendReset := func() {
		lines = append(lines, line)
		line = make(words_t, 0, 10)
	}

	for t := ws.nextWord(); t != nil; t = ws.nextWord() {
		if t.startsWith("```") {
			nobrk = !nobrk
			continue
		}

		read := func(t *word_t, appendMark bool) {
			adjusted := false

		AGAIN:
			if length+t.getLen() > o.Columns {
				len1 := o.Columns - length
				length = 0

				if nobrk {
					if len1 > 0 {
						t2 := t.dup()
						t2.value = t.value[:len1]
						t2.setLen(len1)
						line = append(line, t2, lineContinues.dup())
						appendReset()
						t.value = t.value[len1:]
						t.incLen(-len1)
					} else {
						line = append(line, lineContinues.dup())
						appendReset()
					}
					goto AGAIN
				}

				if t.isInMap(canStayAtEnd) && t.len <= 3 {
					t.setType(runeExtraAtEnd)
					line = append(line, t)
					appendReset()
					return
				}

				if !adjusted && len(line) > 0 && line[len(line)-1].isInMap(cannotStayAtEnd) {
					last := line[len(line)-1]
					line = line[:len(line)-1]
					appendReset()
					length = last.getLen()
					line = append(line, last)
					adjusted = true
					goto AGAIN
				}

				if appendMark {
					line = append(line, lineContinues.dup())
				}
				appendReset()
			}

			length += t.getLen()
			line = append(line, t)

			if t.getType() == runeNewline {
				length = 0
				lines = append(lines, line)
				line = words_t{}
			}
		}

		if words := t.split(o.Columns-length, o.Columns); words == nil {
			read(t, false)
		} else {
			for _, w := range words {
				read(w, true)
			}
		}
	}

	if len(line) > 0 {
		lines = append(lines, line)
	}

	urlWordList, inURLSpace := words_t{}, false
	toc := []words_t{}

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		if len(line) > 1 && line[0].startsWith("####") {
			line = line[1:]
			toc = append(toc, line)
			lines[i] = line
		}

		for i := 0; i < len(line); {
			word := line[i]
			if inURLSpace {
				if word.getType() != runeContinues &&
					(word.isSpacesOnly() || word.getType() == runeNewline || !word.isInMap(validURIChars)) {
					inURLSpace = false
					urlWordList.updateURL(o)
				} else {
					urlWordList = append(urlWordList, word)
				}
			} else if word.isInMap(uriSchemes) && i < len(line)-1 && line[i+1].startsWith("://") {
				inURLSpace = true
				urlWordList = []*word_t{word, line[i+1]}
				i += 2
				continue
			}
			i++
		}
	}

	if inURLSpace {
		urlWordList.updateURL(o)
	}

	lastURL := ""
	for i := 0; i < len(lines); {
		line := lines[i]
		if len(line) == 0 {
			i++
			continue
		}

		if url := line[0].getURL(o.urls); url != "" &&
			(strings.HasSuffix(url, ".jpg") || strings.HasSuffix(url, ".png") ||
				strings.HasSuffix(url, ".gif") || strings.HasSuffix(url, ".webp")) {
			if url == lastURL {
				lines = append(lines[:i], lines[i+1:]...)
				continue
			}

			lines[i] = line[:1]
			line[0].setType(runeImage)
			lastURL = url
		}

		i++
	}

	if tocnum := len(toc); tocnum > 0 && !o.SkipToC {
		toclines := make([]words_t, tocnum+1)
		id := strconv.FormatInt(o.ID, 10)

		for i, t := range toc {
			num := strconv.Itoa(i + 1)
			toclines[i] = make(words_t, 3, 10)
			toclines[i][0] = (&word_t{}).setType(runeLatin).setLen(uint32(len(num))).setValue([]rune(num))
			toclines[i][1] = (&word_t{}).setType(runeHalfDelim).setLen(1).setValue([]rune{'.'})
			toclines[i][2] = (&word_t{}).setType(runeSpace).setLen(1).setValue([]rune{' '})
			toclines[i] = append(toclines[i], t.dup()...) // TODO: bad

			if toclines[i].last().getType() != runeNewline {
				toclines[i] = append(toclines[i], newLine)
			}

			o.urls = append(o.urls, "#toc-f-"+id+"-"+num)
			for _, w := range toclines[i] {
				w.setURL(uint16(len(o.urls)))
			}

			o.urls = append(o.urls, "#toc-r-"+id+"-"+num)
			for _, w := range t {
				w.setURL(uint16(len(o.urls)))
			}
		}

		toclines[tocnum] = words_t{newLine}
		lines = append(toclines, lines...)
	}

	for _, line := range lines {
		if line != nil {
			line.adjustableJoin(o)
		}
	}

	return o.len, nil
}
