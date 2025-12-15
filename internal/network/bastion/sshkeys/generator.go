package sshkeys

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	"golang.org/x/crypto/ssh"
)

const (
	rsaKeySize     = 2048
	privateKeyPerm = 0600
	publicKeyPerm  = 0644
)

// GenerateKeyPair generates an RSA key pair for SSH authentication
// If keysFolder is empty, creates a temporary directory
func GenerateKeyPair(keysFolder string) (*KeyPair, error) {
	// Create keys folder if needed
	if keysFolder == "" {
		tmpDir, err := os.MkdirTemp("", "aadsshcert-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp directory: %w", err)
		}
		keysFolder = tmpDir
		logger.Debug("Created temporary keys folder: %s", keysFolder)
	} else {
		// Ensure directory exists
		if err := os.MkdirAll(keysFolder, 0755); err != nil {
			return nil, fmt.Errorf("failed to create keys folder: %w", err)
		}
	}

	// Generate RSA private key
	logger.Debug("Generating %d-bit RSA key pair...", rsaKeySize)
	privateKey, err := rsa.GenerateKey(rand.Reader, rsaKeySize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Save private key
	privateKeyPath := filepath.Join(keysFolder, "id_rsa")
	if err := savePrivateKey(privateKey, privateKeyPath); err != nil {
		return nil, fmt.Errorf("failed to save private key: %w", err)
	}
	logger.Debug("Saved private key to: %s", privateKeyPath)

	// Generate and save public key
	publicKeyPath := filepath.Join(keysFolder, "id_rsa.pub")
	if err := savePublicKey(&privateKey.PublicKey, publicKeyPath); err != nil {
		return nil, fmt.Errorf("failed to save public key: %w", err)
	}
	logger.Debug("Saved public key to: %s", publicKeyPath)

	// Parse public key to extract modulus and exponent for JWK
	modulus, exponent, err := extractPublicKeyComponents(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to extract public key components: %w", err)
	}

	return &KeyPair{
		PrivateKey:     privateKey,
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  publicKeyPath,
		Modulus:        modulus,
		Exponent:       exponent,
	}, nil
}

// LoadOrGenerateKeyPair loads an existing key pair or generates a new one
func LoadOrGenerateKeyPair(publicKeyPath, privateKeyPath, keysFolder string) (*KeyPair, error) {
	// If both paths provided, load existing keys
	if publicKeyPath != "" && privateKeyPath != "" {
		return LoadKeyPair(publicKeyPath, privateKeyPath)
	}

	// If public key provided but no private key, try to infer private key path
	if publicKeyPath != "" {
		inferredPrivateKeyPath := publicKeyPath
		if filepath.Ext(publicKeyPath) == ".pub" {
			inferredPrivateKeyPath = publicKeyPath[:len(publicKeyPath)-4]
		}

		// Check if private key exists
		if _, err := os.Stat(inferredPrivateKeyPath); err == nil {
			return LoadKeyPair(publicKeyPath, inferredPrivateKeyPath)
		}
	}

	// Generate new key pair
	return GenerateKeyPair(keysFolder)
}

// LoadKeyPair loads an existing SSH key pair from files
func LoadKeyPair(publicKeyPath, privateKeyPath string) (*KeyPair, error) {
	logger.Debug("Loading key pair from %s and %s", publicKeyPath, privateKeyPath)

	// Read private key
	privateKeyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	// Parse PEM block
	block, _ := pem.Decode(privateKeyData)
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block from private key")
	}

	// Parse RSA private key
	var privateKey *rsa.PrivateKey
	switch block.Type {
	case "RSA PRIVATE KEY":
		privateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
		}
		var ok bool
		privateKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	default:
		return nil, fmt.Errorf("unsupported private key type: %s", block.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Extract public key components
	modulus, exponent, err := extractPublicKeyComponents(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to extract public key components: %w", err)
	}

	return &KeyPair{
		PrivateKey:     privateKey,
		PrivateKeyPath: privateKeyPath,
		PublicKeyPath:  publicKeyPath,
		Modulus:        modulus,
		Exponent:       exponent,
	}, nil
}

// savePrivateKey saves an RSA private key to a file in PEM format
func savePrivateKey(key *rsa.PrivateKey, path string) error {
	// Encode private key to PKCS1 format
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(key)
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	// Write to file with restrictive permissions
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, privateKeyPerm)
	if err != nil {
		return err
	}
	defer file.Close()

	return pem.Encode(file, privateKeyPEM)
}

// savePublicKey saves an RSA public key to a file in SSH format
func savePublicKey(key *rsa.PublicKey, path string) error {
	// Convert to SSH public key format
	sshPublicKey, err := ssh.NewPublicKey(key)
	if err != nil {
		return fmt.Errorf("failed to create SSH public key: %w", err)
	}

	// Marshal to authorized_keys format
	publicKeyBytes := ssh.MarshalAuthorizedKey(sshPublicKey)

	// Write to file
	return os.WriteFile(path, publicKeyBytes, publicKeyPerm)
}
