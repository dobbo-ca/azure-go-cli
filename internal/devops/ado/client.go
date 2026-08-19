package ado

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cdobbyn/azure-go-cli/pkg/logger"
)

// Client is an authenticated Azure DevOps REST client for one organization.
type Client struct {
	// Org is the organization base URL with no trailing slash, e.g.
	// "https://dev.azure.com/myorg" or "https://myorg.visualstudio.com".
	// Commands read it directly when hand-building --open web URLs.
	Org  string
	HTTP *http.Client

	primary  string // Authorization header value actually used
	fallback string // PAT header, retried once on 401/203 when primary is AAD

	// lastContinuationToken is set by Do from the most recent response's
	// X-MS-ContinuationToken header, for List to pick up.
	// ponytail: single-threaded assumption; make it a Do return value if a
	// command ever fans out.
	lastContinuationToken string
}

// Request describes one Azure DevOps REST call.
type Request struct {
	// Method defaults to GET when empty.
	Method string

	// Host selects the resource-area subdomain. "" means the organization's own
	// host. Known values: "vssps" (Graph/Identity), "vsrm" (Release Management),
	// "vsaex" (member entitlements / user), "extmgmt" (extension management).
	Host string

	// Scope is the path between the org and "_apis":
	//   ""                  org-scoped     -> {org}/_apis/...
	//   "MyProject"         project-scoped -> {org}/MyProject/_apis/...
	//   "MyProject/MyTeam"  team-scoped    -> {org}/MyProject/MyTeam/_apis/...
	// Segments are path-escaped individually, so "/" is the separator and
	// project names containing spaces are handled. Legacy collection-scoped
	// routes are just a two-segment Scope, e.g. "DefaultCollection/MyProject".
	Scope string

	// Path is everything after "_apis/", already formatted, e.g.
	// "git/repositories/myrepo/pullRequests/42". Segment casing matters and
	// differs per area (git/repositories lowercase, build/Definitions and
	// build/Builds capitalised, wit/workItems and .../pullRequests camelCase) —
	// copy the exact casing from the per-command surface docs, never guess it
	// from a sibling operation.
	Path string

	// APIVersion is required, e.g. "5.0", "5.0-preview.1", "5.1", "7.2-preview.1".
	// Sent as the ?api-version= query parameter.
	APIVersion string

	Query url.Values // optional extra query parameters

	// Body, when non-nil, is JSON-marshalled into the request body.
	Body any

	// JSONPatch sends Content-Type: application/json-patch+json instead of
	// application/json. Required for wit/workItems create+update and for
	// vsaex userentitlements PATCH.
	JSONPatch bool
}

// NewClient builds a client for an already-resolved organization URL. It
// resolves credentials eagerly (see auth.go) so a missing login fails before
// any request is sent. org is NOT validated here — Resolve* already did
// that, and leaving NewClient permissive is what lets tests point it at an
// httptest server.
func NewClient(ctx context.Context, org string) (*Client, error) {
	primary, fallback, err := ResolveAuth(ctx, org)
	if err != nil {
		return nil, err
	}

	return &Client{
		Org:      org,
		HTTP:     &http.Client{Timeout: 100 * time.Second},
		primary:  primary,
		fallback: fallback,
	}, nil
}

