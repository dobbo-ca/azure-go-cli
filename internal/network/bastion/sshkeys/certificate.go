package sshkeys

import (
  "fmt"
  "os"
  "path/filepath"
  "strings"

  "github.com/cdobbyn/azure-go-cli/pkg/logger"
  "golang.org/x/crypto/ssh"
)

// WriteCertificate writes an SSH certificate to a file
func WriteCertificate(certificateData, publicKeyPath string) (string, error) {
  // Determine certificate path (public key path + "-aadcert.pub")
  certPath := publicKeyPath + "-aadcert.pub"

  logger.Debug("Writing SSH certificate to: %s", certPath)

  // Azure returns just the base64-encoded certificate blob
  // OpenSSH expects format: <type> <base64-cert> [comment]
  // If the certificate doesn't start with a type, add it
  certData := certificateData
  if !strings.HasPrefix(certData, "ssh-") {
    // Add the certificate type prefix
    // The type is embedded in the certificate itself, but we need it in the file format
    certData = "ssh-rsa-cert-v01@openssh.com " + certificateData
    logger.Debug("Added certificate type prefix to match OpenSSH format")
  }

  // Write certificate to file
  if err := os.WriteFile(certPath, []byte(certData+"\n"), publicKeyPerm); err != nil {
    return "", fmt.Errorf("failed to write certificate: %w", err)
  }

  return certPath, nil
}

// ParseCertificate parses an SSH certificate and extracts its principals
func ParseCertificate(certPath string) (*Certificate, error) {
  logger.Debug("Parsing SSH certificate from: %s", certPath)

  // Read certificate file
  certData, err := os.ReadFile(certPath)
  if err != nil {
    return nil, fmt.Errorf("failed to read certificate: %w", err)
  }

  // Parse SSH certificate format
  // Format: <type> <base64-cert> [comment]
  fields := strings.Fields(string(certData))
  if len(fields) < 2 {
    return nil, fmt.Errorf("invalid certificate format")
  }

  // Parse public key (which should be a certificate)
  publicKey, _, _, _, err := ssh.ParseAuthorizedKey(certData)
  if err != nil {
    return nil, fmt.Errorf("failed to parse certificate: %w", err)
  }

  // Check if it's actually a certificate
  cert, ok := publicKey.(*ssh.Certificate)
  if !ok {
    return nil, fmt.Errorf("public key is not a certificate")
  }

  // Extract principals (valid usernames)
  if len(cert.ValidPrincipals) == 0 {
    return nil, fmt.Errorf("certificate has no valid principals")
  }

  logger.Debug("Certificate principals: %v", cert.ValidPrincipals)
  logger.Debug("Certificate valid from %d to %d", cert.ValidAfter, cert.ValidBefore)

  return &Certificate{
    CertPath:    certPath,
    Principals:  cert.ValidPrincipals,
    ValidAfter:  cert.ValidAfter,
    ValidBefore: cert.ValidBefore,
  }, nil
}

// GetPrimaryPrincipal returns the primary (first) principal from the certificate
// This is the username to use for SSH authentication
func (c *Certificate) GetPrimaryPrincipal() string {
  if len(c.Principals) > 0 {
    return strings.ToLower(c.Principals[0])
  }
  return ""
}

// CleanupKeyFiles removes temporary key and certificate files
func CleanupKeyFiles(keysFolder string) {
  if keysFolder == "" {
    return
  }

  // Check if this is a temporary directory (contains "aadsshcert")
  if !strings.Contains(filepath.Base(keysFolder), "aadsshcert") {
    logger.Debug("Skipping cleanup of non-temporary folder: %s", keysFolder)
    return
  }

  logger.Debug("Cleaning up temporary keys folder: %s", keysFolder)
  if err := os.RemoveAll(keysFolder); err != nil {
    logger.Debug("Failed to cleanup keys folder: %v", err)
  }
}
