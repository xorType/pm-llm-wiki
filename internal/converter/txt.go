package converter

import "os"

// fromText passes through .txt and .md files as-is.
func fromText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
