// semantic starts a new chunk where cosine similarity between
// consecutive sentences drops, so a topic change cuts even without a
// blank line. The hashing embedder is local and keyless; a real model
// embeds the same way through the same interface.
package main

import (
	"fmt"

	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/semantic"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func main() {
	c, err := semantic.New(tokenizer.Word{}, embed.Hashing{}, 64, 0.3)
	if err != nil {
		panic(err)
	}

	text := "The reactor converts water to steam. Steam drives the turbine. " +
		"Quarterly revenue grew across all regions. The board approved the dividend."
	chunks, err := c.Chunk(text)
	if err != nil {
		panic(err)
	}
	for _, ch := range chunks {
		fmt.Printf("%q\n", ch.Text)
	}
}
