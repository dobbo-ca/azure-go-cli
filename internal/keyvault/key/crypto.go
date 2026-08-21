package key

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// CryptoOptions carries the flags shared by encrypt, decrypt, sign and verify.
type CryptoOptions struct {
	VaultName string
	Name      string
	Version   string
	Algorithm string
	Value     string
	DataType  string
	IV        string
	AAD       string
	Tag       string
	Digest    string
	Signature string
}

// decodeValue reads --value the way validate_encryption does
// (_validators.py:719): base64 by default, raw bytes when --data-type is
// plaintext.
func decodeValue(value, dataType string) ([]byte, error) {
	if dataType == "plaintext" {
		return []byte(value), nil
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("failed to decode --value as base64: %w", err)
	}
	return decoded, nil
}

// decodeHex reads --iv, --aad and --tag, which azure-cli passes through
// binascii.unhexlify (custom.py:1318).
func decodeHex(value, flag string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("failed to decode --%s as hex: %w", flag, err)
	}
	return decoded, nil
}

func hexOrNil(b []byte) *string {
	if len(b) == 0 {
		return nil
	}
	return to.Ptr(hex.EncodeToString(b))
}

func kidOrNil(id *azkeys.ID) *string {
	if id == nil {
		return nil
	}
	return to.Ptr(string(*id))
}

// encryptOutput mirrors transform_key_encryption_output
// (_transformers.py:52): a base64 result, and hex for the three byte fields.
type encryptOutput struct {
	Kid       *string `json:"kid"`
	Result    string  `json:"result"`
	Algorithm string  `json:"algorithm"`
	IV        *string `json:"iv"`
	Tag       *string `json:"tag"`
	AAD       *string `json:"aad"`
}

// decryptOutput mirrors transform_key_decryption_output
// (_transformers.py:70). The result is base64 unless --data-type is plaintext.
type decryptOutput struct {
	Kid       *string `json:"kid"`
	Result    string  `json:"result"`
	Algorithm string  `json:"algorithm"`
}

func Encrypt(ctx context.Context, cmd *cobra.Command, opts CryptoOptions) error {
	value, err := decodeValue(opts.Value, opts.DataType)
	if err != nil {
		return err
	}
	iv, err := decodeHex(opts.IV, "iv")
	if err != nil {
		return err
	}
	aad, err := decodeHex(opts.AAD, "aad")
	if err != nil {
		return err
	}
	client, err := keyClient(opts.VaultName)
	if err != nil {
		return err
	}
	params := azkeys.KeyOperationParameters{
		Algorithm:                   to.Ptr(azkeys.EncryptionAlgorithm(opts.Algorithm)),
		Value:                       value,
		IV:                          iv,
		AdditionalAuthenticatedData: aad,
	}
	resp, err := client.Encrypt(ctx, opts.Name, opts.Version, params, nil)
	if err != nil {
		return fmt.Errorf("failed to encrypt: %w", err)
	}
	return output.PrintJSON(cmd, encryptOutput{
		Kid:       kidOrNil(resp.KID),
		Result:    base64.StdEncoding.EncodeToString(resp.Result),
		Algorithm: opts.Algorithm,
		IV:        hexOrNil(resp.IV),
		Tag:       hexOrNil(resp.AuthenticationTag),
		AAD:       hexOrNil(resp.AdditionalAuthenticatedData),
	})
}

func Decrypt(ctx context.Context, cmd *cobra.Command, opts CryptoOptions) error {
	// validate_decryption (_validators.py:727) always base64-decodes --value,
	// whatever --data-type says; data_type only shapes the output.
	value, err := base64.StdEncoding.DecodeString(opts.Value)
	if err != nil {
		return fmt.Errorf("failed to decode --value as base64: %w", err)
	}
	iv, err := decodeHex(opts.IV, "iv")
	if err != nil {
		return err
	}
	aad, err := decodeHex(opts.AAD, "aad")
	if err != nil {
		return err
	}
	tag, err := decodeHex(opts.Tag, "tag")
	if err != nil {
		return err
	}
	client, err := keyClient(opts.VaultName)
	if err != nil {
		return err
	}
	params := azkeys.KeyOperationParameters{
		Algorithm:                   to.Ptr(azkeys.EncryptionAlgorithm(opts.Algorithm)),
		Value:                       value,
		IV:                          iv,
		AdditionalAuthenticatedData: aad,
		AuthenticationTag:           tag,
	}
	resp, err := client.Decrypt(ctx, opts.Name, opts.Version, params, nil)
	if err != nil {
		return fmt.Errorf("failed to decrypt: %w", err)
	}
	result := string(resp.Result)
	if opts.DataType != "plaintext" {
		result = base64.StdEncoding.EncodeToString(resp.Result)
	}
	return output.PrintJSON(cmd, decryptOutput{
		Kid:       kidOrNil(resp.KID),
		Result:    result,
		Algorithm: opts.Algorithm,
	})
}

func Sign(ctx context.Context, cmd *cobra.Command, opts CryptoOptions) error {
	digest, err := base64.StdEncoding.DecodeString(opts.Digest)
	if err != nil {
		return fmt.Errorf("failed to decode --digest as base64: %w", err)
	}
	client, err := keyClient(opts.VaultName)
	if err != nil {
		return err
	}
	params := azkeys.SignParameters{
		Algorithm: to.Ptr(azkeys.SignatureAlgorithm(opts.Algorithm)),
		Value:     digest,
	}
	resp, err := client.Sign(ctx, opts.Name, opts.Version, params, nil)
	if err != nil {
		return fmt.Errorf("failed to sign: %w", err)
	}
	return output.PrintJSON(cmd, resp.KeyOperationResult)
}

func Verify(ctx context.Context, cmd *cobra.Command, opts CryptoOptions) error {
	digest, err := base64.StdEncoding.DecodeString(opts.Digest)
	if err != nil {
		return fmt.Errorf("failed to decode --digest as base64: %w", err)
	}
	signature, err := base64.StdEncoding.DecodeString(opts.Signature)
	if err != nil {
		return fmt.Errorf("failed to decode --signature as base64: %w", err)
	}
	client, err := keyClient(opts.VaultName)
	if err != nil {
		return err
	}
	params := azkeys.VerifyParameters{
		Algorithm: to.Ptr(azkeys.SignatureAlgorithm(opts.Algorithm)),
		Digest:    digest,
		Signature: signature,
	}
	resp, err := client.Verify(ctx, opts.Name, opts.Version, params, nil)
	if err != nil {
		return fmt.Errorf("failed to verify: %w", err)
	}
	return output.PrintJSON(cmd, resp.KeyVerifyResult)
}
