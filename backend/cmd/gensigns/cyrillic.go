package main

import (
	"strings"
	"unicode"
)

// singleLatn maps single Uzbek Latin letters to Uzbek Cyrillic. Digraphs
// (o' g' sh ch ts, y+vowel) and the context-sensitive "e" are handled
// separately in toCyrillic.
var singleLatn = map[rune]string{
	'a': "а", 'b': "б", 'c': "ц", 'd': "д", 'f': "ф", 'g': "г",
	'h': "ҳ", 'i': "и", 'j': "ж", 'k': "к", 'l': "л", 'm': "м", 'n': "н",
	'o': "о", 'p': "п", 'q': "қ", 'r': "р", 's': "с", 't': "т", 'u': "у",
	'v': "в", 'x': "х", 'y': "й", 'z': "з",
}

func isLatinVowel(r rune) bool {
	switch unicode.ToLower(r) {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

func isLatinConsonant(r rune) bool {
	return unicode.IsLetter(r) && !isLatinVowel(r)
}

// toCyrillic transliterates Uzbek Latin text to Uzbek Cyrillic. uz-Latn and
// uz-Cyrl are two scripts for the SAME language, so this is a deterministic
// transliteration, not a translation. It follows standard Uzbek Cyrillic
// orthography; the cases handled explicitly are:
//   - o'→ў and g'→ғ (the apostrophe is the modifier, not the tutuq belgisi),
//   - the tutuq belgisi ' → ъ (any apostrophe not forming o'/g'),
//   - e → э at a word start or after a vowel (Aeroport→Аэропорт); else e → е,
//   - iotated y+vowel → я/ё/ю/е, EXCEPT: when the vowel begins an o' digraph
//     (yo'l→йўл), and consonant+ye → ъе for loanwords (obyekt→объект),
//   - ts → ц (mototsikl→мотоцикл).
func toCyrillic(s string) string {
	r := []rune(s)
	var b strings.Builder
	cap := func(lower string, isUpper bool) string {
		if isUpper {
			return strings.ToUpper(lower)
		}
		return lower
	}
	isSep := func(x rune) bool { return !unicode.IsLetter(x) && x != '\'' }

	for i := 0; i < len(r); {
		c := r[i]
		isUpper := unicode.IsUpper(c)
		lc := unicode.ToLower(c)
		var ln rune
		if i+1 < len(r) {
			ln = unicode.ToLower(r[i+1])
		}
		prevVowel := i > 0 && isLatinVowel(r[i-1])
		prevConsonant := i > 0 && isLatinConsonant(r[i-1])
		wordStart := i == 0 || isSep(r[i-1])

		// o' g' — the apostrophe modifier takes priority.
		if i+1 < len(r) && r[i+1] == '\'' && (lc == 'o' || lc == 'g') {
			if lc == 'o' {
				b.WriteString(cap("ў", isUpper))
			} else {
				b.WriteString(cap("ғ", isUpper))
			}
			i += 2
			continue
		}
		// sh, ch, ts
		if lc == 's' && ln == 'h' {
			b.WriteString(cap("ш", isUpper))
			i += 2
			continue
		}
		if lc == 'c' && ln == 'h' {
			b.WriteString(cap("ч", isUpper))
			i += 2
			continue
		}
		if lc == 't' && ln == 's' {
			b.WriteString(cap("ц", isUpper))
			i += 2
			continue
		}
		// y + vowel → iotated vowel, with two exceptions.
		if lc == 'y' && (ln == 'a' || ln == 'o' || ln == 'u' || ln == 'e') {
			startsApostrophe := ln == 'o' && i+2 < len(r) && r[i+2] == '\''
			if !startsApostrophe {
				var m string
				switch ln {
				case 'a':
					m = "я"
				case 'o':
					m = "ё"
				case 'u':
					m = "ю"
				case 'e':
					// loanword consonant+ye keeps the hard sign (obyekt→объект)
					if prevConsonant {
						m = "ъе"
					} else {
						m = "е"
					}
				}
				b.WriteString(cap(m, isUpper))
				i += 2
				continue
			}
		}
		// tutuq belgisi
		if c == '\'' {
			b.WriteString("ъ")
			i++
			continue
		}
		// e → э at a word start or after a vowel; else е.
		if lc == 'e' {
			if wordStart || prevVowel {
				b.WriteString(cap("э", isUpper))
			} else {
				b.WriteString(cap("е", isUpper))
			}
			i++
			continue
		}
		if m, ok := singleLatn[lc]; ok {
			b.WriteString(cap(m, isUpper))
			i++
			continue
		}
		// passthrough: spaces, punctuation, guillemets, digits, etc.
		b.WriteRune(c)
		i++
	}
	return b.String()
}
