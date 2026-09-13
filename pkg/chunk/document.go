package chunk

// Document binds a source path, the original text, and its chunks.
type Document struct {
	Path    string  `json:"path,omitempty"`
	Content string  `json:"content"`
	Chunks  []Chunk `json:"chunks"`
}

// NewDocument builds a Document. A nil chunk slice becomes empty.
func NewDocument(path, content string, chunks []Chunk) Document {
	if chunks == nil {
		chunks = []Chunk{}
	}
	return Document{Path: path, Content: content, Chunks: chunks}
}
