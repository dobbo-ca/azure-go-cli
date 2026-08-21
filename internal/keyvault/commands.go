package keyvault

import (
	"context"

	"github.com/cdobbyn/azure-go-cli/internal/keyvault/certificate"
	"github.com/cdobbyn/azure-go-cli/internal/keyvault/key"
	"github.com/cdobbyn/azure-go-cli/internal/keyvault/secret"
	"github.com/spf13/cobra"
)

func NewKeyVaultCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keyvault",
		Short: "Manage Azure Key Vault",
		Long:  "Commands to manage Azure Key Vault instances",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List key vaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return List(context.Background(), cmd, resourceGroup)
		},
	}
	listCmd.Flags().StringP("resource-group", "g", "", "Resource group name (optional, lists all if not specified)")

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Show(context.Background(), cmd, vaultName, resourceGroup)
		},
	}
	showCmd.Flags().StringP("name", "n", "", "Key vault name")
	showCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	showCmd.MarkFlagRequired("name")
	showCmd.MarkFlagRequired("resource-group")

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Create(context.Background(), cmd, name, resourceGroup, location, tags)
		},
	}
	createCmd.Flags().StringP("name", "n", "", "Key vault name")
	createCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	createCmd.Flags().StringP("location", "l", "", "Location (e.g., eastus, westus2)")
	createCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	createCmd.MarkFlagRequired("name")
	createCmd.MarkFlagRequired("resource-group")
	createCmd.MarkFlagRequired("location")

	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return Delete(context.Background(), name, resourceGroup)
		},
	}
	deleteCmd.Flags().StringP("name", "n", "", "Key vault name")
	deleteCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	deleteCmd.MarkFlagRequired("name")
	deleteCmd.MarkFlagRequired("resource-group")

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			tags, _ := cmd.Flags().GetStringToString("tags")
			return Update(context.Background(), cmd, name, resourceGroup, tags)
		},
	}
	updateCmd.Flags().StringP("name", "n", "", "Key vault name")
	updateCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	updateCmd.Flags().StringToString("tags", nil, "Space-separated tags: key1=value1 key2=value2")
	updateCmd.Flags().Bool("enable-rbac-authorization", false, "Enable Azure RBAC for data-plane authorization")
	updateCmd.Flags().Bool("enabled-for-deployment", false, "Allow VMs to retrieve certificates as secrets")
	updateCmd.Flags().Bool("enabled-for-disk-encryption", false, "Allow Azure Disk Encryption to retrieve secrets and unwrap keys")
	updateCmd.Flags().Bool("enabled-for-template-deployment", false, "Allow Resource Manager to retrieve secrets")
	updateCmd.MarkFlagRequired("name")
	updateCmd.MarkFlagRequired("resource-group")

	listDeletedCmd := &cobra.Command{
		Use:   "list-deleted",
		Short: "List soft-deleted key vaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ListDeleted(context.Background(), cmd)
		},
	}

	showDeletedCmd := &cobra.Command{
		Use:   "show-deleted",
		Short: "Show a soft-deleted key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			location, _ := cmd.Flags().GetString("location")
			return ShowDeleted(context.Background(), cmd, name, location)
		},
	}
	showDeletedCmd.Flags().StringP("name", "n", "", "Key vault name")
	showDeletedCmd.Flags().StringP("location", "l", "", "Location of the deleted vault")
	showDeletedCmd.MarkFlagRequired("name")
	showDeletedCmd.MarkFlagRequired("location")

	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Permanently delete a soft-deleted key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			location, _ := cmd.Flags().GetString("location")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Purge(context.Background(), cmd, name, location, noWait)
		},
	}
	purgeCmd.Flags().StringP("name", "n", "", "Key vault name")
	purgeCmd.Flags().StringP("location", "l", "", "Location of the deleted vault")
	purgeCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	purgeCmd.MarkFlagRequired("name")
	purgeCmd.MarkFlagRequired("location")

	recoverCmd := &cobra.Command{
		Use:   "recover",
		Short: "Recover a soft-deleted key vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			location, _ := cmd.Flags().GetString("location")
			noWait, _ := cmd.Flags().GetBool("no-wait")
			return Recover(context.Background(), cmd, name, resourceGroup, location, noWait)
		},
	}
	recoverCmd.Flags().StringP("name", "n", "", "Key vault name")
	recoverCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	recoverCmd.Flags().StringP("location", "l", "", "Location of the deleted vault")
	recoverCmd.Flags().Bool("no-wait", false, "Do not wait for the long-running operation to finish")
	recoverCmd.MarkFlagRequired("name")
	recoverCmd.MarkFlagRequired("resource-group")
	recoverCmd.MarkFlagRequired("location")

	checkNameCmd := &cobra.Command{
		Use:   "check-name",
		Short: "Check whether a key vault name is available",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			return CheckName(context.Background(), cmd, name)
		},
	}
	checkNameCmd.Flags().StringP("name", "n", "", "Key vault name to check")
	checkNameCmd.MarkFlagRequired("name")

	policyFlags := func(c *cobra.Command) {
		c.Flags().StringP("name", "n", "", "Key vault name")
		c.Flags().StringP("resource-group", "g", "", "Resource group name")
		c.Flags().String("object-id", "", "A GUID that identifies the principal that will receive permissions")
		c.Flags().String("application-id", "", "Application ID of the client making request on behalf of a principal")
		c.Flags().String("spn", "", "Name of a service principal that will receive permissions")
		c.Flags().String("upn", "", "Name of a user principal that will receive permissions")
		c.MarkFlagRequired("name")
		c.MarkFlagRequired("resource-group")
	}
	policyOpts := func(cmd *cobra.Command) PolicyOptions {
		opts := PolicyOptions{}
		opts.VaultName, _ = cmd.Flags().GetString("name")
		opts.ResourceGroup, _ = cmd.Flags().GetString("resource-group")
		opts.ObjectID, _ = cmd.Flags().GetString("object-id")
		opts.ApplicationID, _ = cmd.Flags().GetString("application-id")
		opts.SPN, _ = cmd.Flags().GetString("spn")
		opts.UPN, _ = cmd.Flags().GetString("upn")
		// A permission flag the caller left out keeps its previous value, so
		// an unset flag must stay nil rather than become an empty list.
		if cmd.Flags().Changed("key-permissions") {
			opts.KeyPermissions, _ = cmd.Flags().GetStringSlice("key-permissions")
		}
		if cmd.Flags().Changed("secret-permissions") {
			opts.SecretPermissions, _ = cmd.Flags().GetStringSlice("secret-permissions")
		}
		if cmd.Flags().Changed("certificate-permissions") {
			opts.CertificatePermissions, _ = cmd.Flags().GetStringSlice("certificate-permissions")
		}
		if cmd.Flags().Changed("storage-permissions") {
			opts.StoragePermissions, _ = cmd.Flags().GetStringSlice("storage-permissions")
		}
		return opts
	}

	setPolicyCmd := &cobra.Command{
		Use:   "set-policy",
		Short: "Update security policy settings for a Key Vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			return SetPolicy(context.Background(), cmd, policyOpts(cmd))
		},
	}
	policyFlags(setPolicyCmd)
	setPolicyCmd.Flags().StringSlice("key-permissions", nil, "List of key permissions to assign")
	setPolicyCmd.Flags().StringSlice("secret-permissions", nil, "List of secret permissions to assign")
	setPolicyCmd.Flags().StringSlice("certificate-permissions", nil, "List of certificate permissions to assign")
	setPolicyCmd.Flags().StringSlice("storage-permissions", nil, "List of storage permissions to assign")

	deletePolicyCmd := &cobra.Command{
		Use:   "delete-policy",
		Short: "Delete security policy settings for a Key Vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			return DeletePolicy(context.Background(), cmd, policyOpts(cmd))
		},
	}
	policyFlags(deletePolicyCmd)

	networkRuleCmd := &cobra.Command{
		Use:   "network-rule",
		Short: "Manage vault network ACLs",
	}

	networkRuleAddCmd := &cobra.Command{
		Use:   "add",
		Short: "Add a network rule to the network ACLs for a Key Vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			ipAddresses, _ := cmd.Flags().GetStringSlice("ip-address")
			subnet, _ := cmd.Flags().GetString("subnet")
			vnetName, _ := cmd.Flags().GetString("vnet-name")
			return AddNetworkRule(context.Background(), cmd, vaultName, resourceGroup, ipAddresses, subnet, vnetName)
		},
	}
	networkRuleRemoveCmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a network rule from the network ACLs for a Key Vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			ipAddresses, _ := cmd.Flags().GetStringSlice("ip-address")
			subnet, _ := cmd.Flags().GetString("subnet")
			vnetName, _ := cmd.Flags().GetString("vnet-name")
			return RemoveNetworkRule(context.Background(), cmd, vaultName, resourceGroup, ipAddresses, subnet, vnetName)
		},
	}
	for _, c := range []*cobra.Command{networkRuleAddCmd, networkRuleRemoveCmd} {
		c.Flags().StringP("name", "n", "", "Key vault name")
		c.Flags().StringP("resource-group", "g", "", "Resource group name")
		c.Flags().StringSlice("ip-address", nil, "IPv4 address or CIDR range. Can supply a list")
		c.Flags().String("subnet", "", "Name or ID of subnet. If a name is supplied, --vnet-name must be supplied")
		c.Flags().String("vnet-name", "", "Name of a virtual network")
		c.MarkFlagRequired("name")
		c.MarkFlagRequired("resource-group")
	}

	networkRuleListCmd := &cobra.Command{
		Use:   "list",
		Short: "List the network rules from the network ACLs for a Key Vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return ListNetworkRules(context.Background(), cmd, vaultName, resourceGroup)
		},
	}
	networkRuleListCmd.Flags().StringP("name", "n", "", "Key vault name")
	networkRuleListCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	networkRuleListCmd.MarkFlagRequired("name")
	networkRuleListCmd.MarkFlagRequired("resource-group")

	networkRuleCmd.AddCommand(networkRuleAddCmd, networkRuleRemoveCmd, networkRuleListCmd)

	pecCmd := &cobra.Command{
		Use:   "private-endpoint-connection",
		Short: "Manage vault private endpoint connections",
	}

	pecApproveCmd := &cobra.Command{
		Use:   "approve",
		Short: "Approve a private endpoint connection request for a Key Vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			description, _ := cmd.Flags().GetString("description")
			return SetPrivateEndpointConnectionStatus(context.Background(), cmd, vaultName, resourceGroup, name, description, true)
		},
	}
	pecRejectCmd := &cobra.Command{
		Use:   "reject",
		Short: "Reject a private endpoint connection request for a Key Vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			description, _ := cmd.Flags().GetString("description")
			return SetPrivateEndpointConnectionStatus(context.Background(), cmd, vaultName, resourceGroup, name, description, false)
		},
	}
	pecDeleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a private endpoint connection request for a Key Vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return DeletePrivateEndpointConnection(context.Background(), cmd, vaultName, resourceGroup, name)
		},
	}
	pecShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a private endpoint connection associated with a Key Vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			name, _ := cmd.Flags().GetString("name")
			return ShowPrivateEndpointConnection(context.Background(), cmd, vaultName, resourceGroup, name)
		},
	}
	for _, c := range []*cobra.Command{pecApproveCmd, pecRejectCmd, pecDeleteCmd, pecShowCmd} {
		c.Flags().String("vault-name", "", "Name of the Key Vault")
		c.Flags().StringP("resource-group", "g", "", "Resource group name")
		c.Flags().StringP("name", "n", "", "Name of the private endpoint connection")
		c.MarkFlagRequired("vault-name")
		c.MarkFlagRequired("resource-group")
		c.MarkFlagRequired("name")
	}
	pecApproveCmd.Flags().String("description", "", "Comments for the approve operation")
	pecRejectCmd.Flags().String("description", "", "Comments for the reject operation")

	pecListCmd := &cobra.Command{
		Use:   "list",
		Short: "List the private endpoint connections of a Key Vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return ListPrivateEndpointConnections(context.Background(), cmd, vaultName, resourceGroup)
		},
	}
	pecListCmd.Flags().String("vault-name", "", "Name of the Key Vault")
	pecListCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	pecListCmd.MarkFlagRequired("vault-name")
	pecListCmd.MarkFlagRequired("resource-group")

	pecCmd.AddCommand(pecApproveCmd, pecRejectCmd, pecDeleteCmd, pecShowCmd, pecListCmd)

	plrCmd := &cobra.Command{
		Use:   "private-link-resource",
		Short: "Manage vault private link resources",
	}
	plrListCmd := &cobra.Command{
		Use:   "list",
		Short: "List the private link resources supported for a Key Vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			vaultName, _ := cmd.Flags().GetString("vault-name")
			resourceGroup, _ := cmd.Flags().GetString("resource-group")
			return ListPrivateLinkResources(context.Background(), cmd, vaultName, resourceGroup)
		},
	}
	plrListCmd.Flags().String("vault-name", "", "Name of the Key Vault")
	plrListCmd.Flags().StringP("resource-group", "g", "", "Resource group name")
	plrListCmd.MarkFlagRequired("vault-name")
	plrListCmd.MarkFlagRequired("resource-group")
	plrCmd.AddCommand(plrListCmd)

	cmd.AddCommand(
		listCmd,
		showCmd,
		createCmd,
		deleteCmd,
		updateCmd,
		listDeletedCmd,
		showDeletedCmd,
		purgeCmd,
		recoverCmd,
		checkNameCmd,
		setPolicyCmd,
		deletePolicyCmd,
		networkRuleCmd,
		pecCmd,
		plrCmd,
		key.NewKeyCommand(),
		certificate.NewCertificateCommand(),
		secret.NewSecretCommand(),
	)
	return cmd
}
