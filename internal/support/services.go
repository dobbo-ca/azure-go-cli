package support

import (
	"github.com/spf13/cobra"

	"github.com/cdobbyn/azure-go-cli/pkg/output"
)

func newServicesCmd() *cobra.Command {
	list := &cobra.Command{
		Use:   "list",
		Short: "List all Azure services available for support",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, false)
			if err != nil {
				return err
			}
			items, err := c.ListServices(cmd.Context())
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, listOrEmpty(items))
		},
	}
	show := &cobra.Command{
		Use:   "show",
		Short: "Get a specific Azure service for support ticket creation",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, false)
			if err != nil {
				return err
			}
			name, _ := cmd.Flags().GetString("service-name")
			s, err := c.GetService(cmd.Context(), name)
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, s)
		},
	}
	show.Flags().String("service-name", "", "Name of the Azure service")
	_ = show.MarkFlagRequired("service-name")

	pcList := &cobra.Command{
		Use:   "list",
		Short: "List problem classifications for a service",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, false)
			if err != nil {
				return err
			}
			name, _ := cmd.Flags().GetString("service-name")
			items, err := c.ListProblemClassifications(cmd.Context(), name)
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, listOrEmpty(items))
		},
	}
	pcList.Flags().String("service-name", "", "Name of the Azure service")
	_ = pcList.MarkFlagRequired("service-name")

	pcShow := &cobra.Command{
		Use:   "show",
		Short: "Get a problem classification of a service",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, false)
			if err != nil {
				return err
			}
			svc, _ := cmd.Flags().GetString("service-name")
			pc, _ := cmd.Flags().GetString("problem-classification-name")
			p, err := c.GetProblemClassification(cmd.Context(), svc, pc)
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, p)
		},
	}
	pcShow.Flags().String("service-name", "", "Name of the Azure service")
	pcShow.Flags().String("problem-classification-name", "", "Name of the problem classification")
	_ = pcShow.MarkFlagRequired("service-name")
	_ = pcShow.MarkFlagRequired("problem-classification-name")

	return group("services", "Manage support services",
		list, show, group("problem-classifications", "Manage problem classifications of a service", pcList, pcShow))
}
