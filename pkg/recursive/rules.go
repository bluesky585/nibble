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
