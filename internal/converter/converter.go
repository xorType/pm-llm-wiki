package converter

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ToText converts any supported file at path to plain UTF-8 text / markdown.
// Supported: .txt .md .vtt .pdf .docx .xlsx .csv
func ToText(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md":
		return fromText(path)
	case ".vtt":
		return fromVTT(path)
	case ".pdf":
		return fromPDF(path)
	case ".docx":
		return fromDOCX(path)
	case ".xlsx", ".csv":
		return fromXLSX(path)
	default:
		return "", fmt.Errorf("unsupported file type: %q (add a converter for %s)", ext, filepath.Base(path))
	}
}
