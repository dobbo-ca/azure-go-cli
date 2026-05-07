package resource

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
  "github.com/spf13/cobra"
)

func newMoveCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "move",
    Short: "Move resources to another resource group or subscription",
    RunE:  runMove,
  }
  cmd.Flags().StringSlice("ids", nil, "One or more resource IDs to move (must share a resource group)")
  cmd.Flags().String("destination-group", "", "Target resource group name")
  cmd.Flags().String("destination-subscription-id", "", "Target subscription ID (defaults to current)")
  cmd.MarkFlagRequired("ids")
  cmd.MarkFlagRequired("destination-group")
  return cmd
}

func runMove(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, _ := cmd.Flags().GetStringSlice("ids")
  destGroup, _ := cmd.Flags().GetString("destination-group")
  destSub, _ := cmd.Flags().GetString("destination-subscription-id")

  if len(ids) == 0 {
    return fmt.Errorf("--ids is required")
  }

  // All IDs must share a source subscription and resource group.
  sourceSub, sourceGroup := "", ""
  for i, id := range ids {
    sub, group, _, _, _, err := ParseResourceID(id)
    if err != nil {
      return err
    }
    if i == 0 {
      sourceSub, sourceGroup = sub, group
      continue
    }
    if sub != sourceSub || group != sourceGroup {
      return fmt.Errorf("all --ids must share the same source subscription and resource group")
    }
  }

  client, _, _, err := newGenericClient(cmd)
  if err != nil {
    return err
  }

  // Build target resource group ID.
  targetSub := sourceSub
  if destSub != "" {
    targetSub = destSub
  }
  targetGroupID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", targetSub, destGroup)

  resources := make([]*string, 0, len(ids))
  for _, id := range ids {
    resources = append(resources, to.Ptr(id))
  }

  poller, err := client.BeginMoveResources(ctx, sourceGroup, armresources.MoveInfo{
    Resources:           resources,
    TargetResourceGroup: to.Ptr(targetGroupID),
  }, nil)
  if err != nil {
    return fmt.Errorf("move: %w", err)
  }
  if _, err := poller.PollUntilDone(ctx, nil); err != nil {
    return fmt.Errorf("move: %w", err)
  }
  fmt.Fprintf(cmd.OutOrStdout(), "Moved %d resource(s) to %s\n", len(ids), targetGroupID)
  return nil
}
