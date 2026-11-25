package support

import (
	"sort"
	"strings"
	"unicode"
)

// rank returns the indices of items that contain at least one word of query,
// best first (most words matched; ties keep input order). An empty query
// returns every index in order.
func rank(query string, items []string) []int {
	seen := map[string]bool{}
	var words []string
	for _, w := range strings.Fields(strings.ToLower(query)) {
		if !seen[w] {
			seen[w] = true
			words = append(words, w)
		}
	}
	idx := make([]int, 0, len(items))
	if len(words) == 0 {
		for i := range items {
			idx = append(idx, i)
		}
		return idx
	}
	scores := make([]int, len(items))
	for i, it := range items {
		lower := strings.ToLower(it)
		for _, w := range words {
			if strings.Contains(lower, w) {
				scores[i]++
			}
		}
		if scores[i] > 0 {
			idx = append(idx, i)
		}
	}
	sort.SliceStable(idx, func(a, b int) bool { return scores[idx[a]] > scores[idx[b]] })
	return idx
}

var titleStopWords = map[string]bool{}

func init() {
	for _, w := range strings.Fields("the and for with not from this that are was have has does when our you your can cannot unable after before into") {
		titleStopWords[w] = true
	}
}

// queryFromTitle turns a ticket title into a search query of its distinctive words.
func queryFromTitle(title string) string {
	words := strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	seen := map[string]bool{}
	var out []string
	for _, w := range words {
		if len([]rune(w)) < 3 || titleStopWords[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return strings.Join(out, " ")
}
