package embed

import "testing"

func TestLookupOpenAINeedsKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("OPENAI_EMBED_MODEL", "")

	_, err := Lookup("openai")
	if err == nil {
		t.Fatal("expected missing API key")
	}
}
