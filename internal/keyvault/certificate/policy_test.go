package certificate

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

// TestParsePolicyAcceptsCamelAndSnake feeds the same policy in both spellings
// a user can meet: the camelCase that get-default-policy prints, and the
// snake_case that azure-cli's own docs use. azure-cli accepts the same two,
// because get_json_object snake-cases the document before
// build_certificate_policy reads it (_validators.py:893).
func TestParsePolicyAcceptsCamelAndSnake(t *testing.T) {
	forms := map[string]string{
		"camel": defaultPolicyJSON,
		"snake": `{"issuer_parameters":{"name":"Self"},"key_properties":{"exportable":true,"key_size":2048,"key_type":"RSA","reuse_key":true},"secret_properties":{"content_type":"application/x-pkcs12"},"x509_certificate_properties":{"subject":"CN=CLIGetDefaultPolicy","validity_in_months":12},"lifetime_actions":[{"action":{"action_type":"AutoRenew"},"trigger":{"days_before_expiry":90}}]}`,
	}
	for name, body := range forms {
		policy, err := ParsePolicy(body, 0)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if policy.KeyProperties == nil || *policy.KeyProperties.KeySize != 2048 || string(*policy.KeyProperties.KeyType) != "RSA" {
			t.Errorf("%s: key properties came back as %+v", name, policy.KeyProperties)
		}
		if policy.X509CertificateProperties == nil || *policy.X509CertificateProperties.ValidityInMonths != 12 {
			t.Errorf("%s: validity came back as %+v", name, policy.X509CertificateProperties)
		}
		if *policy.IssuerParameters.Name != "Self" {
			t.Errorf("%s: issuer came back as %+v", name, policy.IssuerParameters)
		}
		if *policy.SecretProperties.ContentType != "application/x-pkcs12" {
			t.Errorf("%s: content type came back as %+v", name, policy.SecretProperties)
		}
		if len(policy.LifetimeActions) != 1 || *policy.LifetimeActions[0].Trigger.DaysBeforeExpiry != 90 {
			t.Errorf("%s: lifetime action came back as %+v", name, policy.LifetimeActions)
		}
	}
}

// TestParsePolicyValidityOverrides pins _validators.py:914, where --validity
// wins over the months in the policy.
func TestParsePolicyValidityOverrides(t *testing.T) {
	policy, err := ParsePolicy(defaultPolicyJSON, 24)
	if err != nil {
		t.Fatal(err)
	}
	if got := *policy.X509CertificateProperties.ValidityInMonths; got != 24 {
		t.Errorf("validity is %d, want 24", got)
	}
}

func TestParsePolicyScaffoldParses(t *testing.T) {
	policy, err := ParsePolicy(scaffoldPolicyJSON, 0)
	if err != nil {
		t.Fatal(err)
	}
	san := policy.X509CertificateProperties.SubjectAlternativeNames
	if san == nil || len(san.DNSNames) != 2 || *san.Emails[0] != "hello@contoso.com" {
		t.Errorf("subject alternative names came back as %+v", san)
	}
	if len(policy.X509CertificateProperties.EnhancedKeyUsage) != 1 {
		t.Errorf("ekus came back as %+v", policy.X509CertificateProperties.EnhancedKeyUsage)
	}
}

func TestParsePolicyReadsAtFile(t *testing.T) {
	path := t.TempDir() + "/policy.json"
	if err := os.WriteFile(path, []byte(defaultPolicyJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	policy, err := ParsePolicy("@"+path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if policy.KeyProperties == nil {
		t.Error("the @file form lost the key properties")
	}
}

func TestParsePolicyRejectsGarbage(t *testing.T) {
	if _, err := ParsePolicy("not json", 0); err == nil {
		t.Error("expected an error for a non-JSON policy")
	}
}

func TestSplitCertificateChain(t *testing.T) {
	one := encodeCertificatePEM([]byte("first certificate body"))
	two := encodeCertificatePEM([]byte("second certificate body"))
	chain, err := splitCertificateChain(append(one, two...))
	if err != nil {
		t.Fatal(err)
	}
	if len(chain) != 2 || string(chain[0]) != "first certificate body" {
		t.Errorf("chain came back as %q", chain)
	}
	if _, err := splitCertificateChain([]byte("no certificate here")); err == nil {
		t.Error("expected an error for a file with no certificate")
	}
}

// TestEncodeCertificatePEM checks the wrapping: 76 characters per line, as
// Python's base64.encodebytes writes it.
func TestEncodeCertificatePEM(t *testing.T) {
	body := make([]byte, 200)
	encoded := string(encodeCertificatePEM(body))
	lines := strings.Split(strings.TrimSuffix(encoded, "\n"), "\n")
	if lines[0] != "-----BEGIN CERTIFICATE-----" || lines[len(lines)-1] != "-----END CERTIFICATE-----" {
		t.Fatalf("got %q", encoded)
	}
	for _, line := range lines[1 : len(lines)-1] {
		if len(line) > 76 {
			t.Errorf("line of %d characters: %q", len(line), line)
		}
	}
	joined := strings.Join(lines[1:len(lines)-1], "")
	decoded, err := base64.StdEncoding.DecodeString(joined)
	if err != nil || len(decoded) != 200 {
		t.Errorf("the body did not round-trip: %v", err)
	}
}

func TestParseCertTags(t *testing.T) {
	tags := parseCertTags([]string{"env=prod", "bare"})
	if *tags["env"] != "prod" || *tags["bare"] != "" {
		t.Errorf("got %v", tags)
	}
	if parseCertTags(nil) != nil {
		t.Error("parseCertTags(nil) should be nil")
	}
}
