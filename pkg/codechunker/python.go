package codechunker

import (
	"strings"
	"unicode/utf8"
)

// pythonIndentWidth is how many columns a tab counts as when indentation is
// compared. Python only asks that a file be consistent with itself; this is
// a ruler for deciding "deeper than", not a reader of the file's convention.
const pythonIndentWidth = 8

// pythonPieces cuts Python source at top-level statement boundaries.
//
// Go has a parser in the standard library, so goDeclPieces reads exact
// declaration spans. Python has none, and nibble does not take a dependency
// for one. This is a line scanner instead: it tracks bracket depth, string
// literals, and comments, and cuts where a statement at column 0 begins.
//
// The rule is deliberately conservative. A cut that is wrong only makes a
// chunk smaller, because a cut never drops text and the packer joins pieces
// back together when they fit the budget. A cut that is missing costs a
// boundary, which the token fallback covers. So anything the scanner cannot
// be sure about — a line inside brackets, inside a triple-quoted string, or
// continuing after a backslash — is left as part of the piece it is in.
func pythonPieces(text string) []piece {
	if text == "" {
		return nil
	}
	lines := scanPythonLines(text)
	total := utf8.RuneCountInString(text)

	starts := pythonCuts(lines)
	if len(starts) == 0 {
		// Nothing at top level to cut on. The token fallback handles it.
		return nil
	}

	type cut struct {
		byteAt int
		runeAt int
	}
	cuts := make([]cut, 0, len(starts)+2)
	cuts = append(cuts, cut{0, 0})
	for _, i := range starts {
		b, r := lines[i].byteStart, lines[i].runeStart
		// Both ends are already covered by the cuts above and below. A
		// boundary that does not advance would repeat one, which a
		// definition whose decorator is its own cut can produce.
		if r <= 0 || r >= total || r <= cuts[len(cuts)-1].runeAt {
			continue
		}
		cuts = append(cuts, cut{b, r})
	}
	cuts = append(cuts, cut{len(text), total})

	// The walk back can leave exactly one cut, at rune 0, when the file is a
	// single documented definition: that is one piece covering everything,
	// which is the wanted result. A decorated definition keeps its decorators
	// and stays whole.
	out := make([]piece, 0, len(cuts)-1)
	for i := 0; i < len(cuts)-1; i++ {
		out = append(out, piece{
			Text:  text[cuts[i].byteAt:cuts[i+1].byteAt],
			Start: cuts[i].runeAt,
			End:   cuts[i+1].runeAt,
		})
	}
	return out
}

// pyLine is one source line with both its offsets and what the scanner
// learned about where it sits in the file.
type pyLine struct {
	byteStart int
	byteEnd   int // past the newline, if there was one
	runeStart int
	runeEnd   int
	indent    int
	blank     bool
	comment   bool // the first non-space character is '#'
	// topLevel is true when the line begins at bracket depth 0, outside any
	// triple-quoted string, and not continuing the previous line. That is
	// what "a statement starts here" means to this scanner.
	topLevel bool
	text     string
}

// scanPythonLines splits text into lines and, in one pass, records for each
// line whether it starts at top level. The state that carries between lines
// is bracket depth and an open triple-quoted string; a single-quoted string
// cannot span a line in valid Python, so it needs none.
func scanPythonLines(text string) []pyLine {
	var out []pyLine
	depth := 0
	triple := "" // the open `"""` or `'''`, empty when not in one
	prevBackslash := false

	for i, runeAt := 0, 0; i <= len(text); {
		nl := strings.IndexByte(text[i:], '\n')
		lineEnd, next := len(text), len(text)
		if nl >= 0 {
			lineEnd = i + nl
			next = lineEnd + 1
		}
		line := text[i:lineEnd]
		runes := utf8.RuneCountInString(line)
		newlineRunes := 0
		if nl >= 0 {
			newlineRunes = 1
		}

		indent, blank, comment := pythonLineLead(line)
		out = append(out, pyLine{
			byteStart: i,
			byteEnd:   next,
			runeStart: runeAt,
			runeEnd:   runeAt + runes + newlineRunes,
			indent:    indent,
			blank:     blank,
			comment:   comment,
			topLevel:  depth == 0 && triple == "" && !prevBackslash,
			text:      line,
		})

		depth, triple, prevBackslash = scanPythonLine(line, depth, triple)
		runeAt += runes + newlineRunes
		i = next
		if nl < 0 {
			break
		}
	}
	return out
}

