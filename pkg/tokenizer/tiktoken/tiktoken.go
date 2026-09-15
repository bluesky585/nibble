// Package tiktoken counts text with a real BPE vocabulary, so a size limit
// is a model's token budget rather than a rune count.
//
// It lives in its own package on purpose. Every chunker depends on
// pkg/tokenizer for the Tokenizer interface, so putting this there would
// pull a third-party vocabulary into every build of the library. Here only
// the programs that ask for it pay for it.
//
// The vocabulary is not embedded. The first call downloads the encoding
// table once and caches it on disk, so a process that never picks this
// tokenizer never touches the network.
package tiktoken

import (
	"fmt"
	"sync"
	"unicode/utf8"

	tk "github.com/pkoukk/tiktoken-go"

	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// DefaultEncoding is used when New is given an empty name. cl100k_base is
// the encoding of the gpt-4 and gpt-3.5-turbo family.
const DefaultEncoding = "cl100k_base"

// Handles are cached by encoding name. Loading one compiles the tokenizer's
// pattern, which costs a couple hundred milliseconds, and a caller that
// builds a chunker per request would pay that every time. The handle is
// safe to share: encoding and decoding keep no per-call state, only maps
// that are read after loading.
var (
	handleMu sync.Mutex
	handles  = map[string]*tk.Tiktoken{}
)

func loadHandle(encodingName string) (*tk.Tiktoken, error) {
	handleMu.Lock()
	defer handleMu.Unlock()
	if h, ok := handles[encodingName]; ok {
		return h, nil
	}
	h, err := tk.GetEncoding(encodingName)
	if err != nil {
		return nil, err
	}
	handles[encodingName] = h
	return h, nil
}

// Tiktoken counts and splits with a BPE encoding.
//
// Its Split is not the raw BPE segmentation. A BPE token can end in the
// middle of a UTF-8 character, and a Chunk is a rune range, so pieces are
// cut on rune boundaries instead: a token whose bytes end inside a
// character yields an empty piece, and the character lands on the token
// that completes it. Count still equals len(Split) and joining the pieces
// still restores the input. The trade is that a piece may be empty, which
// tokenizer.Tokenizer allows, and a piece can hold more runes than its
// token's bytes suggest, because a character straddling a boundary is
// attributed whole to the piece that completes it.
type Tiktoken struct {
	enc     *tk.Tiktoken
	encName string
}

var _ tokenizer.Tokenizer = Tiktoken{}

// New loads an encoding by name. An empty name uses DefaultEncoding.
// The encoding table is downloaded and cached on first use.
func New(encodingName string) (Tiktoken, error) {
	if encodingName == "" {
		encodingName = DefaultEncoding
	}
	enc, err := loadHandle(encodingName)
	if err != nil {
		return Tiktoken{}, fmt.Errorf("load tiktoken encoding %q: %w", encodingName, err)
	}
	return Tiktoken{enc: enc, encName: encodingName}, nil
}

// Encoding returns the name of the loaded encoding.
func (t Tiktoken) Encoding() string {
	return t.encName
}

// Count returns the number of tokens in text.
func (t Tiktoken) Count(text string) int {
	if text == "" {
		return 0
	}
	return len(t.enc.EncodeOrdinary(text))
}

// Split returns one piece per token, cut on rune boundaries. Empty pieces
// mark tokens whose bytes ended inside a character.
func (t Tiktoken) Split(text string) []string {
	if text == "" {
		return nil
	}
	ids := t.enc.EncodeOrdinary(text)
	if len(ids) == 0 {
		return []string{text}
	}
	// A token decodes to the bytes it holds, and the tokens concatenate to
	// the input bytes, so each token's byte length is the boundary after it.
	sizes := make([]int, len(ids))
	for i, id := range ids {
		sizes[i] = len(t.enc.Decode([]int{id}))
	}
	return splitOnRunes(text, sizes)
}

// splitOnRunes cuts text into len(sizes) pieces at the cumulative byte
// boundaries in sizes, moving each boundary forward to the end of the rune
// it lands inside.
//
// Keeping this separate from the vocabulary lets the rune-boundary rule be
// tested without loading an encoding table.
func splitOnRunes(text string, sizes []int) []string {
	parts := make([]string, 0, len(sizes))
	pieceStart, byteAt, byteEnd := 0, 0, 0
	for _, size := range sizes {
		byteEnd += size
		if byteEnd > len(text) {
			byteEnd = len(text)
		}
		// Consume whole runes up to byteEnd; a rune straddling the boundary
		// stays whole and is taken by the piece that completes it.
		for byteAt < byteEnd {
			_, w := utf8.DecodeRuneInString(text[byteAt:])
			if byteAt+w > byteEnd {
				break
			}
			byteAt += w
		}
		parts = append(parts, text[pieceStart:byteAt])
		pieceStart = byteAt
	}
	return parts
}
