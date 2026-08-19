package devops

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

var serviceendpointAuthorizedColumns = []ado.Column{
	{Header: "ID", Field: "id"},
	{Header: "Allow Pipelines", Field: "authorized"},
	{Header: "Name", Field: "name"},
	{Header: "Type", Field: "type"},
	{Header: "Is Ready", Field: "isReady"},
	{Header: "Created By", Field: "createdBy.displayName"},
}

// serviceendpointAuthorizedFields are exactly the 16 attributes
// ServiceEndpointAuthorized.__init__ copies from the endpoint plus
// `authorized` (service_endpoint.py:26-48), so -o json carries that fixed
// subset rather than the raw GET response's full field set.
var serviceendpointAuthorizedFields = []string{
	"administratorsGroup", "authorization", "createdBy", "data", "description",
	"groupScopeId", "id", "isReady", "isShared", "name", "operationStatus",
	"owner", "readersGroup", "type", "url",
}

func serviceendpointNewUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a service endpoint",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serviceendpointRunUpdate(context.Background(), cmd, args)
		},
	}

	cmd.Flags().String("id", "", "ID of the service endpoint.")
	_ = cmd.MarkFlagRequired("id")
	// arguments.py:86-88 registers enable-for-all with get_three_state_flag()
	// (nargs='?'), and service_endpoint.py:210-211 raises
	// CLIError('Atleast one property to be updated must be specified.') only
	// when it's altogether absent — so it stays a tri-state flag, not a
	// required bool (a plain Bool here made "--enable-for-all false"
	// authorize instead of de-authorize, since a bare bool flag also
	// consumes "false" as its NoOptDefVal-driven true value).
	cmd.Flags().String("enable-for-all", "", "Allow all pipelines to access this service endpoint. true or false.")
	cmd.Flags().Lookup("enable-for-all").NoOptDefVal = "true"

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

// serviceendpointRunUpdate reproduces service_endpoint.py:204-225 — this
// command never touches the Service Endpoint API's own PATCH route. It reads
// the endpoint's name via a `show`-equivalent GET, then authorizes/
// unauthorizes it for all pipelines through the Build API's
// authorizedresources resource, then re-reads that same resource to confirm,
// and wraps the result into the synthetic ServiceEndpointAuthorized shape.
func serviceendpointRunUpdate(ctx context.Context, cmd *cobra.Command, args []string) error {
	id, _ := cmd.Flags().GetString("id")

	if !cmd.Flags().Changed("enable-for-all") {
		return errors.New("Atleast one property to be updated must be specified.")
	}
	raw, _ := cmd.Flags().GetString("enable-for-all")
	// NoOptDefVal only fires for the bare "--enable-for-all" form; a
	// space-separated "--enable-for-all false" leaves "false" as a stray
	// positional (pflag has no lookahead for a string flag's value), so pick
	// it up here the same way core_invoke.go's nargs handling does.
	if len(args) > 0 {
		raw = args[0]
	}
	enableForAll, err := extensionParseTriState(raw)
	if err != nil {
		return fmt.Errorf("--enable-for-all: %w", err)
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var endpoint map[string]any
	if err := client.Do(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "serviceendpoint/endpoints/" + url.PathEscape(id),
		APIVersion: "5.0-preview.2",
	}, &endpoint); err != nil {
		return fmt.Errorf("failed to get service endpoint: %w", err)
	}

	authBody := []map[string]any{{
		"authorized": enableForAll,
		"id":         endpoint["id"],
		"name":       endpoint["name"],
		"type":       "endpoint",
	}}
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Scope:      dctx.Project,
		Path:       "build/authorizedresources",
		APIVersion: "5.0-preview.1",
		Body:       authBody,
	}, nil); err != nil {
		return fmt.Errorf("failed to authorize service endpoint: %w", err)
	}

	q := url.Values{}
	q.Set("type", "endpoint")
	q.Set("id", fmt.Sprint(endpoint["id"]))
	var resources []map[string]any
	if err := client.List(ctx, ado.Request{
		Scope:      dctx.Project,
		Path:       "build/authorizedresources",
		APIVersion: "5.0-preview.1",
		Query:      q,
	}, &resources); err != nil {
		return fmt.Errorf("failed to confirm service endpoint authorization: %w", err)
	}

	authorized := false
	if len(resources) > 0 {
		if a, ok := resources[0]["authorized"].(bool); ok {
			authorized = a
		}
	}

	result := map[string]any{"authorized": authorized}
	for _, f := range serviceendpointAuthorizedFields {
		result[f] = endpoint[f]
	}
	return ado.Print(cmd, result, serviceendpointAuthorizedColumns...)
}
