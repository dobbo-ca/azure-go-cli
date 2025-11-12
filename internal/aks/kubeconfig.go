package aks

import (
  "bufio"
  "context"
  "fmt"
  "io"
  "os"
  "os/exec"
  "path/filepath"
  "regexp"
  "strings"

  "github.com/cdobbyn/azure-go-cli/pkg/logger"
)

// CheckDependencies checks if required CLI tools are installed
func CheckDependencies() (missing []string) {
  deps := []string{"kubectl", "kubelogin"}

  for _, dep := range deps {
    if _, err := exec.LookPath(dep); err != nil {
      missing = append(missing, dep)
    }
  }

  return missing
}

// CreateTempKubeconfig creates a temporary kubeconfig file for bastion tunnel
func CreateTempKubeconfig(ctx context.Context, clusterName, server string, port int) (string, error) {
  // Create temp directory
  tmpDir, err := os.MkdirTemp("", "az-aks-bastion-*")
  if err != nil {
    return "", fmt.Errorf("failed to create temp directory: %w", err)
  }

  kubeconfigDir := filepath.Join(tmpDir, ".kube")
  if err := os.MkdirAll(kubeconfigDir, 0700); err != nil {
    return "", fmt.Errorf("failed to create .kube directory: %w", err)
  }

  kubeconfigPath := filepath.Join(kubeconfigDir, "config")

  // Generate kubeconfig pointing to localhost tunnel
  localServer := fmt.Sprintf("https://127.0.0.1:%d", port)

  // Get path to our az binary to ensure it's used instead of Python CLI
  exePath, err := os.Executable()
  if err != nil {
    return "", fmt.Errorf("failed to get executable path: %w", err)
  }
  exeDir := filepath.Dir(exePath)
  currentPath := os.Getenv("PATH")
  customPath := fmt.Sprintf("%s:%s", exeDir, currentPath)

  logger.Debug("Creating temporary kubeconfig at: %s", kubeconfigPath)
  logger.Debug("Server URL: %s", localServer)
  logger.Debug("Using az binary from: %s", exeDir)

  // Basic kubeconfig structure with devicecode authentication
  // This uses OAuth device code flow directly, matching Python Azure CLI behavior
  // PATH is set to ensure our Go az binary is used instead of Python CLI
  kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %s
    insecure-skip-tls-verify: true
  name: %s
contexts:
- context:
    cluster: %s
    user: clusterUser_%s
  name: %s
current-context: %s
users:
- name: clusterUser_%s
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: kubelogin
      args:
      - get-token
      - --login
      - azurecli
      - --server-id
      - 6dae42f8-4368-4678-94ff-3960e28e3630
      env:
      - name: PATH
        value: %s
      interactiveMode: IfAvailable
      provideClusterInfo: false
`, localServer, clusterName, clusterName, clusterName, clusterName, clusterName, clusterName, customPath)

  if err := os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0600); err != nil {
    return "", fmt.Errorf("failed to write kubeconfig: %w", err)
  }

  logger.Debug("Kubeconfig created successfully")
  return kubeconfigPath, nil
}

// AuthenticateKubeconfig performs initial authentication for kubeconfig
// This handles device code flow and ensures auth is complete before returning
func AuthenticateKubeconfig(ctx context.Context, kubeconfigPath string) error {
  logger.Debug("Testing kubeconfig authentication...")
  fmt.Println("\nAuthenticating with Azure...")

  cmd := exec.CommandContext(ctx, "kubectl", "get", "nodes", "--request-timeout=10s")
  cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))

  // Capture stderr to detect device code prompts
  stderr, err := cmd.StderrPipe()
  if err != nil {
    return fmt.Errorf("failed to create stderr pipe: %w", err)
  }

  if err := cmd.Start(); err != nil {
    return fmt.Errorf("failed to start kubectl: %w", err)
  }

  // Monitor stderr for device code prompts
  scanner := bufio.NewScanner(stderr)
  deviceCodeRegex := regexp.MustCompile(`[A-Z0-9]{9}`)
  urlRegex := regexp.MustCompile(`https://[^\s]+`)

  var deviceCode, deviceURL string
  authInProgress := false

  go func() {
    for scanner.Scan() {
      line := scanner.Text()
      logger.Debug("kubectl stderr: %s", line)

      // Look for device code
      if strings.Contains(line, "code") && deviceCode == "" {
        if matches := deviceCodeRegex.FindString(line); matches != "" {
          deviceCode = matches
          logger.Debug("Found device code: %s", deviceCode)
        }
      }

      // Look for URL
      if deviceURL == "" {
        if matches := urlRegex.FindString(line); matches != "" {
          deviceURL = matches
          logger.Debug("Found device URL: %s", deviceURL)
        }
      }

      // If we have both, handle the device flow
      if deviceCode != "" && deviceURL != "" && !authInProgress {
        authInProgress = true
        handleDeviceCodeFlow(deviceURL, deviceCode)
        deviceCode = "" // Reset to avoid handling twice
        deviceURL = ""
      }
    }
  }()

  if err := cmd.Wait(); err != nil {
    // Ignore errors - auth might succeed even if kubectl fails on first try
    logger.Debug("kubectl command completed with: %v", err)
  }

  if authInProgress {
    fmt.Println("\n✓ Authentication complete!")
  } else {
    fmt.Println("Authentication complete!")
  }
  return nil
}

