package kkformat

import (
	"fmt"
	"image"
	"image/color"
	"strconv"

	"golang.org/x/image/math/fixed"
)

type words_t []*word_t

var (
	grayFG = image.NewUniform(color.RGBA{0xcc, 0xcc, 0xcc, 255})
)

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

	// the leading spaces of 2, 4, 8, 16 ... will be preserved, others will be discarded
	if l, _ := words[0].surroundingSpaces(); l != 2 && l%4 != 0 {
		if words.last().getType() == runeContToNext {
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
	if !naturalEnd && opt.wp.last().isInMap(extendablePunc) {
		gap++
	}

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
	dy := opt.calcDy()

	if opt.CurrentY+dy*2 > opt.Img.Dst.Bounds().Dy() {
		return false
	}

	opt.CurrentY += dy

	dx := (int(opt.Img.MeasureString("a")) >> 6)
	if len(words) > 0 && words[0].getType() == runeContFromPrev {
		opt.Img.Dot = fixed.P(1, opt.CurrentY+dy/4)
		opt.Img.Src = grayFG
		opt.Img.DrawString("\u2937")
		opt.Img.Src = image.Black
		words = words[1:]
	}

	x := dx*2 + 2

	for i := 0; i < len(words); i++ {
		word := words[i]
		switch word.getType() {
		case runeContToNext:
			x++
			opt.Img.Dot = fixed.P(x, opt.CurrentY+dy/4)
			opt.Img.Src = grayFG
			opt.Img.DrawString("\u2936")
			opt.Img.Src = image.Black
			x += dx*2 + 1
		default:
			opt.Img.Src = opt.Theme[TNNormal]

			if !word.isCode() {
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
			} else {
				str := string(word.value)

				if _, err := strconv.ParseFloat(str, 64); err == nil {
					opt.Img.Src = opt.Theme[TNNumber]
				}

				if word.isInMap(latinSymbol) {
					opt.Img.Src = opt.Theme[TNSymbol]
				}

				if opt.inQuote {
					opt.Img.Src = opt.Theme[TNString]
				}

				if opt.inComment != "" {
					opt.Img.Src = opt.Theme[TNComment]
				}

				for i, r := range word.value {
					next, prev := func() rune {
						if i == len(word.value)-1 {
							return 0
						}
						return word.value[i+1]
					}, func() rune {
						if i == 0 {
							return 0
						}
						return word.value[i-1]
					}

					if w := RuneWidth(r); w == 1 {
						do := func() {
							opt.Img.Dot = fixed.P(x, opt.CurrentY)
							opt.Img.DrawString(string(r))
							x += dx + 1
						}

						if r == '"' && prev() != '\\' {
							if opt.inComment != "" {
								do()
							} else {
								opt.inQuote = !opt.inQuote
								if opt.inQuote {
									opt.Img.Src = opt.Theme[TNString]
									do()
								} else {
									do()
									opt.Img.Src = opt.Theme[TNNormal]
								}
							}
						} else if (r == '/' && next() == '/') || (r == '/' && next() == '*') {
							if opt.inQuote || opt.inComment != "" {
								do()
							} else {
								opt.inComment = string(r) + string(next())
								opt.Img.Src = opt.Theme[TNComment]
								do()
							}
						} else if r == '*' && next() == '/' {
							if opt.inQuote {
								do()
							} else {
								opt.inComment = ""
								do()
								opt.Img.Src = opt.Theme[TNNormal]
							}
						} else {
							do()
						}
					} else {
						x++
						opt.Img.Dot = fixed.P(x, opt.CurrentY)
						opt.Img.DrawString(string(r))
						x += dx*2 + 1
					}
				}
			}

			opt.Img.Src = opt.Theme[TNNormal]
		}
	}

	if last := words.last(); last != nil && opt.inComment == "//" && last.getType() != runeContToNext {
		opt.inComment = ""
	}

	return true
}

func (w *words_t) last() *word_t {
	if len(*w) == 0 {
		return nil
	}

	return (*w)[len(*w)-1]
}
