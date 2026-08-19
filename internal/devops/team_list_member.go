package devops

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// teamNewListMemberCmd is `az devops team list-member`, port of
// dev/team/team.py:68 get_team_members.
func teamNewListMemberCmd() *cobra.Command {
	var team string
	var top, skip int

	cmd := &cobra.Command{
		Use:   "list-member",
		Short: "List members of a team",
		Long:  "List members of a team",
		RunE: func(cmd *cobra.Command, args []string) error {
			return teamRunListMember(context.Background(), cmd, team, top, skip)
		},
	}

	cmd.Flags().StringVar(&team, "team", "", "The name or id of the team to show members of.")
	cmd.MarkFlagRequired("team")
	cmd.Flags().IntVar(&top, "top", 0, "Maximum number of members to return.")
	cmd.Flags().IntVar(&skip, "skip", 0, "Number of members to skip.")

	ado.AddProjectFlag(cmd)
	ado.AddOrgFlags(cmd)

	return cmd
}

func teamRunListMember(ctx context.Context, cmd *cobra.Command, team string, top, skip int) error {
	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	return teamListMember(ctx, cmd, dctx, team, top, skip)
}

// teamListMember does the actual client call, split out from
// teamRunListMember so tests can supply a dctx pointing at an httptest
// server without going through org validation.
func teamListMember(ctx context.Context, cmd *cobra.Command, dctx ado.Context, team string, top, skip int) error {
	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	req := ado.Request{
		Path:       "projects/" + url.PathEscape(dctx.Project) + "/teams/" + url.PathEscape(team) + "/members",
		APIVersion: "5.0",
	}
	req.Query = teamTopSkipQuery(cmd, top, skip)

	var members []map[string]any
	if err := client.List(ctx, req, &members); err != nil {
		return fmt.Errorf("failed to list team members: %w", err)
	}

	// transform_team_members_table_output sorts by identity.uniqueName.lower()
	// (_get_member_key, dev/team/_format.py:314-318), but commands.py:127
	// wires it only as this command's table_transformer — knack applies that
	// solely for -o table with no --query; JSON/tsv keep the server's order.
	if ado.TableMode(cmd) {
		sort.Slice(members, func(i, j int) bool {
			return strings.ToLower(teamMemberUniqueName(members[i])) < strings.ToLower(teamMemberUniqueName(members[j]))
		})
	}

	return ado.Print(cmd, members, teamMemberColumns...)
}

func teamMemberUniqueName(row map[string]any) string {
	identity, _ := row["identity"].(map[string]any)
	return devopsStr(identity["uniqueName"])
}
