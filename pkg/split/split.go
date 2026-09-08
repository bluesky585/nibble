// Package split cuts text on delimiters without dropping any characters.
package split

import (
	"fmt"
	"sort"
	"unicode/utf8"
)

// Attach controls which piece a matched delimiter belongs to.
type Attach int

const (
	// AttachPrev puts the delimiter at the end of the preceding piece.
	AttachPrev Attach = iota
	// AttachNext puts the delimiter at the start of the following piece.
	AttachNext
	// AttachOwn keeps the delimiter as its own piece.
	AttachOwn
)

// Options configure a delimiter split.
type Options struct {
	Delimiters []string
	Attach     Attach
	// MinRunes merges a piece with the next ones until it has at least
	// this many runes. The last piece may still be shorter. 0 disables merging.
	MinRunes int
}

// Piece is a substring of the original text in rune offsets [Start, End).
type Piece struct {
	Text  string
	Start int
	End   int
}

// Text splits text on Delimiters. Join of piece texts always equals text.
func Text(text string, opt Options) ([]Piece, error) {
	if opt.MinRunes < 0 {
		return nil, fmt.Errorf("MinRunes must be >= 0, got %d", opt.MinRunes)
	}
	switch opt.Attach {
	case AttachPrev, AttachNext, AttachOwn:
	default:
		return nil, fmt.Errorf("unknown Attach %d", opt.Attach)
	}
	for _, d := range opt.Delimiters {
		if d == "" {
			return nil, fmt.Errorf("delimiters must not be empty strings")
		}
	}
	if text == "" {
		return nil, nil
	}
	if len(opt.Delimiters) == 0 {
		n := utf8.RuneCountInString(text)
		return []Piece{{Text: text, Start: 0, End: n}}, nil
	}

	delims := append([]string(nil), opt.Delimiters...)
	sort.SliceStable(delims, func(i, j int) bool {
		return len(delims[i]) > len(delims[j])
	})

	pieces := make([]Piece, 0, 8)
	startB, startR := 0, 0
	b, r := 0, 0

	emit := func(endB, endR int) {
		if endB < startB {
			return
		}
		if endB == startB {
			return
		}
		pieces = append(pieces, Piece{
			Text:  text[startB:endB],
			Start: startR,
			End:   endR,
		})
		startB, startR = endB, endR
	}

	for b < len(text) {
		if d, ok := match(delims, text[b:]); ok {
			endB := b + len(d)
			endR := r + utf8.RuneCountInString(d)
			switch opt.Attach {
			case AttachPrev:
				emit(endB, endR)
			case AttachNext:
				emit(b, r)
				startB, startR = b, r
			case AttachOwn:
				emit(b, r)
				startB, startR = b, r
				emit(endB, endR)
			}
			b, r = endB, endR
			continue
		}
		_, w := utf8.DecodeRuneInString(text[b:])
		b += w
		r++
	}
	if startB < len(text) {
		emit(len(text), r)
	}

	return mergeShort(pieces, opt.MinRunes), nil
}

func match(delims []string, rest string) (string, bool) {
	for _, d := range delims {
		if len(d) <= len(rest) && rest[:len(d)] == d {
			return d, true
		}
	}
	return "", false
}

func mergeShort(pieces []Piece, min int) []Piece {
	if min <= 0 || len(pieces) < 2 {
		return pieces
	}

	out := make([]Piece, 0, len(pieces))
	cur := pieces[0]
	for _, p := range pieces[1:] {
		if utf8.RuneCountInString(cur.Text) < min {
			cur.Text += p.Text
			cur.End = p.End
			continue
		}
		out = append(out, cur)
		cur = p
	}
	out = append(out, cur)
	return out
}
