package boards

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

// workitemDateLayouts is a pragmatic subset of what Python's dateutil.parser
// accepts, covering the formats documented on --as-of (work_item.py:207-209):
// a bare date, a date+time, and a date+time with a literal "UTC" suffix.
// ponytail: not a general dateutil-equivalent parser; extend the layout list
// if a real --as-of value fails to parse.
var workitemDateLayouts = []string{
	"2006-01-02 15:04:05 MST",
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

// workitemParseAsOf ports convert_date_string_to_iso8601 (common/arguments.py:15-28):
// local time zone is assumed when the input carries no zone/offset.
func workitemParseAsOf(value string) (string, error) {
	for _, layout := range workitemDateLayouts {
		loc := time.Local
		if layout == "2006-01-02 15:04:05 MST" {
			loc = time.UTC
		}
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			// Python's d.isoformat() always emits a numeric UTC offset
			// (e.g. "+00:00"), never "Z" — "-07:00" (not "Z07:00") forces
			// that in Go's time.Format too.
			return t.Format("2006-01-02T15:04:05.999999999-07:00"), nil
		}
	}
	return "", fmt.Errorf("The --as-of argument must be a valid ISO 8601 string.")
}

// workitemShowCmd is `az boards work-item show`, port of show_work_item
// (work_item.py:206). Note there is no --project flag at all -- the
// function signature has no project param, so ids are looked up
// organization-scoped.
func workitemShowCmd() *cobra.Command {
	var id int
	var asOf, expand, fields string
	var open bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details for a work item.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := workitemValidateExpand(expand); err != nil {
				return err
			}
			return workitemRunShow(context.Background(), cmd, id, asOf, expand, fields, open)
		},
	}

	cmd.Flags().IntVar(&id, "id", 0, "The ID of the work item")
	cmd.MarkFlagRequired("id")
	cmd.Flags().StringVar(&asOf, "as-of", "", "Work item details as of a particular date and time. Provide a date or date time string. Assumes local time zone. Example: '2019-01-20', '2019-01-20 00:20:00'. For UTC, append 'UTC' to the date time string, '2019-01-20 00:20:00 UTC'.")
	cmd.Flags().StringVar(&expand, "expand", "all", "The expand parameters for work item attributes. Possible options are {none, relations, fields, links, all}.")
	cmd.Flags().StringVarP(&fields, "fields", "f", "", "Comma-separated list of requested fields. Example:System.Id,System.AreaPath.")
	cmd.Flags().BoolVar(&open, "open", false, "Open the work item in the default web browser.")

	ado.AddOrgFlags(cmd)

	return cmd
}

func workitemRunShow(ctx context.Context, cmd *cobra.Command, id int, asOf, expand, fields string, open bool) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}
	return workitemShow(ctx, cmd, dctx, id, asOf, expand, fields, open)
}

// workitemShow does the actual client call, split out from workitemRunShow so
// tests can supply a dctx pointing at an httptest server without going
// through org validation.
func workitemShow(ctx context.Context, cmd *cobra.Command, dctx ado.Context, id int, asOf, expand, fields string, open bool) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	q := url.Values{}
	q.Set("$expand", expand)
	if fields != "" {
		q.Set("fields", fields)
	}
	if asOf != "" {
		asOfISO, err := workitemParseAsOf(asOf)
		if err != nil {
			return err
		}
		q.Set("asOf", asOfISO)
	}

	var wi map[string]any
	if err := client.Do(ctx, ado.Request{
		Path:       "wit/workitems/" + url.PathEscape(strconv.Itoa(id)),
		APIVersion: "5.0",
		Query:      q,
	}, &wi); err != nil {
		return fmt.Errorf("failed to get work item: %w", err)
	}

	if open {
		if err := ado.OpenBrowser(workitemOpenBrowserURL(dctx.Org, wi)); err != nil {
			logger.Warning("failed to open web browser: %v", err)
		}
	}

	return ado.Print(cmd, wi, workitemColumns...)
}
