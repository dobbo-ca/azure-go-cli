package boards

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

// newQueryCommand is `az boards query`, port of query_work_items
// (azext_devops/dev/boards/work_item.py:241), registered standalone (not
// under `work-item`) at dev/boards/commands.py:54.
func newQueryCommand() *cobra.Command {
	var wiql, id, path string

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query for a list of work items. Only supports flat queries.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return queryRunQuery(context.Background(), cmd, wiql, id, path)
		},
	}

	cmd.Flags().StringVar(&wiql, "wiql", "", "The query in Work Item Query Language format.  Ignored if --id or --path is specified.")
	cmd.Flags().StringVar(&id, "id", "", "The ID of an existing query.  Required unless --path or --wiql are specified.")
	cmd.Flags().StringVar(&path, "path", "", "The path of an existing query.  Ignored if --id is specified.")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func queryRunQuery(ctx context.Context, cmd *cobra.Command, wiql, id, path string) error {
	// work_item.py:251-252: enforced by the CLI wrapper itself, not argparse.
	if wiql == "" && id == "" && path == "" {
		return fmt.Errorf("Either the --wiql, --id, or --path argument must be specified.")
	}

	// work_item.py:253-254: project_required=False -- project is optional
	// unless --path is used without --id (checked below).
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return queryQuery(ctx, cmd, client, dctx, wiql, id, path)
}

// queryFieldRef is WorkItemFieldReference (name, referenceName), as returned
// in WorkItemQueryResult.columns.
type queryFieldRef struct {
	Name          string `json:"name"`
	ReferenceName string `json:"referenceName"`
}

// queryWorkItemRef is WorkItemReference: just the id/url pair a query result
// returns -- no field data yet, hydrated separately (queryHydrate).
type queryWorkItemRef struct {
	ID int `json:"id"`
}

// queryResult is WorkItemQueryResult, trimmed to the fields query_work_items
// actually reads.
type queryResult struct {
	AsOf      string             `json:"asOf"`
	Columns   []queryFieldRef    `json:"columns"`
	WorkItems []queryWorkItemRef `json:"workItems"`
}

// queryQuery does the actual client call sequence, split out from
// queryRunQuery so tests can supply a dctx pointing at an httptest server
// without going through org validation.
func queryQuery(ctx context.Context, cmd *cobra.Command, client *ado.Client, dctx ado.Context, wiql, id, path string) error {
	// work_item.py:257-262: --path is only consulted when --id was not
	// given; resolving it costs one extra GET whose response's id becomes
	// the effective id for the wiql-by-id call below.
	if id == "" && path != "" {
		if dctx.Project == "" {
			return fmt.Errorf("The --project argument must be specified for this query.")
		}
		resolvedID, err := queryResolvePath(ctx, client, dctx.Project, path)
		if err != nil {
			return fmt.Errorf("failed to resolve query path: %w", err)
		}
		id = resolvedID
	}

	result, err := queryRunWiql(ctx, client, id, wiql)
	if err != nil {
		return fmt.Errorf("failed to run query: %w", err)
	}

	// work_item.py:335-336,343: zero work items returns Python None, not an
	// empty list -- match it with a nil result rather than crashing the
	// table transformer on an empty range.
	if len(result.WorkItems) == 0 {
		return ado.Print(cmd, nil)
	}

	workItems, err := queryHydrate(ctx, client, dctx.Org, result)
	if err != nil {
		return fmt.Errorf("failed to get work items: %w", err)
	}

	return ado.Print(cmd, workItems, queryBuildColumns(result.Columns)...)
}

// queryResolvePath is get_query (work_item_tracking_client.py, location_id
// a67d190c-...): GET .../{project}/_apis/wit/queries/{path}. path is a
// saved-query folder path (e.g. "Shared Queries/My Query") -- each "/"
// segment is escaped individually, the separator itself is not.
func queryResolvePath(ctx context.Context, client *ado.Client, project, path string) (string, error) {
	var q struct {
		ID string `json:"id"`
	}
	if err := client.Do(ctx, ado.Request{
		Scope:      project,
		Path:       "wit/queries/" + queryEscapePath(path),
		APIVersion: "5.0",
	}, &q); err != nil {
		return "", err
	}
	return q.ID, nil
}

