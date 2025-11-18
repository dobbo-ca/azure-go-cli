package bastion

import (
  "context"
  "fmt"
  "math/rand"
  "os"
  "os/exec"
  "os/signal"
  "path/filepath"
  "strings"
  "syscall"
  "time"

  "github.com/cdobbyn/azure-go-cli/internal/network/bastion/sshkeys"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/logger"
)

// SSH opens an SSH session to a VM through Azure Bastion
func SSH(ctx context.Context, bastionName, resourceGroup, targetResourceID, authType, username string, bufferConfig BufferConfig) error {
  // Use random high port for local tunnel
  rand.Seed(time.Now().UnixNano())
  localPort := 49152 + rand.Intn(16384) // Ephemeral port range: 49152-65535

  fmt.Printf("Opening SSH tunnel through Bastion %s...\n", bastionName)
  fmt.Printf("Target: %s\n", targetResourceID)
  fmt.Printf("Local port: %d\n", localPort)

  // Start bastion tunnel in background
  tunnelCtx, cancelTunnel := context.WithCancel(ctx)
  defer cancelTunnel()

  tunnelErrCh := make(chan error, 1)
  go func() {
    tunnelErrCh <- TunnelSSH(tunnelCtx, bastionName, resourceGroup, targetResourceID, localPort, username, bufferConfig)
  }()

  // Wait a moment for tunnel to establish
  time.Sleep(2 * time.Second)

  // Check if tunnel failed to start
  select {
  case err := <-tunnelErrCh:
    if err != nil {
      return fmt.Errorf("tunnel failed to start: %w", err)
    }
  default:
    // Tunnel is running
  }

  fmt.Println("Tunnel established, launching SSH...")

  // Handle AAD authentication
  var keysFolder string
  var sshArgs []string

  if strings.ToLower(authType) == "aad" {
    fmt.Println("Generating AAD SSH certificate...")

    // Generate key pair
    keyPair, err := sshkeys.GenerateKeyPair("")
    if err != nil {
      cancelTunnel()
      return fmt.Errorf("failed to generate key pair: %w", err)
    }
    keysFolder = filepath.Dir(keyPair.PrivateKeyPath)
    defer sshkeys.CleanupKeyFiles(keysFolder)

    // Get Azure credential
    cred, err := azure.GetCredential()
    if err != nil {
      cancelTunnel()
      return fmt.Errorf("failed to get Azure credential: %w", err)
    }

    // Get AAD SSH certificate
    certData, err := GetAADSSHCertificate(ctx, cred, keyPair, "azurecloud")
    if err != nil {
      cancelTunnel()
      return fmt.Errorf("failed to get AAD certificate: %w", err)
    }

    // Write certificate
    certPath, err := sshkeys.WriteCertificate(certData, keyPair.PublicKeyPath)
    if err != nil {
      cancelTunnel()
      return fmt.Errorf("failed to write certificate: %w", err)
    }

    // Parse certificate to get username
    cert, err := sshkeys.ParseCertificate(certPath)
    if err != nil {
      cancelTunnel()
      return fmt.Errorf("failed to parse certificate: %w", err)
    }
    username = cert.GetPrimaryPrincipal()

    fmt.Printf("Using AAD certificate for user: %s\n", username)

    // Build SSH args with certificate
    sshArgs = []string{
      "-i", keyPair.PrivateKeyPath,
      "-o", fmt.Sprintf("CertificateFile=%s", certPath),
      "-o", "StrictHostKeyChecking=no",
      "-o", "UserKnownHostsFile=/dev/null",
      "-p", fmt.Sprintf("%d", localPort),
    }
  } else {
    // Standard SSH args for password/key auth
    sshArgs = []string{
      "-o", "StrictHostKeyChecking=no",
      "-o", "UserKnownHostsFile=/dev/null",
      "-p", fmt.Sprintf("%d", localPort),
    }
  }

  // Connect with username if provided
  if username != "" {
    sshArgs = append(sshArgs, fmt.Sprintf("%s@localhost", username))
  } else {
    sshArgs = append(sshArgs, "localhost")
  }

  sshCmd := exec.CommandContext(ctx, "ssh", sshArgs...)
  sshCmd.Stdin = os.Stdin
  sshCmd.Stdout = os.Stdout
  sshCmd.Stderr = os.Stderr

  // Set up signal handling
  sigCh := make(chan os.Signal, 1)
  signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

  // Disable debug logging once SSH starts to avoid mangled output
  // The tunnel goroutine will continue running but won't output debug messages
  logger.Debug("Starting SSH command...")
  logger.DisableDebug()

  // Start SSH in a goroutine
  sshErrCh := make(chan error, 1)
  go func() {
    sshErrCh <- sshCmd.Run()
  }()

  // Wait for SSH to exit, tunnel to exit, error, or interrupt signal
  select {
  case err := <-sshErrCh:
    logger.Debug("SSH exited")
    cancelTunnel()
    if err != nil {
      return fmt.Errorf("SSH session failed: %w", err)
    }
    return nil
  case err := <-tunnelErrCh:
    logger.Debug("Tunnel exited unexpectedly")
    return fmt.Errorf("tunnel failed: %w", err)
  case <-sigCh:
    fmt.Println("\nReceived interrupt signal, closing SSH session...")
    cancelTunnel()
    return nil
  }
}
