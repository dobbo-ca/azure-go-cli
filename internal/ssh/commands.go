package ssh

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// NewSSHCommand returns `az ssh`. Upstream ships it as the "ssh"
// extension; here it is built in.
func NewSSHCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh",
		Short: "SSH into resources (Azure VMs, Arc servers, etc) using AAD issued openssh certificates.",
		Long:  "SSH into Arc-enabled servers without requiring a public IP address or SSH into Azure Virtual Machines. AAD issued openssh certificates for authentication currently only supported for Linux.",
	}
	cmd.AddCommand(newVMCommand(), newConfigCommand(), newCertCommand(), newArcCommand())
	return cmd
}

const (
	clientFolderHelp = "Folder path that contains ssh executables (ssh.exe, ssh-keygen.exe, etc). Default to ssh pre-installed if not provided."
	proxyFolderHelp  = "Path to the folder where the ssh proxy should be saved. Default to .clientsshproxy folder in user's home directory if not provided."
	deleteCredsHelp  = "This is an internal argument. This argument is used by Azure Portal to provide a one click SSH login experience in Cloud shell."
	yesHelp          = "Update service configuration without prompting user"
)

// aliases maps upstream's extra option names to the flag that owns them.
var aliases = map[string]string{
	"vm-name":            "name",
	"hostname":           "ip",
	"delete-private-key": "force-delete-credentials",
	"rdp":                "winrdp",
	"yes-without-prompt": "yes",
	"keys-dest-folder":   "keys-destination-folder",
}

func normalizeAliases(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	if canonical, ok := aliases[name]; ok {
		name = canonical
	}
	return pflag.NormalizedName(name)
}

// interruptContext keeps Ctrl+C from killing us while ssh runs, so cleanup
// still happens; before ssh starts it cancels the work in progress.
func interruptContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}

func newVMCommand() *cobra.Command {
	s := &session{}
	cmd := &cobra.Command{
		Use:   "vm [-- SSH_ARGS...]",
		Short: "SSH into Azure VMs or Arc Servers.",
		Long:  "Users can login using AAD issued certificates or using local user credentials. We recommend login using AAD issued certificates. To SSH using local user credentials, you must provide the local user name using the --local-user parameter.",
		Example: `  # Give a resource group name and machine name to SSH using AAD issued certificates
  az ssh vm --resource-group myResourceGroup --name myVM

  # Give the public IP (or hostname) of a VM to SSH using AAD issued certificates
  az ssh vm --ip 1.2.3.4
  az ssh vm --hostname example.com

  # Using a custom private key file
  az ssh vm --ip 1.2.3.4 --private-key-file key --public-key-file key.pub

  # Using additional ssh arguments
  az ssh vm --ip 1.2.3.4 -- -A -o ForwardX11=yes

  # Give the Resource Type of the target. Useful when there is an Azure VM and an Arc Server with the same name in the same resource group.
  az ssh vm --resource-type [Microsoft.Compute/virtualMachines|Microsoft.HybridCompute/machines] --resource-group myResourceGroup --name myVM

  # Give a local user name to SSH with local user credentials using certificate based authentication.
  az ssh vm --local-user username --ip 1.2.3.4 --certificate-file cert.pub --private-key-file key

  # Give a local user name to SSH with local user credentials using key based authentication.
  az ssh vm --local-user username --resource-group myResourceGroup --name myVM --private-key-file key

  # Give a local user name to SSH with local user credentials using password based authentication.
  az ssh vm --local-user username --resource-group myResourceGroup --name myArcServer

  # Give a SSH Client Folder to use the ssh executables in that folder, like ssh-keygen.exe and ssh.exe.
  # If not provided, the pre-installed OpenSSH client is used (on Windows, under C:\Windows\System32\OpenSSH).
  az ssh vm --resource-group myResourceGroup --name myVM --ssh-client-folder "C:\Program Files\OpenSSH"

  # Open RDP connection over SSH. Useful for connecting via RDP to Arc Servers with no public IP address. Currently only supported for Windows clients.
  az ssh vm --resource-group myResourceGroup --name myVM --local-user username --rdp`,
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			s.sshArgs = args
			s.subscription, _ = cmd.Flags().GetString("subscription")
			debug, _ := cmd.Flags().GetBool("debug")
			ctx, stop := interruptContext()
			defer stop()
			return exitWith(runVM(ctx, s, debug))
		},
	}
	f := cmd.Flags()
	f.SetNormalizeFunc(normalizeAliases)
	f.StringVarP(&s.resourceGroup, "resource-group", "g", "", "Name of resource group.")
	f.StringVarP(&s.vmName, "name", "n", "", "The name of the VM (alias: --vm-name)")
	f.StringVar(&s.ip, "ip", "", "The public (or reachable private) IP address (or hostname) of the VM (alias: --hostname)")
	addKeyFlags(cmd, s)
	f.BoolVar(&s.usePrivateIP, "prefer-private-ip", false, "Will prefer private IP.  Requires connectivity to the private IP.")
	f.StringVar(&s.localUser, "local-user", "", "The username for a local user")
	f.StringVarP(&s.certFile, "certificate-file", "c", "", "Path to a certificate file used for authentication when using local user credentials.")
	f.StringVar(&s.port, "port", "", "SSH port")
	f.StringVar(&s.resourceType, "resource-type", "", "Resource type should be either Microsoft.Compute/virtualMachines, Microsoft.HybridCompute/machines, or Microsoft.ConnectedVMwarevSphere/virtualMachines.")
	addConnectionFlags(cmd, s)
	return cmd
}

