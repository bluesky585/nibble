// Package markdown finds structural blocks in Markdown text.
package markdown

import (
	"strings"
	"unicode/utf8"
)

// Table is a GFM table span in rune offsets [Start, End).
type Table struct {
	Text  string
	Start int
	End   int
}

type line struct {
	Text  string
	Start int
	End   int
}

// Tables returns GFM pipe tables in order. Spans do not overlap.
func Tables(text string) []Table {
	lines := splitLines(text)
	var out []Table
	for i := 0; i < len(lines); {
		if !isTableStart(lines, i) {
			i++
			continue
		}
		end := tableEnd(lines, i)
		seg := lines[i:end]
		out = append(out, Table{
			Text:  joinLines(seg),
			Start: seg[0].Start,
			End:   seg[len(seg)-1].End,
		})
		i = end
	}
	return out
}

func splitLines(text string) []line {
	if text == "" {
		return nil
	}
	var out []line
	startB, startR := 0, 0
	b, r := 0, 0
	for b < len(text) {
		if text[b] == '\n' {
			endB := b + 1
			endR := r + 1
			out = append(out, line{Text: text[startB:endB], Start: startR, End: endR})
			startB, startR = endB, endR
			b, r = endB, endR
			continue
		}
		_, w := utf8.DecodeRuneInString(text[b:])
		b += w
		r++
	}
	if startB < len(text) {
		out = append(out, line{Text: text[startB:], Start: startR, End: r})
	}
	return out
}

func joinLines(seg []line) string {
	var b strings.Builder
	for _, l := range seg {
		b.WriteString(l.Text)
	}
	return b.String()
}

func isTableStart(lines []line, i int) bool {
	return i+1 < len(lines) && isPipeRow(lines[i].Text) && isSeparator(lines[i+1].Text)
}

func tableEnd(lines []line, i int) int {
	j := i + 2
	for j < len(lines) && isPipeRow(lines[j].Text) {
		j++
	}
	return j
}

func isPipeRow(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "|") && strings.Count(t, "|") >= 2
}

func isSeparator(s string) bool {
	t := strings.TrimSpace(s)
	if !strings.Contains(t, "|") || !strings.Contains(t, "-") {
		return false
	}
	for _, r := range t {
		switch r {
		case '|', '-', ':', ' ', '\t', '\n', '\r':
		default:
			return false
		}
	}
	return true
}
