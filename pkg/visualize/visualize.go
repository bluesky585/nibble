// Package visualize renders chunks over their source text as a
// self-contained HTML page, so a split can be eyeballed instead of
// counted.
package visualize

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/bluesky585/nibble/pkg/chunk"
)

// slots is the number of background colors in the page stylesheet.
// Chunk colors cycle through them by index.
const slots = 12

// Segment is a slice of the source. A chunk segment is one chunk; a
// plain segment is text no chunk covers.
type Segment struct {
	Text string
	// Start and End are rune offsets into the source.
	Start int
	End   int
	// Chunk reports whether this segment is a chunk. Index, Slot, and
	// Tokens are set only when it is.
	Chunk   bool
	Index   int
	Slot    int
	Tokens  int
	Context string
	// Overlap is set when the chunk starts before the text already
	// rendered by an earlier chunk. Only the uncovered tail is shown, so
	// the page always reads exactly as the source.
	Overlap bool
	// Mismatch is set when the chunk does not agree with its own
	// offsets: its span is out of range, or the source there is not Text.
	Mismatch bool
}

// Legend is one row describing a chunk.
type Legend struct {
	Index      int
	Slot       int
	Start      int
	End        int
	Tokens     int
	HasContext bool
	Mismatch   bool
	// Covered is set when an earlier chunk already rendered this chunk's
	// whole range, so no text of it reaches the page.
	Covered bool
}

// Doc is one source text and its chunks.
type Doc struct {
	Source   string
	RuneSize int
	Segments []Segment
	Legend   []Legend
}

// NewDoc lays chunks out over original. Segments tile original exactly,
// whether or not the chunks do.
func NewDoc(source, original string, chunks []chunk.Chunk) Doc {
	runes := []rune(original)
	n := len(runes)

	var (
		segs   []Segment
		legend []Legend
	)
	cursor := 0

	for i, ch := range chunks {
		slot := i % slots
		start, end := ch.Start, ch.End
		bad := start < 0 || end > n || start > end
		if !bad && string(runes[start:end]) != ch.Text {
			bad = true
		}

		lg := Legend{Index: i, Slot: slot, Start: ch.Start, End: ch.End, Tokens: ch.TokenCount, HasContext: ch.Context != "", Mismatch: bad}

		// Clamp so a bad offset cannot slice out of range.
		clampStart, clampEnd := start, end
		if clampStart < 0 {
			clampStart = 0
		}
		if clampStart > n {
			clampStart = n
		}
		if clampEnd < 0 {
			clampEnd = 0
		}
		if clampEnd > n {
			clampEnd = n
		}

		if clampStart > cursor {
			segs = append(segs, Segment{Text: string(runes[cursor:clampStart]), Start: cursor, End: clampStart})
			cursor = clampStart
		}

		if clampEnd > cursor {
			segs = append(segs, Segment{
				Text:     string(runes[cursor:clampEnd]),
				Start:    cursor,
				End:      clampEnd,
				Chunk:    true,
				Index:    i,
				Slot:     slot,
				Tokens:   ch.TokenCount,
				Context:  ch.Context,
				Overlap:  clampStart < cursor,
				Mismatch: bad,
			})
			cursor = clampEnd
		} else {
			lg.Covered = true
		}
		legend = append(legend, lg)
	}

	if cursor < n {
		segs = append(segs, Segment{Text: string(runes[cursor:]), Start: cursor, End: n})
	}

	return Doc{Source: source, RuneSize: n, Segments: segs, Legend: legend}
}

// ChunkCount reports how many chunks the document was built from.
func (d Doc) ChunkCount() int { return len(d.Legend) }

type page struct {
	Title string
	Docs  []Doc
}

// HTML renders docs as a complete HTML page.
func HTML(docs []Doc) (string, error) {
	var b bytes.Buffer
	if err := pageTemplate.Execute(&b, page{Title: pageTitle(docs), Docs: docs}); err != nil {
		return "", fmt.Errorf("render html: %w", err)
	}
	return b.String(), nil
}

func pageTitle(docs []Doc) string {
	if len(docs) == 1 && docs[0].Source != "" {
		return docs[0].Source
	}
	if len(docs) == 1 {
		return "chunks"
	}
	return fmt.Sprintf("chunks (%d documents)", len(docs))
}

var pageTemplate = template.Must(template.New("page").Parse(pageHTML))

