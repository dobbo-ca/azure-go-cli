package sshkeys

import (
	"crypto/rsa"
)

// KeyPair represents an SSH RSA key pair
type KeyPair struct {
	PrivateKey     *rsa.PrivateKey
	PrivateKeyPath string
	PublicKeyPath  string
	Modulus        string // Base64 URL encoded
	Exponent       string // Base64 URL encoded
}

// JWK represents a JSON Web Key for SSH certificate requests
type JWK struct {
	KeyType  string `json:"kty"` // "RSA"
	Modulus  string `json:"n"`   // Base64 URL encoded modulus
	Exponent string `json:"e"`   // Base64 URL encoded exponent
	KeyID    string `json:"kid"` // SHA256 hash of modulus + exponent
}

// CertificateRequest represents the data structure for AAD SSH cert requests
type CertificateRequest struct {
	TokenType string `json:"token_type"` // "ssh-cert"
	ReqCnf    string `json:"req_cnf"`    // JWK as JSON string
	KeyID     string `json:"key_id"`     // Key ID from JWK
}

// Certificate represents an SSH certificate with its metadata
type Certificate struct {
	CertPath    string
	Principals  []string // Valid usernames from certificate
	ValidAfter  uint64   // Unix timestamp
	ValidBefore uint64   // Unix timestamp
}
