package image

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// termsPlan mirrors Python's _terms_prepare.
func termsPlan(ctx context.Context, cmd *cobra.Command, urn, publisher, offer, plan string) (string, string, string, error) {
	if urn == "" {
		if publisher == "" || offer == "" || plan == "" {
			return "", "", "", fmt.Errorf("if not using --urn, all of --plan, --offer and --publisher should be provided")
		}
		return publisher, offer, plan, nil
	}
	if publisher != "" || offer != "" || plan != "" {
		return "", "", "", fmt.Errorf("if using --urn, do not use any of --plan, --offer, --publisher")
	}
	publisher, offer, _, _, err := parseURN(urn)
	if err != nil {
		return "", "", "", err
	}
	img, err := getImage(ctx, cmd, urn, "", "", "", "", "")
	if err != nil {
		return "", "", "", err
	}
	if img.Properties == nil || img.Properties.Plan == nil || img.Properties.Plan.Name == nil {
		return "", "", "", fmt.Errorf("image '%s' has no terms to accept", urn)
	}
	return publisher, offer, *img.Properties.Plan.Name, nil
}

func agreementURL(subID, publisher, offer, plan string) string {
	return fmt.Sprintf("https://management.azure.com/subscriptions/%s/providers/Microsoft.MarketplaceOrdering/offerTypes/virtualmachine/publishers/%s/offers/%s/plans/%s/agreements/current?api-version=2021-01-01",
		subID, url.PathEscape(publisher), url.PathEscape(offer), url.PathEscape(plan))
}

func doAgreement(ctx context.Context, client *arm.Client, method, u string, body any) (map[string]any, error) {
	req, err := runtime.NewRequest(ctx, method, u)
	if err != nil {
		return nil, err
	}
	if body != nil {
		if err := runtime.MarshalAsJSON(req, body); err != nil {
			return nil, err
		}
	}
	resp, err := client.Pipeline().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s: %s", resp.Status, string(data))
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// flatten hoists "properties" to top level, as the Python SDK model prints.
func flatten(m map[string]any) map[string]any {
	props, _ := m["properties"].(map[string]any)
	out := map[string]any{}
	for k, v := range m {
		if k != "properties" {
			out[k] = v
		}
	}
	for k, v := range props {
		out[k] = v
	}
	return out
}

func terms(ctx context.Context, cmd *cobra.Command, urn, publisher, offer, plan string, accept bool) error {
	publisher, offer, plan, err := termsPlan(ctx, cmd, urn, publisher, offer, plan)
	if err != nil {
		return err
	}
	cred, err := azure.GetCredential()
	if err != nil {
		return err
	}
	subID, err := subscriptionID(cmd)
	if err != nil {
		return err
	}
	client, err := arm.NewClient("github.com/cdobbyn/azure-go-cli/internal/vm/image", "", cred, nil)
	if err != nil {
		return err
	}
	u := agreementURL(subID, publisher, offer, plan)
	cur, err := doAgreement(ctx, client, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("failed to get terms: %w", err)
	}
	if accept {
		props, _ := cur["properties"].(map[string]any)
		if props == nil {
			props = map[string]any{}
			cur["properties"] = props
		}
		props["accepted"] = true
		if cur, err = doAgreement(ctx, client, http.MethodPut, u, cur); err != nil {
			return fmt.Errorf("failed to accept terms: %w", err)
		}
	}
	return output.PrintJSON(cmd, flatten(cur))
}

func TermsShow(ctx context.Context, cmd *cobra.Command, urn, publisher, offer, plan string) error {
	return terms(ctx, cmd, urn, publisher, offer, plan, false)
}

func TermsAccept(ctx context.Context, cmd *cobra.Command, urn, publisher, offer, plan string) error {
	return terms(ctx, cmd, urn, publisher, offer, plan, true)
}
