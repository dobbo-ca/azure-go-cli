package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/cdobbyn/azure-go-cli/internal/storage/sas"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/spf13/cobra"
)

// addAuthFlags registers the credential flags azure-cli puts on every blob
// data-plane command (_register_data_plane_account_arguments and
// _register_data_plane_oauth_arguments in storage/__init__.py).
func addAuthFlags(cmd *cobra.Command) {
	cmd.Flags().String("account-name", "", "Storage account name. Environment variable: AZURE_STORAGE_ACCOUNT")
	cmd.Flags().String("account-key", "", "Storage account key. Environment variable: AZURE_STORAGE_KEY")
	cmd.Flags().String("connection-string", "", "Storage account connection string. Environment variable: AZURE_STORAGE_CONNECTION_STRING")
	cmd.Flags().String("sas-token", "", "A Shared Access Signature (SAS). Environment variable: AZURE_STORAGE_SAS_TOKEN")
	cmd.Flags().String("blob-endpoint", "", "Storage data service endpoint. Environment variable: AZURE_STORAGE_SERVICE_ENDPOINT")
	cmd.Flags().String("auth-mode", "", `The mode in which to run the command. "login" mode will directly use your login credentials for the authentication. The legacy "key" mode will attempt to query for an account key if no authentication parameters for the account are provided. Allowed values: key, login. Environment variable: AZURE_STORAGE_AUTH_MODE`)
}

// authInputs is what the user supplied, before environment fallback.
type authInputs struct {
	authMode         string
	accountName      string
	accountKey       string
	connectionString string
	sasToken         string
	blobEndpoint     string
}

func authInputsFromFlags(cmd *cobra.Command) authInputs {
	get := func(name string) string {
		v, _ := cmd.Flags().GetString(name)
		return v
	}
	return authInputs{
		authMode:         get("auth-mode"),
		accountName:      get("account-name"),
		accountKey:       get("account-key"),
		connectionString: get("connection-string"),
		sasToken:         get("sas-token"),
		blobEndpoint:     get("blob-endpoint"),
	}
}

// resolvedAuth is the credential a blob client is built from. Exactly one of
// login, connectionString, accountKey or sasToken drives the client; with
// none set the request is anonymous, as in azure-cli.
type resolvedAuth struct {
	login            bool
	connectionString string
	accountName      string
	accountKey       string
	sasToken         string
	endpoint         string
}

const noCredsWarning = `
There are no credentials provided in your command and environment, we will query for account key for your storage account.
It is recommended to provide --connection-string, --account-key or --sas-token in your command as credentials.

You also can add ` + "`--auth-mode login`" + ` in your command to use Azure Active Directory (Azure AD) for authorization if your login account is assigned required RBAC roles.
For more information about RBAC roles in storage, visit https://learn.microsoft.com/azure/storage/common/storage-auth-aad-rbac-cli.

In addition, setting the corresponding environment variables can avoid inputting credentials in your command. Please use --help to get more information about environment variable usage.`

// hostnameLabel matches azure-cli's is_valid_hostname_label.
var hostnameLabel = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

