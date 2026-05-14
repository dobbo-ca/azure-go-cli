package credplugin

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
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
