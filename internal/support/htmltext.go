package support

import (
	"html"
	"regexp"
	"strings"
)

var (
	reScript   = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle    = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reBR       = regexp.MustCompile(`(?i)<br\s*/?>`)
	reBlockEnd = regexp.MustCompile(`(?i)</(p|div|li|tr|h[1-6])>`)
	reLI       = regexp.MustCompile(`(?i)<li[^>]*>`)
	reTag      = regexp.MustCompile(`<[^>]*>`)
	reNewlines = regexp.MustCompile(`\n{3,}`)
)

// htmlToText flattens an HTML message body to plain text.
func htmlToText(s string) string {
	s = reScript.ReplaceAllString(s, "")
	s = reStyle.ReplaceAllString(s, "")
	s = reBR.ReplaceAllString(s, "\n")
	s = reBlockEnd.ReplaceAllString(s, "\n")
	s = reLI.ReplaceAllString(s, "- ")
	s = reTag.ReplaceAllString(s, "")
	s = strings.ReplaceAll(html.UnescapeString(s), " ", " ")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.TrimSpace(reNewlines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}
