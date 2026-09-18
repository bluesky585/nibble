// tiktoken counts with a real BPE vocabulary, so size becomes a
// model's token budget rather than a rune budget. The encoding table
// downloads once on first use and caches on disk (TIKTOKEN_CACHE_DIR
// moves it); everything before that is offline.
package main

import (
	"fmt"

	"github.com/bluesky585/nibble/pkg/recursive"
	"github.com/bluesky585/nibble/pkg/tokenizer/tiktoken"
)

func main() {
	tok, err := tiktoken.New("cl100k_base")
	if err != nil {
		panic(err)
	}

	// 20 cl100k_base tokens per chunk — what a model would actually see.
	c, err := recursive.New(tok, 20, nil)
	if err != nil {
		panic(err)
	}

	text := "Chunking for retrieval works best when the budget is the model's own token count, not a character guess."
	chunks, err := c.Chunk(text)
	if err != nil {
		panic(err)
	}
	for _, ch := range chunks {
		fmt.Printf("%2d tokens %q\n", ch.TokenCount, ch.Text)
	}
}
