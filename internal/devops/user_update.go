package devops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// userNewUpdateCmd wires `az devops user update` (user.py:54 update_user_entitlement).
func userNewUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the license for a user.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return userRunUpdate(context.Background(), cmd)
		},
	}

	userAddUserFlag(cmd)
	cmd.Flags().String("license-type", "", userLicenseTypeHelp())
	cmd.MarkFlagRequired("license-type")
	ado.AddOrgFlags(cmd)

	return cmd
}

// userUpdateResponse is UserEntitlementsPatchResponse
// (member_entitlement_management/models.py:955-973).
type userUpdateResponse struct {
	IsSuccess        bool           `json:"isSuccess"`
	UserEntitlement  map[string]any `json:"userEntitlement"`
	OperationResults []struct {
		Errors []map[string]any `json:"errors"`
	} `json:"operationResults"`
}

func userRunUpdate(ctx context.Context, cmd *cobra.Command) error {
	user, _ := cmd.Flags().GetString("user")
	licenseType, _ := cmd.Flags().GetString("license-type")
	normalizedLicenseType, err := userNormalizeLicenseType(licenseType)
	if err != nil {
		return err
	}
	licenseType = normalizedLicenseType

	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	id, err := userResolveID(ctx, client, user)
	if err != nil {
		return err
	}

	// user.py:62-65 _create_patch_operation('replace', '/accessLevel', {...}).
	body := []map[string]any{
		{
			"op":    "replace",
			"path":  "/accessLevel",
			"value": map[string]any{"accountLicenseType": licenseType},
		},
	}

	var resp userUpdateResponse
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Host:       "vsaex",
		Path:       "userentitlements/" + url.PathEscape(id),
		APIVersion: "5.0-preview.2",
		Body:       body,
		JSONPatch:  true,
	}, &resp); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	// user.py:71-73: a per-operation error is surfaced as the error message
	// text, not the raw envelope.
	if !resp.IsSuccess && len(resp.OperationResults) > 0 && len(resp.OperationResults[0].Errors) > 0 && resp.OperationResults[0].Errors[0] != nil {
		if v, ok := resp.OperationResults[0].Errors[0]["value"]; ok {
			return fmt.Errorf("%v", v)
		}
	}

	return ado.Print(cmd, resp.UserEntitlement, userColumns...)
}
