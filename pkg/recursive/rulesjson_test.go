package recursive

import (
	"strings"
	"testing"

	"github.com/bluesky585/nibble/pkg/split"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func TestRulesFromJSONDefaults(t *testing.T) {
	t.Parallel()

	// An empty array means DefaultRules; the JSON path exists to change
	// the hierarchy, not to force callers to spell out the default.
	got, err := RulesFromJSON("[]")
	if err != nil {
		t.Fatal(err)
	}
	want := DefaultRules()
	if len(got) != len(want) {
		t.Fatalf("levels=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Token != want[i].Token {
			t.Fatalf("level %d token=%v want %v", i, got[i].Token, want[i].Token)
		}
		if strings.Join(got[i].Delimiters, ",") != strings.Join(want[i].Delimiters, ",") {
			t.Fatalf("level %d delims=%v want %v", i, got[i].Delimiters, want[i].Delimiters)
		}
	}
}

func TestRulesFromJSONCustom(t *testing.T) {
	t.Parallel()

	const rules = `[
		{"delimiters": ["\n\n"], "attach": "prev"},
		{"delimiters": ["。", "."], "attach": "prev"},
		{"token": true}
	]`
	got, err := RulesFromJSON(rules)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("levels=%d want 3", len(got))
	}
	if strings.Join(got[0].Delimiters, ",") != "\n\n" {
		t.Fatalf("level0=%v", got[0].Delimiters)
	}
	if got[1].Delimiters[0] != "。" || got[1].Delimiters[1] != "." {
		t.Fatalf("level1=%v", got[1].Delimiters)
	}
	if !got[2].Token {
		t.Fatalf("level2 should be a token level: %+v", got[2])
	}
	if got[0].Attach != split.AttachPrev {
		t.Fatalf("attach=%v want AttachPrev", got[0].Attach)
	}
}

// Attach "next" is rejected: nibble's sentence split attaches the
// delimiter to the previous piece, and a rule that claims otherwise
// would be silently ignored by the packer.
func TestRulesFromJSONRejectsNextAttach(t *testing.T) {
	t.Parallel()

	_, err := RulesFromJSON(`[{"delimiters": ["."], "attach": "next"}]`)
	if err == nil || !strings.Contains(err.Error(), "attach") {
		t.Fatalf("err=%v", err)
	}
}

func TestRulesFromJSONValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		rules string
		want  string
	}{
		{"empty string", ``, "rules: EOF"},
		{"empty object", `[{}]`, `must set delimiters or "token": true`},
		{"empty delimiter", `[{"delimiters": [""]}]`, "must not be empty"},
		{"token with delims", `[{"token": true, "delimiters": ["."]}]`, "cannot set delimiters"},
		{"bad json", `{`, "rules"},
		{"unknown field", `[{"bogus": 1}]`, "unknown field"},
	}
	for _, tc := range cases {
		_, err := RulesFromJSON(tc.rules)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: err=%v want substring %q", tc.name, err, tc.want)
		}
	}
}

// A parsed custom rule set drives a real Chunker.
func TestRulesFromJSONChunks(t *testing.T) {
	t.Parallel()

	// One delimiter level, then tokens. Pieces that fit the budget pack
	// together, so the split only shows where the budget forces a cut.
	rules, err := RulesFromJSON(`[
		{"delimiters": ["|"], "attach": "prev"},
		{"token": true}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(tokenizer.Character{}, 6, rules)
	if err != nil {
		t.Fatal(err)
	}
	original := "alpha|beta|gamma"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	var joined string
	for _, ch := range got {
		joined += ch.Text
	}
	if joined != original {
		t.Fatalf("reconstruct broken: %q", joined)
	}
	// Each piece ends at a "|" (attached to the previous piece) or at the
	// token-budget hard cut; no chunk holds a "|" with text after it.
	for _, ch := range got {
		if i := strings.Index(ch.Text, "|"); i >= 0 && i != len(ch.Text)-1 {
			t.Fatalf("chunk %q holds a | with text after it; the | rule did not drive the cut", ch.Text)
		}
	}
}
