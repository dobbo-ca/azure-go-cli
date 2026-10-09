package sig

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewSigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sig",
		Short: "Manage shared image galleries",
		Long:  "Commands to manage Azure Compute Gallery (shared image gallery) resources",
	}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a shared image gallery",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return Create(context.Background(), cmd)
		},
	}
	f := createCmd.Flags()
	f.StringP("resource-group", "g", "", "Resource group name")
	f.StringP("gallery-name", "r", "", "Gallery name")
	f.StringP("location", "l", "", "Location (default: the resource group's location)")
	f.String("description", "", "Description of the gallery")
	f.StringToString("tags", nil, "Tags: key1=value1,key2=value2")
	f.String("permissions", "", "Sharing permissions (Private, Groups, Community)")
	f.String("publisher-uri", "", "Community gallery publisher URI")
	f.String("publisher-email", "", "Community gallery publisher contact email")
	f.String("eula", "", "Community gallery EULA")
	f.String("public-name-prefix", "", "Community gallery public name prefix")
	f.Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	f.SetNormalizeFunc(func(_ *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "publisher-contact" {
			name = "publisher-email"
		}
		return pflag.NormalizedName(name)
	})
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("gallery-name")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a shared image gallery",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Show(context.Background(), cmd)
		},
	}
	f = showCmd.Flags()
	f.StringP("resource-group", "g", "", "Resource group name")
	f.StringP("gallery-name", "r", "", "Gallery name")
	f.String("select", "", "Select expression (Permissions)")
	f.Bool("sharing-groups", false, "Expand shared gallery groups")
	showCmd.MarkFlagRequired("resource-group")
	showCmd.MarkFlagRequired("gallery-name")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List shared image galleries",
		RunE: func(cmd *cobra.Command, args []string) error {
			return List(context.Background(), cmd)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

	imageDefCmd := &cobra.Command{
		Use:   "image-definition",
		Short: "Manage shared gallery image definitions",
	}

	defCreateCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a gallery image definition",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return CreateImageDefinition(context.Background(), cmd)
		},
	}
	f = defCreateCmd.Flags()
	f.StringP("resource-group", "g", "", "Resource group name")
	f.StringP("gallery-name", "r", "", "Gallery name")
	f.StringP("gallery-image-definition", "i", "", "Gallery image definition name")
	f.StringP("publisher", "p", "", "Image publisher")
	f.StringP("offer", "f", "", "Image offer")
	f.StringP("sku", "s", "", "Image SKU")
	f.String("os-type", "", "OS type (Linux, Windows)")
	f.String("os-state", "Generalized", "OS state (Generalized, Specialized)")
	f.String("hyper-v-generation", "V2", "Hypervisor generation (V1, V2)")
	f.String("architecture", "", "CPU architecture (x64, Arm64)")
	f.String("features", "", `Space-separated Name=Value pairs, e.g. "SecurityType=TrustedLaunch"`)
	f.String("description", "", "Description of the image definition")
	f.String("eula", "", "EULA agreement for the image")
	f.String("privacy-statement-uri", "", "Privacy statement URI")
	f.String("release-note-uri", "", "Release note URI")
	f.String("end-of-life-date", "", "End of life date, e.g. 2030-12-31")
	f.String("plan-name", "", "Purchase plan name")
	f.String("plan-publisher", "", "Purchase plan publisher")
	f.String("plan-product", "", "Purchase plan product")
	f.StringToString("tags", nil, "Tags: key1=value1,key2=value2")
	f.StringP("location", "l", "", "Location (default: the resource group's location)")
	f.Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	for _, n := range []string{"resource-group", "gallery-name", "gallery-image-definition", "publisher", "offer", "sku", "os-type"} {
		defCreateCmd.MarkFlagRequired(n)
	}

	defShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show a gallery image definition",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ShowImageDefinition(context.Background(), cmd)
		},
	}
	f = defShowCmd.Flags()
	f.StringP("resource-group", "g", "", "Resource group name")
	f.StringP("gallery-name", "r", "", "Gallery name")
	f.StringP("gallery-image-definition", "i", "", "Gallery image definition name")
	for _, n := range []string{"resource-group", "gallery-name", "gallery-image-definition"} {
		defShowCmd.MarkFlagRequired(n)
	}

	imageDefCmd.AddCommand(defCreateCmd, defShowCmd)
	cmd.AddCommand(createCmd, showCmd, listCmd, imageDefCmd)
	return cmd
}

// subscription resolves the --subscription override or the default.
func subscription(cmd *cobra.Command) (string, error) {
	s, _ := cmd.Flags().GetString("subscription")
	return config.GetSubscription(s)
}

func newClients(cmd *cobra.Command) (*armcompute.GalleriesClient, *armcompute.GalleryImagesClient, string, error) {
	cred, err := azure.GetCredential()
	if err != nil {
		return nil, nil, "", err
	}
	subID, err := subscription(cmd)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get subscription: %w", err)
	}
	g, err := armcompute.NewGalleriesClient(subID, cred, nil)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to create galleries client: %w", err)
	}
	gi, err := armcompute.NewGalleryImagesClient(subID, cred, nil)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to create gallery images client: %w", err)
	}
	return g, gi, subID, nil
}

// locationOrGroup returns --location, else the resource group's location.
func locationOrGroup(ctx context.Context, cmd *cobra.Command, subID, rg string) (string, error) {
	if loc, _ := cmd.Flags().GetString("location"); loc != "" {
		return loc, nil
	}
	cred, err := azure.GetCredential()
	if err != nil {
		return "", err
	}
	rgc, err := armresources.NewResourceGroupsClient(subID, cred, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create resource groups client: %w", err)
	}
	resp, err := rgc.Get(ctx, rg, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get resource group location: %w", err)
	}
	return *resp.Location, nil
}

// enumValue matches v case-insensitively against allowed, returning the canonical form.
func enumValue(flag, v string, allowed ...string) (string, error) {
	for _, a := range allowed {
		if strings.EqualFold(v, a) {
			return a, nil
		}
	}
	return "", fmt.Errorf("invalid --%s %q (valid values: %s)", flag, v, strings.Join(allowed, ", "))
}
