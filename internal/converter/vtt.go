package converter

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

var (
	vttTimestampRe = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}`)
	vttHTMLTagRe   = regexp.MustCompile(`<[^>]+>`)
	vttNoteRe      = regexp.MustCompile(`^NOTE\b`)
)

// fromVTT strips WebVTT markers and returns the spoken transcript as plain text.
// It handles both Zoom and Google Meet VTT formats.
func fromVTT(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var sb strings.Builder
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "WEBVTT", line == "":
			continue
		case vttTimestampRe.MatchString(line): // e.g. 00:01:23.456 --> 00:01:25.789
			continue
		case vttNoteRe.MatchString(line): // NOTE blocks
			continue
		default:
			// Strip any inline HTML tags (e.g. <v Speaker Name>)
			clean := vttHTMLTagRe.ReplaceAllString(line, "")
			clean = strings.TrimSpace(clean)
			if clean != "" {
				sb.WriteString(clean)
				sb.WriteByte('\n')
			}
		}
	}
	return sb.String(), scanner.Err()
}
