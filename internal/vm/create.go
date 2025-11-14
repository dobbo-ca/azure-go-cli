package vm

import (
  "context"
  "fmt"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
  "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
  "github.com/cdobbyn/azure-go-cli/pkg/azure"
  "github.com/cdobbyn/azure-go-cli/pkg/config"
  "github.com/cdobbyn/azure-go-cli/pkg/output"
  "github.com/spf13/cobra"
)

type CreateParams struct {
  Name                  string
  ResourceGroup         string
  Location              string
  NicID                 string
  Size                  string
  Image                 string
  OSDiskSizeGB          int32
  AdminUsername         string
  AdminPassword         string
  SSHKeyValue           string
  Tags                  map[string]string
}

func Create(ctx context.Context, cmd *cobra.Command, params CreateParams) error {
  cred, err := azure.GetCredential()
  if err != nil {
    return err
  }

  subscriptionID, err := config.GetDefaultSubscription()
  if err != nil {
    return fmt.Errorf("failed to get subscription: %w", err)
  }

  client, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
  if err != nil {
    return fmt.Errorf("failed to create VM client: %w", err)
  }

  // Convert tags
  azureTags := make(map[string]*string)
  for k, v := range params.Tags {
    azureTags[k] = to.Ptr(v)
  }

  // Parse image (format: publisher:offer:sku:version or UbuntuLTS, etc.)
  imageRef, err := parseImageReference(params.Image)
  if err != nil {
    return fmt.Errorf("invalid image: %w", err)
  }

  // Build VM parameters
  vmParams := armcompute.VirtualMachine{
    Location: to.Ptr(params.Location),
    Tags:     azureTags,
    Properties: &armcompute.VirtualMachineProperties{
      HardwareProfile: &armcompute.HardwareProfile{
        VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(params.Size)),
      },
      StorageProfile: &armcompute.StorageProfile{
        ImageReference: imageRef,
        OSDisk: &armcompute.OSDisk{
          Name:         to.Ptr(params.Name + "-osdisk"),
          CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
          ManagedDisk: &armcompute.ManagedDiskParameters{
            StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
          },
        },
      },
      NetworkProfile: &armcompute.NetworkProfile{
        NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
          {
            ID: to.Ptr(params.NicID),
            Properties: &armcompute.NetworkInterfaceReferenceProperties{
              Primary: to.Ptr(true),
            },
          },
        },
      },
      OSProfile: &armcompute.OSProfile{
        ComputerName:  to.Ptr(params.Name),
        AdminUsername: to.Ptr(params.AdminUsername),
      },
    },
  }

  // Set OS disk size if specified
  if params.OSDiskSizeGB > 0 {
    vmParams.Properties.StorageProfile.OSDisk.DiskSizeGB = to.Ptr(params.OSDiskSizeGB)
  }

  // Configure authentication
  if params.SSHKeyValue != "" {
    // SSH key authentication (Linux)
    vmParams.Properties.OSProfile.LinuxConfiguration = &armcompute.LinuxConfiguration{
      DisablePasswordAuthentication: to.Ptr(true),
      SSH: &armcompute.SSHConfiguration{
        PublicKeys: []*armcompute.SSHPublicKey{
          {
            Path:    to.Ptr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", params.AdminUsername)),
            KeyData: to.Ptr(params.SSHKeyValue),
          },
        },
      },
    }
  } else if params.AdminPassword != "" {
    // Password authentication
    vmParams.Properties.OSProfile.AdminPassword = to.Ptr(params.AdminPassword)
  } else {
    return fmt.Errorf("either --admin-password or --ssh-key-value must be provided")
  }

  fmt.Printf("Creating virtual machine '%s'...\n", params.Name)
  poller, err := client.BeginCreateOrUpdate(ctx, params.ResourceGroup, params.Name, vmParams, nil)
  if err != nil {
    return fmt.Errorf("failed to create VM: %w", err)
  }

  result, err := poller.PollUntilDone(ctx, nil)
  if err != nil {
    return fmt.Errorf("failed to complete VM creation: %w", err)
  }

  fmt.Printf("Created virtual machine '%s'\n", params.Name)
  return output.PrintJSON(cmd, result.VirtualMachine)
}

func parseImageReference(image string) (*armcompute.ImageReference, error) {
  // Handle common aliases
  aliases := map[string]*armcompute.ImageReference{
    "UbuntuLTS": {
      Publisher: to.Ptr("Canonical"),
      Offer:     to.Ptr("0001-com-ubuntu-server-jammy"),
      SKU:       to.Ptr("22_04-lts-gen2"),
      Version:   to.Ptr("latest"),
    },
    "Ubuntu2204": {
      Publisher: to.Ptr("Canonical"),
      Offer:     to.Ptr("0001-com-ubuntu-server-jammy"),
      SKU:       to.Ptr("22_04-lts-gen2"),
      Version:   to.Ptr("latest"),
    },
    "Ubuntu2004": {
      Publisher: to.Ptr("Canonical"),
      Offer:     to.Ptr("0001-com-ubuntu-server-focal"),
      SKU:       to.Ptr("20_04-lts-gen2"),
      Version:   to.Ptr("latest"),
    },
    "Debian11": {
      Publisher: to.Ptr("Debian"),
      Offer:     to.Ptr("debian-11"),
      SKU:       to.Ptr("11-gen2"),
      Version:   to.Ptr("latest"),
    },
    "CentOS85": {
      Publisher: to.Ptr("OpenLogic"),
      Offer:     to.Ptr("CentOS"),
      SKU:       to.Ptr("8_5-gen2"),
      Version:   to.Ptr("latest"),
    },
    "Win2022Datacenter": {
      Publisher: to.Ptr("MicrosoftWindowsServer"),
      Offer:     to.Ptr("WindowsServer"),
      SKU:       to.Ptr("2022-datacenter-g2"),
      Version:   to.Ptr("latest"),
    },
    "Win2019Datacenter": {
      Publisher: to.Ptr("MicrosoftWindowsServer"),
      Offer:     to.Ptr("WindowsServer"),
      SKU:       to.Ptr("2019-datacenter-gensecond"),
      Version:   to.Ptr("latest"),
    },
  }

  if ref, ok := aliases[image]; ok {
    return ref, nil
  }

  // TODO: Parse custom format like "Canonical:0001-com-ubuntu-server-jammy:22_04-lts-gen2:latest"
  // For now, just return an error for unknown aliases
  return nil, fmt.Errorf("unknown image alias '%s'. Supported: UbuntuLTS, Ubuntu2204, Ubuntu2004, Debian11, CentOS85, Win2022Datacenter, Win2019Datacenter", image)
}
