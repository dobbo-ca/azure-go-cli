package devops

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

func newExtensionEnableCmd() *cobra.Command {
	var publisherID, extensionID string

	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable an extension.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtensionSetState(context.Background(), cmd, publisherID, extensionID, false)
		},
	}

	extensionAddIDFlags(cmd, &publisherID, &extensionID)
	ado.AddOrgFlags(cmd)

	return cmd
}

func newExtensionDisableCmd() *cobra.Command {
	var publisherID, extensionID string

	cmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable an extension.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtensionSetState(context.Background(), cmd, publisherID, extensionID, true)
		},
	}

	extensionAddIDFlags(cmd, &publisherID, &extensionID)
	ado.AddOrgFlags(cmd)

	return cmd
}

// runExtensionSetState ports _update_extension_state (extension.py:165-194):
// fetch the current extension, mutate installState.flags client-side by
// adding/removing the "disabled" token, then PATCH the whole object back.
// Not idempotent — enabling an already-enabled extension (or disabling an
// already-disabled one) is a hard error, matching Python exactly.
func runExtensionSetState(ctx context.Context, cmd *cobra.Command, publisherID, extensionID string, disable bool) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := extensionNewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	var extension map[string]any
	if err := client.Do(ctx, ado.Request{
		Host:       "extmgmt",
		Path:       extensionByNamePath(publisherID, extensionID),
		APIVersion: "5.0-preview.1",
	}, &extension); err != nil {
		return fmt.Errorf("failed to get extension: %w", err)
	}

	installState, _ := extension["installState"].(map[string]any)
	if installState == nil {
		installState = map[string]any{}
		extension["installState"] = installState
	}

	updated, err := extensionApplyDisabledFlag(extensionFlagsList(devopsStr(installState["flags"])), disable)
	if err != nil {
		return err
	}
	installState["flags"] = updated

	var result map[string]any
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Host:       "extmgmt",
		Path:       "extensionmanagement/installedextensions",
		APIVersion: "5.0-preview.1",
		Body:       extension,
	}, &result); err != nil {
		return fmt.Errorf("failed to update extension: %w", err)
	}

	return ado.Print(cmd, result, extensionColumns...)
}

// extensionApplyDisabledFlag adds or removes the "disabled" token per
// extension.py:180-192, erroring if the extension is already in the
// requested state. Error text is verbatim Python (CLIError), hence the
// capitalised, unwrapped sentences.
func extensionApplyDisabledFlag(flags []string, disable bool) (string, error) {
	has := false
	for _, f := range flags {
		if f == "disabled" {
			has = true
			break
		}
	}

	if disable {
		if has {
			return "", fmt.Errorf("Extension is already in disabled state")
		}
		flags = append(flags, "disabled")
	} else {
		if !has {
			return "", fmt.Errorf("Extension is already in enabled state")
		}
		out := flags[:0]
		for _, f := range flags {
			if f != "disabled" {
				out = append(out, f)
			}
		}
		flags = out
	}

	return strings.Join(flags, ", "), nil
}
