package devops

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"unicode/utf16"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// coreHTTPMethods mirrors _HTTP_METHOD_VALUES (arguments.py:20).
var coreHTTPMethods = []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS", "PUT", "HEAD"}

// coreFileEncodings mirrors FILE_ENCODING_TYPES (common/utils.py:11).
var coreFileEncodings = []string{"ascii", "utf-16be", "utf-16le", "utf-8"}

func newCoreInvokeCmd() *cobra.Command {
	var routeParameters, queryParameters []string

	cmd := &cobra.Command{
		Use:   "invoke",
		Short: "Makes a request to a specific area and resource in Azure DevOps to get information",
		Long: "Please use only json output as the response of this command is not fixed. " +
			"Helpful docs - https://docs.microsoft.com/en-us/rest/api/azure/devops/",
		RunE: func(cmd *cobra.Command, args []string) error {
			route, query := coreAppendNargsStar(routeParameters, queryParameters, args)
			return runCoreInvoke(context.Background(), cmd, route, query)
		},
	}

	cmd.Flags().String("area", "", "The area to find the resource.")
	cmd.Flags().String("resource", "", "The name of the resource to operate on.")
	// arguments.py:91-94 registers these with nargs='*': a single
	// "--route-parameters project=X wikiIdentifier=Y" call is meant to
	// capture both tokens. pflag only ever consumes one token per flag
	// occurrence, so "wikiIdentifier=Y" is left as a stray positional —
	// coreAppendNargsStar folds args (cobra's leftover positionals) back in.
	// StringArrayVar (not StringSliceVar) so a single value isn't
	// comma-split — invoke.py's stringToDict takes each token verbatim.
	cmd.Flags().StringArrayVar(&routeParameters, "route-parameters", nil, "Specifies the list of route parameters.")
	cmd.Flags().StringArrayVar(&queryParameters, "query-parameters", nil, "Specifies the list of query parameters.")
	cmd.Flags().String("api-version", "5.0", "The version of the API to target.")
	cmd.Flags().String("http-method", "GET", "Specifies the method used for the request: GET, POST, PATCH, DELETE, OPTIONS, PUT, HEAD.")
	cmd.Flags().String("in-file", "", "Path and file name to the file that contains the contents of the request.")
	cmd.Flags().String("encoding", "utf-8", "Encoding of the input file. Used in conjunction with --in-file: ascii, utf-16be, utf-16le, utf-8.")
	cmd.Flags().String("media-type", "application/json", "Specifies the content type of the request.")
	cmd.Flags().String("accept-media-type", "application/json", "Specifies the content type of the response.")
	cmd.Flags().String("out-file", "", "Path and file name to the file for which this function saves the response body.")

	ado.AddOrgFlags(cmd) // invoke's Python signature has both organization and detect

	return cmd
}

// coreKV is one parsed "key=value" token, in the order it was given on the
// command line — needed because --route-parameters project=X is what
// selects the project scope segment (see coreInvokeRoute).
type coreKV struct{ Key, Value string }

// coreAppendNargsStar folds cobra's leftover positional args — tokens pflag
// couldn't attach to --route-parameters/--query-parameters (see the flags'
// registration comment) — back into whichever flag was actually given,
// approximating argparse's nargs='*' (arguments.py:91-94).
//
// DEVIATION: pflag has no lookahead, so when BOTH flags are given multiple
// values in the same invocation there is no way to tell which leftover
// token belongs to which flag; leftovers go to route-parameters (the
// documented multi-value case, _help.py:161-166) unless only
// query-parameters was given.
func coreAppendNargsStar(route, query, leftover []string) (routeOut, queryOut []string) {
	if len(leftover) == 0 {
		return route, query
	}
	if len(route) == 0 && len(query) > 0 {
		return route, append(append([]string(nil), query...), leftover...)
	}
	return append(append([]string(nil), route...), leftover...), query
}

// coreParseKV ports stringToDict (invoke.py:143-156).
func coreParseKV(items []string) ([]coreKV, error) {
	result := make([]coreKV, 0, len(items))
	for _, item := range items {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("%s is not valid it needs to be in format param=value", item)
		}
		result = append(result, coreKV{parts[0], parts[1]})
	}
	return result, nil
}

