package certificate

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
)

// defaultPolicyJSON is the policy _default_certificate_profile builds
// (custom.py:44), rendered in the camelCase form azure-cli prints. The Go SDK
// marshals CertificatePolicy with the wire names instead, so the shape is
// written out here rather than taken from the SDK model.
const defaultPolicyJSON = `{
  "issuerParameters": {
    "name": "Self"
  },
  "keyProperties": {
    "exportable": true,
    "keySize": 2048,
    "keyType": "RSA",
    "reuseKey": true
  },
  "lifetimeActions": [
    {
      "action": {
        "actionType": "AutoRenew"
      },
      "trigger": {
        "daysBeforeExpiry": 90
      }
    }
  ],
  "secretProperties": {
    "contentType": "application/x-pkcs12"
  },
  "x509CertificateProperties": {
    "keyUsage": [
      "cRLSign",
      "dataEncipherment",
      "digitalSignature",
      "keyEncipherment",
      "keyAgreement",
      "keyCertSign"
    ],
    "subject": "CN=CLIGetDefaultPolicy",
    "validityInMonths": 12
  }
}`

// scaffoldPolicyJSON is the policy _scaffold_certificate_profile builds
// (custom.py:106): the same shape with every optional field filled in with a
// placeholder, for a user to edit.
const scaffoldPolicyJSON = `{
  "issuerParameters": {
    "certificateType": "(optional) DigiCert, GlobalSign or WoSign",
    "name": "Unknown, Self, or {IssuerName}"
  },
  "keyProperties": {
    "exportable": true,
    "keySize": 2048,
    "keyType": "(optional) RSA or RSA-HSM (default RSA)",
    "reuseKey": true
  },
  "lifetimeActions": [
    {
      "action": {
        "actionType": "AutoRenew"
      },
      "trigger": {
        "daysBeforeExpiry": 90
      }
    }
  ],
  "secretProperties": {
    "contentType": "application/x-pkcs12 or application/x-pem-file"
  },
  "x509CertificateProperties": {
    "ekus": [
      "1.3.6.1.5.5.7.3.1"
    ],
    "keyUsage": [
      "cRLSign",
      "dataEncipherment",
      "digitalSignature",
      "keyEncipherment",
      "keyAgreement",
      "keyCertSign"
    ],
    "subject": "C=US, ST=WA, L=Redmond, O=Contoso, OU=Contoso HR, CN=www.contoso.com",
    "subjectAlternativeNames": {
      "dnsNames": [
        "hr.contoso.com",
        "m.contoso.com"
      ],
      "emails": [
        "hello@contoso.com"
      ],
      "upns": []
    },
    "validityInMonths": 24
  }
}`

// rawCertificatePolicy is the parsed --policy document. azure-cli snake-cases
// every key through get_json_object before build_certificate_policy reads it
// (_validators.py:893), so camelCase, snake_case and the REST wire names must
// all be accepted. normalizePolicyKeys folds every key to lowercase without
// underscores, which is why these tags carry neither.
type rawCertificatePolicy struct {
	IssuerParameters struct {
		Name                    string `json:"name"`
		CertificateType         string `json:"certificatetype"`
		CertificateTransparency *bool  `json:"certificatetransparency"`
	} `json:"issuerparameters"`
	KeyProperties struct {
		Exportable *bool  `json:"exportable"`
		KeyType    string `json:"keytype"`
		Kty        string `json:"kty"`
		KeySize    *int32 `json:"keysize"`
		ReuseKey   *bool  `json:"reusekey"`
		Curve      string `json:"curve"`
		CurveName  string `json:"curvename"`
	} `json:"keyproperties"`
	SecretProperties struct {
		ContentType string `json:"contenttype"`
	} `json:"secretproperties"`
	X509CertificateProperties struct {
		Subject                 string   `json:"subject"`
		KeyUsage                []string `json:"keyusage"`
		EKUs                    []string `json:"ekus"`
		EnhancedKeyUsage        []string `json:"enhancedkeyusage"`
		ValidityInMonths        *int32   `json:"validityinmonths"`
		SubjectAlternativeNames struct {
			Emails   []string `json:"emails"`
			DNSNames []string `json:"dnsnames"`
			UPNs     []string `json:"upns"`
		} `json:"subjectalternativenames"`
	} `json:"x509certificateproperties"`
	LifetimeActions []struct {
		Action struct {
			ActionType string `json:"actiontype"`
		} `json:"action"`
		Trigger struct {
			DaysBeforeExpiry   *int32 `json:"daysbeforeexpiry"`
			LifetimePercentage *int32 `json:"lifetimepercentage"`
		} `json:"trigger"`
	} `json:"lifetimeactions"`
}

// normalizePolicyKeys lowercases every object key and drops underscores.
func normalizePolicyKeys(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[strings.ReplaceAll(strings.ToLower(k), "_", "")] = normalizePolicyKeys(val)
		}
		return out
	case []any:
		for i, val := range t {
			t[i] = normalizePolicyKeys(val)
		}
		return t
	default:
		return v
	}
}

