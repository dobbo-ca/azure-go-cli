package devops

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func serviceendpointNewGitHubCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a GitHub service endpoint.",
		Long: "For automation, set GitHub PAT token in AZURE_DEVOPS_EXT_GITHUB_PAT " +
			"environment variable. You can learn more about this at https://aka.ms/azure-devops-cli-service-endpoint",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serviceendpointRunGitHubCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("name", "", "Name of service endpoint to create")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().String("github-url", "", "Url for github for creating service endpoint")
	_ = cmd.MarkFlagRequired("github-url")

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func serviceendpointRunGitHubCreate(ctx context.Context, cmd *cobra.Command) error {
	name, _ := cmd.Flags().GetString("name")
	githubURL, _ := cmd.Flags().GetString("github-url")

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	token, err := serviceendpointSecretFromEnvOrPrompt(
		"AZURE_DEVOPS_EXT_GITHUB_PAT",
		"GitHub access token",
		"Please pass GitHub access token in AZURE_DEVOPS_EXT_GITHUB_PAT environment variable in non-interactive mode.",
	)
	if err != nil {
		return err
	}

	body := map[string]any{
		"name": name,
		"type": "github",
		"url":  githubURL,
		"authorization": map[string]any{
			"scheme":     "PersonalAccessToken",
			"parameters": map[string]any{"accessToken": token},
		},
	}

	var endpoint map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPost,
		Scope:      dctx.Project,
		Path:       "serviceendpoint/endpoints",
		APIVersion: "5.0-preview.2",
		Body:       body,
	}, &endpoint); err != nil {
		return fmt.Errorf("failed to create service endpoint: %w", err)
	}

	// No table transformer.
	return ado.Print(cmd, endpoint)
}
