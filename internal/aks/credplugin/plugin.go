package credplugin

import (
	"encoding/json"
	"fmt"
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
