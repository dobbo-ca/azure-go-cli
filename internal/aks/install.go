package aks

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/cdobbyn/azure-go-cli/pkg/logger"
)

var kubeVersionRe = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// InstallCLI installs kubectl to /usr/local/bin
func InstallCLI(ctx context.Context) error {
	if os.Geteuid() != 0 {
		fmt.Println("This command requires sudo privileges to install to /usr/local/bin")
		fmt.Println("Please run: sudo az aks install-cli")
		return fmt.Errorf("requires sudo privileges")
	}

	fmt.Println("Installing kubectl...")

	osName := runtime.GOOS
	arch := runtime.GOARCH
	logger.Debug("OS: %s, Arch: %s", osName, arch)

	if err := installKubectl(ctx, osName, arch); err != nil {
		return fmt.Errorf("failed to install kubectl: %w", err)
	}

	fmt.Println("\nSuccessfully installed:")
	fmt.Println("  - kubectl")
	fmt.Println("\n(kubelogin is no longer needed — its functionality is built into this binary.)")
	fmt.Println("\nYou can now use 'az aks bastion' and 'kubectl' against AKS clusters.")
	return nil
}

func installKubectl(ctx context.Context, osName, arch string) error {
	logger.Debug("Installing kubectl...")

	// Check if kubectl is already installed
	if _, err := exec.LookPath("kubectl"); err == nil {
		fmt.Println("kubectl is already installed, skipping...")
		return nil
	}

	switch osName {
	case "darwin", "linux":
	default:
		return fmt.Errorf("unsupported OS: %s", osName)
	}
	if arch != "arm64" {
		arch = "amd64"
	}

	resp, err := httpGet(ctx, "https://dl.k8s.io/release/stable.txt")
	if err != nil {
		return fmt.Errorf("failed to fetch stable version: %w", err)
	}
	versionBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to read stable version: %w", err)
	}
	version := strings.TrimSpace(string(versionBytes))
	// remote value becomes a URL path
	if !kubeVersionRe.MatchString(version) {
		return fmt.Errorf("unexpected kubectl version format: %q", version)
	}

	downloadURL := fmt.Sprintf("https://dl.k8s.io/release/%s/bin/%s/%s/kubectl", version, osName, arch)

	fmt.Printf("Downloading kubectl...\n")
	logger.Debug("Download URL: %s", downloadURL)

	dlResp, err := httpGet(ctx, downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download kubectl: %w", err)
	}
	defer dlResp.Body.Close()

	f, err := os.CreateTemp("/usr/local/bin", ".kubectl-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(f.Name())

	if _, err := io.Copy(f, dlResp.Body); err != nil {
		f.Close()
		return fmt.Errorf("failed to write kubectl: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close kubectl file: %w", err)
	}
	if err := os.Chmod(f.Name(), 0o755); err != nil {
		return fmt.Errorf("failed to chmod kubectl: %w", err)
	}
	if err := os.Rename(f.Name(), "/usr/local/bin/kubectl"); err != nil {
		return fmt.Errorf("failed to install kubectl: %w", err)
	}

	fmt.Println("kubectl installed successfully")
	return nil
}

func httpGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}
	return resp, nil
}
