package kkformat

import (
	"bytes"
	"image"
	"image/color"
	"regexp"
	"strings"
	"unicode"
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
	fullSpace = '　' // it is NOT a ' '
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
	runeImage
	runeExtraAtEnd
	runeContToNext
	runeContFromPrev
	runeEndOfBuffer
)

const (
	TNBackground = iota
	TNNormal
	TNLineWrap
	TNSymbol
	TNString
	TNNumber
	TNComment
)

var WhiteTheme = []image.Image{
	image.White,
	image.Black,
	grayFG,
	image.NewUniform(color.RGBA{0x5d, 0x40, 0x37, 255}),
	image.NewUniform(color.RGBA{0x51, 0x2d, 0xa8, 255}),
	image.NewUniform(color.RGBA{0xff, 0x57, 0x22, 255}),
	image.NewUniform(color.RGBA{0x00, 0x79, 0x6b, 255}),
}

var PureWhiteTheme = []image.Image{
	image.White,
	image.Black,
	grayFG,
	image.Black,
	image.Black,
	image.Black,
	image.Black,
}

var PureBlackTheme = []image.Image{
	image.Black,
	image.White,
	grayFG,
	image.White,
	image.White,
	image.White,
	image.White,
}

var BlackTheme = []image.Image{
	image.Black,
	image.White,
	grayFG,
	image.NewUniform(color.RGBA{0xbb, 0xbb, 0xbb, 255}),
	image.NewUniform(color.RGBA{0x00, 0xbc, 0xd4, 255}),
	image.NewUniform(color.RGBA{0xff, 0x98, 0x00, 255}),
	image.NewUniform(color.RGBA{0x00, 0x96, 0x88, 255}),
}

func GetTheme(theme []image.Image) (color.Palette, image.Image) {
	p := make(color.Palette, TNComment+1)
	for i := 0; i <= TNComment; i++ {
		p[i] = theme[i].At(0, 0)
	}
	return p, theme[TNBackground]
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