// Do sends r and JSON-decodes a 2xx response body into out. out may be nil to
// discard the body. Non-2xx becomes an *APIError (see errors.go).
func (c *Client) Do(ctx context.Context, r Request, out any) error {
	resp, err := c.roundTrip(ctx, r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil && err != io.EOF {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

// List sends r (GET unless overridden), unwraps the {"count":N,"value":[...]}
// envelope, and follows X-MS-ContinuationToken until the server stops returning
// one. out must be a pointer to a slice. If r.Query sets $top/top, List sends
// exactly one request and returns that page unfollowed — $top is a page-size
// cap, not a total, and the server still emits a token (build_client.py:441-446
// never follows it either).
func (c *Client) List(ctx context.Context, r Request, out any) error {
	var all []json.RawMessage
	seen := map[string]bool{}

	// build_client.py:441-446: a $top/top caller wants a page size, not a
	// total, and Python never follows the continuation token — it does one
	// _send and returns that page. Auto-following here would silently
	// override the caller's requested cap.
	capped := r.Query.Get("$top") != "" || r.Query.Get("top") != ""

	for {
		var page struct {
			Count int               `json:"count"`
			Value []json.RawMessage `json:"value"`
		}
		if err := c.Do(ctx, r, &page); err != nil {
			return err
		}
		all = append(all, page.Value...)

		tok := c.lastContinuationToken
		if tok == "" || seen[tok] || capped { // ponytail: repeat-token guard, cheaper than a page cap
			break
		}
		seen[tok] = true

		q := url.Values{}
		for k, v := range r.Query {
			q[k] = v
		}
		q.Set("continuationToken", tok)
		r.Query = q
	}

	b, err := json.Marshal(all)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// roundTrip sends r, retrying once on 401/203 with the fallback credential
// (see auth.go §3.3), and returns the final response. For 2xx the caller
// owns and must close resp.Body. For non-2xx the body is consumed and an
// *APIError is returned instead.
func (c *Client) roundTrip(ctx context.Context, r Request) (*http.Response, error) {
	resp, err := c.doOnce(ctx, r)
	if err != nil {
		return nil, err
	}

	// ponytail: replaces Python's up-front get_projects probe (services.py:86-98)
	// with a one-shot retry, saving a round trip per command. Same observable
	// outcome. 203 is included because Azure DevOps answers unauthenticated
	// requests to some orgs with a 203 + HTML sign-in page rather than a 401.
	if (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNonAuthoritativeInfo) && c.fallback != "" {
		resp.Body.Close()
		c.primary, c.fallback = c.fallback, ""
		resp, err = c.doOnce(ctx, r)
		if err != nil {
			return nil, err
		}
	}

	c.lastContinuationToken = resp.Header.Get("X-MS-ContinuationToken")

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, newAPIError(resp.StatusCode, resp.Header.Get("Content-Type"), body, resp.Request.URL.String())
	}

	return resp, nil
}

func (c *Client) doOnce(ctx context.Context, r Request) (*http.Response, error) {
	u, err := c.url(r)
	if err != nil {
		return nil, err
	}

	method := r.Method
	if method == "" {
		method = http.MethodGet
	}

	var body io.Reader
	if r.Body != nil {
		b, err := json.Marshal(r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("Authorization", c.primary)
	req.Header.Set("Accept", "application/json")
	// client.py:41-42 sets both flags true unconditionally and client.py:98-101
	// sends the headers on every request; without them ADO can answer an
	// expired/federated credential with an HTML sign-in page instead of 401.
	req.Header.Set("X-TFS-FedAuthRedirect", "Suppress")
	req.Header.Set("X-VSS-ForceMsaPassThrough", "true")
	if r.Body != nil {
		if r.JSONPatch {
			req.Header.Set("Content-Type", "application/json-patch+json")
		} else {
			req.Header.Set("Content-Type", "application/json; charset=utf-8")
		}
	}

	logger.Debug("%s %s", method, u) // never log the Authorization header

	return c.HTTP.Do(req)
}

// url builds the request URL for r against c.Org.
func (c *Client) url(r Request) (string, error) {
	if r.APIVersion == "" {
		return "", fmt.Errorf("request to %q: APIVersion is required", r.Path)
	}

	u, err := url.Parse(c.Org)
	if err != nil {
		return "", fmt.Errorf("failed to parse organization URL %q: %w", c.Org, err)
	}

	host := u.Host
	if r.Host != "" {
		host = hostFor(host, r.Host)
	}

	path := strings.TrimSuffix(u.Path, "/")
	for _, seg := range strings.Split(r.Scope, "/") {
		if seg == "" {
			continue
		}
		path += "/" + url.PathEscape(seg)
	}
	// Path is appended verbatim: callers that interpolate user data into it
	// must url.PathEscape those values themselves.
	path += "/_apis/" + r.Path

	q := url.Values{}
	for k, v := range r.Query {
		q[k] = v
	}
	q.Set("api-version", r.APIVersion)

	full := u.Scheme + "://" + host + path
	if rq := q.Encode(); rq != "" {
		full += "?" + rq
	}
	return full, nil
}

// hostFor returns host with the resource-area subdomain sub applied.
//
//	dev.azure.com          + "vssps" -> vssps.dev.azure.com          (org lives in the path)
//	myorg.visualstudio.com + "vssps" -> myorg.vssps.visualstudio.com (org is the first label)
func hostFor(host, sub string) string {
	if sub == "" {
		return host
	}
	if strings.HasSuffix(host, ".visualstudio.com") {
		labels := strings.SplitN(host, ".", 2)
		return labels[0] + "." + sub + "." + labels[1]
	}
	return sub + "." + host
}