func ptrList(values []string) []*string {
	if len(values) == 0 {
		return nil
	}
	out := make([]*string, 0, len(values))
	for _, v := range values {
		out = append(out, to.Ptr(v))
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ParsePolicy turns the --policy argument into an SDK policy. azure-cli takes
// a JSON document, or @file to read one from disk (_params.py:722).
func ParsePolicy(value string, validity int32) (*azcertificates.CertificatePolicy, error) {
	if value == "" && validity == 0 {
		return nil, nil
	}

	raw := rawCertificatePolicy{}
	if value != "" {
		data := []byte(value)
		if strings.HasPrefix(value, "@") {
			var err error
			data, err = os.ReadFile(strings.TrimPrefix(value, "@"))
			if err != nil {
				return nil, fmt.Errorf("failed to read the policy file: %w", err)
			}
		}
		var doc any
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("incorrect usage: policy should be a JSON encoded string or can use @{file} to load from a file(e.g.@my_policy.json)")
		}
		normalized, err := json.Marshal(normalizePolicyKeys(doc))
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(normalized, &raw); err != nil {
			return nil, fmt.Errorf("incorrect usage: policy should be a JSON encoded string or can use @{file} to load from a file(e.g.@my_policy.json)")
		}
	}

	policy := &azcertificates.CertificatePolicy{}
	if raw.IssuerParameters.Name != "" || raw.IssuerParameters.CertificateType != "" || raw.IssuerParameters.CertificateTransparency != nil {
		policy.IssuerParameters = &azcertificates.IssuerParameters{
			CertificateTransparency: raw.IssuerParameters.CertificateTransparency,
		}
		if raw.IssuerParameters.Name != "" {
			policy.IssuerParameters.Name = to.Ptr(raw.IssuerParameters.Name)
		}
		if raw.IssuerParameters.CertificateType != "" {
			policy.IssuerParameters.CertificateType = to.Ptr(raw.IssuerParameters.CertificateType)
		}
	}

	kty := firstNonEmpty(raw.KeyProperties.KeyType, raw.KeyProperties.Kty)
	curve := firstNonEmpty(raw.KeyProperties.Curve, raw.KeyProperties.CurveName)
	if kty != "" || curve != "" || raw.KeyProperties.Exportable != nil || raw.KeyProperties.KeySize != nil || raw.KeyProperties.ReuseKey != nil {
		policy.KeyProperties = &azcertificates.KeyProperties{
			Exportable: raw.KeyProperties.Exportable,
			KeySize:    raw.KeyProperties.KeySize,
			ReuseKey:   raw.KeyProperties.ReuseKey,
		}
		if kty != "" {
			policy.KeyProperties.KeyType = to.Ptr(azcertificates.KeyType(kty))
		}
		if curve != "" {
			policy.KeyProperties.Curve = to.Ptr(azcertificates.CurveName(curve))
		}
	}

	if raw.SecretProperties.ContentType != "" {
		policy.SecretProperties = &azcertificates.SecretProperties{
			ContentType: to.Ptr(raw.SecretProperties.ContentType),
		}
	}

	x509 := &azcertificates.X509CertificateProperties{
		ValidityInMonths: raw.X509CertificateProperties.ValidityInMonths,
		EnhancedKeyUsage: ptrList(append(append([]string{}, raw.X509CertificateProperties.EKUs...), raw.X509CertificateProperties.EnhancedKeyUsage...)),
	}
	if raw.X509CertificateProperties.Subject != "" {
		x509.Subject = to.Ptr(raw.X509CertificateProperties.Subject)
	}
	for _, usage := range raw.X509CertificateProperties.KeyUsage {
		x509.KeyUsage = append(x509.KeyUsage, to.Ptr(azcertificates.KeyUsageType(usage)))
	}
	san := raw.X509CertificateProperties.SubjectAlternativeNames
	if len(san.Emails) > 0 || len(san.DNSNames) > 0 || len(san.UPNs) > 0 {
		x509.SubjectAlternativeNames = &azcertificates.SubjectAlternativeNames{
			Emails:             ptrList(san.Emails),
			DNSNames:           ptrList(san.DNSNames),
			UserPrincipalNames: ptrList(san.UPNs),
		}
	}
	// --validity overrides whatever the policy says (_validators.py:914).
	if validity > 0 {
		x509.ValidityInMonths = to.Ptr(validity)
	}
	if x509.Subject != nil || x509.ValidityInMonths != nil || len(x509.KeyUsage) > 0 ||
		len(x509.EnhancedKeyUsage) > 0 || x509.SubjectAlternativeNames != nil {
		policy.X509CertificateProperties = x509
	}

	for _, action := range raw.LifetimeActions {
		lifetime := &azcertificates.LifetimeAction{
			Trigger: &azcertificates.LifetimeActionTrigger{
				DaysBeforeExpiry:   action.Trigger.DaysBeforeExpiry,
				LifetimePercentage: action.Trigger.LifetimePercentage,
			},
		}
		if action.Action.ActionType != "" {
			lifetime.Action = &azcertificates.LifetimeActionType{
				ActionType: to.Ptr(azcertificates.CertificatePolicyAction(action.Action.ActionType)),
			}
		}
		policy.LifetimeActions = append(policy.LifetimeActions, lifetime)
	}

	return policy, nil
}
