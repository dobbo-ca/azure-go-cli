package key

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/spf13/cobra"
)

// curvesByName maps the JWK curve names azure-cli accepts on download
// (custom.py:1574) onto their Go equivalents.
var curvesByName = map[string]elliptic.Curve{
	"P-256": elliptic.P256(),
	"P-384": elliptic.P384(),
	"P-521": elliptic.P521(),
}

// publicKeyFromJWK rebuilds the public half of a key from its JWK, as
// _extract_rsa_public_key_from_jwk and _extract_ec_public_key_from_jwk do
// (custom.py:1559-1583).
func publicKeyFromJWK(jwk *azkeys.JSONWebKey) (any, error) {
	if jwk == nil || jwk.Kty == nil {
		return nil, fmt.Errorf("the key has no key material")
	}
	kty := string(*jwk.Kty)
	switch kty {
	case "RSA", "RSA-HSM":
		if len(jwk.N) == 0 || len(jwk.E) == 0 {
			return nil, fmt.Errorf("invalid RSA key: missing properties(n, e)")
		}
		return &rsa.PublicKey{
			N: new(big.Int).SetBytes(jwk.N),
			E: int(new(big.Int).SetBytes(jwk.E).Int64()),
		}, nil
	case "EC", "EC-HSM":
		if len(jwk.X) == 0 || len(jwk.Y) == 0 || jwk.Crv == nil {
			return nil, fmt.Errorf("invalid EC key: missing properties(x, y, crv)")
		}
		curve, ok := curvesByName[string(*jwk.Crv)]
		if !ok {
			// azure-cli also accepts SECP256K1, which Go's standard
			// library does not implement.
			return nil, fmt.Errorf("unsupported curve: %s", *jwk.Crv)
		}
		return &ecdsa.PublicKey{
			Curve: curve,
			X:     new(big.Int).SetBytes(jwk.X),
			Y:     new(big.Int).SetBytes(jwk.Y),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported key type: %s. (Supported key types: RSA, RSA-HSM, EC, EC-HSM)", kty)
	}
}

// encodePublicKey writes the SubjectPublicKeyInfo form, as _export_public_key
// does (custom.py:1586).
func encodePublicKey(pub any, encoding string) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the public key: %w", err)
	}
	switch strings.ToUpper(encoding) {
	case "DER":
		return der, nil
	case "PEM":
		return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
	default:
		return nil, fmt.Errorf("unsupported encoding: %s. (Supported encodings: DER, PEM)", encoding)
	}
}

func Download(ctx context.Context, _ *cobra.Command, vaultName, name, version, filePath, encoding string) error {
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("file or directory named '%s' already exists", filePath)
	}
	client, err := keyClient(vaultName)
	if err != nil {
		return err
	}
	resp, err := client.GetKey(ctx, name, version, nil)
	if err != nil {
		return fmt.Errorf("failed to get key: %w", err)
	}
	pub, err := publicKeyFromJWK(resp.Key)
	if err != nil {
		return err
	}
	encoded, err := encodePublicKey(pub, encoding)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filePath, encoded, 0o600); err != nil {
		return fmt.Errorf("failed to write '%s': %w", filePath, err)
	}
	// download_key (custom.py:1602) returns nothing on success.
	return nil
}
