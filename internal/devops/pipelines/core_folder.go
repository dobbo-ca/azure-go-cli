package pipelines

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func coreNewFolderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "folder",
		Short: "Manage folders for pipelines",
		Long:  "Manage folders for pipelines",
	}

	cmd.AddCommand(coreNewFolderCreateCmd())
	cmd.AddCommand(coreNewFolderDeleteCmd())
	cmd.AddCommand(coreNewFolderListCmd())
	cmd.AddCommand(coreNewFolderUpdateCmd())

	return cmd
}

// coreFolderColumns ports _format.py:392-401 _transform_pipeline_folder_row:
// `Path, Description` (description truncated to 80 chars with "...").
func coreFolderColumns() []ado.Column {
	return []ado.Column{
		{Header: "Path", Value: func(row map[string]any) string {
			p, _ := row["path"].(string)
			return p
		}},
		{Header: "Description", Value: func(row map[string]any) string {
			d, _ := row["description"].(string)
			// Python's row builder assigns table_row['Description']
			// unconditionally (None -> ''), so an empty result must stay a
			// kept blank cell (" "), not "" (which ado.Column treats as
			// omit-the-column) -- otherwise a folder list where every row
			// lacks a description would lose the column entirely.
			t := coreTruncate(d, 80, "...")
			if t == "" {
				return " "
			}
			return t
		}},
	}
}

// coreFolderAPIPath builds the "build/folders[/{path}]" segment. path is a
// route value in the underlying SDK (build_client.py's get_folders/
// create_folder/delete_folder/update_folder all set
// route_values['path'] = path), so it is escaped as a single path segment —
// not split on "/" the way ado.Request.Scope segments are.
func coreFolderAPIPath(path string) string {
	if path == "" {
		return "build/folders"
	}
	return "build/folders/" + url.PathEscape(path)
}

func coreNewFolderCreateCmd() *cobra.Command {
	var path, description string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a folder",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return coreRunFolderCreate(context.Background(), cmd, path, description)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	cmd.Flags().StringVar(&path, "path", "", `Full path of the folder, e.g. \MyFolder\SubFolder.`)
	cmd.Flags().StringVar(&description, "description", "", "Description of the folder.")
	cmd.MarkFlagRequired("path")

	return cmd
}

func coreRunFolderCreate(ctx context.Context, cmd *cobra.Command, path, description string) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return coreFolderCreate(ctx, cmd, dctx, path, description)
}

// coreFolderCreate does the actual client call, split out from
// coreRunFolderCreate for testability (see coreList's doc comment).
func coreFolderCreate(ctx context.Context, cmd *cobra.Command, dctx ado.Context, path, description string) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	// pipeline_folders.py:29-31 leaves folder.description as None when
	// --description is omitted, and msrest drops None attributes from the
	// serialized body — omit the key entirely rather than sending "".
	body := map[string]any{"path": path}
	if description != "" {
		body["description"] = description
	}

	var folder map[string]any
	if err := client.Do(ctx, ado.Request{Method: http.MethodPut, Scope: dctx.Project, Path: coreFolderAPIPath(path), APIVersion: "5.0-preview.2", Body: body}, &folder); err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}

	return ado.Print(cmd, folder, coreFolderColumns()...)
}

func coreNewFolderDeleteCmd() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a folder",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return coreRunFolderDelete(context.Background(), cmd, path)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddYesFlag(cmd)
	cmd.Flags().StringVar(&path, "path", "", `Full path of the folder, e.g. \MyFolder\SubFolder.`)
	cmd.MarkFlagRequired("path")

	return cmd
}

// coreRunFolderDelete ports pipeline_folders.py:36-47. Deletion cascades to
// every pipeline (and build) under the folder, hence the stronger
// confirmation message (commands.py:196-198). delete_folder returns no
// content — Python still registers a table transformer for it, which per the
// surface spec would error rendering a None result; that's a Python-side
// crash this port does not reproduce (Python-bug policy), so nothing is
// printed on success, matching `pipelines delete`.
func coreRunFolderDelete(ctx context.Context, cmd *cobra.Command, path string) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	if err := ado.Confirm(cmd, "This will delete all pipelines in this folder. Are you sure you want to delete this folder?"); err != nil {
		return err
	}

	return coreFolderDelete(ctx, cmd, dctx, path)
}

// coreFolderDelete does the actual client call, split out from
// coreRunFolderDelete for testability (see coreList's doc comment).
func coreFolderDelete(ctx context.Context, cmd *cobra.Command, dctx ado.Context, path string) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	if err := client.Do(ctx, ado.Request{Method: http.MethodDelete, Scope: dctx.Project, Path: coreFolderAPIPath(path), APIVersion: "5.0-preview.2"}, nil); err != nil {
		return fmt.Errorf("failed to delete folder: %w", err)
	}

	return nil
}

