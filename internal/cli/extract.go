package cli

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bluesky585/nibble/pkg/extract"
)

// lowerExt is the extension of path, lowercased.
func lowerExt(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

// extractFormat names the container format a path's extension pulls
// plain text from. The empty string means the bytes are already the
// text; nothing reads between them and the chunker.
func extractFormat(path string) string {
	switch lowerExt(path) {
	case ".html", ".htm", ".xhtml":
		return "html"
	case ".epub":
		return "epub"
	default:
		return ""
	}
}

// makeJob builds one input job from the named bytes. Tabular and
// container formats are recognized by extension here; every later stage
// of the run sees plain text and chunks, never the container.
func makeJob(path string, b []byte) (inputJob, error) {
	job := inputJob{path: path}
	if sep, tabular := sepForExt(path); tabular {
		job.text = string(b)
		job.raw = true
		job.sep = sep
		return job, nil
	}
	switch format := extractFormat(path); format {
	case "":
		job.text = string(b)
	case "html":
		text, err := extract.HTML(bytes.NewReader(b))
		if err != nil {
			return job, fmt.Errorf("%s: %w", path, err)
		}
		job.text = text
	case "epub":
		text, err := extract.EPUB(bytes.NewReader(b))
		if err != nil {
			return job, fmt.Errorf("%s: %w", path, err)
		}
		job.text = text
	}
	return job, nil
}
