package ssh

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cdobbyn/azure-go-cli/internal/network/bastion"
	"github.com/cdobbyn/azure-go-cli/internal/network/bastion/sshkeys"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
	gossh "golang.org/x/crypto/ssh"
)

// checkOrCreatePublicPrivateFiles is _check_or_create_public_private_files.
// ssh-keygen makes the keys, as upstream does: it gives the private key
// the Windows ACL OpenSSH requires and asks before overwriting a key.
func checkOrCreatePublicPrivateFiles(pub, priv, credentialsFolder, clientFolder string) (string, string, bool, error) {
	deleteKeys := false
	if pub == "" && priv == "" {
		deleteKeys = true
		if credentialsFolder == "" {
			dir, err := os.MkdirTemp("", "aadsshcert*")
			if err != nil {
				return "", "", false, err
			}
			credentialsFolder = dir
		} else if !isDir(credentialsFolder) {
			if err := os.MkdirAll(credentialsFolder, 0o755); err != nil {
				return "", "", false, err
			}
		}
		pub = filepath.Join(credentialsFolder, "id_rsa.pub")
		priv = filepath.Join(credentialsFolder, "id_rsa")
		if err := createSSHKeyfile(priv, clientFolder); err != nil {
			return "", "", false, err
		}
	}

	if pub == "" {
		pub = priv + ".pub"
	}
	if !isFile(pub) {
		return "", "", false, fmt.Errorf("Public key file %s not found", pub)
	}
	// No private key is fine: it may be in ssh-agent or a hardware token.
	if priv != "" && !isFile(priv) {
		return "", "", false, fmt.Errorf("Private key file %s not found", priv)
	}
	if priv == "" && strings.HasSuffix(pub, ".pub") && isFile(strings.TrimSuffix(pub, ".pub")) {
		priv = strings.TrimSuffix(pub, ".pub")
	}
	return pub, priv, deleteKeys, nil
}

// createSSHKeyfile is ssh_utils.create_ssh_keyfile; like upstream it ignores
// ssh-keygen's exit code and lets the key file checks report failure.
func createSSHKeyfile(priv, clientFolder string) error {
	keygen, err := getSSHClientPath("ssh-keygen", clientFolder)
	if err != nil {
		return err
	}
	args := []string{"-f", priv, "-t", "rsa", "-q", "-N", ""}
	logger.Debug("Running ssh-keygen command %s %s", keygen, strings.Join(args, " "))
	cmd := exec.Command(keygen, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil
		}
		return withRecommendation(fmt.Sprintf("Failed to create ssh key file with error: %s.", err), recommendationSSHClientNotFound)
	}
	return nil
}

// prepareJWKData is _prepare_jwk_data, byte for byte: padded base64 of the
// raw mpint fields and json.dumps' default separators.
func prepareJWKData(pub string) (*sshkeys.CertificateRequest, error) {
	if !isFile(pub) {
		return nil, fmt.Errorf("Public key file '%s' was not found", pub)
	}
	text, err := os.ReadFile(pub)
	if err != nil {
		return nil, err
	}
	modulus, exponent, err := parseRSAPublicKey(string(text))
	if err != nil {
		return nil, fmt.Errorf("Could not parse public key. Error: %s", err)
	}
	sum := sha256.Sum256([]byte(modulus + exponent))
	keyID := hex.EncodeToString(sum[:])
	jwk := fmt.Sprintf(`{"kty": "RSA", "n": "%s", "e": "%s", "kid": "%s"}`, modulus, exponent, keyID)
	return &sshkeys.CertificateRequest{TokenType: "ssh-cert", ReqCnf: jwk, KeyID: keyID}, nil
}

// parseRSAPublicKey is rsa_parser.RSAParser.parse.
func parseRSAPublicKey(text string) (modulus, exponent string, err error) {
	parts := strings.Split(text, " ")
	if len(parts) < 2 {
		return "", "", errors.New("Incorrectly formatted public key. Key must be format '<algorithm> <base64_key>'")
	}
	if parts[0] != "ssh-rsa" {
		return "", "", fmt.Errorf("Public key is not ssh-rsa algorithm (%s)", parts[0])
	}
	keyBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(parts[1]))
	if err != nil {
		return "", "", err
	}
	var fields [][]byte
	for read := 0; read < len(keyBytes); {
		if read+4 > len(keyBytes) {
			return "", "", errors.New("unpack requires a buffer of 4 bytes")
		}
		length := int(binary.BigEndian.Uint32(keyBytes[read : read+4]))
		read += 4
		end := min(read+length, len(keyBytes))
		fields = append(fields, keyBytes[read:end])
		read += length
	}
	if len(fields) < 3 {
		return "", "", errors.New("Incorrectly encoded public key. Encoded key must be base64 encoded <algorithm><exponent><modulus>")
	}
	if string(fields[0]) != "ssh-rsa" {
		return "", "", fmt.Errorf("Encoded public key is not ssh-rsa algorithm (%s)", fields[0])
	}
	return base64.URLEncoding.EncodeToString(fields[2]), base64.URLEncoding.EncodeToString(fields[1]), nil
}

// getAndWriteCertificate is _get_and_write_certificate. It returns the
// certificate path and the lowercased first principal.
func getAndWriteCertificate(ctx context.Context, pub, certFile string) (string, string, error) {
	certReq, err := prepareJWKData(pub)
	if err != nil {
		return "", "", err
	}
	// The cert comes from the default subscription's tenant, as upstream.
	cred, err := azure.GetCredential()
	if err != nil {
		return "", "", err
	}
	token, err := bastion.RequestAADSSHCertificate(ctx, cred, aadSSHLoginForLinuxAppID+"/.default", certReq, "azurecloud")
	if err != nil {
		return "", "", err
	}
	if certFile == "" {
		certFile = pub + "-aadcert.pub"
	}
	logger.Debug("Generating certificate %s", certFile)
	if err := os.WriteFile(certFile, []byte("ssh-rsa-cert-v01@openssh.com "+token), 0o644); err != nil {
		return "", "", err
	}
	if err := os.Chmod(certFile, 0o644); err != nil {
		return "", "", err
	}
	cert, err := readCertificate(certFile)
	if err != nil {
		return "", "", err
	}
	if len(cert.ValidPrincipals) == 0 {
		return "", "", fmt.Errorf("certificate %s has no principals", certFile)
	}
	return certFile, strings.ToLower(cert.ValidPrincipals[0]), nil
}

func readCertificate(path string) (*gossh.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, _, _, _, err := gossh.ParseAuthorizedKey(data)
	if err != nil {
		return nil, err
	}
	cert, ok := key.(*gossh.Certificate)
	if !ok {
		return nil, fmt.Errorf("%s is not an SSH certificate", path)
	}
	return cert, nil
}

// certificateTimes replaces parsing `ssh-keygen -L`'s "Valid:" line.
func certificateTimes(path string) (time.Time, time.Time, error) {
	cert, err := readCertificate(path)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if cert.ValidBefore == gossh.CertTimeInfinity {
		return time.Time{}, time.Time{}, errors.New("certificate has no expiry")
	}
	return time.Unix(int64(cert.ValidAfter), 0), time.Unix(int64(cert.ValidBefore), 0), nil
}
