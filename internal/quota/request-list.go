package quota

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/quota/armquota"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
)

type QuotaRequestInfo struct {
	Name           string `json:"name"`
	RequestedLimit int32  `json:"requestedLimit"`
	Status         string `json:"status"`
	Message        string `json:"message"`
}

func RequestList(ctx context.Context, scope, outputFormat string) error {
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}

	client, err := armquota.NewRequestStatusClient(cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create quota request status client: %w", err)
	}

	var requests []QuotaRequestInfo
	pager := client.NewListPager(scope, nil)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to get quota requests page: %w", err)
		}

		for _, request := range page.Value {
			if request.Properties == nil {
				continue
			}

			props := request.Properties
			reqInfo := QuotaRequestInfo{
				Name: getStringValue(request.Name),
			}

			if props.ProvisioningState != nil {
				reqInfo.Status = string(*props.ProvisioningState)
			}

			if props.Message != nil {
				reqInfo.Message = *props.Message
			}

			// Extract requested limit if available
			if props.Value != nil && len(props.Value) > 0 {
				if props.Value[0].Limit != nil {
					if limitObj, ok := props.Value[0].Limit.(*armquota.LimitObject); ok && limitObj.Value != nil {
						reqInfo.RequestedLimit = *limitObj.Value
					}
				}
			}

			requests = append(requests, reqInfo)
		}
	}

	if len(requests) == 0 {
		fmt.Printf("No quota requests found for scope '%s'\n", scope)
		return nil
	}

	if outputFormat == "json" {
		data, err := json.MarshalIndent(requests, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format requests: %w", err)
		}
		fmt.Println(string(data))
	} else {
		// Table output
		fmt.Printf("%-40s %-15s %-15s %-30s\n", "Name", "Requested Limit", "Status", "Message")
		fmt.Println(strings.Repeat("-", 105))
		for _, req := range requests {
			msg := req.Message
			if len(msg) > 30 {
				msg = msg[:27] + "..."
			}
			fmt.Printf("%-40s %-15d %-15s %-30s\n",
				req.Name, req.RequestedLimit, req.Status, msg)
		}
		fmt.Printf("\nTotal: %d requests\n", len(requests))
	}

	return nil
}
