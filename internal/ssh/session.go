package ssh

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Port of the azure-cli "ssh" extension (azext_ssh 2.0.9). Comments name the
// Python function a piece of code mirrors so the two can be diffed.

const (
	aadSSHLoginForLinuxAppID   = "ce6ff14a-7fdc-4685-bbe0-f6afdfcfa8e0"
	agentMinimumVersionMajor   = 1
	agentMinimumVersionMinor   = 31
	clientProxyVersion         = "1.3.033291"
	clientProxyMCRRepo         = "azureconnectivity/proxy"
	cleanupTimeLimit           = 120 * time.Second
	relayInfoMaxDurationSecs   = 3600
	retryDelay                 = 10 * time.Second
	serviceConnectionDelaySecs = 15
	windowsInvalidFolderChars  = "\\/*:<>?\"|"
	rdpTerminateWait           = 30 * time.Second
	arcResourceTypePlaceholder = "arc_resource_type_placeholder"

	recommendationSSHClientNotFound = "Ensure OpenSSH is installed correctly.\nAlternatively, use --ssh-client-folder to provide OpenSSH folder path."
	recommendationResourceNotFound  = "Please ensure the active subscription is set properly and resource exists."

	typeCompute       = "Microsoft.Compute/virtualMachines"
	typeHybridCompute = "Microsoft.HybridCompute/machines"
	typeVSphere       = "Microsoft.ConnectedVMwarevSphere/virtualMachines"
	typeScVmm         = "Microsoft.ScVmm/virtualMachines"
	typeHCI           = "Microsoft.AzureStackHCI/virtualMachines"
)

// resourceTypeCase maps SUPPORTED_RESOURCE_TYPES to their correct case.
var resourceTypeCase = map[string]string{
	"microsoft.hybridcompute/machines":                 typeHybridCompute,
	"microsoft.compute/virtualmachines":                typeCompute,
	"microsoft.connectedvmwarevsphere/virtualmachines": typeVSphere,
	"microsoft.scvmm/virtualmachines":                  typeScVmm,
	"microsoft.azurestackhci/virtualmachines":          typeHCI,
}

// legacyResourceType maps provider-only values older versions accepted.
var legacyResourceType = map[string]string{
	"microsoft.hybridcompute":          typeHybridCompute,
	"microsoft.compute":                typeCompute,
	"microsoft.connectedvmwarevsphere": typeVSphere,
	"microsoft.azurestackhci":          typeHCI,
	"microsoft.scvmm":                  typeScVmm,
}

// session is SSHSession and ConfigSession from ssh_info.py in one struct.
type session struct {
	resourceGroup     string
	vmName            string
	ip                string
	publicKeyFile     string
	privateKeyFile    string
	usePrivateIP      bool
	localUser         string
	certFile          string
	port              string
	resourceType      string
	sshClientFolder   string
	sshProxyFolder    string
	credentialsFolder string
	sshArgs           []string
	deleteCredentials bool
	winrdp            bool
	yes               bool
	subscription      string

	proxyPath        string
	relay            *relayInfo
	newServiceConfig bool

	// az ssh config only.
	configPath    string
	overwrite     bool
	relayInfoPath string

	arm *armTarget
}

// isArc includes ScVmm and HCI for config too; upstream's ConfigSession
// omits them and then fails looking for a Compute VM.
func (s *session) isArc() bool {
	switch s.resourceType {
	case typeHybridCompute, typeVSphere, typeScVmm, typeHCI:
		return true
	}
	return false
}

func (s *session) host() (string, error) {
	if !s.isArc() && s.ip != "" {
		return s.ip, nil
	}
	if s.isArc() && s.vmName != "" {
		return s.vmName, nil
	}
	return "", errors.New("Unable to determine host.")
}

func (s *session) buildArgs() []string {
	var args []string
	if s.isArc() {
		proxy := `ProxyCommand="` + s.proxyPath + `"`
		if s.port != "" {
			proxy += " -p " + s.port
		}
		args = append(args, "-o", proxy)
	}
	if s.privateKeyFile != "" {
		args = append(args, "-i", s.privateKeyFile)
	}
	if s.certFile != "" {
		args = append(args, "-o", `CertificateFile="`+s.certFile+`"`)
	}
	if !s.isArc() && s.port != "" {
		args = append(args, "-p", s.port)
	}
	return args
}

func (s *session) makePathsAbsolute() {
	for _, p := range []*string{&s.publicKeyFile, &s.privateKeyFile, &s.certFile, &s.sshClientFolder,
		&s.sshProxyFolder, &s.credentialsFolder, &s.configPath} {
		*p = absPath(*p)
	}
}

func absPath(p string) string {
	if p == "" {
		return ""
	}
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

// assertArgs is _assert_args.
func assertArgs(resourceGroup, vmName, ip, resourceType, certFile, localUser string) error {
	if resourceType != "" {
		lower := strings.ToLower(resourceType)
		_, supported := resourceTypeCase[lower]
		_, legacy := legacyResourceType[lower]
		if !supported && !legacy && resourceType != arcResourceTypePlaceholder {
			return errors.New(`--resource-type must be either "Microsoft.Compute/virtualMachines", ` +
				`"Microsoft.HybridCompute/machines", or "Microsoft.ConnectedVMwarevSphere/virtualMachines".`)
		}
	}
	if resourceGroup == "" && vmName == "" && ip == "" {
		return errors.New("The VM must be specified by --ip or --resource-group and --vm-name/--name")
	}
	if (resourceGroup != "") != (vmName != "") {
		return errors.New("--resource-group and --vm-name/--name must be provided together")
	}
	if ip != "" && (vmName != "" || resourceGroup != "") {
		return errors.New("--ip cannot be used with --resource-group or --vm-name/--name")
	}
	if certFile != "" && localUser == "" {
		return errors.New("To authenticate with a certificate you need to provide a --local-user")
	}
	if certFile != "" && !isFile(certFile) {
		return fmt.Errorf("Certificate file %s not found", certFile)
	}
	return nil
}

func hasVerboseFlag(args []string) bool {
	for _, a := range args {
		if a == "-v" || a == "-vv" || a == "-vvv" {
			return true
		}
	}
	return false
}

func withRecommendation(msg, recommendation string) error {
	return fmt.Errorf("%s\n%s", msg, recommendation)
}

// warnf matches knack's logger.warning output.
func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "WARNING: "+format+"\n", args...)
}

// successf matches print_styled_text, which writes to stderr.
func successf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// sleepCtx sleeps unless Ctrl+C cancels ctx first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
