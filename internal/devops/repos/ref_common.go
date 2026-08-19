package repos

import (
	"context"
	"fmt"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// refZeroObjectID is git's null-SHA sentinel. create sends it as
// oldObjectId, delete sends it as newObjectId, to signal a ref
// creation/deletion rather than a fast-forward update (ref.py:53,86).
const refZeroObjectID = "0000000000000000000000000000000000000000"

// refUpdateEnvelope is the {"value":[...],"count":N} wrapper the git refs
// POST endpoint returns (recording-verified, test_ref_createDeleteFlow.yaml),
// unlike most single-object PATCH/GET responses which are bare.
type refUpdateEnvelope struct {
	Value []map[string]any `json:"value"`
}

// refColumns are the table columns shared by every `repos ref` command
// (ref.py's transform_ref_table_output / transform_refs_table_output both
// delegate to the same _transform_ref_row, _format.py:262-292).
var refColumns = []ado.Column{
	{Header: "Object ID", Value: refObjectIDCell},
	{Header: "Name", Value: refNameCell},
	{Header: "Success", Value: refSuccessCell},
	{Header: "Update Status", Value: refUpdateStatusCell},
}

func refObjectIDCell(row map[string]any) string {
	if v, ok := row["objectId"].(string); ok {
		return v
	}
	oldID, _ := row["oldObjectId"].(string)
	newID, _ := row["newObjectId"].(string)
	switch {
	case oldID == "" && newID == "":
		return ""
	case oldID == refZeroObjectID:
		return newID
	case newID == refZeroObjectID:
		return oldID
	default:
		// _format.py:286-288: a genuine fast-forward update (neither side
		// the zero sentinel) renders as two separate "Old Object ID"/"New
		// Object ID" columns instead. None of the 5 `repos ref` commands
		// produce that shape (create's oldObjectId and delete's
		// newObjectId are always the sentinel), and ado.Print's column
		// list is static per command rather than per row, so this case
		// falls back to showing both ids together instead of splitting
		// the table.
		return oldID + " -> " + newID
	}
}

func refNameCell(row map[string]any) string {
	name, _ := row["name"].(string)
	return strings.TrimPrefix(name, "refs/")
}

func refSuccessCell(row map[string]any) string {
	v, ok := row["success"]
	if !ok || v == nil {
		return ""
	}
	if b, ok := v.(bool); ok {
		if b {
			return "True"
		}
		return "False"
	}
	return fmt.Sprint(v)
}

func refUpdateStatusCell(row map[string]any) string {
	v, ok := row["updateStatus"]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// refWithPrefix prepends "refs/" if not already present
// (common/git.go:122-129 resolve_git_refs — the bare refs/ prefixer used by
// create/delete; distinct from the refs/heads/-specific one used elsewhere
// in the repos surface and NOT applied by lock/unlock, see ref_lock.go).
func refWithPrefix(name string) string {
	if !strings.HasPrefix(name, "refs/") {
		return "refs/" + name
	}
	return name
}

// refAddFlags registers the flags common to every `repos ref` command:
// --organization/--org, --detect, --project/-p, --repository/-r.
func refAddFlags(cmd *cobra.Command) {
	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)
	ado.AddRepoFlag(cmd)
}

// refClient resolves org+project and builds a client. Repository is
// deliberately NOT required here: resolve_instance_project_and_repo is
// called by every ref.py function with repo_required left at its default of
// False (services.py:326-332), so an omitted --repository is sent through
// as an empty repositoryId route segment and rejected server-side rather
// than caught client-side (surface/repos.md's "ref create" notes this
// exact quirk).
func refClient(ctx context.Context, cmd *cobra.Command) (*ado.Client, ado.Context, error) {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return nil, ado.Context{}, err
	}
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return nil, ado.Context{}, err
	}
	return client, dctx, nil
}