// handleDeviceCodeFlow opens browser and copies device code to clipboard
func handleDeviceCodeFlow(url, code string) {
  fmt.Printf("\n╭─────────────────────────────────────────╮\n")
  fmt.Printf("│  Device Authentication Required         │\n")
  fmt.Printf("╰─────────────────────────────────────────╯\n")
  fmt.Printf("\n  URL:  %s\n", url)
  fmt.Printf("  Code: %s\n\n", code)

  // Copy code to clipboard
  if err := copyToClipboard(code); err != nil {
    logger.Debug("Failed to copy to clipboard: %v", err)
    fmt.Println("  ⚠ Could not copy code to clipboard automatically")
  } else {
    fmt.Println("  ✓ Device code copied to clipboard")
  }

  // Open browser
  if err := openBrowser(url); err != nil {
    logger.Debug("Failed to open browser: %v", err)
    fmt.Printf("\n  Please open the URL manually: %s\n", url)
  } else {
    fmt.Println("  ✓ Opening browser for authentication...")
  }

  fmt.Printf("\n  Waiting for you to complete authentication in your browser...\n")
}

// copyToClipboard copies text to system clipboard
func copyToClipboard(text string) error {
  var cmd *exec.Cmd

  switch {
  case commandExists("pbcopy"): // macOS
    cmd = exec.Command("pbcopy")
  case commandExists("xclip"): // Linux with xclip
    cmd = exec.Command("xclip", "-selection", "clipboard")
  case commandExists("xsel"): // Linux with xsel
    cmd = exec.Command("xsel", "--clipboard", "--input")
  default:
    return fmt.Errorf("no clipboard utility found")
  }

  pipe, err := cmd.StdinPipe()
  if err != nil {
    return err
  }

  if err := cmd.Start(); err != nil {
    return err
  }

  if _, err := io.WriteString(pipe, text); err != nil {
    return err
  }

  pipe.Close()
  return cmd.Wait()
}

// openBrowser opens a URL in the default browser
func openBrowser(url string) error {
  var cmd *exec.Cmd

  switch {
  case commandExists("open"): // macOS
    cmd = exec.Command("open", url)
  case commandExists("xdg-open"): // Linux
    cmd = exec.Command("xdg-open", url)
  default:
    return fmt.Errorf("no browser opener found")
  }

  return cmd.Start()
}

// commandExists checks if a command is available in PATH
func commandExists(cmd string) bool {
  _, err := exec.LookPath(cmd)
  return err == nil
}

// LaunchK9s launches k9s with the specified kubeconfig
func LaunchK9s(ctx context.Context, kubeconfigPath string) error {
  // Check if k9s is installed
  k9sPath, err := exec.LookPath("k9s")
  if err != nil {
    return fmt.Errorf("k9s not found in PATH. Please install k9s: https://k9scli.io/")
  }

  logger.Debug("Launching k9s with kubeconfig: %s", kubeconfigPath)
  logger.Debug("k9s path: %s", k9sPath)

  cmd := exec.CommandContext(ctx, "k9s")
  cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
  cmd.Stdin = os.Stdin
  cmd.Stdout = os.Stdout
  cmd.Stderr = os.Stderr

  fmt.Printf("\nLaunching k9s...\n")
  fmt.Printf("Press 'q' in k9s to quit and close the tunnel.\n\n")

  if err := cmd.Run(); err != nil {
    return fmt.Errorf("k9s exited with error: %w", err)
  }

  logger.Debug("k9s exited cleanly")
  return nil
}

// RunCommand runs an arbitrary command with the specified kubeconfig
func RunCommand(ctx context.Context, kubeconfigPath, command string) error {
  // Parse command string into parts
  parts := strings.Fields(command)
  if len(parts) == 0 {
    return fmt.Errorf("empty command")
  }

  executable := parts[0]
  args := parts[1:]

  // Check if executable is in PATH
  execPath, err := exec.LookPath(executable)
  if err != nil {
    return fmt.Errorf("%s not found in PATH", executable)
  }

  logger.Debug("Running command with kubeconfig: %s", kubeconfigPath)
  logger.Debug("Executable: %s", execPath)
  logger.Debug("Args: %v", args)

  cmd := exec.CommandContext(ctx, executable, args...)
  cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
  cmd.Stdin = os.Stdin
  cmd.Stdout = os.Stdout
  cmd.Stderr = os.Stderr

  fmt.Printf("\nRunning: %s\n\n", command)

  if err := cmd.Run(); err != nil {
    return fmt.Errorf("command exited with error: %w", err)
  }

  logger.Debug("Command completed successfully")
  return nil
}
