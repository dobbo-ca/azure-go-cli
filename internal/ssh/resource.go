package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
	"github.com/cdobbyn/azure-go-cli/pkg/logger"
)

const armEndpoint = "https://management.azure.com"

// armTarget is the subscription and a credential for its tenant, which can
// differ from the default subscription's tenant under --subscription.
type armTarget struct {
	subscriptionID string
	cred           azcore.TokenCredential
}

func (s *session) armClient() (*armTarget, error) {
	if s.arm != nil {
		return s.arm, nil
	}
	subID, err := config.GetSubscription(s.subscription)
	if err != nil {
		return nil, err
	}
	var cred azcore.TokenCredential
	if tenantID, terr := config.GetTenantID(subID); terr == nil {
		profile, err := config.Load()
		if err != nil {
			return nil, fmt.Errorf("not authenticated. Please run 'az login' first: %w", err)
		}
		var rec azidentity.AuthenticationRecord
		if profile.AuthenticationRecord != nil {
			rec = *profile.AuthenticationRecord
		}
		cred, err = azure.TenantCredential(tenantID, rec)
		if err != nil {
			return nil, err
		}
	} else if cred, err = azure.GetCredential(); err != nil {
		return nil, err
	}
	s.arm = &armTarget{subscriptionID: subID, cred: cred}
	return s.arm, nil
}

// armDo sends one ARM REST call and returns the status and body.
func (s *session) armDo(ctx context.Context, method, path, apiVersion string, query url.Values, body any) (int, []byte, error) {
	a, err := s.armClient()
	if err != nil {
		return 0, nil, err
	}
	// azcore rejects an empty module version.
	client, err := arm.NewClient("github.com/cdobbyn/azure-go-cli/internal/ssh", "v1.0.0", a.cred, nil)
	if err != nil {
		return 0, nil, err
	}
	req, err := runtime.NewRequest(ctx, method, armEndpoint+path)
	if err != nil {
		return 0, nil, err
	}
	q := req.Raw().URL.Query()
	for k, v := range query {
		q[k] = v
	}
	q.Set("api-version", apiVersion)
	req.Raw().URL.RawQuery = q.Encode()
	req.Raw().Header.Set("Accept", "application/json")
	if body != nil {
		if err := runtime.MarshalAsJSON(req, body); err != nil {
			return 0, nil, err
		}
	}
	resp, err := client.Pipeline().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return resp.StatusCode, data, err
}

func httpErrorText(status int, body []byte) string {
	return fmt.Sprintf("(%d) %s: %s", status, http.StatusText(status), strings.TrimSpace(string(body)))
}

// decideResourceType is resource_type_utils.decide_resource_type.
func decideResourceType(ctx context.Context, s *session) (string, error) {
	// An IP target is treated as an Azure VM, even for an Arc server.
	if s.ip != "" {
		return typeCompute, nil
	}

	a, err := s.armClient()
	if err != nil {
		return "", err
	}
	client, err := armresources.NewClient(a.subscriptionID, a.cred, nil)
	if err != nil {
		return "", err
	}
	filter := fmt.Sprintf("name eq '%s'", s.vmName)
	pager := client.NewListByResourceGroupPager(s.resourceGroup, &armresources.ClientListByResourceGroupOptions{Filter: &filter})
	present := map[string]bool{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return "", err
		}
		for _, r := range page.Value {
			if r == nil || r.Type == nil {
				continue
			}
			if t := strings.ToLower(*r.Type); resourceTypeCase[t] != "" {
				present[t] = true
			}
		}
	}

	var target string
	if s.resourceType != "" && s.resourceType != arcResourceTypePlaceholder {
		if full, ok := legacyResourceType[strings.ToLower(s.resourceType)]; ok {
			s.resourceType = full
		}
		lower := strings.ToLower(s.resourceType)
		if !present[lower] {
			return "", withRecommendation(fmt.Sprintf("Unable to find resource %s of type %s under the resource group %s",
				s.vmName, resourceTypeCase[lower], s.resourceGroup), recommendationResourceNotFound)
		}
		target = resourceTypeCase[lower]
	} else {
		if s.resourceType == arcResourceTypePlaceholder {
			delete(present, "microsoft.compute/virtualmachines")
		}
		if len(present) > 1 {
			return "", withRecommendation(fmt.Sprintf("%s has more than one valid target with the same name: %s.",
				s.resourceGroup, s.vmName), "Please provide a --resource-type.")
		}
		if len(present) < 1 {
			return "", withRecommendation(fmt.Sprintf("A valid resource %s in the resource group %s was not found. ",
				s.vmName, s.resourceGroup), recommendationResourceNotFound)
		}
		for t := range present {
			target = resourceTypeCase[t]
		}
	}
	logger.Debug("Target Resource Type: %s", target)
	return target, nil
}

