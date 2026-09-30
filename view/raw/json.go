package raw

import "strings"

// jsonLine highlights one line of indented JSON. json.Indent puts every
// string on one line, so a line can be highlighted on its own: a string
// followed by a colon is a key, other strings are str, numbers are num,
// true, false and null are lit, and structural characters are punct.
func jsonLine(s string) line {
	var l lineBuilder
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '"':
			end := quotedEnd(s, i)
			class := "str"
			if strings.HasPrefix(strings.TrimLeft(s[end:], " "), ":") {
				class = "key"
			}
			l.add(class, s[i:end])
			i = end
		case c == '-' || isDigit(c):
			end := i + 1
			for end < len(s) && strings.IndexByte("0123456789.eE+-", s[end]) >= 0 {
				end++
			}
			l.add("num", s[i:end])
			i = end
		case 'a' <= c && c <= 'z':
			end := i + 1
			for end < len(s) && 'a' <= s[end] && s[end] <= 'z' {
				end++
			}
			l.add("lit", s[i:end])
			i = end
		case strings.IndexByte("{}[],:", c) >= 0:
			l.add("punct", s[i:i+1])
			i++
		default:
			l.add("", s[i:i+1])
			i++
		}
	}
	return l.spans
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }
