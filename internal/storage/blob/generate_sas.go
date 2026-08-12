package blob

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/cdobbyn/azure-go-cli/internal/storage/sas"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// NewGenerateSASCommand builds `az storage blob generate-sas`.
func NewGenerateSASCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-sas",
		Short: "Generate a shared access signature for the blob",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerateSAS(context.Background(), cmd)
		},
	}

	cmd.Flags().StringP("name", "n", "", "The blob name")
	cmd.Flags().StringP("container-name", "c", "", "The container name")
	cmd.Flags().String("permissions", "", "The permissions the SAS grants. Allowed values: "+sas.BlobPerms+". Can be combined")
	cmd.Flags().String("expiry", "", "UTC datetime (Y-m-d'T'H:M'Z') at which the SAS becomes invalid")
	cmd.Flags().String("start", "", "UTC datetime (Y-m-d'T'H:M'Z') at which the SAS becomes valid. Defaults to the time of the request")
	cmd.Flags().String("ip", "", "IP address or range of IP addresses from which to accept requests. IPv4 only")
	cmd.Flags().Bool("https-only", false, "Only permit requests made with the HTTPS protocol")
	cmd.Flags().String("policy-name", "", "The name of a stored access policy within the container's ACL")
	cmd.Flags().Bool("as-user", false, "Return the SAS signed with the user delegation key. Requires --expiry and --auth-mode login")
	cmd.Flags().String("auth-mode", "key", "The mode in which to run the command. Allowed values: key, login")
	cmd.Flags().String("user-delegation-oid", "", "Entra ID of the user authorized to use the resulting SAS URL. Requires --as-user")
	cmd.Flags().Bool("full-uri", false, "Indicates that this command return the full blob URI and the shared access signature token")
	cmd.Flags().String("snapshot", "", "An optional blob snapshot ID. Opaque DateTime value that, when present, specifies the blob snapshot to grant permission")
	cmd.Flags().String("blob-url", "", "The full endpoint URL to the blob. An alternative to --name plus --container-name")
	cmd.Flags().String("cache-control", "", "Response header value for Cache-Control when the resource is accessed using this SAS")
	cmd.Flags().String("content-disposition", "", "Response header value for Content-Disposition when the resource is accessed using this SAS")
	cmd.Flags().String("content-encoding", "", "Response header value for Content-Encoding when the resource is accessed using this SAS")
	cmd.Flags().String("content-language", "", "Response header value for Content-Language when the resource is accessed using this SAS")
	cmd.Flags().String("content-type", "", "Response header value for Content-Type when the resource is accessed using this SAS")
	cmd.Flags().String("account-name", "", "Storage account name. Environment variable: AZURE_STORAGE_ACCOUNT")
	cmd.Flags().String("account-key", "", "Storage account key. Environment variable: AZURE_STORAGE_KEY")
	cmd.Flags().String("connection-string", "", "Storage account connection string. Environment variable: AZURE_STORAGE_CONNECTION_STRING")
	cmd.Flags().String("blob-endpoint", "", "Storage data service endpoint. Use for a sovereign cloud, a private endpoint, or a local emulator. Environment variable: AZURE_STORAGE_SERVICE_ENDPOINT")
	cmd.Flags().String("encryption-scope", "", "A predefined encryption scope used to encrypt the data on the service")

	cmd.MarkFlagRequired("permissions")

	return cmd
}