// pythonCuts returns the line indices to cut before, in increasing order.
// The result is empty when there is nothing worth cutting on, and it can
// contain 0 when the first line is itself a top-level statement; that entry
// duplicates the start of the text rather than making a boundary, and
// pythonPieces drops it.
func pythonCuts(lines []pyLine) []int {
	var cuts []int
	for i, l := range lines {
		if !l.topLevel || l.indent != 0 || l.blank || l.comment {
			continue
		}
		// A decorator line is not a statement of its own: it belongs to the
		// definition below it, which absorbs it when the boundary is walked
		// back. Treating it as a cut would strand it in the piece before.
		if isDecorator(l.text) {
			continue
		}
		cuts = append(cuts, i)
	}

	// Walk each boundary that starts a definition back over the lines that
	// belong to it, without crossing the boundary before it. Only decorators
	// and comments are absorbed, and a blank line ends the walk: a def two
	// paragraphs down is a new statement, not a documented one. This is the
	// same rule Go's parser uses for a doc comment, and it is why a comment
	// set off by a blank line stays in the piece before the definition.
	for k := len(cuts) - 1; k >= 0; k-- {
		if !isDefClass(lines[cuts[k]].text) {
			continue
		}
		limit := 0
		if k > 0 {
			limit = cuts[k-1] + 1
		}
		for j := cuts[k] - 1; j >= limit; j-- {
			l := lines[j]
			if l.blank || (!l.comment && !isDecorator(l.text)) {
				break
			}
			cuts[k] = j
		}
	}

	// The walk back only ever moves a boundary earlier, and never past the one
	// before it, so the result is still strictly increasing and needs no
	// deduplication.
	return cuts
}

// pythonLineLead measures leading whitespace and classifies the line.
func pythonLineLead(line string) (indent int, blank, comment bool) {
	i := 0
	for i < len(line) {
		switch line[i] {
		case ' ':
			indent++
		case '\t':
			indent += pythonIndentWidth
		default:
			rest := strings.TrimRight(line[i:], " \t\r")
			if rest == "" {
				return indent, true, false
			}
			return indent, false, rest[0] == '#'
		}
		i++
	}
	return indent, true, false
}

// scanPythonLine carries bracket depth and triple-quote state across one
// line, and reports whether the line continues with a backslash.
func scanPythonLine(line string, depth int, triple string) (int, string, bool) {
	i := 0
	if triple != "" {
		idx := strings.Index(line, triple)
		if idx < 0 {
			return depth, triple, false
		}
		i = idx + len(triple)
		triple = ""
	}

	for i < len(line) {
		switch c := line[i]; c {
		case '#':
			// The rest of the line is a comment, so nothing after it can
			// change the depth or open a string.
			return depth, triple, false
		case '"', '\'':
			quote := string(c)
			if strings.HasPrefix(line[i:], quote+quote+quote) {
				closeAt := strings.Index(line[i+3:], quote+quote+quote)
				if closeAt < 0 {
					return depth, quote + quote + quote, false
				}
				i += 3 + closeAt + 3
				continue
			}
			// A single-quoted string cannot cross a line, so an unterminated
			// one ends at the newline.
			i++
			for i < len(line) {
				if line[i] == '\\' {
					i += 2
					continue
				}
				if line[i] == c {
					i++
					break
				}
				i++
			}
			continue
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case '\\':
			if i == len(line)-1 {
				return depth, triple, true
			}
		}
		i++
	}
	return depth, triple, false
}

func isDefClass(line string) bool {
	t := strings.TrimSpace(line)
	t = strings.TrimPrefix(t, "async ")
	return strings.HasPrefix(t, "def ") || strings.HasPrefix(t, "class ") || t == "def" || t == "class"
}

func isDecorator(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "@")
}
