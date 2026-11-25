package support

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTMLToText(t *testing.T) {
	cases := map[string]string{
		"plain text":                                       "plain text",
		"<p>Hello&nbsp;there</p><p>Second</p>":             "Hello there\nSecond",
		"Line1<br>Line2<BR/>Line3":                         "Line1\nLine2\nLine3",
		"<ul><li>a</li><li>b</li></ul>":                    "- a\n- b",
		"<style>p{}</style><div>x &amp; y &lt;z&gt;</div>": "x & y <z>",
		"<p>a</p>\n\n\n\n<p>b</p>":                         "a\n\nb",
	}
	for in, want := range cases {
		require.Equal(t, want, htmlToText(in), in)
	}
}
