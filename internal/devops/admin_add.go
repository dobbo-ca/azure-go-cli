package devops

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// adminBannerAddCmd is "devops admin banner add" (banner.py:30-64).
func adminBannerAddCmd() *cobra.Command {
	var message, bannerType, id, expiration string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new banner and immediately show it.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var expirationArg *string
			if cmd.Flags().Changed("expiration") {
				expirationArg = &expiration
			}
			return runAdminBannerAdd(context.Background(), cmd, message, bannerType, id, expirationArg)
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Message (string) to show in the banner.")
	_ = cmd.MarkFlagRequired("message")
	cmd.Flags().StringVarP(&bannerType, "type", "t", "", "Type of banner to present: info, warning, or error.")
	cmd.Flags().StringVar(&id, "id", "", "Identifier for the new banner. A unique identifier is automatically created if one is not specified.")
	cmd.Flags().StringVar(&expiration, "expiration", "", "Date/time when the banner should no longer be presented to users.")
	ado.AddOrgFlags(cmd)

	return cmd
}

func runAdminBannerAdd(ctx context.Context, cmd *cobra.Command, message, bannerType, id string, expiration *string) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return adminBannerAddDo(ctx, cmd, client, message, bannerType, id, expiration)
}

func adminBannerAddDo(ctx context.Context, cmd *cobra.Command, client *ado.Client, message, bannerType, id string, expiration *string) error {
	if err := adminValidateBannerType(bannerType); err != nil {
		return err
	}

	var expirationISO string
	if expiration != nil {
		iso, err := adminParseExpiration(*expiration)
		if err != nil {
			return err
		}
		expirationISO = iso
	}

	if id == "" {
		id = adminNewUUID()
	}

	entry := map[string]any{"message": message}
	if bannerType != "" {
		entry["level"] = bannerType
	}
	if expiration != nil {
		entry["expirationDate"] = expirationISO
	}

	key := adminBannerKey + "/" + id
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Path:       "Settings/Entries/host",
		APIVersion: adminSettingsAPIVersion,
		Body:       map[string]any{key: entry},
	}, nil); err != nil {
		return fmt.Errorf("failed to add banner: %w", err)
	}

	// banner_add returns the locally-constructed entry, not a re-fetch
	// (banner.py:64) — reflects exactly what was sent.
	return adminPrintBanners(cmd, map[string]map[string]any{id: entry})
}
