package aks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/cdobbyn/azure-go-cli/pkg/logger"
)

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

	var downloadURL string
	switch osName {
	case "darwin":
		if arch == "arm64" {
			downloadURL = "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/darwin/arm64/kubectl"
		} else {
			downloadURL = "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/darwin/amd64/kubectl"
		}
	case "linux":
		if arch == "arm64" {
			downloadURL = "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/arm64/kubectl"
		} else {
			downloadURL = "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
		}
	default:
		return fmt.Errorf("unsupported OS: %s", osName)
	}

	fmt.Printf("Downloading kubectl...\n")
	logger.Debug("Download URL: %s", downloadURL)

	// Download and install kubectl
	cmd := exec.CommandContext(ctx, "sh", "-c",
		fmt.Sprintf("curl -LO %s && chmod +x kubectl && mv kubectl /usr/local/bin/kubectl", downloadURL))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download kubectl: %w", err)
	}

	fmt.Println("kubectl installed successfully")
	return nil
}