// handleTargetOSType is target_os_utils.handle_target_os_type. Lookup
// failures are ignored, as upstream does.
func handleTargetOSType(ctx context.Context, s *session) error {
	var osType, agentVersion string
	switch strings.ToLower(s.resourceType) {
	case "microsoft.compute/virtualmachines":
		osType = azureVMOS(ctx, s)
	case "microsoft.hybridcompute/machines":
		osType, agentVersion = arcResourceOS(ctx, s, "Microsoft.HybridCompute/machines", "2022-11-10")
	case "microsoft.connectedvmwarevsphere/virtualmachines":
		osType, agentVersion = arcResourceOS(ctx, s, "Microsoft.ConnectedVMwarevSphere/virtualMachines", "2022-07-15-preview")
	}
	if osType != "" {
		logger.Debug("Target OS Type: %s", osType)
	}
	if strings.EqualFold(osType, "windows") && s.localUser == "" {
		return withRecommendation("SSH Login using AAD credentials is not currently supported for Windows.",
			"Please provide --local-user.")
	}
	if s.isArc() && agentVersion != "" && agentTooOld(agentVersion) {
		warnf("The version of the Arc Agent, %s running on the target machine is not compatible with this "+
			"version of the ssh extension. Please update to the latest version.", agentVersion)
	}
	return nil
}

// agentTooOld fixes upstream's check, which compares minor without major.
func agentTooOld(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 4 {
		return false
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return major < agentMinimumVersionMajor || (major == agentMinimumVersionMajor && minor < agentMinimumVersionMinor)
}

func azureVMOS(ctx context.Context, s *session) string {
	// With --ip there is no VM to look up.
	if s.resourceGroup == "" || s.vmName == "" {
		return ""
	}
	a, err := s.armClient()
	if err != nil {
		return ""
	}
	client, err := armcompute.NewVirtualMachinesClient(a.subscriptionID, a.cred, nil)
	if err != nil {
		return ""
	}
	vm, err := client.Get(ctx, s.resourceGroup, s.vmName, nil)
	if err != nil || vm.Properties == nil || vm.Properties.StorageProfile == nil ||
		vm.Properties.StorageProfile.OSDisk == nil || vm.Properties.StorageProfile.OSDisk.OSType == nil {
		return ""
	}
	return string(*vm.Properties.StorageProfile.OSDisk.OSType)
}

// arcResourceOS reads the OS and agent version of an Arc or vSphere machine.
func arcResourceOS(ctx context.Context, s *session, resourceType, apiVersion string) (string, string) {
	a, err := s.armClient()
	if err != nil {
		return "", ""
	}
	path := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/%s/%s",
		a.subscriptionID, s.resourceGroup, resourceType, s.vmName)
	status, data, err := s.armDo(ctx, http.MethodGet, path, apiVersion, nil, nil)
	if err != nil || status != http.StatusOK {
		return "", ""
	}
	var r struct {
		Properties struct {
			OSName       string `json:"osName"`
			OSType       string `json:"osType"`
			AgentVersion string `json:"agentVersion"`
			OSProfile    struct {
				OSType string `json:"osType"`
			} `json:"osProfile"`
			GuestAgentProfile struct {
				AgentVersion string `json:"agentVersion"`
			} `json:"guestAgentProfile"`
		} `json:"properties"`
	}
	if json.Unmarshal(data, &r) != nil {
		return "", ""
	}
	p := r.Properties
	if resourceType == typeVSphere {
		return p.OSProfile.OSType, p.GuestAgentProfile.AgentVersion
	}
	if p.OSName != "" {
		return p.OSName, p.AgentVersion
	}
	return p.OSType, p.AgentVersion
}

