package raw

import "strings"

// metricsLine highlights one line of the Prometheus or OpenMetrics text
// format. A line starting with "#" is a comment (HELP, TYPE, EOF). A sample
// line is a metric name, optional labels in braces and whitespace-separated
// value and timestamp: the name is name, label names are label, label values
// are str, braces, "=" and "," are punct, and value and timestamp are num.
func metricsLine(s string) line {
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "#") {
		return line{{Class: "comment", Text: s}}
	}
	var l lineBuilder
	end := strings.IndexAny(s, "{ \t")
	if end < 0 {
		end = len(s)
	}
	l.add("name", s[:end])
	i := end
	if i < len(s) && s[i] == '{' {
		i = labels(&l, s, i)
	}
	for i < len(s) {
		j := i
		isSpace := s[i] == ' ' || s[i] == '\t'
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') == isSpace {
			j++
		}
		class := "num"
		if isSpace {
			class = ""
		}
		l.add(class, s[i:j])
		i = j
	}
	return l.spans
}

// labels highlights the label set that starts with the "{" at s[start] and
// returns the index after its closing "}", or len(s) when it is not closed.
func labels(l *lineBuilder, s string, start int) int {
	l.add("punct", "{")
	for i := start + 1; i < len(s); {
		switch s[i] {
		case '}':
			l.add("punct", "}")
			return i + 1
		case '"':
			end := quotedEnd(s, i)
			l.add("str", s[i:end])
			i = end
		case '=', ',':
			l.add("punct", s[i:i+1])
			i++
		case ' ':
			l.add("", " ")
			i++
		default:
			end := i + 1
			for end < len(s) && strings.IndexByte("=,}\" ", s[end]) < 0 {
				end++
			}
			l.add("label", s[i:end])
			i = end
		}
	}
	return len(s)
}
