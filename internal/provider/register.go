package provider

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/spf13/cobra"
)

func Register(ctx context.Context, cmd *cobra.Command, namespace, mg string, wait, consent bool) error {
	client, err := newClient(cmd)
	if err != nil {
		return err
	}

	if mg != "" {
		if _, err := client.RegisterAtManagementGroupScope(ctx, namespace, mg, nil); err != nil {
			return fmt.Errorf("failed to register provider: %w", err)
		}
		return nil
	}

	var opts *armresources.ProvidersClientRegisterOptions
	if consent {
		opts = &armresources.ProvidersClientRegisterOptions{
			Properties: &armresources.ProviderRegistrationRequest{
				ThirdPartyProviderConsent: &armresources.ProviderConsentDefinition{ConsentToAuthorization: &consent},
			},
		}
	}
	r, err := client.Register(ctx, namespace, opts)
	if err != nil {
		return fmt.Errorf("failed to register provider: %w", err)
	}

	if isRegistered(r.RegistrationState) {
		return nil
	}
	if !wait {
		fmt.Fprintf(os.Stderr, "Registering is still on-going. You can monitor using 'az provider show -n %s'\n", namespace)
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
		p, err := client.Get(ctx, namespace, nil)
		if err != nil {
			return fmt.Errorf("failed to get provider: %w", err)
		}
		if isRegistered(p.RegistrationState) {
			return nil
		}
	}
}

func isRegistered(state *string) bool {
	return state != nil && *state == "Registered"
}
