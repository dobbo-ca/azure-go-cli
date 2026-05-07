package resource

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

func newTagCmd() *cobra.Command {
  cmd := &cobra.Command{
    Use:   "tag",
    Short: "Add or replace tags on a resource",
    RunE:  runTag,
  }
  AddSelectorFlags(cmd)
  cmd.Flags().StringToString("tags", nil, "Tags as key=value pairs (space-separated)")
  cmd.Flags().Bool("is-incremental", false, "Merge tags with existing ones instead of replacing")
  cmd.MarkFlagRequired("tags")
  return cmd
}

func runTag(cmd *cobra.Command, args []string) error {
  ctx := context.Background()
  ids, err := ResolveSelector(cmd)
  if err != nil {
    return err
  }
  tagsClient, err := newTagsClient(cmd)
  if err != nil {
    return err
  }
  raw, _ := cmd.Flags().GetStringToString("tags")
  incremental, _ := cmd.Flags().GetBool("is-incremental")
  op := armresources.TagsPatchOperationReplace
  if incremental {
    op = armresources.TagsPatchOperationMerge
  }

  tags := map[string]*string{}
  for k, v := range raw {
    tags[k] = to.Ptr(v)
  }

  results := make([]interface{}, 0, len(ids))
  for _, id := range ids {
    resp, err := tagsClient.UpdateAtScope(ctx, id, armresources.TagsPatchResource{
      Operation:  &op,
      Properties: &armresources.Tags{Tags: tags},
    }, nil)
    if err != nil {
      return fmt.Errorf("tag %s: %w", id, err)
    }
    results = append(results, resp.TagsResource)
  }
  if len(results) == 1 {
    return output.PrintJSON(cmd, results[0])
  }
  return output.PrintJSON(cmd, results)
}