// pageHTML keeps every {{range}} body on one line inside <pre>: a stray
// newline between segments would render as text in the output.
const pageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
:root {
  color-scheme: light dark;
  --ink: #1a1a1a;
  --dim: #666;
  --line: #d8d8d8;
  --paper: #fff;
}
@media (prefers-color-scheme: dark) {
  :root { --ink: #eaeaea; --dim: #9a9a9a; --line: #3a3a3a; --paper: #16181c; }
}
* { box-sizing: border-box; }
body {
  margin: 0 auto;
  padding: 2rem 1.25rem 4rem;
  max-width: 68rem;
  background: var(--paper);
  color: var(--ink);
  font: 15px/1.55 ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
}
h1 { font-size: 1.35rem; margin: 0 0 .25rem; }
h2 { font-size: 1.05rem; margin: 2rem 0 .5rem; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.meta { color: var(--dim); margin: 0 0 1.5rem; font-size: .9rem; }
.legend { list-style: none; display: flex; flex-wrap: wrap; gap: .35rem .9rem; padding: 0; margin: 0 0 1rem; font-size: .85rem; }
.legend li { display: flex; align-items: center; gap: .4rem; }
.legend code { color: var(--dim); }
.swatch { width: .85rem; height: .85rem; border-radius: .2rem; border: 1px solid var(--line); display: inline-block; }
.flag { font-size: .75rem; text-transform: uppercase; letter-spacing: .04em; padding: 0 .3rem; border-radius: .2rem; }
.flag-warn { background: #ffd9d9; color: #8a1b1b; }
pre.source {
  margin: 0;
  padding: 1rem;
  border: 1px solid var(--line);
  border-radius: .4rem;
  overflow-x: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font: 13px/1.6 ui-monospace, SFMono-Regular, Menlo, monospace;
}
pre.source span { border-radius: .15rem; }
pre.source span:hover { outline: 2px solid #333; }
.gap {
  background: repeating-linear-gradient(45deg, transparent, transparent 6px, #ffd9d9 6px, #ffd9d9 12px);
}
.overlap { border-bottom: 2px solid #c00; }
.mismatch { background: #ffb3b3 !important; text-decoration: underline wavy #c00; }
.c0  { background: #ffe0e0; color: #1a1a1a; }
.c1  { background: #ffe9cc; color: #1a1a1a; }
.c2  { background: #fff7cc; color: #1a1a1a; }
.c3  { background: #e6f7d4; color: #1a1a1a; }
.c4  { background: #d6f5e3; color: #1a1a1a; }
.c5  { background: #d4f0f0; color: #1a1a1a; }
.c6  { background: #d9ecff; color: #1a1a1a; }
.c7  { background: #e2e2fb; color: #1a1a1a; }
.c8  { background: #f3ddfa; color: #1a1a1a; }
.c9  { background: #fbdcec; color: #1a1a1a; }
.c10 { background: #ece2d8; color: #1a1a1a; }
.c11 { background: #e8e8ea; color: #1a1a1a; }
</style>
</head>
<body>
<h1>{{.Title}}</h1>
{{range .Docs}}
<section>
<h2>{{if .Source}}{{.Source}}{{else}}(stdin){{end}}</h2>
<p class="meta">{{.ChunkCount}} chunks &middot; {{.RuneSize}} runes</p>
<ul class="legend">{{range .Legend}}<li><span class="swatch c{{.Slot}}"></span><code>#{{.Index}} [{{.Start}},{{.End}}) &middot; {{.Tokens}} tok</code>{{if .HasContext}} <span class="flag">context</span>{{end}}{{if .Covered}} <span class="flag flag-warn">covered</span>{{end}}{{if .Mismatch}} <span class="flag flag-warn">mismatch</span>{{end}}</li>{{end}}</ul>
<pre class="source">{{range .Segments}}<span class="{{if .Chunk}}c{{.Slot}}{{else}}gap{{end}}{{if .Overlap}} overlap{{end}}{{if .Mismatch}} mismatch{{end}}"{{if .Context}} title="context: {{.Context}}"{{end}}>{{.Text}}</span>{{end}}</pre>
</section>
{{end}}
</body>
</html>
`

// paletteSlots lists the class names the renderer can emit, so a test can
// assert the stylesheet defines every one of them.
func paletteSlots() []string {
	out := make([]string, 0, slots)
	for i := 0; i < slots; i++ {
		out = append(out, fmt.Sprintf("c%d", i))
	}
	return out
}
