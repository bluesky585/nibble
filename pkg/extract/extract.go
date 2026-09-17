// Package extract pulls plain text out of container formats — HTML and
// EPUB — so the chunkers can work on documents that are not already
// plain text. It lives in its own package on purpose: the golang.org/x/net
// HTML parser is this library's only second dependency beyond the
// quarantined tiktoken, and a caller that feeds plain text never pays
// for it.
//
// What extraction means here is deliberately plain. Block-level tags
// become paragraph breaks (the \n\n the recursive rules split on),
// inline tags disappear, script and style content is dropped, and
// entities decode to characters. Attributes, classes, and layout are
// gone; retrieval wants the words, not the markup.
package extract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"

	"golang.org/x/net/html"
)

// HTML reads r as HTML and returns its visible text. Block-level
// elements separate paragraphs; consecutive breaks collapse to one.
func HTML(r io.Reader) (string, error) {
	root, err := html.Parse(r)
	if err != nil {
		return "", fmt.Errorf("html: %w", err)
	}
	var b strings.Builder
	walkHTML(root, &b)
	return collapse(b.String()), nil
}

// blockTags are the elements that end a paragraph. Only block starts
// count: the break is written when the element begins, so a paragraph
// boundary lands before the next run of text rather than inside it.
var blockTags = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true,
	"details": true, "dialog": true, "dd": true, "div": true, "dl": true,
	"dt": true, "fieldset": true, "figcaption": true, "figure": true,
	"footer": true, "form": true, "h1": true, "h2": true, "h3": true,
	"h4": true, "h5": true, "h6": true, "header": true, "hgroup": true,
	"hr": true, "li": true, "main": true, "nav": true, "ol": true,
	"p": true, "pre": true, "section": true, "table": true, "ul": true,
}

// voidTags hold no text and never close; descending into them is
// meaningless but harmless, so only skip is needed for their content.
var skipTags = map[string]bool{
	"script": true, "style": true, "head": true, "template": true,
}

// walkHTML appends the text under n to b. Text nodes are written as
// they are (the HTML tokenizer has decoded entities by this point);
// block elements are preceded by a break marker.
func walkHTML(n *html.Node, b *strings.Builder) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		return
	}
	if n.Type != html.ElementNode {
		// Comments and doctype carry no visible text.
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkHTML(c, b)
		}
		return
	}
	if skipTags[n.Data] {
		return
	}
	if blockTags[n.Data] {
		b.WriteString("\x00")
	}
	// br breaks a line without ending a paragraph.
	if n.Data == "br" {
		b.WriteString(" ")
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkHTML(c, b)
	}
}

// collapse turns the walk's break markers into paragraph breaks and
// tidies what the raw walk leaves behind: runs of whitespace inside a
// paragraph become single spaces, each break marker becomes exactly one
// \n\n, and markers with no text between them collapse. The result is
// trimmed: leading and trailing breaks say nothing.
func collapse(s string) string {
	var out strings.Builder
	paras := strings.Split(s, "\x00")
	for _, p := range paras {
		p = strings.Join(strings.Fields(p), " ")
		if p == "" {
			continue
		}
		if out.Len() > 0 {
			out.WriteString("\n\n")
		}
		out.WriteString(p)
	}
	return out.String()
}

// EPUB reads r as an EPUB archive and returns the concatenated text of
// its spine documents, in spine order, separated by paragraph breaks.
// The spine, not the zip order, is the book's own idea of sequence.
func EPUB(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("epub: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("epub: not a readable archive: %w", err)
	}

	files := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		files[f.Name] = f
	}

	container, ok := files["META-INF/container.xml"]
	if !ok {
		return "", fmt.Errorf("epub: no META-INF/container.xml; not an EPUB")
	}
	opfPath, err := rootfile(container)
	if err != nil {
		return "", err
	}
	opf, ok := files[opfPath]
	if !ok {
		return "", fmt.Errorf("epub: container points at %q, which the archive does not hold", opfPath)
	}
	docs, err := spineDocs(opf, path.Dir(opfPath))
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for _, name := range docs {
		f, ok := files[name]
		if !ok {
			return "", fmt.Errorf("epub: spine names %q, which the archive does not hold", name)
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("epub: open %s: %w", name, err)
		}
		text, err := HTML(rc)
		rc.Close()
		if err != nil {
			return "", fmt.Errorf("epub: %s: %w", name, err)
		}
		if b.Len() > 0 && text != "" {
			b.WriteString("\n\n")
		}
		b.WriteString(text)
	}
	return b.String(), nil
}

// rootfile reads container.xml and returns the full-path attribute of
// the first rootfile: the OPF that describes the book.
func rootfile(container *zip.File) (string, error) {
	rc, err := container.Open()
	if err != nil {
		return "", fmt.Errorf("epub: open container: %w", err)
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return "", fmt.Errorf("epub: container.xml names no rootfile")
		}
		if err != nil {
			return "", fmt.Errorf("epub: container: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "rootfile" {
			for _, a := range se.Attr {
				if a.Name.Local == "full-path" && a.Value != "" {
					return a.Value, nil
				}
			}
		}
	}
}

// spineDocs reads the OPF at opfPath within the archive and returns the
// spine's document hrefs, resolved against the OPF's directory.
func spineDocs(opf *zip.File, opfDir string) ([]string, error) {
	rc, err := opf.Open()
	if err != nil {
		return nil, fmt.Errorf("epub: open %s: %w", opf.Name, err)
	}
	defer rc.Close()

	// manifest id -> href, so an idref in the spine resolves to a file.
	hrefs := make(map[string]string)
	var order []string

	dec := xml.NewDecoder(rc)
	inSpine := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("epub: %s: %w", opf.Name, err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "item":
			var id, href string
			for _, a := range se.Attr {
				switch a.Name.Local {
				case "id":
					id = a.Value
				case "href":
					href = a.Value
				}
			}
			if id != "" && href != "" {
				hrefs[id] = href
			}
		case "spine":
			inSpine = true
		case "itemref":
			if !inSpine {
				continue
			}
			for _, a := range se.Attr {
				if a.Name.Local == "idref" {
					if href, ok := hrefs[a.Value]; ok {
						order = append(order, path.Join(opfDir, href))
					}
				}
			}
		case "package":
			// The spine element closes; a second spine would be
			// malformed and is treated as a continuation.
		}
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("epub: %s: the spine names no documents", opf.Name)
	}
	return order, nil
}
