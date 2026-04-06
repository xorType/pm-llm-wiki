package converter

import (
	"fmt"
	"os/exec"
	"strings"
)

// fromPDF extracts text from a PDF using pdftotext (poppler-utils).
//
// Windows install: winget install --id=freedesktop.poppler -e
// Linux/macOS:     apt install poppler-utils  /  brew install poppler
func fromPDF(path string) (string, error) {
	// -layout preserves column/table layout; "-" writes to stdout
	out, err := exec.Command("pdftotext", "-layout", path, "-").Output()
	if err != nil {
		hint := ""
		if _, e := exec.LookPath("pdftotext"); e != nil {
			hint = "\n  → pdftotext not found. Install poppler:\n" +
				"    Windows: winget install --id=freedesktop.poppler -e\n" +
				"    Linux:   sudo apt install poppler-utils\n" +
				"    macOS:   brew install poppler"
		}
		return "", fmt.Errorf("pdftotext failed for %q: %w%s", path, err, hint)
	}
	return strings.TrimSpace(string(out)), nil
}
