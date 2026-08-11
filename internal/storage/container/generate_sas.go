package container

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cdobbyn/azure-go-cli/internal/storage/sas"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewGenerateSASCommand builds `az storage container generate-sas`.
func NewGenerateSASCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-sas",
		Short: "Generate a SAS token for a storage container",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerateSAS(context.Background(), cmd)
		},
	}

	cmd.Flags().StringP("name", "n", "", "The container name")
	cmd.Flags().String("permissions", "", "The permissions the SAS grants. Allowed values: "+sas.ContainerPerms+". Can be combined")
	cmd.Flags().String("expiry", "", "UTC datetime (Y-m-d'T'H:M'Z') at which the SAS becomes invalid")
	cmd.Flags().String("start", "", "UTC datetime (Y-m-d'T'H:M'Z') at which the SAS becomes valid. Defaults to the time of the request")
	cmd.Flags().String("ip", "", "IP address or range of IP addresses from which to accept requests. IPv4 only")
	cmd.Flags().Bool("https-only", false, "Only permit requests made with the HTTPS protocol")
	cmd.Flags().String("policy-name", "", "The name of a stored access policy within the container's ACL")
	cmd.Flags().Bool("as-user", false, "Return the SAS signed with the user delegation key. Requires --expiry and --auth-mode login")
	cmd.Flags().String("auth-mode", "key", "The mode in which to run the command. Allowed values: key, login")
	cmd.Flags().String("user-delegation-oid", "", "Entra ID of the user authorized to use the resulting SAS URL. Requires --as-user")
	cmd.Flags().String("cache-control", "", "Response header value for Cache-Control when the resource is accessed using this SAS")
	cmd.Flags().String("content-disposition", "", "Response header value for Content-Disposition when the resource is accessed using this SAS")
	cmd.Flags().String("content-encoding", "", "Response header value for Content-Encoding when the resource is accessed using this SAS")
	cmd.Flags().String("content-language", "", "Response header value for Content-Language when the resource is accessed using this SAS")
	cmd.Flags().String("content-type", "", "Response header value for Content-Type when the resource is accessed using this SAS")
	cmd.Flags().String("account-name", "", "Storage account name. Environment variable: AZURE_STORAGE_ACCOUNT")
	cmd.Flags().String("account-key", "", "Storage account key. Environment variable: AZURE_STORAGE_KEY")
	cmd.Flags().String("connection-string", "", "Storage account connection string. Environment variable: AZURE_STORAGE_CONNECTION_STRING")
	cmd.Flags().String("encryption-scope", "", "A predefined encryption scope used to encrypt the data on the service")

	cmd.MarkFlagRequired("name")

	return cmd
}

func runGenerateSAS(ctx context.Context, cmd *cobra.Command) error {
	containerName, _ := cmd.Flags().GetString("name")
	permissions, _ := cmd.Flags().GetString("permissions")
	expiryStr, _ := cmd.Flags().GetString("expiry")
	startStr, _ := cmd.Flags().GetString("start")
	ip, _ := cmd.Flags().GetString("ip")
	httpsOnly, _ := cmd.Flags().GetBool("https-only")
	policyName, _ := cmd.Flags().GetString("policy-name")
	asUser, _ := cmd.Flags().GetBool("as-user")
	authMode, _ := cmd.Flags().GetString("auth-mode")
	delegationOID, _ := cmd.Flags().GetString("user-delegation-oid")
	cacheControl, _ := cmd.Flags().GetString("cache-control")
	contentDisposition, _ := cmd.Flags().GetString("content-disposition")
	contentEncoding, _ := cmd.Flags().GetString("content-encoding")
	contentLanguage, _ := cmd.Flags().GetString("content-language")
	contentType, _ := cmd.Flags().GetString("content-type")
	accountName, _ := cmd.Flags().GetString("account-name")
	accountKey, _ := cmd.Flags().GetString("account-key")
	connectionString, _ := cmd.Flags().GetString("connection-string")
	encryptionScope, _ := cmd.Flags().GetString("encryption-scope")

	if policyName == "" && permissions == "" {
		return fmt.Errorf("--permissions is required unless --policy-name is specified")
	}
	if policyName != "" && (permissions != "" || expiryStr != "") {
		fmt.Fprintln(os.Stderr, "Please do not specify --expiry and --permissions if they are already specified in your policy.")
	}
	if authMode != "key" && authMode != "login" {
		return fmt.Errorf("invalid --auth-mode %q: valid values are key, login", authMode)
	}

	var expiry time.Time
	var err error
	if expiryStr != "" {
		expiry, err = sas.ParseTime(expiryStr)
		if err != nil {
			return fmt.Errorf("--expiry: %w", err)
		}
	}
	var start time.Time
	if startStr != "" {
		start, err = sas.ParseTime(startStr)
		if err != nil {
			return fmt.Errorf("--start: %w", err)
		}
	}

	if err := sas.ValidateAsUser(asUser, authMode, expiryStr, expiry, time.Now().UTC()); err != nil {
		return err
	}
	if delegationOID != "" && !asUser {
		return fmt.Errorf("incorrect usage: need to specify '--as-user' when '--user-delegation-oid' is provided")
	}

	protocol := ""
	if httpsOnly {
		protocol = "https"
	}

	opts := sas.BlobScopeOptions{
		ContainerName:      containerName,
		Permissions:        permissions,
		Identifier:         policyName,
		IPRange:            ip,
		Protocol:           protocol,
		EncryptionScope:    encryptionScope,
		CacheControl:       cacheControl,
		ContentDisposition: contentDisposition,
		ContentEncoding:    contentEncoding,
		ContentLanguage:    contentLanguage,
		ContentType:        contentType,
		AuthorizedObjectID: delegationOID,
		Start:              start,
		Expiry:             expiry,
	}

	var key string
	if asUser {
		opts.AccountName = sas.ResolveInputs(accountName, accountKey, connectionString).AccountName
		if opts.AccountName == "" {
			return fmt.Errorf("--account-name is required (or set AZURE_STORAGE_ACCOUNT)")
		}
	} else {
		creds, err := sas.Resolve(ctx, accountName, accountKey, connectionString)
		if err != nil {
			return err
		}
		opts.AccountName = creds.AccountName
		key = creds.AccountKey
	}

	token, err := sas.SignBlobScope(ctx, opts, key, asUser)
	if err != nil {
		return err
	}

	return output.PrintFormatted(cmd, token, sas.OutputFormat(cmd))
}
