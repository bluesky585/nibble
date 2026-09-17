package recursive

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bluesky585/nibble/pkg/split"
)

// rulesJSON mirrors one Level for JSON input. It is separate from Level
// so the JSON shape can stay small (a string attach name instead of an
// integer constant) and so unknown fields are rejected before they reach
// a Chunker.
type rulesJSON struct {
	Delimiters []string `json:"delimiters"`
	Attach     string   `json:"attach"`
	Token      bool     `json:"token"`
}

// RulesFromJSON parses a rule hierarchy from JSON. Each array element is
// one Level, coarsest first: a level with "token": true hard-splits with
// the tokenizer; a level with "delimiters" splits on those strings.
// "attach" is optional and currently only accepts "prev", the way
// nibble's sentence split always works; "next" and "own" are rejected
// rather than silently ignored.
//
// An empty JSON array means DefaultRules: this format exists to change
// the hierarchy, not to force callers to spell the default out.
func RulesFromJSON(data string) ([]Level, error) {
	dec := json.NewDecoder(strings.NewReader(data))
	dec.DisallowUnknownFields()
	var specs []rulesJSON
	if err := dec.Decode(&specs); err != nil {
		return nil, fmt.Errorf("rules: %w", err)
	}
	if len(specs) == 0 {
		return DefaultRules(), nil
	}

	rules := make([]Level, len(specs))
	for i, spec := range specs {
		if spec.Token && len(spec.Delimiters) > 0 {
			return nil, fmt.Errorf("rules: level %d: a token level cannot set delimiters", i)
		}
		if !spec.Token && len(spec.Delimiters) == 0 {
			return nil, fmt.Errorf("rules: level %d: must set delimiters or \"token\": true", i)
		}
		for _, d := range spec.Delimiters {
			if d == "" {
				return nil, fmt.Errorf("rules: level %d: delimiters must not be empty strings", i)
			}
		}
		l := Level{Delimiters: spec.Delimiters, Token: spec.Token}
		switch spec.Attach {
		case "", "prev":
			l.Attach = split.AttachPrev
		default:
			return nil, fmt.Errorf("rules: level %d: attach must be \"prev\", got %q", i, spec.Attach)
		}
		rules[i] = l
	}
	return rules, nil
}

// RulesFromJSONOr parses rules when data is not empty and returns when
// it is. Callers that hold rules as an optional string — a CLI flag, a
// JSON request field — pass their fallback here instead of branching
// around an empty check themselves.
func RulesFromJSONOr(data string, fallback []Level) ([]Level, error) {
	if data == "" {
		return fallback, nil
	}
	return RulesFromJSON(data)
}
