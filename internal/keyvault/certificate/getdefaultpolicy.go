package certificate

import (
	"encoding/json"

	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// GetDefaultPolicy prints the policy template for az keyvault certificate
// create (custom.py:280).
func GetDefaultPolicy(cmd *cobra.Command, scaffold bool) error {
	body := defaultPolicyJSON
	if scaffold {
		body = scaffoldPolicyJSON
	}
	var policy any
	if err := json.Unmarshal([]byte(body), &policy); err != nil {
		return err
	}
	return output.PrintJSON(cmd, policy)
}
