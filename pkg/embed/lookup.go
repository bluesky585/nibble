package embed

import "fmt"

// Lookup returns a named embedder.
//
//	hashing (default): local bag-of-words, no network
//	openai: OpenAI-compatible HTTP, requires OPENAI_API_KEY
func Lookup(name string) (Embedder, error) {
	switch name {
	case "", "hashing":
		return Hashing{}, nil
	case "openai":
		return NewOpenAIFromEnv()
	default:
		return nil, fmt.Errorf("unknown embedder %q", name)
	}
}
