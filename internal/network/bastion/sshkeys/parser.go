package sshkeys

import (
  "crypto/rsa"
  "crypto/sha256"
  "encoding/base64"
  "fmt"
  "math/big"
)

// extractPublicKeyComponents extracts the modulus and exponent from an RSA public key
// and encodes them in base64 URL format (without padding) for JWK
func extractPublicKeyComponents(publicKey *rsa.PublicKey) (modulus, exponent string, err error) {
  // Get modulus as bytes
  modulusBytes := publicKey.N.Bytes()

  // Get exponent as bytes (converting from int to big.Int to bytes)
  expBigInt := big.NewInt(int64(publicKey.E))
  exponentBytes := expBigInt.Bytes()

  // Encode to base64 URL format without padding
  modulus = base64.RawURLEncoding.EncodeToString(modulusBytes)
  exponent = base64.RawURLEncoding.EncodeToString(exponentBytes)

  return modulus, exponent, nil
}

// GenerateKeyID creates a key ID by hashing the modulus and exponent
// This matches Azure's key ID generation algorithm
func GenerateKeyID(modulus, exponent string) string {
  h := sha256.New()
  h.Write([]byte(modulus))
  h.Write([]byte(exponent))
  return fmt.Sprintf("%x", h.Sum(nil))
}
