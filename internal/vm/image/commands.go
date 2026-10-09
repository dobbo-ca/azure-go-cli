package image

import (
	"context"

	"github.com/spf13/cobra"
)

func NewImageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Information on available virtual machine images",
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Get the details for a VM image available in the Azure Marketplace",
		RunE: func(cmd *cobra.Command, args []string) error {
			urn, _ := cmd.Flags().GetString("urn")
			location, _ := cmd.Flags().GetString("location")
			publisher, _ := cmd.Flags().GetString("publisher")
			offer, _ := cmd.Flags().GetString("offer")
			sku, _ := cmd.Flags().GetString("sku")
			version, _ := cmd.Flags().GetString("version")
			return Show(context.Background(), cmd, urn, location, publisher, offer, sku, version)
		},
	}
	showCmd.Flags().String("urn", "", "URN in the format publisher:offer:sku:version (version may be 'latest')")
	showCmd.Flags().StringP("location", "l", "", "Location (defaults to the first subscription location)")
	showCmd.Flags().StringP("publisher", "p", "", "Image publisher")
	showCmd.Flags().StringP("offer", "f", "", "Image offer")
	showCmd.Flags().StringP("sku", "s", "", "Image SKU")
	showCmd.Flags().String("version", "", "Image version")

	skusCmd := &cobra.Command{
		Use:   "list-skus",
		Short: "List the VM image SKUs available in the Azure Marketplace",
		RunE: func(cmd *cobra.Command, args []string) error {
			location, _ := cmd.Flags().GetString("location")
			publisher, _ := cmd.Flags().GetString("publisher")
			offer, _ := cmd.Flags().GetString("offer")
			return ListSKUs(context.Background(), cmd, location, publisher, offer)
		},
	}
	skusCmd.Flags().StringP("location", "l", "", "Location")
	skusCmd.Flags().StringP("publisher", "p", "", "Image publisher")
	skusCmd.Flags().StringP("offer", "f", "", "Image offer")
	skusCmd.MarkFlagRequired("location")
	skusCmd.MarkFlagRequired("publisher")
	skusCmd.MarkFlagRequired("offer")

	termsCmd := &cobra.Command{
		Use:   "terms",
		Short: "Manage Marketplace image terms",
	}
	termsCmd.AddCommand(termsLeaf("show", "Get the details of Marketplace image terms", TermsShow))
	termsCmd.AddCommand(termsLeaf("accept", "Accept Marketplace image terms so the image can be used to create VMs", TermsAccept))

	cmd.AddCommand(showCmd, skusCmd, termsCmd)
	return cmd
}

func termsLeaf(use, short string, run func(context.Context, *cobra.Command, string, string, string, string) error) *cobra.Command {
	c := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			urn, _ := cmd.Flags().GetString("urn")
			publisher, _ := cmd.Flags().GetString("publisher")
			offer, _ := cmd.Flags().GetString("offer")
			plan, _ := cmd.Flags().GetString("plan")
			return run(context.Background(), cmd, urn, publisher, offer, plan)
		},
	}
	c.Flags().String("urn", "", "URN in the format publisher:offer:sku:version; other values can be omitted")
	c.Flags().String("publisher", "", "Image publisher")
	c.Flags().String("offer", "", "Image offer")
	c.Flags().String("plan", "", "Image billing plan")
	return c
}