func runGenerateSAS(ctx context.Context, cmd *cobra.Command) error {
	blobName, _ := cmd.Flags().GetString("name")
	containerName, _ := cmd.Flags().GetString("container-name")
	permissions, _ := cmd.Flags().GetString("permissions")
	expiryStr, _ := cmd.Flags().GetString("expiry")
	startStr, _ := cmd.Flags().GetString("start")
	ip, _ := cmd.Flags().GetString("ip")
	httpsOnly, _ := cmd.Flags().GetBool("https-only")
	policyName, _ := cmd.Flags().GetString("policy-name")
	asUser, _ := cmd.Flags().GetBool("as-user")
	authMode, _ := cmd.Flags().GetString("auth-mode")
	delegationOID, _ := cmd.Flags().GetString("user-delegation-oid")
	fullURIFlag, _ := cmd.Flags().GetBool("full-uri")
	snapshot, _ := cmd.Flags().GetString("snapshot")
	blobURL, _ := cmd.Flags().GetString("blob-url")
	cacheControl, _ := cmd.Flags().GetString("cache-control")
	contentDisposition, _ := cmd.Flags().GetString("content-disposition")
	contentEncoding, _ := cmd.Flags().GetString("content-encoding")
	contentLanguage, _ := cmd.Flags().GetString("content-language")
	contentType, _ := cmd.Flags().GetString("content-type")
	accountName, _ := cmd.Flags().GetString("account-name")
	accountKey, _ := cmd.Flags().GetString("account-key")
	connectionString, _ := cmd.Flags().GetString("connection-string")
	blobEndpoint, _ := cmd.Flags().GetString("blob-endpoint")
	encryptionScope, _ := cmd.Flags().GetString("encryption-scope")

	// --blob-url is an alternative to naming the blob and container. It also
	// carries the account name, so it wins over --account-name.
	var endpoint string
	if blobURL != "" {
		ref, err := parseBlobURL(blobURL)
		if err != nil {
			return err
		}
		accountName = ref.accountName
		containerName = ref.containerName
		blobName = ref.blobName
		if snapshot == "" {
			snapshot = ref.snapshot
		}
		endpoint = ref.endpoint
	}
	if containerName == "" || blobName == "" {
		return fmt.Errorf("specify --name and --container-name, or --blob-url")
	}
	// --blob-endpoint can supply the account name on its own, so
	// --account-name is not required alongside it.
	if accountName == "" && blobURL == "" {
		accountName = sas.AccountFromEndpoint(sas.RawServiceEndpoint(blobEndpoint))
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
		BlobName:           blobName,
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
		Snapshot:           snapshot,
		ServiceEndpoint:    blobEndpoint,
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

	// azure-cli percent-encodes the blob token with this exact safe set
	// (operations/blob.py:906). The container command does not.
	quoted := sas.Quote(token, "&%()$=',~")
	if fullURIFlag {
		if endpoint == "" {
			endpoint = sas.ServiceEndpoint(blobEndpoint, opts.AccountName)
		}
		return output.PrintFormatted(cmd, fullURI(endpoint, containerName, blobName, snapshot, quoted), sas.OutputFormat(cmd))
	}
	return output.PrintFormatted(cmd, quoted, sas.OutputFormat(cmd))
}

// blobRef is a --blob-url decomposed into the pieces this command needs.
type blobRef struct {
	accountName   string
	containerName string
	blobName      string
	snapshot      string
	endpoint      string // scheme://host, plus the account segment for IP-style
}

// parseBlobURL splits a --blob-url into its parts.
//
// Two endpoint shapes exist. The public shape carries the account as the first
// host label (https://acct.blob.core.windows.net/container/blob). The IP shape,
// used by the Azurite emulator and by private endpoints addressed by IP,
// carries it as the first PATH segment
// (http://127.0.0.1:10000/devstoreaccount1/container/blob).
//
// Taking the account from the host in the IP case yields "127", which is wrong
// twice: it signs the canonical resource /blob/127/... so the service rejects
// the token, and it drops the account from the emitted --full-uri, producing a
// 400. Both were observed against Azurite before this was split out.
func parseBlobURL(blobURL string) (blobRef, error) {
	parts, err := azblob.ParseURL(blobURL)
	if err != nil {
		return blobRef{}, fmt.Errorf("invalid --blob-url: %w", err)
	}

	ref := blobRef{
		accountName:   parts.IPEndpointStyleInfo.AccountName,
		containerName: parts.ContainerName,
		blobName:      parts.BlobName,
		snapshot:      parts.Snapshot,
		// Preserve the caller's own scheme+host (sovereign cloud, emulator)
		// instead of reassembling against the public windows.net suffix.
		endpoint: parts.Scheme + "://" + parts.Host,
	}
	if ref.accountName == "" {
		ref.accountName = strings.Split(parts.Host, ".")[0]
	} else {
		ref.endpoint += "/" + ref.accountName
	}
	return ref, nil
}

// pathSafe is the safe set for the container/blob path segments.
//
// It is deliberately just "/", which reproduces azure-cli byte for byte. The
// name a user passes to -n is a LITERAL blob name, so every character that is
// not valid in a URL path gets percent-encoded: a space becomes %20 and, by
// the same rule, a literal '%' becomes %25. Only '/' stays safe, so
// virtual-directory names keep their slashes instead of becoming %2F.
//
// Two earlier safe sets were wrong, both by copying azure-cli's SECOND pass:
//
//	"/()$=',~%"  wrong. Passed a literal '%' straight through, emitting an
//	             unparseable URL for "a%b", and silently retargeting
//	             "my%20file.txt" at the blob "my file.txt" while the signature
//	             still covered "my%20file.txt".
//	"/()$=',~"   valid, but not byte-identical: it left ( ) $ = ' unescaped
//	             where azure-cli escapes them.
//
// azure-cli quotes twice. BlobClient._format_url runs quote(name, safe='~/')
// and does all the real work; encode_url_path then re-quotes with
// SAFE_CHARS = "/()$=',~" (url_quote_util.py:14), which is a no-op on
// already-escaped text. Note SAFE_CHARS contains no '%' at all. Since we quote
// once, matching azure-cli means matching its FIRST pass: safe='~/'. In both
// Python and this package '~' is unconditionally safe, so "/" is equivalent.
const pathSafe = "/"

// fullURI assembles the blob URL with the SAS token appended, matching
// operations/blob.py:902-905. The path is quoted with azure-cli's safe set;
// the token is not re-quoted, since the caller has already quoted it. When a
// snapshot is present it's threaded into the query ahead of the token, since
// a snapshot-scoped SAS (sr=bs) is meaningless without it.
func fullURI(endpoint, containerName, blobName, snapshot, token string) string {
	query := token
	if snapshot != "" {
		query = "snapshot=" + sas.Quote(snapshot, "") + "&" + token
	}
	return fmt.Sprintf("%s/%s/%s?%s",
		endpoint, sas.Quote(containerName, pathSafe), sas.Quote(blobName, pathSafe), query)
}
