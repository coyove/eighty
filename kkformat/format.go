package kkformat

import (
	"fmt"
	"image"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/image/font"
)

type stream_t struct {
	buf       []byte
	idx       int
	beforeEnd bool
}

func (s *stream_t) nextRune() (rune, int) {
	return utf8.DecodeRune(s.buf[s.idx:])
}

func (s *stream_t) prevprevRune() (rune, rune) {
	p, w := utf8.DecodeLastRune(s.buf[:s.idx])
	if w > 0 {
		pp, _ := utf8.DecodeLastRune(s.buf[:s.idx-w])
		return pp, p
	}
	return 0, 0
}

func (s *stream_t) nextRuneIsEndOfLine() (bool, int) {
	idx := s.idx
	if idx >= len(s.buf) {
		return true, idx
	}

	r, w := utf8.DecodeRune(s.buf[idx:])
	if r == '\n' {
		return true, idx + w
	} else if r == '\r' && idx+w < len(s.buf) {
		idx += w
		r, w = utf8.DecodeRune(s.buf[idx:])
		return r == '\n', idx + w
	}

	return false, idx
}

func (s *stream_t) nextWord() *word_t {
	if s.beforeEnd {
		return nil
	}

	if s.idx >= len(s.buf) {
		s.beforeEnd = true
		return (&word_t{}).setType(runeEndOfBuffer)
	}

	pp, p := s.prevprevRune()
	r, w := s.nextRune()
	s.idx += w

	t := runeType(r)
	if t == runeUnknown {
		if r != '\r' {
			//fmt.Println("unknown:", string(r), "=", r)
		}
		return s.nextWord()
	}

	ret := (&word_t{}).setType(t)

	isSpecial := func(in rune) bool {
		switch n, w := s.nextRune(); in {
		case '/':
			switch n {
			case '/':
				s.idx += w
				ret.setSpecialType(specialComment).setValue([]rune("//")).setLen(2)
				return true
			case '*':
				s.idx += w
				ret.setSpecialType(specialCommentStart).setValue([]rune("/*")).setLen(2)
				return true
			}
		case '*':
			if n == '/' {
				s.idx += w
				ret.setSpecialType(specialCommentEnd).setValue([]rune("*/")).setLen(2)
				return true
			}
		case '#':
			ret.setSpecialType(specialCommentHash).setValue([]rune("#")).setLen(1)
			return true
		case '"':
			if p != '\\' || pp == '\\' {
				ret.setSpecialType(specialDoubleQuote).setValue([]rune{'"'}).setLen(1)
				return true
			}
		case '\'':
			if p != '\\' || pp == '\\' {
				ret.setSpecialType(specialSingleQuote).setValue([]rune{'\''}).setLen(1)
				return true
			}
		}

		return false
	}

	icSpace := func(r rune) string {
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

	keepReading := func() {
		for s.idx < len(s.buf) {
			r, w := s.nextRune()
			if r == '/' || r == '*' || r == '#' || r == '"' || r == '\'' || r == '\\' {
				break
			}

			s.idx += w

			if runeType(r) == t {
				if t == runeSpace {
					sp := icSpace(r)
					ret.value = append(ret.value, []rune(sp)...)
					ret.len += StringWidth(sp)
				} else {
					ret.value = append(ret.value, r)
					ret.len += RuneWidth(r)
				}
				continue
			}

			s.idx -= w
			break
		}
	}

	if isSpecial(r) {
		return ret
	}

	switch t {
	case runeNewline:
		return ret // len = 0
	case runeSpace:
		sp := icSpace(r)
		ret.value = []rune(sp)
		ret.len = StringWidth(sp)
		keepReading()
	case runeHalfDelim, runeLatin:
		ret.value = []rune{r}
		ret.len = RuneWidth(r)
		keepReading()
	default:
		ret.value = []rune{r}
		ret.len = RuneWidth(r)
	}

	return ret
}

func splitRune(in []rune, at uint32) ([]rune, []rune, bool) {
	a := at
	for i := 0; i < len(in); i++ {
		w := RuneWidth(in[i])
		if w == at {
			return in[:i+1], in[i+1:], true
		}

		if w < at {
			at -= w
			continue
		}

		return in[:i], in[i:], false
	}

	panic(string(in) + " " + strconv.Itoa(int(a)))
}

// Formatter struct
type Formatter struct {
	Source     []byte // source buffer of the input
	Columns    uint32 // columns of the output
	curSpecial uint16
	Rows       int
	Img        *font.Drawer
	LineHeight int
	CurrentY   int
	Theme      []image.Image

	wp words_t // for a single line, wp holds the content whose spaces have been processed
	wd words_t // for a single line, wd holds the delimeters in it
	wl words_t // for a single line, wl holds the latin characters, it will be appended to wd eventually
}

func (o *Formatter) resetPDL() {
	o.wp = o.wp[:0]
	o.wd = o.wd[:0]
	o.wl = o.wl[:0]
}

// Render renders the content into an image
func (o *Formatter) Render() image.Image {
	// Init Formatter
	o.wp, o.wd, o.wl = make(words_t, 0, 32), make(words_t, 0, 32), make(words_t, 0, 32)
	ws := stream_t{buf: o.Source}

	line, length, lineNo := make(words_t, 0, 10), uint32(0), 0
	nobrk := false
	cont := true
	nextWordIsNaturalStart := true

	insertlineNo := func() {
		lineNo++
		s := strconv.Itoa(lineNo)
		num := (&word_t{}).setType(runeLatin).setValue([]rune(s)).setLen(uint32(len(s))).setSpecialType(specialLineNumber)
		space := spaceWord.dup().setIsCode()

		switch len(s) {
		case 1:
			line = append(line, space, space, num, space)
		case 2:
			line = append(line, space, num, space)
		case 3:
			line = append(line, num, space)
		default:
			panic("?")
		}

		length = 4
	}

	appendReset := func() {
		// lines = append(lines, line)
		last := line.last()
		cont = line.adjustableJoin(o)

		line = line[:0]
		if last != nil && last.getType() == runeContToNext {
			line = append(line, lineContFrom)
		} else if nobrk {
			insertlineNo()
		}
	}

	_ = fmt.Println
	var lastWord *word_t
	for t := ws.nextWord(); t != nil && cont; t = ws.nextWord() {
		// fmt.Println(string(t.value), t.getType(), t.getSpecialType(), ws.idx)

		if t.startsWith("```") && (lastWord == nil || lastWord.getType() == runeNewline) {
			nobrk = !nobrk

			for t := ws.nextWord(); t != nil; t = ws.nextWord() {
				if t.getType() == runeNewline || t.getType() == runeEndOfBuffer {
					break
				}
			}

			if nobrk {
				lineNo = 0
				insertlineNo()
			}
			continue
		}

		if nobrk {
			t.setIsCode()
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
						var perfect bool
						t2.value, t.value, perfect = splitRune(t.value, len1)
						t2.len, t.len = StringWidth(t2.value), StringWidth(t.value)
						line = append(line, t2)
						if !perfect {
							line = append(line, spaceWord.dup())
						}
					}
					line = append(line, lineContTo.dup())
					appendReset()
					goto AGAIN
				}

				if t.isInMap(canStayAtEnd) && t.len <= 3 {
					t.setType(runeExtraAtEnd)
					line = append(line, t)
					appendReset()

					if y, _ := ws.nextRuneIsEndOfLine(); y {
						lastWord = ws.nextWord()
					}
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
					line = append(line, lineContTo.dup())
				}
				appendReset()
			}

			length += t.getLen()
			line = append(line, t)

			if t.getType() == runeNewline {
				length = 0
				appendReset()
				nextWordIsNaturalStart = true
			}
		}

		lastWord = t
		if words := t.split(o.Columns-length, o.Columns); words == nil {
			read(t, false)
			if nextWordIsNaturalStart {
				t.setIsNaturalStart()
			}
		} else {
			for _, w := range words {
				read(w, true)
			}
		}

		nextWordIsNaturalStart = false
	}

	if len(line) > 0 && cont {
		appendReset()
	}

	o.CurrentY += o.LineHeight / 2

	height := o.CurrentY
	maxHeight := o.Img.Dst.Bounds().Dy()
	if height > maxHeight {
		height = maxHeight
	}

	type image_i interface {
		SubImage(image.Rectangle) image.Image
	}

	return o.Img.Dst.(image_i).SubImage(image.Rect(0, 0, o.Img.Dst.Bounds().Dx(), height))
}