// resolveAuth mirrors azure-cli's validate_client_parameters
// (storage/_validators.py). --auth-mode has no default there: unless it is
// "login", the key path runs, and an account name with no key or SAS
// triggers a listKeys lookup over ARM. A failed lookup only warns, leaving
// the request anonymous, which is what azure-cli does too.
func resolveAuth(ctx context.Context, in authInputs, fetchKey func(context.Context, string) (string, error), warn io.Writer) (resolvedAuth, error) {
	// The flag is an enum, matched case-insensitively like azure-cli's
	// get_enum_type; the environment variable is not validated there, so any
	// value other than "login" just takes the key path.
	mode := strings.ToLower(in.authMode)
	if mode != "" && mode != "login" && mode != "key" {
		return resolvedAuth{}, fmt.Errorf("invalid --auth-mode %q: allowed values are key, login", in.authMode)
	}
	if mode == "" {
		mode = strings.ToLower(os.Getenv("AZURE_STORAGE_AUTH_MODE"))
	}

	r := resolvedAuth{endpoint: sas.RawServiceEndpoint(in.blobEndpoint)}
	name := in.accountName
	if name == "" {
		name = os.Getenv("AZURE_STORAGE_ACCOUNT")
	}

	if mode == "login" {
		var ignored []string
		for _, f := range [][2]string{{"--account-key", in.accountKey}, {"--sas-token", in.sasToken}, {"--connection-string", in.connectionString}} {
			if f[1] != "" {
				ignored = append(ignored, f[0])
			}
		}
		if len(ignored) > 0 {
			fmt.Fprintf(warn, "In \"login\" auth mode, the following arguments are ignored: %s\n", strings.Join(ignored, " ,"))
		}
		r.login = true
		r.accountName = name
		return r, r.requireAccount()
	}

	key, sasToken, cs := in.accountKey, in.sasToken, in.connectionString
	if cs == "" && key == "" && sasToken == "" {
		cs = os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
	}
	if cs != "" {
		// The SDK parses endpoints, key and SAS out of the string itself.
		r.connectionString = cs
		return r, nil
	}
	if key == "" && sasToken == "" {
		key = os.Getenv("AZURE_STORAGE_KEY")
	}
	if sasToken == "" {
		sasToken = os.Getenv("AZURE_STORAGE_SAS_TOKEN")
	}
	sasToken = strings.TrimLeft(sasToken, "?")

	if name != "" && !hostnameLabel.MatchString(name) && r.endpoint == "" {
		return r, fmt.Errorf("invalid storage account name. The account name may only contain letters, numbers and hyphens")
	}
	if name != "" && key == "" && sasToken == "" && hostnameLabel.MatchString(name) {
		fmt.Fprintln(warn, noCredsWarning)
		k, err := fetchKey(ctx, name)
		if err != nil {
			fmt.Fprintf(warn, "\nSkip querying account key due to failure: %v\n", err)
		}
		key = k
	}
	if r.endpoint != "" && key == "" && sasToken == "" {
		return r, fmt.Errorf("there are no credentials provided in your command and environment.\nPlease provide --connection-string, --account-key or --sas-token in your command as credentials")
	}

	r.accountName, r.accountKey, r.sasToken = name, key, sasToken
	return r, r.requireAccount()
}

func (r resolvedAuth) requireAccount() error {
	if r.accountName == "" && r.endpoint == "" {
		return fmt.Errorf("please provide storage account name or connection string")
	}
	return nil
}

// newClient builds a blob service client from the command's credential flags.
func newClient(ctx context.Context, cmd *cobra.Command) (*azblob.Client, error) {
	r, err := resolveAuth(ctx, authInputsFromFlags(cmd), sas.FetchAccountKey, os.Stderr)
	if err != nil {
		return nil, err
	}
	if r.connectionString != "" {
		client, err := azblob.NewClientFromConnectionString(r.connectionString, nil)
		if err != nil {
			return nil, fmt.Errorf("invalid connection string: %w", err)
		}
		return client, nil
	}

	serviceURL := sas.ServiceEndpoint(r.endpoint, r.accountName) + "/"
	switch {
	case r.login:
		cred, err := azure.GetCredential()
		if err != nil {
			return nil, err
		}
		return azblob.NewClient(serviceURL, cred, nil)
	case r.accountKey != "":
		// With only --blob-endpoint, the SDK would sign for an empty account;
		// take the name from the endpoint as the Python SDK does.
		name := r.accountName
		if name == "" {
			name = sas.AccountFromEndpoint(r.endpoint)
		}
		cred, err := sas.NewSharedKey(name, r.accountKey)
		if err != nil {
			return nil, err
		}
		return azblob.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	case r.sasToken != "":
		return azblob.NewClientWithNoCredential(serviceURL+"?"+r.sasToken, nil)
	default:
		return azblob.NewClientWithNoCredential(serviceURL, nil)
	}
}

// storageError adds azure-cli's hints for common 403s
// (get_handler_suppress_some_400 in storage/__init__.py).
func storageError(err error) error {
	var re *azcore.ResponseError
	if !errors.As(err, &re) || re.StatusCode != 403 {
		return err
	}
	var hint string
	switch re.ErrorCode {
	case "AuthorizationPermissionMismatch":
		hint = `You do not have the required permissions needed to perform this operation.
Depending on your operation, you may need to be assigned one of the following roles:
    "Storage Blob Data Owner"
    "Storage Blob Data Contributor"
    "Storage Blob Data Reader"

If you want to use the old authentication method and allow querying for the right account key, please use the "--auth-mode" parameter and "key" value.`
	case "AuthorizationFailure":
		hint = `The request may be blocked by network rules of storage account. Please check network rule set using 'az storage account show -n accountname --query networkRuleSet'.
If you want to change the default action to apply when no rule matches, please use 'az storage account update'.`
	case "AuthenticationFailed":
		hint = "Authentication failure. This may be caused by either invalid account key, connection string or sas token value provided for your storage account."
	default:
		return err
	}
	return fmt.Errorf("%w\n\n%s", err, hint)
}
