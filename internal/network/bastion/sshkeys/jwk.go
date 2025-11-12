package sshkeys

import (
  "encoding/json"
  "fmt"
)

// CreateJWK creates a JSON Web Key from a key pair
func CreateJWK(keyPair *KeyPair) (*JWK, error) {
  keyID := GenerateKeyID(keyPair.Modulus, keyPair.Exponent)

  jwk := &JWK{
    KeyType:  "RSA",
    Modulus:  keyPair.Modulus,
    Exponent: keyPair.Exponent,
    KeyID:    keyID,
  }

  return jwk, nil
}

// CreateCertificateRequest creates the data structure for requesting an SSH certificate from AAD
func CreateCertificateRequest(keyPair *KeyPair) (*CertificateRequest, error) {
  // Create JWK
  jwk, err := CreateJWK(keyPair)
  if err != nil {
    return nil, fmt.Errorf("failed to create JWK: %w", err)
  }

  // Marshal JWK to JSON string
  jwkJSON, err := json.Marshal(jwk)
  if err != nil {
    return nil, fmt.Errorf("failed to marshal JWK: %w", err)
  }

  // Create certificate request
  certReq := &CertificateRequest{
    TokenType: "ssh-cert",
    ReqCnf:    string(jwkJSON),
    KeyID:     jwk.KeyID,
  }

  return certReq, nil
}

// MarshalJWK marshals a JWK to JSON
func (j *JWK) MarshalJSON() ([]byte, error) {
  return json.Marshal(map[string]string{
    "kty": j.KeyType,
    "n":   j.Modulus,
    "e":   j.Exponent,
    "kid": j.KeyID,
  })
}