func newArcCommand() *cobra.Command {
	s := &session{}
	cmd := &cobra.Command{
		Use:   "arc [-- SSH_ARGS...]",
		Short: "SSH into Azure Arc Servers",
		Long:  "Users can login using AAD issued certificates or using local user credentials. We recommend login using AAD issued certificates. To SSH using local user credentials you must provide the local user name using the --local-user parameter.",
		Example: `  # Give a resource group name and machine name to SSH using AAD issued certificates
  az ssh arc --resource-group myResourceGroup --name myMachine

  # Using a custom private key file
  az ssh arc --resource-group myResourceGroup --name myMachine --private-key-file key --public-key-file key.pub

  # Using additional ssh arguments
  az ssh arc --resource-group myResourceGroup --name myMachine -- -A -o ForwardX11=yes

  # Give a local user name to SSH with local user credentials using certificate based authentication.
  az ssh arc --local-user username --resource-group myResourceGroup --name myMachine --certificate-file cert.pub --private-key-file key

  # Give a local user name to SSH with local user credentials using key based authentication.
  az ssh arc --local-user username --resource-group myResourceGroup --name myMachine --private-key-file key

  # Give a local user name to SSH with local user credentials using password based authentication.
  az ssh arc --local-user username --resource-group myResourceGroup --name myMachine

  # Give a SSH Client Folder to use the ssh executables in that folder, like ssh-keygen.exe and ssh.exe.
  az ssh arc --resource-group myResourceGroup --name myMachine --ssh-client-folder "C:\Program Files\OpenSSH"

  # Open RDP connection over SSH. Useful for connecting via RDP to Arc Servers with no public IP address. Currently only supported for Windows clients.
  az ssh arc --resource-group myResourceGroup --name myVM --local-user username --rdp`,
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			s.sshArgs = args
			s.subscription, _ = cmd.Flags().GetString("subscription")
			if s.resourceType == "" {
				s.resourceType = arcResourceTypePlaceholder
			}
			debug, _ := cmd.Flags().GetBool("debug")
			ctx, stop := interruptContext()
			defer stop()
			return exitWith(runVM(ctx, s, debug))
		},
	}
	f := cmd.Flags()
	f.SetNormalizeFunc(normalizeAliases)
	f.StringVarP(&s.resourceGroup, "resource-group", "g", "", "Name of resource group.")
	f.StringVarP(&s.vmName, "name", "n", "", "The name of the Arc Server (alias: --vm-name)")
	addKeyFlags(cmd, s)
	f.StringVar(&s.localUser, "local-user", "", "The username for a local user")
	f.StringVarP(&s.certFile, "certificate-file", "c", "", "Path to certificate file")
	f.StringVar(&s.port, "port", "", "Port to connect to on the remote host.")
	f.StringVar(&s.resourceType, "resource-type", "", "Resource type should be either Microsoft.HybridCompute/machines or Microsoft.ConnectedVMwarevSphere/virtualMachines.")
	addConnectionFlags(cmd, s)
	return cmd
}

