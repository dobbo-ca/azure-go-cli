package operation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/cdobbyn/azure-go-cli/pkg/query"
	"github.com/spf13/cobra"
)

const operationAPIVersion = "2025-02-01"

func Show(ctx context.Context, cmd *cobra.Command, clusterName, resourceGroup, operationID string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://management.azure.com/.default"},
	})
	if err != nil {
		return fmt.Errorf("failed to acquire token: %w", err)
	}

	endpoint := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerService/managedClusters/%s/operations/%s?api-version=%s",
		url.PathEscape(subscriptionID),
		url.PathEscape(resourceGroup),
		url.PathEscape(clusterName),
		url.PathEscape(operationID),
		operationAPIVersion,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("operation lookup failed: %s: %s", resp.Status, string(body))
	}

	format, _ := cmd.Flags().GetString("output")
	queryStr, _ := cmd.Flags().GetString("query")

	if f := strings.ToLower(format); f == "" || f == "json" {
		// Verbatim path: preserve the server's key order and numeric
		// literals instead of round-tripping through map[string]interface{},
		// which would sort keys and coerce large integers through float64.
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, body, "", "  "); err != nil {
			fmt.Println(string(body))
			return nil
		}
		out := pretty.Bytes()
		if queryStr != "" {
			var err error
			out, err = query.ApplyJMESPathToJSON(out, queryStr)
			if err != nil {
				return err
			}
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return nil
	}

	var result any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&result); err != nil {
		fmt.Println(string(body))
		return nil
	}

	return output.PrintFormatted(cmd, result, format)
}

func ShowLatest(ctx context.Context, cmd *cobra.Command, clusterName, resourceGroup string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create clusters client: %w", err)
	}

	cluster, err := client.Get(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return fmt.Errorf("failed to get cluster: %w", err)
	}

	result := map[string]any{
		"cluster": clusterName,
	}

	if cluster.Properties != nil {
		if cluster.Properties.ProvisioningState != nil {
			result["provisioningState"] = *cluster.Properties.ProvisioningState
		}
		if cluster.Properties.PowerState != nil && cluster.Properties.PowerState.Code != nil {
			result["powerState"] = *cluster.Properties.PowerState.Code
		}
	}

	return output.PrintJSON(cmd, result)
}
