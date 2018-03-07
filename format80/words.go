package format80

import (
	"bytes"
	"strings"
)

type words_t []*word_t

func (w *words_t) adjustableJoin(opt *Formatter) {
	words := *w
	if len(words) == 0 {
		return
	}

	length := 0
	naturalEnd := words.last().ty == runeNewline || words.last().ty == runeEndOfBuffer

	opt.resetPDL()
	var exEnding *word_t

	for i, word := range words {
		if !naturalEnd {
			// the leading spaces of 2, 4, 8, 16 ... will be preserved, others will be discarded
			if word.len > 0 && i == 0 {
				if l, _ := word.surroundingSpaces(); l == 2 || l%4 != 0 {
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
			if word.ty == runeExtraAtEnd || word.ty == runeContinues {
				exEnding = word
				continue
			}

			if word.url == nil && i < len(words)-1 {
				if word.ty == runeDelim {
					opt.wd = append(opt.wd, word)
				}
				if word.ty == runeLatin {
					opt.wl = append(opt.wl, word)
				}
			}

			length += word.len
			opt.wp = append(opt.wp, word)
		}
	}

	opt.wd = append(opt.wd, opt.wl...)

	// adjust
	gap, fillstart := opt.Columns-length, 0
	if len(opt.wp) <= 1 {
		if exEnding != nil {
			opt.wp = append(opt.wp, exEnding)
		}
		opt.wp.join(opt)
		return
	}

	// extendable punctuations are those which are full wide but look like half wide. If it stays at the end
	// of a line, the format would be a little awkward because there seems 1 space after it.
	if !naturalEnd && opt.wp.last().isInMap(extendablePunc) {
		gap++
	}

	if naturalEnd || gap == 0 {
		if exEnding != nil {
			opt.wp = append(opt.wp, exEnding)
		}
		opt.wp.join(opt)
		return
	}

	if opt.wp[0].startsWith("  ") {
		fillstart = 1
	}

	ln := len(opt.wp) - 1 - fillstart
	lnh := len(opt.wd)
	if ln == 0 {
		opt.wp.join(opt)
		return
	}

	if lnh >= gap {
		// we have enough delimeters / latin characters, append spaces to them
		if gap == 1 {
			opt.wd[lnh/2].value = append(opt.wd[lnh/2].value, ' ')
		} else {
			dk := lnh / gap
			for i := 0; i < len(opt.wd); i += dk {
				opt.wd[i].value = append(opt.wd[i].value, ' ')
				if gap--; gap <= 0 {
					break
				}
			}
		}
	} else if gap+1 <= ln {
		dk := ln / (gap + 1)
		for i := fillstart + dk; i < len(opt.wp); i += dk {
			opt.wp[i].value = append(opt.wp[i].value, ' ')
			if gap--; gap <= 0 {
				break
			}
		}
	} else {
		for i := fillstart; i < len(opt.wp)-1; i++ {
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

	if exEnding != nil {
		opt.wp = append(opt.wp, exEnding)
	}
	opt.wp.join(opt)
}

func (w *words_t) updateURL(o *Formatter) {
	o.urls = append(o.urls, w.rawJoin())
	for _, word := range *w {
		word.url = &o.urls[len(o.urls)-1]
	}
}

func (w words_t) dup() words_t {
	w2 := make(words_t, len(w))
	for i, word := range w {
		w2[i] = word.dup()
	}

	return w2
}

func (w *words_t) join(opt *Formatter) {
	words := *w

	opt.write("<div")
	if len(words) > 0 && words[0].url != nil {
		u := *words[0].url
		if strings.HasPrefix(u, "#toc-f-") {
			opt.write(" class=cls-", u[1:26], " id=toc-r-", u[7:])
		}

		if strings.HasPrefix(u, "#toc-r-") {
			opt.write(" class=cls-", u[1:26], " id=toc-f-", u[7:])
		}
	}
	opt.write(">")

	for i := 0; i < len(words); i++ {
		word := words[i]
		if word.url != nil {
			if word.ty == runeImage {
				opt.write("<img class='_image' src='", *word.url, "' alt='", *word.url, "'>")
				break
			}

			if i == 0 || words[i-1].url == nil || (*words[i-1].url != *word.url) {
				opt.write("<a ")
				if (*word.url)[0] != '#' {
					opt.write(opt.LinkTarget)
				}
				opt.write(" href='", *word.url, "'>")
			}
		}

		if i == 0 && len(word.value) >= 4 && word.startsWith("====") {
			opt.tmp.Reset()
			opt.write("<hr>")
			opt.flush()
			return
		}

		if bytes.HasSuffix(opt.tmp.Bytes(), []byte("</dl>")) {
			opt.tmp.Truncate(opt.tmp.Len() - 5)
		} else {
			opt.write("<dl>")
		}

		opt.write(CalcTag(word.value))
		if word.ty == runeContinues {
			opt.tmp.Truncate(opt.tmp.Len() - 2)
			opt.write(" class=conj>&rarr;")
		}

		opt.write("</dl>")

		if word.url != nil {
			if i == len(words)-1 || words[i+1].url == nil || *words[i+1].url != *word.url {
				opt.write("</a>")
			}
		}
	}

	opt.write("</div>")
	opt.flush()
}

func (w *words_t) rawJoin() string {
	buf := &bytes.Buffer{}

	for _, word := range *w {
		if word.value != nil && word.ty != runeContinues {
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
