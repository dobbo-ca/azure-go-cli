package output

import (
	"encoding/json"
	"fmt"

	"github.com/cdobbyn/azure-go-cli/pkg/query"
	"github.com/spf13/cobra"
)

// PrintJSON prints data as JSON, optionally applying a JMESPath query
func PrintJSON(cmd *cobra.Command, data interface{}) error {
	queryStr, _ := cmd.Flags().GetString("query")

	// Marshal to JSON first
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Apply query if specified
	if queryStr != "" {
		jsonData, err = query.ApplyJMESPathToJSON(jsonData, queryStr)
		if err != nil {
			return err
		}
	}

	fmt.Println(string(jsonData))
	return nil
}
