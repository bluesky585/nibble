package markdown

import "strings"

// Code is a fenced code block span in rune offsets [Start, End).
type Code struct {
	Text     string
	Language string
	Start    int
	End      int
}

// CodeBlocks returns backtick-fenced code blocks in order.
// Unclosed fences are ignored. Spans do not overlap.
func CodeBlocks(text string) []Code {
	lines := splitLines(text)
	var out []Code
	for i := 0; i < len(lines); i++ {
		lang, ok := fenceOpen(lines[i].Text)
		if !ok {
			continue
		}
		j := i + 1
		for j < len(lines) && !fenceClose(lines[j].Text) {
			j++
		}
		if j == len(lines) {
			continue
		}
		seg := lines[i : j+1]
		out = append(out, Code{
			Text:     joinLines(seg),
			Language: lang,
			Start:    seg[0].Start,
			End:      seg[len(seg)-1].End,
		})
		i = j
	}
	return out
}

func fenceOpen(s string) (string, bool) {
	t := strings.TrimLeft(s, " \t")
	t = strings.TrimRight(t, "\r\n")
	n := 0
	for n < len(t) && t[n] == '`' {
		n++
	}
	if n < 3 {
		return "", false
	}
	lang := strings.TrimSpace(t[n:])
	if strings.Contains(lang, "`") {
		return "", false
	}
	return lang, true
}

func fenceClose(s string) bool {
	t := strings.TrimLeft(s, " \t")
	t = strings.TrimRight(t, "\r\n \t")
	if len(t) < 3 {
		return false
	}
	for i := 0; i < len(t); i++ {
		if t[i] != '`' {
			return false
		}
	}
	return true
}
