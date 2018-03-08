package kkformat

import (
	"bytes"
	"regexp"
	"unicode"
)

var (
	uriSchemes = map[string]bool{
		"zzz": true, "gopher": true, "news": true, "snmp": true,
		"aaa": true, "h323": true, "nfs": true, "stun": true,
		"aaas": true, "http": true, "ni": true, "stuns": true,
		"about": true, "https": true, "nih": true, "tag": true,
		"acap": true, "iax": true, "nntp": true, "tel": true,
		"acct": true, "icap": true, "opaquelocktoken": true, "telnet": true,
		"cap": true, "im": true, "pkcs11": true, "tftp": true,
		"cid": true, "imap": true, "pop": true, "thismessage": true,
		"coap": true, "info": true, "pres": true, "tip": true,
		"coaps": true, "ipp": true, "reload": true, "tn3270": true,
		"crid": true, "ipps": true, "rtsp": true, "turn": true,
		"data": true, "iris": true, "rtsps": true, "turns": true,
		"dav": true, "jabber": true, "rtspu": true, "tv": true,
		"dict": true, "ldap": true, "service": true, "urn": true,
		"dns": true, "mailto": true, "session": true, "vemmi": true,
		"example": true, "mid": true, "shttp": true, "vnc": true,
		"file": true, "msrp": true, "sieve": true, "ws": true,
		"ftp": true, "msrps": true, "sip": true, "wss": true,
		"geo": true, "mtqp": true, "sips": true, "xcon": true,
		"go": true, "mupdate": true, "sms": true, "xmpp": true,
	}

	canStayAtEnd = map[string]bool{
		".": true, ",": true, ":": true, ")": true, "）": true, "]": true, "}": true,
		"。": true, "，": true, "：": true, "．": true, "、": true,
		"”": true, "〉": true, "》": true, "」": true, "』": true,
		"】": true, "〕": true, "〗": true, "〙": true, "〛": true,
	}

	cannotStayAtEnd = map[string]bool{
		"(": true, "（": true, "[": true, "{": true, "\"": true,
		"“": true, "〈": true, "《": true, "「": true, "『": true,
		"【": true, "〔": true, "〖": true, "〘": true, "〚": true,
	}

	extendablePunc = map[string]bool{
		"。": true, "，": true, "：": true, "．": true, "、": true,
	}

	validURIChars = map[string]bool{
		"A": true, "B": true, "C": true, "D": true, "E": true, "F": true, "G": true, "H": true, "I": true, "J": true, "K": true, "L": true, "M": true,
		"N": true, "O": true, "P": true, "Q": true, "R": true, "S": true, "T": true, "U": true, "V": true, "W": true, "X": true, "Y": true, "Z": true,
		"a": true, "b": true, "c": true, "d": true, "e": true, "f": true, "g": true, "h": true, "i": true, "j": true, "k": true, "l": true, "m": true,
		"n": true, "o": true, "p": true, "q": true, "r": true, "s": true, "t": true, "u": true, "v": true, "w": true, "x": true, "y": true, "z": true,
		"0": true, "1": true, "2": true, "3": true, "4": true, "5": true, "6": true, "7": true, "8": true, "9": true, "-": true,
		".": true, "_": true, "~": true, ":": true, "/": true, "?": true, "#": true, "[": true, "]": true, "@": true, "!": true,
		"$": true, "&": true, "'": true, "(": true, ")": true, "*": true, "+": true, ",": true, ";": true, "=": true, "`": true, "%": true,
	}

	doubleBytes = regexp.MustCompile(`[^\x00-\xff]`)

	lineContinues = (&word_t{}).setType(runeContinues).setLen(1).setValue([]rune{'\\'})

	newLine = (&word_t{}).setType(runeNewline)
)

const (
	tabWidth  = 4
	fullSpace = '　'
	spaces    = "                                                                                " // 80 spaces
)

const (
	runeUnknown = iota

	runeDelim
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
	} else if (unicode.IsPunct(r) || unicode.IsSymbol(r)) && runeWidth(r) == 1 {
		return runeDelim
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
