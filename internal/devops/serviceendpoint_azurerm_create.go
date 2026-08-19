package devops

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func serviceendpointNewAzureRMCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an Azure RM type service endpoint.",
		Long: "For automation, set service principal password/secret in " +
			"AZURE_DEVOPS_EXT_AZURE_RM_SERVICE_PRINCIPAL_KEY environment variable. You can learn " +
			"more about this at https://aka.ms/azure-devops-cli-azurerm-service-endpoint",
		RunE: func(cmd *cobra.Command, args []string) error {
			return serviceendpointRunAzureRMCreate(context.Background(), cmd)
		},
	}

	cmd.Flags().String("name", "", "Name of service endpoint to create")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().String("azure-rm-tenant-id", "", "tenant id for creating azure rm service endpoint")
	_ = cmd.MarkFlagRequired("azure-rm-tenant-id")
	cmd.Flags().String("azure-rm-service-principal-id", "", "service principal id for creating azure rm service endpoint")
	_ = cmd.MarkFlagRequired("azure-rm-service-principal-id")
	cmd.Flags().String("azure-rm-subscription-id", "", "subscription id for azure rm service endpoint")
	_ = cmd.MarkFlagRequired("azure-rm-subscription-id")
	cmd.Flags().String("azure-rm-subscription-name", "", "name of azure subscription for azure rm service endpoint")
	_ = cmd.MarkFlagRequired("azure-rm-subscription-name")
	cmd.Flags().String("azure-rm-service-principal-certificate-path", "",
		`Path to (.pem) which is certificate. Create using command `+
			`"openssl pkcs12 -in file.pfx -out file.pem -nodes -password pass:password_here".`)

	ado.AddOrgFlags(cmd)
	ado.AddProjectFlag(cmd)

	return cmd
}

func serviceendpointRunAzureRMCreate(ctx context.Context, cmd *cobra.Command) error {
	name, _ := cmd.Flags().GetString("name")
	tenantID, _ := cmd.Flags().GetString("azure-rm-tenant-id")
	spID, _ := cmd.Flags().GetString("azure-rm-service-principal-id")
	subID, _ := cmd.Flags().GetString("azure-rm-subscription-id")
	subName, _ := cmd.Flags().GetString("azure-rm-subscription-name")
	certPath, _ := cmd.Flags().GetString("azure-rm-service-principal-certificate-path")

	dctx, err := ado.ResolveProject(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	params := map[string]any{
		"tenantid":           tenantID,
		"serviceprincipalid": spID,
	}

	if certPath == "" {
		key, err := serviceendpointSecretFromEnvOrPrompt(
			"AZURE_DEVOPS_EXT_AZURE_RM_SERVICE_PRINCIPAL_KEY",
			"Azure RM service principal key",
			"Please specify azure service principal key in AZURE_DEVOPS_EXT_AZURE_RM_SERVICE_PRINCIPAL_KEY "+
				"environment variable in non-interactive mode or use --azure-rm-service-principal-certificate-path.",
		)
		if err != nil {
			return err
		}
		params["authenticationType"] = "spnKey"
		params["serviceprincipalkey"] = key
	} else {
		pem, err := os.ReadFile(certPath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", certPath, err)
		}
		params["authenticationType"] = "spnCertificate"
		params["servicePrincipalCertificate"] = string(pem)
	}

	body := map[string]any{
		"name": name,
		"type": "azurerm",
		"url":  "https://management.azure.com/",
		"authorization": map[string]any{
			"scheme":     "ServicePrincipal",
			"parameters": params,
		},
		"data": map[string]any{
			"subscriptionId":   subID,
			"subscriptionName": subName,
			"environment":      "AzureCloud",
			"creationMode":     "Manual",
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

	// No table transformer (commands.py:117).
	return ado.Print(cmd, endpoint)
}
