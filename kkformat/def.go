package kkformat

import (
	"bytes"
	"image"
	"image/color"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	uriSchemes      = regexp.MustCompile(`(zzz|gopher|news|snmp|aaa|h323|nfs|stun|aaas|http|ni|stuns|about|https|nih|tag|acap|iax|nntp|tel|acct|icap|opaquelocktoken|telnet|cap|im|pkcs11|tftp|cid|imap|pop|thismessage|coap|info|pres|tip|coaps|ipp|reload|tn3270|crid|ipps|rtsp|turn|data|iris|rtsps|turns|dav|jabber|rtspu|tv|dict|ldap|service|urn|dns|mailto|session|vemmi|example|mid|shttp|vnc|file|msrp|sieve|ws|ftp|msrps|sip|wss|geo|mtqp|sips|xcon|go|mupdate|sms|xmpp)`)
	canStayAtEnd    = regexp.MustCompile(`^[\.\,\:\)\]\}。，：．、”）〉》」』】〕〗〙〛]+$`)
	cannotStayAtEnd = regexp.MustCompile(`^[\(（\[\{\"“〈《「『【〔〖〘〚]+$`)
	extendablePunc  = regexp.MustCompile(`^[。，：．、]+$`)
	validURIChars   = regexp.MustCompile(`^[A-Za-z0-9\-\._~:\/\?\#\[\]\@\!\$\&\'\(\)\*\+\,\;\=\%]+$`)
	doubleBytes     = regexp.MustCompile(`[^\x00-\xff]`)
	latinSymbol     = regexp.MustCompile(`^[\(\)\+\-\*\\\/\!\%\^\&\=<>\[\]\s\:\;]+$`)
	spaces          = strings.Repeat(" ", 80)
	lineContTo      = (&word_t{}).setType(runeContToNext).setLen(1).setValue([]rune{'\\'})
	lineContFrom    = (&word_t{}).setType(runeContFromPrev).setLen(1).setValue([]rune{'\\'})
	spaceWord       = (&word_t{}).setType(runeSpace).setLen(1).setValue([]rune{' '})
	newLine         = (&word_t{}).setType(runeNewline)
)

const (
	tabWidth  = 4
	fullSpace = '\u3000' // CJK space
)

const (
	runeUnknown = iota

	runeHalfDelim
	runeFullDelim
	runeSpace
	runeNewline
	runeLatin
	runeFull

	// not actually runes
	runeExtraAtEnd
	runeContToNext
	runeContFromPrev
	runeEndOfBuffer
)

const (
	specialNone         = iota
	specialComment      // "//"
	specialCommentStart // "/*"
	specialCommentEnd   // "*/"
	specialCommentHash  // "#"
	specialDoubleQuote  // "\""
	specialSingleQuote  // "'"
	specialLineNumber
)

const (
	TNBackground = iota
	TNNormal
	TNLineWrap
	TNLineNumber
	TNSymbol
	TNString
	TNNumber
	TNComment
)

var (
	grayFG     = image.NewUniform(color.RGBA{0xcc, 0xcc, 0xcc, 255})
	darkGrayFG = image.NewUniform(color.RGBA{0x66, 0x66, 0x66, 255})
)

var WhiteTheme = []image.Image{
	image.White,
	image.Black,
	grayFG,
	darkGrayFG,
	image.NewUniform(color.RGBA{0x5d, 0x40, 0x37, 255}),
	image.NewUniform(color.RGBA{0x51, 0x2d, 0xa8, 255}),
	image.NewUniform(color.RGBA{0xff, 0x57, 0x22, 255}),
	image.NewUniform(color.RGBA{0x00, 0x79, 0x6b, 255}),
}

var PureWhiteTheme = []image.Image{
	image.White,
	image.Black,
	grayFG,
	darkGrayFG,
	image.Black,
	image.Black,
	image.Black,
	image.Black,
}

var PureBlackTheme = []image.Image{
	image.Black,
	image.White,
	grayFG,
	darkGrayFG,
	image.White,
	image.White,
	image.White,
	image.White,
}

var BlackTheme = []image.Image{
	image.Black,
	image.White,
	grayFG,
	darkGrayFG,
	image.NewUniform(color.RGBA{0xbb, 0xbb, 0xbb, 255}),
	image.NewUniform(color.RGBA{0x00, 0xbc, 0xd4, 255}),
	image.NewUniform(color.RGBA{0xff, 0x98, 0x00, 255}),
	image.NewUniform(color.RGBA{0x00, 0x96, 0x88, 255}),
}