func newConfigCommand() *cobra.Command {
	s := &session{}
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Create an SSH config for resources (Azure VMs, Arc Servers, etc) which can then be used by clients that support OpenSSH configs and certificates",
		Long:  "Other software (git/rsync/etc) that support setting an SSH command can be set to use the config file by setting the command to 'ssh -F /path/to/config' e.g. rsync -e 'ssh -F /path/to/config'.  Users can create ssh config files that use AAD issued certificates or local user credentials.",
		Example: `  # Give the resource group and machine name for which to create a config using AAD issued certificates, save in a local file, and then ssh into that resource
  az ssh config --resource-group myResourceGroup --name myVm --file ./sshconfig
  ssh -F ./sshconfig myResourceGroup-myVM

  # Give the public IP (or hostname) of an Azure VM for which to create a config and then ssh into that VM
  az ssh config --ip 1.2.3.4 --file ./sshconfig
  ssh -F ./sshconfig 1.2.3.4

  # Give a local user to create a config using local user credentials, save in local file, and then ssh into that resource
  az ssh config --resource-group myResourceGroup --name myMachine --local-user username --certificate-file cert --private-key-file key --file ./sshconfig
  ssh -F ./sshconfig MyResourceGroup-myMachine-username

  # Give Keys Destination Folder to store the generated keys and certificates. If not provided, SSH keys are stored in new folder "az_ssh_config" next to the config file.
  az ssh config --ip 1.2.3.4 --file ./sshconfig --keys-destination-folder /home/user/mykeys

  # Create a generic config for use with any host
  az ssh config --ip \* --file ./sshconfig     # Bash
  az ssh config --ip * --file ./sshconfig      # PowerShell

  # Examples with other software
  az ssh config --ip \* --file ./sshconfig
  rsync -e 'ssh -F ./sshconfig' -avP directory/ myvm:~/directory
  GIT_SSH_COMMAND="ssh -F ./sshconfig" git clone myvm:~/gitrepo

  # Give the Resource Type of the target. Useful when there is an Azure VM and an Arc Server with the same name in the same resource group.
  az ssh config --resource-type [Microsoft.Compute/virtualMachines|Microsoft.HybridCompute/machines] --resource-group myResourceGroup --name myVM --file ./myconfig`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			s.subscription, _ = cmd.Flags().GetString("subscription")
			ctx, stop := interruptContext()
			defer stop()
			return runConfig(ctx, s)
		},
	}
	f := cmd.Flags()
	f.SetNormalizeFunc(normalizeAliases)
	f.StringVarP(&s.configPath, "file", "f", "", "The file path to write the SSH config to")
	f.StringVarP(&s.resourceGroup, "resource-group", "g", "", "Name of resource group.")
	f.StringVarP(&s.vmName, "name", "n", "", "The name of the VM (alias: --vm-name)")
	f.StringVar(&s.ip, "ip", "", "The public IP address (or hostname) of the VM")
	addKeyFlags(cmd, s)
	f.BoolVar(&s.usePrivateIP, "prefer-private-ip", false, "Will use a private IP if available. By default only public IPs are used.")
	f.StringVar(&s.localUser, "local-user", "", "The username for a local user")
	f.BoolVar(&s.overwrite, "overwrite", false, "Overwrites the config file if this flag is set")
	f.StringVar(&s.credentialsFolder, "keys-destination-folder", "", "Folder where new generated keys will be stored. (alias: --keys-dest-folder)")
	f.StringVar(&s.port, "port", "", "SSH Port")
	f.StringVar(&s.resourceType, "resource-type", "", "Resource type should be either Microsoft.Compute/virtualMachines, Microsoft.HybridCompute/machines, or Microsoft.ConnectedVMwarevSphere/virtualMachines.")
	f.StringVarP(&s.certFile, "certificate-file", "c", "", "Path to certificate file")
	f.StringVar(&s.sshProxyFolder, "ssh-proxy-folder", "", proxyFolderHelp)
	f.StringVar(&s.sshClientFolder, "ssh-client-folder", "", clientFolderHelp)
	f.BoolVarP(&s.yes, "yes", "y", false, yesHelp+" (alias: --yes-without-prompt)")
	cmd.MarkFlagRequired("file")
	return cmd
}

func newCertCommand() *cobra.Command {
	var certPath, pub, clientFolder string
	cmd := &cobra.Command{
		Use:   "cert",
		Short: "Create an SSH RSA certificate signed by AAD",
		Example: `  # Create a short lived ssh certificate signed by AAD
  az ssh cert --public-key-file ./id_rsa.pub --file ./id_rsa-aadcert.pub

  # Give a SSH Client Folder to use the ssh executables in that folder, like ssh-keygen.exe.
  az ssh cert --file ./id_rsa-aadcert.pub --ssh-client-folder "C:\Program Files\OpenSSH"`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := interruptContext()
			defer stop()
			return runCert(ctx, certPath, pub, clientFolder)
		},
	}
	f := cmd.Flags()
	f.StringVarP(&certPath, "file", "f", "", "The file path to write the SSH cert to, defaults to public key path with -aadcert.pub appended")
	f.StringVarP(&pub, "public-key-file", "p", "", "The RSA public key file path. If not provided, generated key pair is stored in the same directory as --file.")
	f.StringVar(&clientFolder, "ssh-client-folder", "", clientFolderHelp)
	return cmd
}

func addKeyFlags(cmd *cobra.Command, s *session) {
	cmd.Flags().StringVarP(&s.publicKeyFile, "public-key-file", "p", "", "The RSA public key file path")
	cmd.Flags().StringVarP(&s.privateKeyFile, "private-key-file", "i", "", "The RSA private key file path")
}

// addConnectionFlags adds the flags vm and arc share beyond the target.
func addConnectionFlags(cmd *cobra.Command, s *session) {
	f := cmd.Flags()
	f.StringVar(&s.sshClientFolder, "ssh-client-folder", "", clientFolderHelp)
	f.BoolVar(&s.deleteCredentials, "force-delete-credentials", false, deleteCredsHelp)
	_ = f.MarkDeprecated("force-delete-credentials", "it is an internal argument used by the Azure Portal in Cloud Shell")
	f.StringVar(&s.sshProxyFolder, "ssh-proxy-folder", "", proxyFolderHelp)
	f.BoolVar(&s.winrdp, "winrdp", false, "Start RDP connection over SSH. (alias: --rdp)")
	f.BoolVarP(&s.yes, "yes", "y", false, yesHelp+" (alias: --yes-without-prompt)")
}

// exitWith exits with ssh's own exit code instead of printing an error,
// so scripts see what the remote command returned.
func exitWith(code int, err error) error {
	if err != nil {
		return err
	}
	if code != 0 {
		os.Exit(code)
	}
	return nil
}