func coreNewFolderListCmd() *cobra.Command {
	var path, queryOrder string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all folders",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return coreRunFolderList(context.Background(), cmd, path, queryOrder)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	cmd.Flags().StringVar(&path, "path", "", "Full path of the folder. If omitted, lists from the root.")
	cmd.Flags().StringVar(&queryOrder, "query-order", "", "Order in which folders are returned. Allowed values: Asc, Desc, None.")

	return cmd
}

func coreRunFolderList(ctx context.Context, cmd *cobra.Command, path, queryOrder string) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return coreFolderList(ctx, cmd, dctx, path, queryOrder)
}

// coreFolderList does the actual client call, split out from
// coreRunFolderList for testability (see coreList's doc comment).
func coreFolderList(ctx context.Context, cmd *cobra.Command, dctx ado.Context, path, queryOrder string) error {
	queryOrder, err := coreValidateChoice(queryOrder, "query-order", coreFolderQueryOrderChoices)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	q := url.Values{}
	if order := coreFolderQueryOrder(queryOrder); order != "" {
		q.Set("queryOrder", order)
	}

	var folders []map[string]any
	if err := client.List(ctx, ado.Request{Scope: dctx.Project, Path: coreFolderAPIPath(path), APIVersion: "5.0-preview.2", Query: q}, &folders); err != nil {
		return fmt.Errorf("failed to list folders: %w", err)
	}

	return ado.Print(cmd, folders, coreFolderColumns()...)
}

// coreFolderQueryOrder ports pipeline_folders.py:65-69: only 'asc'/'desc'
// (case-insensitive) are translated; anything else, including the literal
// choice "None", is passed through unchanged.
func coreFolderQueryOrder(queryOrder string) string {
	switch strings.ToLower(queryOrder) {
	case "asc":
		return "folderAscending"
	case "desc":
		return "folderDescending"
	default:
		return queryOrder
	}
}

func coreNewFolderUpdateCmd() *cobra.Command {
	var path, newPath, newDescription string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a folder name or description",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return coreRunFolderUpdate(context.Background(), cmd, path, newPath, newDescription)
		},
	}

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	cmd.Flags().StringVar(&path, "path", "", "Current path of the folder to update.")
	cmd.Flags().StringVar(&newPath, "new-path", "", "New full path of the folder.")
	cmd.Flags().StringVar(&newDescription, "new-description", "", "New description of the folder.")
	cmd.MarkFlagRequired("path")

	return cmd
}

// coreRunFolderUpdate ports pipeline_folders.py:73-101: client-side
// validation before org/project resolution, then a GET-list-and-filter
// (there is no single-item GET-by-exact-path route) followed by a POST to
// the *original* path with the mutated folder body.
func coreRunFolderUpdate(ctx context.Context, cmd *cobra.Command, path, newPath, newDescription string) error {
	if newPath == "" && newDescription == "" {
		return errors.New("either --new-path or --new-description should be specified")
	}

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return coreFolderUpdate(ctx, cmd, dctx, path, newPath, newDescription)
}

// coreFolderUpdate does the actual client calls, split out from
// coreRunFolderUpdate for testability (see coreList's doc comment).
func coreFolderUpdate(ctx context.Context, cmd *cobra.Command, dctx ado.Context, path, newPath, newDescription string) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	q := url.Values{"queryOrder": {"folderAscending"}}
	var folders []map[string]any
	if err := client.List(ctx, ado.Request{Scope: dctx.Project, Path: coreFolderAPIPath(path), APIVersion: "5.0-preview.2", Query: q}, &folders); err != nil {
		return fmt.Errorf("failed to list folders: %w", err)
	}

	var folder map[string]any
	target := strings.Trim(path, `\`)
	for _, f := range folders {
		p, _ := f["path"].(string)
		if strings.Trim(p, `\`) == target {
			folder = f
			break
		}
	}
	if folder == nil {
		return fmt.Errorf("cannot find folder with path %s. Update operation failed", path)
	}

	if newDescription != "" {
		folder["description"] = newDescription
	}
	if newPath != "" {
		folder["path"] = newPath
	}

	var updated map[string]any
	if err := client.Do(ctx, ado.Request{Method: http.MethodPost, Scope: dctx.Project, Path: coreFolderAPIPath(path), APIVersion: "5.0-preview.2", Body: folder}, &updated); err != nil {
		return fmt.Errorf("failed to update folder: %w", err)
	}

	return ado.Print(cmd, updated, coreFolderColumns()...)
}
