package credplugin

import "time"

// API versions accepted in KUBERNETES_EXEC_INFO and emitted on stdout.
const (
	APIVersionV1Beta1 = "client.authentication.k8s.io/v1beta1"
	APIVersionV1      = "client.authentication.k8s.io/v1"

	// AKSServerIDDefault is the first-party AKS AAD application ID. Used as the
	// fallback --server-id when neither the legacy auth-provider config nor an
	// existing exec entry supplies one.
	AKSServerIDDefault = "6dae42f8-4368-4678-94ff-3960e28e3630"
)

// ExecCredential is the wire shape kubectl expects from an exec credential
// plugin (client-go clientauthentication v1 / v1beta1). We hand-roll it to
// avoid vendoring k8s.io/client-go for a 3-field payload.
type ExecCredential struct {
	Kind       string               `json:"kind"`
	APIVersion string               `json:"apiVersion"`
	Status     ExecCredentialStatus `json:"status"`
}

type ExecCredentialStatus struct {
	Token               string    `json:"token"`
	ExpirationTimestamp time.Time `json:"expirationTimestamp"`
}

// execInfoEnvelope is just the apiVersion field we read out of
// KUBERNETES_EXEC_INFO. Anything else in the envelope is ignored.
type execInfoEnvelope struct {
	APIVersion string `json:"apiVersion"`
}
