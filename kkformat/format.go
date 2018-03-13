package kkformat

import (
	"image"
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
				if t == runeSpace {
					sp := icspace(r)
					ret.value = append(ret.value, []rune(sp)...)
					ret.len += StringWidth(sp)
				} else {
					ret.value = append(ret.value, r)
					ret.len += RuneWidth(r)
				}
			} else {
				break
			}

			s.idx += w
		}
	}

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
	inQuote    bool
	inComment  string
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

	// Do the job
	line, length := make(words_t, 0, 10), uint32(0)
	nobrk := false
	cont := true
	nextWordIsNaturalStart := true

	appendReset := func() {
		// lines = append(lines, line)
		last := line.last()
		cont = line.adjustableJoin(o)

		line = line[:0]
		if last != nil && last.getType() == runeContToNext {
			line = append(line, lineContFrom)
		}
	}

	for t := ws.nextWord(); t != nil && cont; t = ws.nextWord() {
		if t.startsWith("```") {
			nobrk = !nobrk
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
						t2.value = t.value[:len1]
						t2.setLen(len1)
						line = append(line, t2, lineContTo.dup())
						appendReset()
						t.value = t.value[len1:]
						t.incLen(-len1)
					} else {
						line = append(line, lineContTo.dup())
						appendReset()
					}
					goto AGAIN
				}

				if t.isInMap(canStayAtEnd) && t.len <= 3 {
					t.setType(runeExtraAtEnd)
					line = append(line, t)
					appendReset()

					if y, ni := ws.nextRuneIsEndOfLine(); y {
						ws.idx = ni
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
