package account

import (
	"context"
	"fmt"
	"time"

	"github.com/cdobbyn/azure-go-cli/internal/storage/sas"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewGenerateSASCommand builds `az storage account generate-sas`.
func NewGenerateSASCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-sas",
		Short: "Generate a shared access signature for the storage account",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerateSAS(context.Background(), cmd)
		},
	}

	cmd.Flags().String("services", "", "The storage services the SAS is applicable for. Allowed values: (b)lob (f)ile (q)ueue (t)able. Can be combined")
	cmd.Flags().String("resource-types", "", "The resource types the SAS is applicable for. Allowed values: (s)ervice (c)ontainer (o)bject. Can be combined")
	cmd.Flags().String("permissions", "", "The permissions the SAS grants. Allowed values: "+sas.AccountPerms+". Can be combined")
	cmd.Flags().String("expiry", "", "UTC datetime (Y-m-d'T'H:M'Z') at which the SAS becomes invalid")
	cmd.Flags().String("start", "", "UTC datetime (Y-m-d'T'H:M'Z') at which the SAS becomes valid. Defaults to the time of the request")
	cmd.Flags().String("ip", "", "IP address or range of IP addresses from which to accept requests. IPv4 only")
	cmd.Flags().Bool("https-only", false, "Only permit requests made with the HTTPS protocol")
	cmd.Flags().String("account-name", "", "Storage account name. Environment variable: AZURE_STORAGE_ACCOUNT")
	cmd.Flags().String("account-key", "", "Storage account key. Environment variable: AZURE_STORAGE_KEY")
	cmd.Flags().String("connection-string", "", "Storage account connection string. Environment variable: AZURE_STORAGE_CONNECTION_STRING")
	cmd.Flags().String("encryption-scope", "", "A predefined encryption scope used to encrypt the data on the service")

	cmd.MarkFlagRequired("services")
	cmd.MarkFlagRequired("resource-types")
	cmd.MarkFlagRequired("permissions")
	cmd.MarkFlagRequired("expiry")

	return cmd
}

func runGenerateSAS(ctx context.Context, cmd *cobra.Command) error {
	services, _ := cmd.Flags().GetString("services")
	resourceTypes, _ := cmd.Flags().GetString("resource-types")
	permissions, _ := cmd.Flags().GetString("permissions")
	expiryStr, _ := cmd.Flags().GetString("expiry")
	startStr, _ := cmd.Flags().GetString("start")
	ip, _ := cmd.Flags().GetString("ip")
	httpsOnly, _ := cmd.Flags().GetBool("https-only")
	accountName, _ := cmd.Flags().GetString("account-name")
	accountKey, _ := cmd.Flags().GetString("account-key")
	connectionString, _ := cmd.Flags().GetString("connection-string")
	encryptionScope, _ := cmd.Flags().GetString("encryption-scope")

	expiry, err := sas.ParseTime(expiryStr)
	if err != nil {
		return fmt.Errorf("--expiry: %w", err)
	}
	var start time.Time
	if startStr != "" {
		start, err = sas.ParseTime(startStr)
		if err != nil {
			return fmt.Errorf("--start: %w", err)
		}
	}

	creds, err := sas.Resolve(ctx, accountName, accountKey, connectionString)
	if err != nil {
		return err
	}

	protocol := ""
	if httpsOnly {
		protocol = "https"
	}

	token, err := sas.SignAccount(sas.AccountOptions{
		AccountName:     creds.AccountName,
		Permissions:     permissions,
		Services:        services,
		ResourceTypes:   resourceTypes,
		IPRange:         ip,
		Protocol:        protocol,
		EncryptionScope: encryptionScope,
		Start:           start,
		Expiry:          expiry,
	}, creds.AccountKey)
	if err != nil {
		return err
	}

	return output.PrintFormatted(cmd, token, sas.OutputFormat(cmd))
}
