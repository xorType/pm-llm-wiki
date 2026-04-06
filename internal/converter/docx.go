package converter

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// fromDOCX extracts readable text from a .docx file by parsing word/document.xml.
// Pure Go — no CGo, no external tools required.
func fromDOCX(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("opening docx %q: %w", path, err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("opening word/document.xml in %q: %w", path, err)
		}
		defer rc.Close()
		return docxXMLToText(rc)
	}
	return "", fmt.Errorf("word/document.xml not found inside %q", path)
}

// docxXMLToText walks the document XML and extracts paragraph text.
// It inserts newlines at paragraph (<w:p>) and table-row (<w:tr>) boundaries.
func docxXMLToText(r io.Reader) (string, error) {
	var sb strings.Builder
	decoder := xml.NewDecoder(r)

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Return what we have so far rather than failing entirely.
			return strings.TrimSpace(sb.String()), nil
		}

		switch t := tok.(type) {
		case xml.StartElement:
			// w:tab → literal tab
			if t.Name.Local == "tab" {
				sb.WriteByte('\t')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p", "tr": // paragraph or table-row end → newline
				sb.WriteByte('\n')
			case "tc": // table-cell end → tab separator
				sb.WriteByte('\t')
			}
		case xml.CharData:
			sb.Write(t)
		}
	}
	return strings.TrimSpace(sb.String()), nil
}