func GetPalette() color.Palette {
	p := make(color.Palette, 0)
	for i := 0; i <= TNComment; i++ {
		p = append(p, WhiteTheme[i].At(0, 0), BlackTheme[i].At(0, 0))
	}
	return p
}

func runeType(r rune) uint16 {
	if r == ' ' || r == '\t' || r == fullSpace {
		return runeSpace
	} else if r == '\n' {
		return runeNewline
	} else if unicode.IsPunct(r) || unicode.IsSymbol(r) {
		if RuneWidth(r) == 1 {
			return runeHalfDelim
		} else {
			return runeFullDelim
		}
	} else if RuneWidth(r) == 2 {
		return runeFull
	} else if unicode.IsDigit(r) || unicode.IsLetter(r) {
		return runeLatin
	}

	return runeUnknown
}

func RuneWidth(r rune) uint32 {
	if r == '\t' {
		return tabWidth
	}

	s := string(r)
	if doubleBytes.FindString(s) == s {
		return 2
	}

	return 1
}

func StringWidth(s interface{}) uint32 {
	i := uint32(0)
	switch s.(type) {
	case string:
		for _, r := range s.(string) {
			i += RuneWidth(r)
		}
	case []rune:
		for _, r := range s.([]rune) {
			i += RuneWidth(r)
		}
	}

	return i
}

func Trunc(s string, w uint32) string {
	if StringWidth(s) <= w {
		return s
	}

	ret := make([]rune, 0, w)
	for _, r := range s {
		rw := RuneWidth(r)
		if w < rw {
			break
		}

		w -= rw
		ret = append(ret, r)
	}
	return string(ret)
}

func appendSpaces(text string, count int, forceAtRight bool) string {
	if count == 1 {
		return text + " "
	}

	if forceAtRight {
		return text + spaces[:count]
	}

	x := count / 2
	return spaces[:x] + text + spaces[:count-x]
}

var spacesRune = []rune{
	' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ',
	' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ',
	' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ',
	' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ',
	' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ', ' ',
}

func appendSpacesRune(runes []rune, count uint32, forceAtRight bool) []rune {
	if count == 1 {
		return append(runes, ' ')
	}

	if forceAtRight {
		return append(runes, spacesRune[:count]...)
	}

	x := count / 2
	buf := make([]rune, uint32(len(runes))+count)
	copy(buf, spacesRune[:x])
	copy(buf[x:], runes)
	copy(buf[x+uint32(len(runes)):], spacesRune[:count-x])

	return buf
}

func calcTag(value []rune) string {
	whole := &bytes.Buffer{}

	for _, r := range value {
		if RuneWidth(r) == 1 {
			whole.WriteString("<dt>")
		} else {
			whole.WriteString("<dd>")
		}
		whole.WriteRune(r)
	}

	return whole.String()
}

func BytesToPlane0String(buf []byte) string {
	str := make([]byte, 0, len(buf))
	enc := make([]byte, 3)

	for i := 0; i < len(buf); {
		if buf[i] < 128 {
			str = append(str, buf[i])
			i++
			continue
		}

		ln := 2 * int((buf[i]&0x7f)+1)
		if i+1+ln > len(buf) {
			return ""
		}

		for j := i + 1; j < i+1+ln; j += 2 {
			n := utf8.EncodeRune(enc, rune(buf[j])<<8+rune(buf[j+1]))
			str = append(str, enc[:n]...)
		}

		i += 1 + ln
	}

	return string(str)
}

func Plane0StringToBytes(str string) []byte {
	buf := make([]byte, 0, len(str))
	queue := make([]byte, 0, 256)

	appendQueue := func() {
		buf = append(buf, byte(len(queue)/2-1)|0x80)
		buf = append(buf, queue...)
		queue = queue[:0]
	}

	for _, r := range str {
		if r < 128 {
			if len(queue) > 0 {
				appendQueue()
			}
			buf = append(buf, byte(r))
		} else {
			queue = append(queue, byte(r>>8), byte(r))
			if len(queue)/2 == 128 {
				appendQueue()
			}
		}
	}

	if len(queue) > 0 {
		appendQueue()
	}

	return buf
}
