package kkformat

import (
	"bytes"
	"encoding/binary"
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
	spaces          = strings.Repeat(" ", 80)
	lineContinues   = (&word_t{}).setType(runeContinues).setLen(1).setValue([]rune{'\\'})
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
	runeContinues
	runeEndOfBuffer
)

func runeType(r rune) uint16 {
	if r == ' ' || r == '\t' || r == fullSpace {
		return runeSpace
	} else if r == '\n' {
		return runeNewline
	} else if unicode.IsPunct(r) || unicode.IsSymbol(r) {
		if runeWidth(r) == 1 {
			return runeHalfDelim
		} else {
			return runeFullDelim
		}
	} else if runeWidth(r) == 2 {
		return runeFull
	} else if unicode.IsDigit(r) || unicode.IsLetter(r) {
		return runeLatin
	}

	return runeUnknown
}

func runeWidth(r rune) uint32 {
	if r == '\t' {
		return tabWidth
	}

	s := string(r)
	if doubleBytes.FindString(s) == s {
		return 2
	}

	return 1
}

func stringWidth(s interface{}) uint32 {
	i := uint32(0)
	switch s.(type) {
	case string:
		for _, r := range s.(string) {
			i += runeWidth(r)
		}
	case []rune:
		for _, r := range s.([]rune) {
			i += runeWidth(r)
		}
	}

	return i
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
		if runeWidth(r) == 1 {
			whole.WriteString("<dt>")
		} else {
			whole.WriteString("<dd>")
		}
		whole.WriteRune(r)
	}

	return whole.String()
}

func itob(id uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, id)
	return buf
}
