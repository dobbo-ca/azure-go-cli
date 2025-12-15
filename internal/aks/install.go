package aks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/cdobbyn/azure-go-cli/pkg/logger"
)

// InstallCLI installs kubectl and kubelogin to /usr/local/bin
func InstallCLI(ctx context.Context) error {
	// Check if we're running as root
	if os.Geteuid() != 0 {
		fmt.Println("This command requires sudo privileges to install to /usr/local/bin")
		fmt.Println("Please run: sudo az aks install-cli")
		return fmt.Errorf("requires sudo privileges")
	}

	fmt.Println("Installing kubectl and kubelogin...")

	// Determine OS and architecture
	osName := runtime.GOOS
	arch := runtime.GOARCH

	logger.Debug("OS: %s, Arch: %s", osName, arch)

	// Install kubectl
	if err := installKubectl(ctx, osName, arch); err != nil {
		return fmt.Errorf("failed to install kubectl: %w", err)
	}

	// Install kubelogin
	if err := installKubelogin(ctx, osName, arch); err != nil {
		return fmt.Errorf("failed to install kubelogin: %w", err)
	}

	fmt.Println("\nSuccessfully installed:")
	fmt.Println("  - kubectl")
	fmt.Println("  - kubelogin")
	fmt.Println("\nYou can now use 'az aks bastion' command.")

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

func installKubelogin(ctx context.Context, osName, arch string) error {
	logger.Debug("Installing kubelogin...")

	// Check if kubelogin is already installed
	if _, err := exec.LookPath("kubelogin"); err == nil {
		fmt.Println("kubelogin is already installed, skipping...")
		return nil
	}

	version := "v0.1.3" // Latest stable version
	var downloadURL string
	var archiveName string

	switch osName {
	case "darwin":
		if arch == "arm64" {
			archiveName = fmt.Sprintf("kubelogin-darwin-arm64-%s.zip", version)
		} else {
			archiveName = fmt.Sprintf("kubelogin-darwin-amd64-%s.zip", version)
		}
	case "linux":
		if arch == "arm64" {
			archiveName = fmt.Sprintf("kubelogin-linux-arm64-%s.zip", version)
		} else {
			archiveName = fmt.Sprintf("kubelogin-linux-amd64-%s.zip", version)
		}
	default:
		return fmt.Errorf("unsupported OS: %s", osName)
	}

	downloadURL = fmt.Sprintf("https://github.com/Azure/kubelogin/releases/download/%s/%s", version, archiveName)

	fmt.Printf("Downloading kubelogin...\n")
	logger.Debug("Download URL: %s", downloadURL)

	// Download and install kubelogin
	cmd := exec.CommandContext(ctx, "sh", "-c",
		fmt.Sprintf("curl -LO %s && unzip -o %s && chmod +x bin/darwin_*/kubelogin && mv bin/darwin_*/kubelogin /usr/local/bin/kubelogin && rm -rf bin %s",
			downloadURL, archiveName, archiveName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download kubelogin: %w", err)
	}

	fmt.Println("kubelogin installed successfully")
	return nil
}
