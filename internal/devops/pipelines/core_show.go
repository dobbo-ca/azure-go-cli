package pipelines

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"github.com/spf13/cobra"
)

func coreNewShowCmd() *cobra.Command {
	var id int
	var name, folderPath string
	var openFlag bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Get the details of a pipeline",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return coreRunShow(context.Background(), cmd, id, name, folderPath, openFlag)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	cmd.Flags().IntVar(&id, "id", 0, "ID of the pipeline.")
	cmd.Flags().StringVar(&name, "name", "", "Name of the pipeline. Ignored if --id is supplied.")
	cmd.Flags().StringVar(&folderPath, "folder-path", "", "Folder path of pipeline. Default is root level folder.")
	cmd.Flags().BoolVar(&openFlag, "open", false, "Open the pipeline summary page in your web browser.")

	return cmd
}

func coreRunShow(ctx context.Context, cmd *cobra.Command, id int, name, folderPath string, openFlag bool) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return coreShow(ctx, cmd, dctx, id, name, folderPath, openFlag)
}

// coreShow does the actual client calls, split out from coreRunShow for
// testability (see coreList's doc comment).
func coreShow(ctx context.Context, cmd *cobra.Command, dctx ado.Context, id int, name, folderPath string, openFlag bool) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	defID := id
	if defID == 0 {
		if name == "" {
			return errors.New("either the --id argument or the --name argument must be supplied for this command")
		}
		defID, err = coreDefinitionIDByName(ctx, client, dctx.Project, name, folderPath, "5.0")
		if err != nil {
			return err
		}
	}

	var def map[string]any
	if err := client.Do(ctx, ado.Request{Scope: dctx.Project, Path: fmt.Sprintf("build/Definitions/%d", defID), APIVersion: "5.0"}, &def); err != nil {
		return fmt.Errorf("failed to get pipeline: %w", err)
	}

	if openFlag {
		coreOpenDefinition(dctx.Org, def)
	}

	return ado.Print(cmd, def, coreDefinitionColumns([]map[string]any{def})...)
}

// coreOpenDefinition ports pipeline.py:196-203 _open_pipeline: build the URL
// by hand from the definition's own project name, not the API response's
// _links (there are none for this shape).
func coreOpenDefinition(org string, def map[string]any) {
	project, _ := def["project"].(map[string]any)
	projectName, _ := project["name"].(string)
	if projectName == "" {
		logger.Warning("failed to open web browser: response did not include a project name")
		return
	}
	u := strings.TrimRight(org, "/") + "/" + url.PathEscape(projectName) + "/_build?definitionId=" + url.PathEscape(coreStr(def["id"]))
	if err := ado.OpenBrowser(u); err != nil {
		logger.Warning("failed to open web browser: %v", err)
	}
}
