package addon

import (
	"encoding/json"
	"fmt"
)

// ListAvailable lists the available addons that can be enabled on an AKS cluster
// Note: This is a static list based on Azure documentation as there's no API to query available addons
func ListAvailable() error {
	addons := []map[string]interface{}{
		{
			"name":        "http_application_routing",
			"description": "HTTP Application Routing - Routes traffic to applications deployed in the cluster",
		},
		{
			"name":        "monitoring",
			"description": "Azure Monitor for containers - Monitor cluster performance and health",
		},
		{
			"name":        "virtual-node",
			"description": "Virtual Node - Run pods on Azure Container Instances",
		},
		{
			"name":        "kube-dashboard",
			"description": "Kubernetes Dashboard - Web-based Kubernetes UI",
		},
		{
			"name":        "azure-policy",
			"description": "Azure Policy - Enforce policies and compliance on cluster",
		},
		{
			"name":        "ingress-appgw",
			"description": "Application Gateway Ingress Controller - Use Azure Application Gateway as ingress",
		},
		{
			"name":        "open-service-mesh",
			"description": "Open Service Mesh - Service mesh for Kubernetes",
		},
		{
			"name":        "azure-keyvault-secrets-provider",
			"description": "Azure Key Vault Secrets Provider - Sync secrets from Key Vault",
		},
		{
			"name":        "gitops",
			"description": "GitOps with Flux - Continuous deployment using GitOps",
		},
		{
			"name":        "web_application_routing",
			"description": "Web Application Routing - Managed NGINX ingress with certificate management",
		},
	}

	data, err := json.MarshalIndent(addons, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format available addons: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
