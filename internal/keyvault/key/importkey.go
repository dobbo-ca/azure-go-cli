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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// ImportOptions carries the flags of az keyvault key import. The field set
// follows import_key (custom.py:1455).
type ImportOptions struct {
	VaultName   string
	Name        string
	Protection  string
	Ops         []string
	Disabled    bool
	Expires     string
	NotBefore   string
	Tags        []string
	PemFile     string
	PemString   string
	PemPassword string
	ByokFile    string
	ByokString  string
	Kty         string
	Curve       string
}

// jwkCurves maps Go curves onto the JWK names azure-cli writes on import
// (custom.py:1438).
var jwkCurves = map[elliptic.Curve]string{
	elliptic.P256(): "P-256",
	elliptic.P384(): "P-384",
	elliptic.P521(): "P-521",
}

// pemToJWK fills a JWK from a PEM private key, as _private_rsa_key_to_jwk and
// _private_ec_key_to_jwk do (custom.py:1425-1452).
func pemToJWK(data []byte, password string) (*azkeys.JSONWebKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("import failed: could not deserialize key data")
	}
	if password != "" {
		return nil, fmt.Errorf("import failed: encrypted PEM files are not supported")
	}

	var parsed any
	var err error
	switch block.Type {
	case "RSA PRIVATE KEY":
		parsed, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		parsed, err = x509.ParseECPrivateKey(block.Bytes)
	default:
		parsed, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	}
	if err != nil {
		return nil, fmt.Errorf("import failed: %w", err)
	}

	switch pkey := parsed.(type) {
	case *rsa.PrivateKey:
		pkey.Precompute()
		return &azkeys.JSONWebKey{
			Kty: to.Ptr(azkeys.KeyType("RSA")),
			N:   pkey.N.Bytes(),
			E:   big.NewInt(int64(pkey.E)).Bytes(),
			D:   pkey.D.Bytes(),
			P:   pkey.Primes[0].Bytes(),
			Q:   pkey.Primes[1].Bytes(),
			DP:  pkey.Precomputed.Dp.Bytes(),
			DQ:  pkey.Precomputed.Dq.Bytes(),
			QI:  pkey.Precomputed.Qinv.Bytes(),
		}, nil
	case *ecdsa.PrivateKey:
		crv, ok := jwkCurves[pkey.Curve]
		if !ok {
			return nil, fmt.Errorf("import failed: unsupported curve, %s", pkey.Curve.Params().Name)
		}
		return &azkeys.JSONWebKey{
			Kty: to.Ptr(azkeys.KeyType("EC")),
			Crv: to.Ptr(azkeys.CurveName(crv)),
			X:   pkey.X.Bytes(),
			Y:   pkey.Y.Bytes(),
			D:   pkey.D.Bytes(),
		}, nil
	default:
		return nil, fmt.Errorf("import failed: unsupported key type, %T", parsed)
	}
}

func Import(ctx context.Context, cmd *cobra.Command, opts ImportOptions) error {
	jwk := &azkeys.JSONWebKey{}
	for _, op := range opts.Ops {
		jwk.KeyOps = append(jwk.KeyOps, to.Ptr(azkeys.KeyOperation(op)))
	}

	switch {
	case opts.PemFile != "" || opts.PemString != "":
		data := []byte(opts.PemString)
		if opts.PemFile != "" {
			var err error
			data, err = os.ReadFile(opts.PemFile)
			if err != nil {
				return fmt.Errorf("failed to read '%s': %w", opts.PemFile, err)
			}
		}
		parsed, err := pemToJWK(data, opts.PemPassword)
		if err != nil {
			return err
		}
		parsed.KeyOps = jwk.KeyOps
		jwk = parsed
	case opts.ByokFile != "" || opts.ByokString != "":
		data := []byte(opts.ByokString)
		if opts.ByokFile != "" {
			var err error
			data, err = os.ReadFile(opts.ByokFile)
			if err != nil {
				return fmt.Errorf("failed to read '%s': %w", opts.ByokFile, err)
			}
		}
		jwk.Kty = to.Ptr(azkeys.KeyType(opts.Kty + "-HSM"))
		jwk.T = data
		if opts.Curve != "" {
			jwk.Crv = to.Ptr(azkeys.CurveName(opts.Curve))
		}
	default:
		return fmt.Errorf("one of --pem-file, --pem-string, --byok-file or --byok-string is required")
	}

	params := azkeys.ImportKeyParameters{
		Key:           jwk,
		HSM:           to.Ptr(opts.Protection == "hsm"),
		Tags:          parseKeyTags(opts.Tags),
		KeyAttributes: &azkeys.KeyAttributes{Enabled: to.Ptr(!opts.Disabled)},
	}
	if opts.Expires != "" {
		expires, err := parseKeyTime(opts.Expires)
		if err != nil {
			return err
		}
		params.KeyAttributes.Expires = &expires
	}
	if opts.NotBefore != "" {
		notBefore, err := parseKeyTime(opts.NotBefore)
		if err != nil {
			return err
		}
		params.KeyAttributes.NotBefore = &notBefore
	}

	client, err := keyClient(opts.VaultName)
	if err != nil {
		return err
	}
	resp, err := client.ImportKey(ctx, opts.Name, params, nil)
	if err != nil {
		return fmt.Errorf("failed to import key: %w", err)
	}
	return output.PrintJSON(cmd, resp.KeyBundle)
}
