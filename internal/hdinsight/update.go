package hdinsight

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/hdinsight/armhdinsight"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// parseTags turns "key=value" pairs into an ARM tag map. A pair without '='
// maps to an empty value, matching az CLI behavior.
func parseTags(pairs []string) map[string]*string {
	if len(pairs) == 0 {
		return nil
	}
	tags := map[string]*string{}
	for _, p := range pairs {
		k, v, _ := strings.Cut(p, "=")
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		tags[k] = to.Ptr(strings.TrimSpace(v))
	}
	return tags
}

func Update(ctx context.Context, cmd *cobra.Command, resourceGroup, clusterName string, tagPairs []string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	subscriptionID, err := config.GetDefaultSubscription()
	if err != nil {
		return err
	}

	client, err := armhdinsight.NewClustersClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create clusters client: %w", err)
	}

	resp, err := client.Update(ctx, resourceGroup, clusterName, armhdinsight.ClusterPatchParameters{
		Tags: parseTags(tagPairs),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to update cluster: %w", err)
	}

	return output.PrintJSON(cmd, resp.Cluster)
}