func runCoreInvoke(ctx context.Context, cmd *cobra.Command, routeParams, queryParams []string) error {
	area, _ := cmd.Flags().GetString("area")
	resource, _ := cmd.Flags().GetString("resource")

	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	switch {
	case area == "" && resource == "":
		// DEVIATION: discovery mode (both omitted -> dump the org's whole
		// REST surface) needs the location-service/OPTIONS-bootstrap
		// machinery foundation-spec.md deliberately defers. Targeted mode
		// below is what's implemented; see the deviations in the report.
		return errors.New("devops invoke: --area and --resource are required; discovery mode (printing the whole REST surface) is not supported by this port")
	case area == "" || resource == "":
		// Python would crash here (AttributeError: 'NoneType' object has no
		// attribute 'lower', invoke.py:75/88) — that's a bug, not shipped
		// behaviour worth reproducing, so this returns a clean error
		// instead (ground rules: fix crashes, keep quirks).
		return errors.New("--area and --resource must be specified together")
	}

	// arguments.py:95 registers http_method via get_enum_type(), whose
	// CaseInsensitiveList choices match case-insensitively; normalize to
	// the canonical uppercase value before sending it.
	httpMethod, _ := cmd.Flags().GetString("http-method")
	normalizedMethod, ok := coreNormalizeEnum(coreHTTPMethods, httpMethod)
	if !ok {
		return fmt.Errorf("--http-method must be one of: %s", strings.Join(coreHTTPMethods, ", "))
	}
	httpMethod = normalizedMethod

	encoding, _ := cmd.Flags().GetString("encoding")
	if !coreContains(coreFileEncodings, encoding) {
		return fmt.Errorf("--encoding must be one of: %s", strings.Join(coreFileEncodings, ", "))
	}

	apiVersion, _ := cmd.Flags().GetString("api-version")

	var body any
	if inFile, _ := cmd.Flags().GetString("in-file"); inFile != "" {
		content, err := coreReadInFile(inFile, encoding)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(content, &body); err != nil {
			return fmt.Errorf("failed to parse --in-file as JSON: %w", err)
		}
	}

	routePairs, err := coreParseKV(routeParams)
	if err != nil {
		return err
	}
	queryPairs, err := coreParseKV(queryParams)
	if err != nil {
		return err
	}

	scope, path := coreInvokeRoute(area, resource, routePairs)

	q := url.Values{}
	for _, kv := range queryPairs {
		q.Set(kv.Key, kv.Value)
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	result := map[string]any{}
	if err := client.Do(ctx, ado.Request{
		Method:     httpMethod,
		Scope:      scope,
		Path:       path,
		APIVersion: apiVersion,
		Query:      q,
		Body:       body,
	}, &result); err != nil {
		return err
	}

	if outFile, _ := cmd.Flags().GetString("out-file"); outFile != "" {
		return coreWriteOutFile(outFile, result)
	}

	// invoke.py:118 always injects continuation_token, from the response's
	// X-MS-ContinuationToken header. DEVIATION: ado.Client.Do only exposes
	// the decoded JSON body, not response headers, so this can't read the
	// real header value — the key is still always present, matching
	// Python's shape, but its value is always nil rather than the token.
	result["continuation_token"] = nil

	return ado.Print(cmd, result)
}

// coreInvokeRoute builds a best-effort Scope/Path for targeted mode.
//
// DEVIATION: the real REST route for an arbitrary --area/--resource pair is
// only knowable via the location-service discovery foundation-spec.md's
// DEFERRED table explicitly defers (GET .../_apis/resourceAreas plus an
// OPTIONS .../_apis bootstrap per area, resolving a routeTemplate mini
// language). Without it, this reproduces the one pattern that covers every
// worked example in the Python help text (_help.py:150-166): a "project"
// route parameter becomes the project scope segment — exactly how every
// other command in this port addresses a project-scoped route — and any
// other route parameter's value is appended as a trailing path segment, in
// the order given on the command line. This will not resolve every real
// routeTemplate (e.g. one with a route parameter positioned before a
// literal path segment), but there is no way to do better without the
// deferred discovery machinery.
func coreInvokeRoute(area, resource string, routeParams []coreKV) (scope, path string) {
	path = area + "/" + resource
	for _, kv := range routeParams {
		if kv.Key == "project" {
			scope = kv.Value
			continue
		}
		path += "/" + url.PathEscape(kv.Value)
	}
	return scope, path
}

// coreWriteOutFile ports invoke.py:122-134's out-file branch: error if the
// file already exists, otherwise write the response body there and print
// nothing.
//
// DEVIATION: Python streams the server's original response bytes
// (client._client.stream_download). ado.Client.Do only exposes the
// JSON-decoded body, so this re-serialises it instead — byte-identical for
// well-behaved JSON responses, but not a literal passthrough, and it can't
// support a genuinely non-JSON response body at all (Do's JSON decode would
// already have failed before reaching here).
func coreWriteOutFile(path string, result map[string]any) error {
	if _, err := os.Stat(path); err == nil {
		return errors.New("Out file already exists, please give a new name.")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check --out-file: %w", err)
	}

	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode response: %w", err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		return fmt.Errorf("failed to write --out-file: %w", err)
	}
	return nil
}

// coreReadInFile ports read_file_content(file_path, encoding) as used by
// invoke.py:41-42, decoding ascii/utf-8 as raw bytes and utf-16be/utf-16le
// via the stdlib unicode/utf16 decoder (no golang.org/x/text dependency).
func coreReadInFile(path, encoding string) ([]byte, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, errors.New("--in-file does not point to a valid file location")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read --in-file: %w", err)
	}

	switch encoding {
	case "utf-16le":
		return coreDecodeUTF16(raw, false)
	case "utf-16be":
		return coreDecodeUTF16(raw, true)
	default: // ascii, utf-8
		return raw, nil
	}
}

func coreDecodeUTF16(raw []byte, bigEndian bool) ([]byte, error) {
	if len(raw)%2 != 0 {
		return nil, errors.New("invalid utf-16 file content: odd byte length")
	}
	u16 := make([]uint16, len(raw)/2)
	for i := range u16 {
		if bigEndian {
			u16[i] = binary.BigEndian.Uint16(raw[i*2:])
		} else {
			u16[i] = binary.LittleEndian.Uint16(raw[i*2:])
		}
	}
	return []byte(string(utf16.Decode(u16))), nil
}

func coreContains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// coreNormalizeEnum matches v against list case-insensitively and returns
// the canonical-cased entry.
func coreNormalizeEnum(list []string, v string) (string, bool) {
	for _, item := range list {
		if strings.EqualFold(item, v) {
			return item, true
		}
	}
	return "", false
}
