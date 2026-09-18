// Two ways to give a chunk a view across its boundary. -overlap (token
// chunker, recursive.Overlap) repeats neighbor text inside Text, so
// reconstruct stops holding. Context copies keep Text a slice of the
// source and reconstruct intact — that is what this example shows.
package main

import (
	"fmt"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/overlap"
	"github.com/bluesky585/nibble/pkg/recursive"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func main() {
	tok := tokenizer.Character{}
	c, err := recursive.New(tok, 20, nil)
	if err != nil {
		panic(err)
	}

	text := "First paragraph about cats.\n\nSecond paragraph about dogs.\n\nThird about birds."
	chunks, err := c.Chunk(text)
	if err != nil {
		panic(err)
	}

	// The previous chunk's last 6 tokens ride along in Context.
	withPrev, err := overlap.Prefix(chunks, tok, 6)
	if err != nil {
		panic(err)
	}
	for _, ch := range withPrev {
		fmt.Printf("text=%q context=%q\n", ch.Text, ch.Context)
	}
	fmt.Println("reconstruct still holds:", joined(withPrev) == text)
}

func joined(chunks []chunk.Chunk) string {
	var s string
	for _, ch := range chunks {
		s += ch.Text
	}
	return s
}
