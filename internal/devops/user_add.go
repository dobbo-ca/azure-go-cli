package devops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// userNewAddCmd wires `az devops user add` (user.py:77 add_user_entitlement).
func userNewAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a user to the organization.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return userRunAdd(context.Background(), cmd)
		},
	}

	cmd.Flags().String("email-id", "", "Email ID of the user to add.")
	cmd.MarkFlagRequired("email-id")
	cmd.Flags().String("license-type", "", userLicenseTypeHelp())
	cmd.MarkFlagRequired("license-type")
	// Tri-state like --detect: unset means "not passed" (command defaults it
	// to true, user.py:85-86), bare --send-email-invite means true, and an
	// explicit value must be "true"/"false" — NoOptDefVal lets the bare form
	// work, matching get_three_state_flag() (arguments.py:118).
	cmd.Flags().String("send-email-invite", "", "Whether to send an email invite to the new user. Default true.")
	cmd.Flags().Lookup("send-email-invite").NoOptDefVal = "true"
	ado.AddOrgFlags(cmd)

	return cmd
}

// userAddResponse is UserEntitlementOperationReference, the bulk-endpoint's
// response envelope (member_entitlement_management/models.py:522-552) — a
// different shape than update's UserEntitlementsPatchResponse: haveResults
// Succeeded/results[] here vs. isSuccess/operationResults[] there.
type userAddResponse struct {
	HaveResultsSucceeded bool `json:"haveResultsSucceeded"`
	Results              []struct {
		Errors []map[string]any `json:"errors"`
		Result map[string]any   `json:"result"`
	} `json:"results"`
}

func userRunAdd(ctx context.Context, cmd *cobra.Command) error {
	emailID, _ := cmd.Flags().GetString("email-id")
	licenseType, _ := cmd.Flags().GetString("license-type")
	normalizedLicenseType, err := userNormalizeLicenseType(licenseType)
	if err != nil {
		return err
	}
	licenseType = normalizedLicenseType

	// user.py:85-87: send_email_invite defaults to True when unset; the wire
	// parameter doNotSendInviteForNewUsers is its inverse.
	sendInvite := true
	if raw, _ := cmd.Flags().GetString("send-email-invite"); raw != "" {
		var err error
		sendInvite, err = userParseTriState("send-email-invite", raw)
		if err != nil {
			return err
		}
	}

	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	// user.py:90-101 _create_patch_operation('add', '', {...}). No email/id
	// resolution here — unlike show/remove/update, add takes a raw email and
	// never calls resolve_identity_as_id, since the user doesn't exist yet.
	body := []map[string]any{
		{
			"op":   "add",
			"path": "",
			"value": map[string]any{
				"accessLevel":         map[string]any{"accountLicenseType": licenseType},
				"extensions":          []any{},
				"projectEntitlements": []any{},
				"user": map[string]any{
					"subjectKind":   "user",
					"principalName": emailID,
				},
			},
		},
	}

	var resp userAddResponse
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Host:       "vsaex",
		Path:       "userentitlements",
		APIVersion: "5.0-preview.2",
		Query:      url.Values{"doNotSendInviteForNewUsers": {strconv.FormatBool(!sendInvite)}},
		Body:       body,
		JSONPatch:  true,
	}, &resp); err != nil {
		return fmt.Errorf("failed to add user: %w", err)
	}

	// user.py:104-105: same per-operation error surfacing as update, but
	// against this envelope's own field names.
	if !resp.HaveResultsSucceeded && len(resp.Results) > 0 && len(resp.Results[0].Errors) > 0 && resp.Results[0].Errors[0] != nil {
		if v, ok := resp.Results[0].Errors[0]["value"]; ok {
			return fmt.Errorf("%v", v)
		}
	}

	var result map[string]any
	if len(resp.Results) > 0 {
		result = resp.Results[0].Result
	}

	return ado.Print(cmd, result, userColumns...)
}
