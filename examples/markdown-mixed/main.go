// markdown routes each region of a document to the chunker that fits
// it: fenced code to code rules, tables to row splitting with the
// header copied into context, everything else to recursive. This
// example prints each chunk's context so the routing is visible.
package main

import (
	"fmt"

	"github.com/bluesky585/nibble/pkg/markdownchunker"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func main() {
	c, err := markdownchunker.New(tokenizer.Character{}, 60)
	if err != nil {
		panic(err)
	}

	text := "# Demo\n\nProse before the table.\n\n| name | color |\n| --- | --- |\n| apple | red |\n| lime | green |\n\n```go\nfunc main() {}\n```\n"
	chunks, err := c.Chunk(text)
	if err != nil {
		panic(err)
	}
	for _, ch := range chunks {
		fmt.Printf("%q context=%q\n", ch.Text, ch.Context)
	}
}