func queryEscapePath(path string) string {
	segs := strings.Split(path, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

// queryRunWiql runs the query itself: query_by_id when id is set, otherwise
// query_by_wiql with the raw text. Neither call ever passes a team_context
// (query_work_items never constructs one), so both are always org-scoped --
// {project} is never populated in the route even though the wiql-by-id/wiql
// SDK methods accept it in principle. This means --project has no effect on
// which project's work items are queried; it only gates --path resolution
// above.
func queryRunWiql(ctx context.Context, client *ado.Client, id, wiql string) (queryResult, error) {
	var result queryResult
	if id != "" {
		err := client.Do(ctx, ado.Request{
			Path:       "wit/wiql/" + url.PathEscape(id),
			APIVersion: "5.0",
		}, &result)
		return result, err
	}

	err := client.Do(ctx, ado.Request{
		Method:     http.MethodPost,
		Path:       "wit/wiql",
		APIVersion: "5.0",
		Body:       map[string]any{"query": wiql},
	}, &result)
	return result, err
}

// queryMaxWorkItems and queryBatchSize are the hard caps query_work_items
// enforces on hydration (work_item.py:286-287).
const (
	queryMaxWorkItems = 1000
	queryBatchSize    = 200
)

// queryURLBudgetSuffix is the literal placeholder Python subtracts from its
// URL-length budget for the longest possible asOf value (work_item.py:277-279).
const queryURLBudgetSuffix = "/_apis/wit/workItems?ids=&fields=&asOf=2017-11-07T17%3A05%3A34.06699999999999999Z"

// queryHydrate fetches full field data for the ids in result.work_items,
// batched per queryComputeBatches, and re-sorts the hydrated items back into
// the original query order -- get_work_items' batched responses don't
// preserve it (work_item.py:264-334).
func queryHydrate(ctx context.Context, client *ado.Client, org string, result queryResult) ([]map[string]any, error) {
	fields, fieldsLen := queryHydrationFields(result.Columns)

	ids := make([]int, len(result.WorkItems))
	order := make(map[int]int, len(result.WorkItems))
	for i, wi := range result.WorkItems {
		ids[i] = wi.ID
		order[wi.ID] = i
	}

	var items []map[string]any
	for _, batch := range queryComputeBatches(org, fieldsLen, ids) {
		idStrs := make([]string, len(batch))
		for i, id := range batch {
			idStrs[i] = strconv.Itoa(id)
		}
		q := url.Values{}
		q.Set("ids", strings.Join(idStrs, ","))
		q.Set("fields", strings.Join(fields, ","))
		if result.AsOf != "" {
			q.Set("asOf", result.AsOf)
		}

		var page []map[string]any
		if err := client.List(ctx, ado.Request{
			Path:       "wit/workitems",
			APIVersion: "5.0",
			Query:      q,
		}, &page); err != nil {
			return nil, err
		}
		items = append(items, page...)
	}

	sort.SliceStable(items, func(i, j int) bool {
		return order[queryItemID(items[i])] < order[queryItemID(items[j])]
	})
	return items, nil
}

func queryItemID(item map[string]any) int {
	n, _ := item["id"].(float64)
	return int(n)
}

// queryHydrationFields builds the hydration field list from the query
// result's columns, capping accumulation once the URL-encoded field portion
// would exceed 800 chars (work_item.py:280-286) -- the field that tips the
// budget over is still included, only later ones are dropped.
func queryHydrationFields(columns []queryFieldRef) ([]string, int) {
	var fields []string
	length := 0
	for _, c := range columns {
		fields = append(fields, c.ReferenceName)
		if length > 0 {
			length += 3 // %2C delimiter
		}
		length += len(url.QueryEscape(c.ReferenceName))
		if length > 800 {
			logger.Info("Not retrieving all fields due to max url length.")
			break
		}
	}
	return fields, length
}

// queryComputeBatches ports the batch-boundary computation in
// query_work_items (work_item.py:269-296): batches close on a client-side
// URL-length budget or 200 ids, whichever comes first, with a hard cap of
// 1000 work items total (excess ids are silently dropped, log-only).
func queryComputeBatches(org string, fieldsLen int, ids []int) [][]int {
	remaining := 2048 - 100 - len(org) - len(queryURLBudgetSuffix) - fieldsLen

	var batches [][]int
	var current []int
	urlLen := 0
	hydrated := 0
	for _, id := range ids {
		if hydrated >= queryMaxWorkItems {
			logger.Info("Only retrieving the first %d work items.", queryMaxWorkItems)
			break
		}
		if urlLen > 0 {
			urlLen += 3 // %2C delimiter
		}
		urlLen += len(strconv.Itoa(id))
		current = append(current, id)

		if remaining-urlLen <= 0 || len(current) == queryBatchSize {
			batches = append(batches, current)
			hydrated += len(current)
			current = nil
			urlLen = 0
		}
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

// queryTitleTruncateLen is _WORK_ITEM_TITLE_TRUNCATION_LENGTH (_format.py:9).
const queryTitleTruncateLen = 70

// queryBuildColumns builds the (up to 5) dynamic table columns for this
// query's result, named from its own column set -- unlike the other boards
// table transformers, this one has no fixed header list
// (transform_work_item_query_result_row_output, _format.py:93-119).
func queryBuildColumns(columns []queryFieldRef) []ado.Column {
	n := len(columns)
	if n > 5 {
		n = 5
	}
	cols := make([]ado.Column, 0, n)
	for _, c := range columns[:n] {
		cols = append(cols, ado.Column{Header: c.Name, Value: queryCellValue(c)})
	}
	return cols
}

// queryCellValue renders one column's cell for one hydrated work item,
// reproducing _format.py:93-119's special cases: a numeric (or boolean
// False, since Python's `False == 0`) zero renders as the literal string
// "0" to dodge knack's "hide falsy" table quirk; System.Title truncates at
// 70 chars; System.AssignedTo unwraps to its uniqueName; anything else
// renders like a normal table cell.
func queryCellValue(fieldRef queryFieldRef) func(row map[string]any) string {
	return func(row map[string]any) string {
		fields, _ := row["fields"].(map[string]any)
		if fields == nil {
			return " "
		}
		v, ok := fields[fieldRef.ReferenceName]
		if !ok {
			return " "
		}
		if n, isNum := v.(float64); isNum && n == 0 {
			return "0"
		}
		if b, isBool := v.(bool); isBool && !b {
			return "0"
		}

		switch fieldRef.ReferenceName {
		case "System.Title":
			s, _ := v.(string)
			runes := []rune(s)
			if len(runes) > queryTitleTruncateLen {
				s = string(runes[:queryTitleTruncateLen-3]) + "..."
			}
			return s
		case "System.AssignedTo":
			m, _ := v.(map[string]any)
			name, _ := m["uniqueName"].(string)
			return name
		default:
			return ado.TSVScalar(v)
		}
	}
}
