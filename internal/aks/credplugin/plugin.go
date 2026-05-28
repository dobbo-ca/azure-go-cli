package credplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

// DetermineAPIVersion inspects the KUBERNETES_EXEC_INFO env value kubectl
// supplies (may be empty) and returns the apiVersion to emit. Defaults to
// v1beta1 for empty input or an envelope missing apiVersion (kubectl <1.22
// behavior). Returns an error for malformed JSON or an unrecognized version.
func DetermineAPIVersion(execInfoEnv string) (string, error) {
	if execInfoEnv == "" {
		return APIVersionV1Beta1, nil
	}
	var env execInfoEnvelope
	if err := json.Unmarshal([]byte(execInfoEnv), &env); err != nil {
		return "", fmt.Errorf("malformed KUBERNETES_EXEC_INFO: %w", err)
	}
	switch env.APIVersion {
	case "":
		return APIVersionV1Beta1, nil
	case APIVersionV1, APIVersionV1Beta1:
		return env.APIVersion, nil
	default:
		return "", fmt.Errorf("unsupported KUBERNETES_EXEC_INFO apiVersion: %s", env.APIVersion)
	}
}

// RenderExecCredential writes the ExecCredential JSON envelope kubectl expects
// to w. apiVersion is whatever DetermineAPIVersion returned.
func RenderExecCredential(token azcore.AccessToken, apiVersion string, w io.Writer) error {
	cred := ExecCredential{
		Kind:       "ExecCredential",
		APIVersion: apiVersion,
		Status: ExecCredentialStatus{
			Token:               token.Token,
			ExpirationTimestamp: token.ExpiresOn,
		},
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(&cred); err != nil {
		return fmt.Errorf("failed to encode ExecCredential: %w", err)
	}
	return nil
}

// GetTokenOptions configures GetToken. ServerID is required. CredentialFactory,
// Stdout, and ExecInfoEnv are optional and default to production values; tests
// inject fakes via the same fields.
type GetTokenOptions struct {
	ServerID string
	TenantID string
	// ClientID is accepted for symmetry with the --client-id flag surface
	// (see Task 9). The credential chain consumes AZURE_CLIENT_ID from the
	// environment rather than this field directly.
	ClientID string

	// CredentialFactory returns the TokenCredential used to mint the access
	// token. Required: callers must pass azure.GetCredential or a stub. The
	// Cobra wrapper in gettoken.go (Task 9) wires the production default.
	CredentialFactory func() (azcore.TokenCredential, error)

	// Stdout is where the ExecCredential JSON is written. Defaults to os.Stdout.
	Stdout io.Writer

	// ExecInfoEnv is the value of $KUBERNETES_EXEC_INFO. If empty, GetToken
	// reads os.Getenv("KUBERNETES_EXEC_INFO").
	ExecInfoEnv string
}

// GetToken is the kubectl exec credential plugin entrypoint. It mints an AKS
// access token at scope <server-id>/.default using the configured credential
// chain and writes an ExecCredential envelope to the writer.
func GetToken(ctx context.Context, opts GetTokenOptions) error {
	if opts.ServerID == "" {
		return fmt.Errorf("--server-id is required")
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	execInfo := opts.ExecInfoEnv
	if execInfo == "" {
		execInfo = os.Getenv("KUBERNETES_EXEC_INFO")
	}

	apiVersion, err := DetermineAPIVersion(execInfo)
	if err != nil {
		return err
	}

	credFactory := opts.CredentialFactory
	if credFactory == nil {
		return fmt.Errorf("CredentialFactory is required (production callers should pass azure.GetCredential)")
	}
	cred, err := credFactory()
	if err != nil {
		return fmt.Errorf("failed to obtain Azure credential: %w", err)
	}

	scope := opts.ServerID + "/.default"
	reqOpts := policy.TokenRequestOptions{
		Scopes:   []string{scope},
		TenantID: opts.TenantID,
	}

	token, err := cred.GetToken(ctx, reqOpts)
	if err != nil {
		return fmt.Errorf("failed to mint token: %w", err)
	}

	return RenderExecCredential(token, apiVersion, stdout)
}