// ipConfig is one NIC IP configuration, in NIC order.
type ipConfig struct {
	private string
	public  string
}

// pickIP is the selection rule in ip_utils.get_ssh_ip: first private IP if
// preferred, else first public IP, else first private IP with a warning.
func pickIP(cfgs []ipConfig, usePrivateIP bool) (ip string, fallback bool) {
	var privateIPs []string
	for _, c := range cfgs {
		if usePrivateIP && c.private != "" {
			return c.private, false
		}
		if c.public != "" {
			return c.public, false
		}
		if c.private != "" {
			privateIPs = append(privateIPs, c.private)
		}
	}
	if len(privateIPs) > 0 {
		return privateIPs[0], true
	}
	return "", false
}

// getSSHIP is ip_utils.get_ssh_ip. Public IPs are only fetched until one
// has an address, as upstream does.
func getSSHIP(ctx context.Context, s *session) (string, error) {
	a, err := s.armClient()
	if err != nil {
		return "", err
	}
	vmClient, err := armcompute.NewVirtualMachinesClient(a.subscriptionID, a.cred, nil)
	if err != nil {
		return "", err
	}
	nicClient, err := armnetwork.NewInterfacesClient(a.subscriptionID, a.cred, nil)
	if err != nil {
		return "", err
	}
	pipClient, err := armnetwork.NewPublicIPAddressesClient(a.subscriptionID, a.cred, nil)
	if err != nil {
		return "", err
	}
	vm, err := vmClient.Get(ctx, s.resourceGroup, s.vmName, nil)
	if err != nil {
		return "", err
	}
	if vm.Properties == nil || vm.Properties.NetworkProfile == nil {
		return "", nil
	}

	var cfgs []ipConfig
	for _, ref := range vm.Properties.NetworkProfile.NetworkInterfaces {
		if ref == nil || ref.ID == nil {
			continue
		}
		nicID, err := arm.ParseResourceID(*ref.ID)
		if err != nil {
			return "", err
		}
		nic, err := nicClient.Get(ctx, nicID.ResourceGroupName, nicID.Name, nil)
		if err != nil {
			return "", err
		}
		if nic.Properties == nil {
			continue
		}
		for _, ipc := range nic.Properties.IPConfigurations {
			if ipc == nil || ipc.Properties == nil {
				continue
			}
			c := ipConfig{private: azure.GetStringValue(ipc.Properties.PrivateIPAddress)}
			if usePrivate := s.usePrivateIP && c.private != ""; !usePrivate {
				if ref := ipc.Properties.PublicIPAddress; ref != nil && ref.ID != nil {
					pipID, err := arm.ParseResourceID(*ref.ID)
					if err != nil {
						return "", err
					}
					pip, err := pipClient.Get(ctx, pipID.ResourceGroupName, pipID.Name, nil)
					if err != nil {
						return "", err
					}
					if pip.Properties != nil {
						c.public = azure.GetStringValue(pip.Properties.IPAddress)
					}
				}
			}
			cfgs = append(cfgs, c)
			if ip, fallback := pickIP(cfgs, s.usePrivateIP); ip != "" && !fallback {
				return ip, nil
			}
		}
	}

	ip, fallback := pickIP(cfgs, s.usePrivateIP)
	if fallback {
		warnf("No public IP detected, attempting private IP (you must bring your own connectivity).")
		warnf("Use --prefer-private-ip to avoid this message.")
	}
	return ip, nil
}
