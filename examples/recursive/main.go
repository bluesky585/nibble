// recursive splits text down a rule hierarchy: blank line, then line,
// then sentence, then word. This example shows the cuts the default
// rules make, and that joining the chunks restores the input.
package main

import (
	"fmt"

	"github.com/bluesky585/nibble/pkg/recursive"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func main() {
	c, err := recursive.New(tokenizer.Character{}, 30, nil)
	if err != nil {
		panic(err) // a nil tokenizer or a non-positive size is a build error
	}

	text := "Cats sleep twelve to sixteen hours a day.\n\nDogs sleep ten to fourteen. Birds sing at dawn."
	chunks, err := c.Chunk(text)
	if err != nil {
		panic(err)
	}

	var joined string
	for _, ch := range chunks {
		fmt.Printf("[%d,%d) %2d tokens %q\n", ch.Start, ch.End, ch.TokenCount, ch.Text)
		joined += ch.Text
	}
	fmt.Println("reconstructs:", joined == text)
}
