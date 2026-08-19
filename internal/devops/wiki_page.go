package devops

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"os"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// wikiPageCmd returns the "az devops wiki page" subgroup
// (dev/team/commands.py:180-184).
func wikiPageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "page",
		Short: "Manage wiki pages",
		Long:  "Manage Azure DevOps wiki pages",
	}

	cmd.AddCommand(wikiPageCreateCmd())
	cmd.AddCommand(wikiPageUpdateCmd())
	cmd.AddCommand(wikiPageShowCmd())
	cmd.AddCommand(wikiPageDeleteCmd())

	return cmd
}

// wikiPageColumns is _transform_wiki_page_row (dev/team/_format.py:301-312),
// shared by create/update/show. Page Path is quoted with literal single
// quotes ('{path}'), and the "order" header is lowercase, not "Order" — copy
// both exactly, they're not typos.
var wikiPageColumns = []ado.Column{
	{Header: "ETag", Field: "eTag"},
	{Header: "Page Path", Value: func(row map[string]any) string {
		page, _ := row["page"].(map[string]any)
		return "'" + devopsStr(page["path"]) + "'"
	}},
	{Header: "Is Parent", Field: "page.isParentPage"},
	{Header: "order", Field: "page.order"},
}

// wikiPageTransport wraps an http.RoundTripper to inject the If-Match
// request header and capture the ETag response header for wiki page
// create/update/show/delete. ado.Client.Do only exposes the decoded JSON
// body, never request/response headers or an If-Match hook — but the wiki
// page API returns the page's version exclusively via the ETag response
// header (recording-verified, never present in the JSON body:
// test_wiki_and_page_createListShowDelete.yaml) and accepts the edit
// version exclusively via the If-Match request header
// (wiki_client.py:111-113). ado.Client.HTTP is an exported field, so
// wrapping its Transport here is an additive use of the foundation's public
// surface, not a reimplementation of its auth/URL/error-handling logic —
// see "deviations" in the task report for why this lives here instead of
// inside internal/devops/ado, which this task may not edit.
type wikiPageTransport struct {
	base    http.RoundTripper
	ifMatch string // when non-empty, set as the If-Match request header
	etag    string // captured from the response's ETag header
}

func (t *wikiPageTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.ifMatch != "" {
		req.Header.Set("If-Match", t.ifMatch)
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err == nil && resp != nil {
		t.etag = resp.Header.Get("ETag")
	}
	return resp, err
}

// wikiDoPage sends r via client, optionally setting If-Match (pass "" for
// none) and always capturing the ETag response header. It returns the
// decoded page body and that header value.
func wikiDoPage(ctx context.Context, client *ado.Client, r ado.Request, ifMatch string) (map[string]any, string, error) {
	transport := &wikiPageTransport{base: client.HTTP.Transport, ifMatch: ifMatch}
	client.HTTP.Transport = transport

	var page map[string]any
	err := client.Do(ctx, r, &page)
	return page, transport.etag, err
}

// wikiFileEncodings is FILE_ENCODING_TYPES (dev/common/utils.py:10).
var wikiFileEncodings = []string{"ascii", "utf-16be", "utf-16le", "utf-8"}

// wikiReadFileContent ports read_file_content (dev/common/utils.py:13-29):
// decode filePath's bytes as encoding, one of wikiFileEncodings. golang.org/x/text
// is only an indirect dependency today (go.mod) and pulling it in directly
// would rewrite go.mod, which this task may not touch — utf-16be/utf-16le are
// decoded with stdlib unicode/utf16 instead.
func wikiReadFileContent(filePath, encoding string) (string, error) {
	valid := false
	for _, e := range wikiFileEncodings {
		if e == encoding {
			valid = true
			break
		}
	}
	if !valid {
		return "", fmt.Errorf("File encoding %s is not supported.", encoding)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	switch encoding {
	case "utf-16be", "utf-16le":
		if len(data)%2 != 0 {
			return "", fmt.Errorf("Unable to decode file '%s' with '%s' encoding.", filePath, encoding)
		}
		units := make([]uint16, len(data)/2)
		for i := range units {
			if encoding == "utf-16be" {
				units[i] = binary.BigEndian.Uint16(data[2*i:])
			} else {
				units[i] = binary.LittleEndian.Uint16(data[2*i:])
			}
		}
		return string(utf16.Decode(units)), nil
	case "ascii":
		for _, b := range data {
			if b > 127 {
				return "", fmt.Errorf("Unable to decode file '%s' with '%s' encoding.", filePath, encoding)
			}
		}
		return string(data), nil
	default: // utf-8
		if !utf8.Valid(data) {
			return "", fmt.Errorf("Unable to decode file '%s' with '%s' encoding.", filePath, encoding)
		}
		return string(data), nil
	}
}
