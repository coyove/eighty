package kkformat

import (
	"fmt"
	"strconv"

	"golang.org/x/image/math/fixed"
)

type words_t []*word_t

func (w *words_t) adjustableJoin(opt *Formatter) bool {
	_ = fmt.Sprintf

	words := *w
	if len(words) == 0 {
		return true
	}

	length := uint32(0)
	naturalEnd := words.last().getType() == runeNewline || words.last().getType() == runeEndOfBuffer

	opt.resetPDL()
	var exEnding, exBegining *word_t
	setEx := func() {
		if exBegining != nil {
			opt.wp = append(opt.wp, nil)
			copy(opt.wp[1:], opt.wp)
			opt.wp[0] = exBegining
		}

		if exEnding != nil {
			opt.wp = append(opt.wp, exEnding)
		}
	}

	if words[0].getType() == runeContFromPrev {
		exBegining = words[0]
		words = words[1:]
	}

	// the leading spaces of 2, 4, 6, 8 ... will be preserved, others will be discarded
	if l, _ := words[0].surroundingSpaces(); l%2 != 0 {
		if words.last().getType() == runeContToNext || words[0].isCode() {
			// ignore
		} else if !naturalEnd || !words[0].isNaturalStart() {
			words[0].value = words[0].value[l:]
			if words[0].len -= l; words[0].len == 0 {
				words = words[1:]
			}
		}
	}

	if !naturalEnd && len(words) > 0 {
		word := words.last()
		if word.getType() == runeContToNext {
			// ignore
		} else {
			// the trailing spaces will always be discarded
			_, r := word.surroundingSpaces()
			word.value = word.value[:uint32(len(word.value))-r]
			if word.len -= r; word.len == 0 {
				words = words[:len(words)-1]
			}
		}
	}

	for i, word := range words {
		// fmt.Println(string(word.value), word.len, word.getType())
		if word.len > 0 {
			if word.getType() == runeExtraAtEnd || word.getType() == runeContToNext {
				exEnding = word
				continue
			}

			if i < len(words)-1 {
				if t := word.getType(); t == runeFullDelim || t == runeHalfDelim {
					opt.wd = append(opt.wd, word)
				}
				if word.getType() == runeLatin {
					opt.wl = append(opt.wl, word)
				}
			}

			length += word.len
			opt.wp = append(opt.wp, word)
		}
	}

	// fmt.Println(length)
	opt.wd = append(opt.wd, opt.wl...)
	// adjust
	gap, fillstart := opt.Columns-length, uint32(0)
	if len(opt.wp) <= 1 {
		setEx()
		return opt.wp.join(opt)
	}

	// extendable punctuations are those which are full wide but look like half wide. If it stays at the end
	// of a line, the format would be a little awkward because there seems 1 space after it.
	// if !naturalEnd && opt.wp.last().isInMap(extendablePunc) {
	// 	if !opt.wp[0].isCode() {
	// 		gap++
	// 	}
	// }

	if naturalEnd || gap == 0 {
		setEx()
		return opt.wp.join(opt)
	}

	if opt.wp[0].startsWith("  ") {
		fillstart = 1
	}

	ln := uint32(len(opt.wp)) - 1 - fillstart
	lnh := uint32(len(opt.wd))
	if ln == 0 {
		setEx()
		return opt.wp.join(opt)
	}

	// fmt.Println("===", lnh, gap)

	if lnh >= gap {
		// we have enough delimeters / latin characters, append spaces to them
		if gap == 1 {
			opt.wd[lnh/2].value = append(opt.wd[lnh/2].value, ' ')
		} else {
			dk := lnh / gap
			for i := uint32(0); i < uint32(len(opt.wd)); i += dk {
				opt.wd[i].value = append(opt.wd[i].value, ' ')
				if gap--; gap <= 0 {
					break
				}
			}
		}
	} else if gap+1 <= ln {
		dk := ln / (gap + 1)
		for i := fillstart + dk; i < uint32(len(opt.wp)); i += dk {
			opt.wp[i].value = append(opt.wp[i].value, ' ')
			if gap--; gap <= 0 {
				break
			}
		}
	} else {
		for i := fillstart; i < uint32(len(opt.wp))-1; i++ {
			r := ln - i + fillstart
			dk := gap / r
			if dk*r != gap {
				dk++
			}

			opt.wp[i].value = appendSpacesRune(opt.wp[i].value, dk, i == fillstart)

			if gap -= dk; gap <= 0 {
				break
			}
		}
	}

	setEx()
	return opt.wp.join(opt)
}

