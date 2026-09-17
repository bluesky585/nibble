// Package corpus holds the fixed inputs the benchmarks measure. The
// texts are small on purpose: a benchmark that takes seconds to warm up
// hides regressions behind noise. They cover the shapes the chunkers
// have to handle — English prose, CJK, emoji outside the BMP, a GFM
// table, a fenced code block, Go and Python sources — while staying a
// few kilobytes each.
package corpus

import _ "embed"

//go:embed data/prose.txt
var prose string

//go:embed data/mixed.md
var mixed string

//go:embed data/naps.go
var goSource string

//go:embed data/naps.py
var pythonSource string

// Prose is English sentences with CJK and emoji mixed in, for the
// sentence, token, recursive, and split benchmarks.
func Prose() string { return prose }

// Mixed is a Markdown document holding prose, a table, a fenced Python
// block, and CJK paragraphs, for the markdown benchmark.
func Mixed() string { return mixed }

// GoSource is a Go file, for the code benchmark with Language("go").
func GoSource() string { return goSource }

// PythonSource is a Python file, for the code benchmark with
// Language("python").
func PythonSource() string { return pythonSource }
