package devops

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cdobbyn/azure-go-cli/internal/devops/ado"
	"github.com/spf13/cobra"
)

// adminBannerUpdateCmd is "devops admin banner update" (banner.py:67-118).
func adminBannerUpdateCmd() *cobra.Command {
	var id, message, bannerType, expiration string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the message, level, or expiration date for a banner.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var messageArg, bannerTypeArg, expirationArg *string
			if cmd.Flags().Changed("message") {
				messageArg = &message
			}
			if cmd.Flags().Changed("type") {
				bannerTypeArg = &bannerType
			}
			if cmd.Flags().Changed("expiration") {
				expirationArg = &expiration
			}
			return runAdminBannerUpdate(context.Background(), cmd, id, messageArg, bannerTypeArg, expirationArg)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "ID of the banner to update.")
	_ = cmd.MarkFlagRequired("id")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Message (string) to show in the banner.")
	cmd.Flags().StringVarP(&bannerType, "type", "t", "", "Type of banner to present: info, warning, or error.")
	cmd.Flags().StringVar(&expiration, "expiration", "", "Date/time when the banner should no longer be presented to users. Supply an empty value to unset the expiration.")
	ado.AddOrgFlags(cmd)

	return cmd
}

func runAdminBannerUpdate(ctx context.Context, cmd *cobra.Command, id string, message, bannerType, expiration *string) error {
	dctx, err := ado.Resolve(cmd)
	if err != nil {
		return err
	}

	client, err := ado.NewClient(ctx, dctx.Org)
	if err != nil {
		return err
	}

	return adminBannerUpdateDo(ctx, cmd, client, id, message, bannerType, expiration)
}

// adminBannerUpdateDo is the 2-step sequence from banner_update
// (banner.py:67-118): fetch-all-and-find-existing, then a 3-way merge per
// field (new value / keep existing / omit) and a single PATCH.
func adminBannerUpdateDo(ctx context.Context, cmd *cobra.Command, client *ado.Client, id string, message, bannerType, expiration *string) error {
	if message == nil && bannerType == nil && expiration == nil {
		return fmt.Errorf("At least one of the following arguments need to be supplied: --message, --type, --expiration.")
	}
	if bannerType != nil {
		if err := adminValidateBannerType(*bannerType); err != nil {
			return err
		}
	}

	// expiration == nil: flag omitted, keep the existing value below.
	// *expiration == "": flag passed empty, clears the expiration.
	// otherwise: converted to ISO 8601 (banner.py:84-87).
	var expirationISO string
	if expiration != nil && *expiration != "" {
		iso, err := adminParseExpiration(*expiration)
		if err != nil {
			return err
		}
		expirationISO = iso
	}

	entries, err := adminFetchBanners(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to update banner: %w", err)
	}

	existing, ok := entries[id]
	if !ok {
		return fmt.Errorf("The following banner was not found: %s", id)
	}

	entry := map[string]any{}

	// banner.py:96-102: unlike level/expirationDate below, "message" always
	// ends up in the outgoing body — even as an explicit null — because the
	// entry dict starts life as {"message": message} before the two
	// conditional branches run. Preserve that quirk rather than "fixing" it
	// to omit the key; it doesn't crash, it's just an odd body.
	if message != nil {
		entry["message"] = *message
	} else if v, ok := existing["message"]; ok {
		entry["message"] = v
	} else {
		entry["message"] = nil
	}

	if bannerType != nil {
		entry["level"] = *bannerType
	} else if v, ok := existing["level"]; ok {
		entry["level"] = v
	}

	if expiration != nil {
		entry["expirationDate"] = expirationISO
	} else if v, ok := existing["expirationDate"]; ok {
		entry["expirationDate"] = v
	}

	key := adminBannerKey + "/" + id
	if err := client.Do(ctx, ado.Request{
		Method:     http.MethodPatch,
		Path:       "Settings/Entries/host",
		APIVersion: adminSettingsAPIVersion,
		Body:       map[string]any{key: entry},
	}, nil); err != nil {
		return fmt.Errorf("failed to update banner: %w", err)
	}

	return adminPrintBanners(cmd, map[string]map[string]any{id: entry})
}