func (w *words_t) join(opt *Formatter) bool {
	words := *w
	opt.Rows++
	dy := opt.LineHeight

	if opt.CurrentY+dy+dy/2 > opt.Img.Dst.Bounds().Dy() {
		return false
	}

	opt.CurrentY += dy
	dx := (int(opt.Img.MeasureString("a")) >> 6)

	var exEnding bool

	if len(words) > 0 && words[0].getType() == runeContFromPrev {
		opt.Img.Dot = fixed.P(1, opt.CurrentY+dy/4)
		opt.Img.Src = opt.Theme[TNLineWrap]
		opt.Img.DrawString("\u2937")
		opt.Img.Src = opt.Theme[TNNormal]
		words = words[1:]
	}

	if len(words) > 0 && words.last().getType() == runeContToNext {
		opt.Img.Dot = fixed.P((dx+1)*(int(opt.Columns)+2), opt.CurrentY+dy/4)
		opt.Img.Src = opt.Theme[TNLineWrap]
		opt.Img.DrawString("\u2936")
		opt.Img.Src = opt.Theme[TNNormal]
		words = words[:len(words)-1]
		exEnding = true
	}

	x := dx*2 + 2

	for i := 0; i < len(words); i++ {
		word := words[i]
		opt.Img.Src = opt.Theme[TNNormal]

		draw := func() {
			for _, r := range word.value {
				if w := RuneWidth(r); w == 1 {
					opt.Img.Dot = fixed.P(x, opt.CurrentY)
					opt.Img.DrawString(string(r))
					x += dx + 1
				} else {
					x++
					opt.Img.Dot = fixed.P(x, opt.CurrentY)
					opt.Img.DrawString(string(r))
					x += dx*2 + 1
				}
			}
		}

		if word.getSpecialType() == specialLineNumber {
			opt.Img.Src = opt.Theme[TNLineNumber]
			draw()
		} else if !word.isCode() {
			draw()
		} else {
			opt.Img.Src = opt.Theme[TNNormal]

			switch opt.curSpecial {
			case specialDoubleQuote, specialSingleQuote:
				opt.Img.Src = opt.Theme[TNString]
			case specialComment, specialCommentStart, specialCommentEnd, specialCommentHash:
				opt.Img.Src = opt.Theme[TNComment]
			default:
				if _, err := strconv.ParseFloat(string(word.value), 64); err == nil {
					opt.Img.Src = opt.Theme[TNNumber]
				}
				if word.isInMap(latinSymbol) {
					opt.Img.Src = opt.Theme[TNSymbol]
				}
			}

			switch sp := word.getSpecialType(); sp {
			case specialDoubleQuote, specialSingleQuote:
				if opt.curSpecial == specialNone { // string starts
					opt.Img.Src = opt.Theme[TNString]
					opt.curSpecial = sp
				} else if sp == opt.curSpecial { // string ends
					opt.Img.Src = opt.Theme[TNString]
					opt.curSpecial = specialNone
				}
				draw()
			case specialComment, specialCommentHash, specialCommentStart:
				if opt.curSpecial == specialNone { // comment starts
					opt.Img.Src = opt.Theme[TNComment]
					opt.curSpecial = sp
				}
				draw()
			case specialCommentEnd:
				if opt.curSpecial == specialCommentStart { // comment ends
					opt.Img.Src = opt.Theme[TNComment]
					opt.curSpecial = specialNone
				}
				draw()
			default:
				draw()
			}

		}

		opt.Img.Src = opt.Theme[TNNormal]
	}

	if !exEnding && (opt.curSpecial == specialCommentHash || opt.curSpecial == specialComment) {
		opt.curSpecial = specialNone
	}

	return true
}

func (w *words_t) last() *word_t {
	if len(*w) == 0 {
		return nil
	}

	return (*w)[len(*w)-1]
}
