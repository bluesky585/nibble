package recursive

import (
	"fmt"

	"github.com/bluesky585/nibble/pkg/split"
)

// Level is one splitting pass. A token level ignores Delimiters and
// hard-splits with the tokenizer.
type Level struct {
	Delimiters []string
	Attach     split.Attach
	Token      bool
}

func (l Level) validate() error {
	if l.Token && len(l.Delimiters) > 0 {
		return fmt.Errorf("token level cannot set delimiters")
	}
	for _, d := range l.Delimiters {
		if d == "" {
			return fmt.Errorf("delimiters must not be empty strings")
		}
	}
	return nil
}

// DefaultRules split from coarse to fine: paragraphs, sentences,
// clauses, whitespace, then tokens.
func DefaultRules() []Level {
	return []Level{
		{Delimiters: []string{"\n\n", "\r\n", "\n", "\r"}, Attach: split.AttachPrev},
		{Delimiters: []string{"。", "！", "？", ".", "!", "?"}, Attach: split.AttachPrev},
		{Delimiters: []string{"，", "；", ",", ";", ":"}, Attach: split.AttachPrev},
		{Delimiters: []string{" ", "\t"}, Attach: split.AttachPrev},
		{Token: true},
	}
}

// FallbackRules returns the levels below a sentence: clauses, then
// whitespace, then tokens. A chunker that has already split on sentences
// uses them to give one over-budget sentence a second, finer cut — at
// commas when the sentence has any — before landing on token windows.
// Text without clause or whitespace delimiters walks straight to the
// token level, which is exactly the hard split it would have taken
// without this second attempt. The whitespace level covers line breaks,
// which the sentence delimiters do not: a hard-wrapped sentence keeps
// its newlines, and they are finer cut points than any token window.
func FallbackRules() []Level {
	return []Level{
		{Delimiters: []string{"，", "；", ",", ";", ":"}, Attach: split.AttachPrev},
		{Delimiters: []string{" ", "\t", "\n", "\r"}, Attach: split.AttachPrev},
		{Token: true},
	}
}
