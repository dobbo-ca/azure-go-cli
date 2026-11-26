package support

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
)

// pageToken is Python az's --next-token: base64 of {"next_link": <page URL or
// null for the first page>, "offset": <index in that page of the first item not shown>}.
type pageToken struct {
	NextLink string // "" = the first page of the query
	Offset   int
}

// encode writes the token byte for byte as Python's json.dumps + b64encode does.
func (t pageToken) encode() string {
	link := []byte("null")
	if t.NextLink != "" {
		var b strings.Builder
		e := json.NewEncoder(&b)
		e.SetEscapeHTML(false) // Python leaves & < > as is
		_ = e.Encode(t.NextLink)
		link = []byte(strings.TrimSuffix(b.String(), "\n"))
	}
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"next_link": %s, "offset": %d}`, link, t.Offset)))
}

// decodeToken parses a --next-token value with Python's error messages.
func decodeToken(s string) (*pageToken, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, errors.New("Invalid Base64 string.")
	}
	if !utf8.Valid(raw) {
		return nil, errors.New("Error decoding UTF-8.")
	}
	var obj any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, errors.New("Invalid JSON object.")
	}
	m, ok := obj.(map[string]any)
	if !ok {
		return nil, errors.New("Decoded object is not a dictionary.")
	}
	link, ok1 := m["next_link"]
	off, ok2 := m["offset"]
	if !ok1 || !ok2 {
		return nil, errors.New("`next_link` or `offset` doesn't exist.")
	}
	t := &pageToken{}
	switch l := link.(type) {
	case nil:
	case string:
		t.NextLink = l
	default:
		return nil, errors.New("Invalid JSON object.")
	}
	n, ok := off.(float64)
	if !ok || n < 0 || n != float64(int(n)) {
		return nil, errors.New("Invalid JSON object.")
	}
	t.Offset = int(n)
	return t, nil
}

// checkLink rejects a token link that would send the bearer token anywhere but
// the ARM endpoint. Like Python's is_trusted_cloud_endpoint it also rejects
// links that are not in normalized form (control characters, backslashes, empty
// or dot path segments), which other parsers may resolve to another host.
func checkLink(link, endpoint string) error {
	bad := fmt.Errorf("`next_link` '%s' is not a valid endpoint.", link)
	ep, err := url.Parse(endpoint)
	if err != nil {
		return bad
	}
	if strings.ContainsAny(link, "\\ ") || strings.ContainsFunc(link, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return bad
	}
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" || u.Host != strings.ToLower(ep.Host) {
		return bad
	}
	if u.Path != "" {
		for _, seg := range strings.Split(u.Path, "/")[1:] {
			if seg == "" || seg == "." || seg == ".." {
				return bad
			}
		}
	}
	return nil
}

// fetchPage GETs one list page by URL through the ARM pipeline.
func fetchPage[T any](ctx context.Context, a *arm.Client, link string) ([]*T, *string, error) {
	req, err := runtime.NewRequest(ctx, http.MethodGet, link)
	if err != nil {
		return nil, nil, err
	}
	resp, err := a.Pipeline().Do(req)
	if err != nil {
		return nil, nil, err
	}
	if !runtime.HasStatusCode(resp, http.StatusOK) {
		return nil, nil, runtime.NewResponseError(resp)
	}
	var page struct {
		Value    []*T    `json:"value"`
		NextLink *string `json:"nextLink"`
	}
	if err := runtime.UnmarshalAsJSON(resp, &page); err != nil {
		return nil, nil, err
	}
	return page.Value, page.NextLink, nil
}

// list pages through a list operation. The first page of the query comes from
// the SDK pager; every later page (and a token's page) is fetched by link. With
// max > 0 it stops after max items and returns the token for the rest, if any;
// tok is used only when max > 0, as in Python's AAZPaged.
func list[R, T any](ctx context.Context, a *arm.Client, p *runtime.Pager[R], page func(R) ([]*T, *string), max int, tok *pageToken) ([]*T, *pageToken, error) {
	if max <= 0 {
		tok = nil
	}
	link, skip := "", 0
	if tok != nil {
		link, skip = tok.NextLink, tok.Offset
		if link != "" {
			if err := checkLink(link, a.Endpoint()); err != nil {
				return nil, nil, err
			}
		}
	}
	var out []*T
	for first := true; first || link != ""; first = false {
		var items []*T
		var next *string
		var err error
		if link == "" {
			var r R
			if r, err = p.NextPage(ctx); err == nil {
				items, next = page(r)
			}
		} else {
			items, next, err = fetchPage[T](ctx, a, link)
		}
		if err != nil {
			return nil, nil, err
		}
		curr := link
		link = ""
		if next != nil {
			link = *next
		}
		start := min(skip, len(items))
		skip = 0
		if max <= 0 {
			out = append(out, items[start:]...)
			continue
		}
		if want := max - len(out); len(items)-start > want {
			// Cut mid-page: resume from this page at the first item not shown.
			return append(out, items[start:start+want]...), &pageToken{NextLink: curr, Offset: start + want}, nil
		}
		items = items[start:]
		out = append(out, items...)
		if len(out) == max {
			if link == "" {
				return out, nil, nil
			}
			return out, &pageToken{NextLink: link}, nil
		}
	}
	return out, nil, nil
}
