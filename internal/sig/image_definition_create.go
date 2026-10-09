package sig

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

// parseFeatures turns "A=1 B=2" into features. As in Python azure-cli,
// SecurityType=Standard is dropped and Hyper-V V2 adds TrustedLaunchSupported
// unless a SecurityType was given.
func parseFeatures(features string, hyperVGen string) ([]*armcompute.GalleryImageFeature, error) {
	var out []*armcompute.GalleryImageFeature
	hasSecurityType := false
	for _, item := range strings.Fields(features) {
		k, v, ok := strings.Cut(item, "=")
		if !ok {
			return nil, fmt.Errorf("usage error: --features KEY=VALUE [KEY=VALUE ...]")
		}
		if k == "SecurityType" {
			hasSecurityType = true
			if v == "Standard" {
				continue
			}
		}
		out = append(out, &armcompute.GalleryImageFeature{Name: to.Ptr(k), Value: to.Ptr(v)})
	}
	if !hasSecurityType && hyperVGen == "V2" {
		out = append(out, &armcompute.GalleryImageFeature{Name: to.Ptr("SecurityType"), Value: to.Ptr("TrustedLaunchSupported")})
	}
	return out, nil
}

func CreateImageDefinition(ctx context.Context, cmd *cobra.Command) error {
	f := cmd.Flags()
	rg, _ := f.GetString("resource-group")
	gallery, _ := f.GetString("gallery-name")
	name, _ := f.GetString("gallery-image-definition")
	publisher, _ := f.GetString("publisher")
	offer, _ := f.GetString("offer")
	sku, _ := f.GetString("sku")
	osType, _ := f.GetString("os-type")
	osState, _ := f.GetString("os-state")
	hyperV, _ := f.GetString("hyper-v-generation")
	arch, _ := f.GetString("architecture")
	features, _ := f.GetString("features")
	description, _ := f.GetString("description")
	eula, _ := f.GetString("eula")
	privacy, _ := f.GetString("privacy-statement-uri")
	releaseNote, _ := f.GetString("release-note-uri")
	eol, _ := f.GetString("end-of-life-date")
	planName, _ := f.GetString("plan-name")
	planPublisher, _ := f.GetString("plan-publisher")
	planProduct, _ := f.GetString("plan-product")
	tags, _ := f.GetStringToString("tags")
	noWait, _ := f.GetBool("no-wait")

	osType, err := enumValue("os-type", osType, "Linux", "Windows")
	if err != nil {
		return err
	}
	osState, err = enumValue("os-state", osState, "Generalized", "Specialized")
	if err != nil {
		return err
	}
	hyperV, err = enumValue("hyper-v-generation", hyperV, "V1", "V2")
	if err != nil {
		return err
	}

	props := &armcompute.GalleryImageProperties{
		Identifier:          &armcompute.GalleryImageIdentifier{Publisher: to.Ptr(publisher), Offer: to.Ptr(offer), SKU: to.Ptr(sku)},
		OSType:              to.Ptr(armcompute.OperatingSystemTypes(osType)),
		OSState:             to.Ptr(armcompute.OperatingSystemStateTypes(osState)),
		HyperVGeneration:    to.Ptr(armcompute.HyperVGeneration(hyperV)),
		Description:         strPtr(description),
		Eula:                strPtr(eula),
		PrivacyStatementURI: strPtr(privacy),
		ReleaseNoteURI:      strPtr(releaseNote),
	}
	if arch != "" {
		a, err := enumValue("architecture", arch, "x64", "Arm64")
		if err != nil {
			return err
		}
		props.Architecture = to.Ptr(armcompute.Architecture(a))
	}
	if props.Features, err = parseFeatures(features, hyperV); err != nil {
		return err
	}
	if eol != "" {
		t, err := time.Parse(time.RFC3339, eol)
		if err != nil {
			// date only: Python appends T12:59:59Z
			if t, err = time.Parse(time.RFC3339, eol+"T12:59:59Z"); err != nil {
				return fmt.Errorf("invalid --end-of-life-date %q: %w", eol, err)
			}
		}
		props.EndOfLifeDate = &t
	}
	if planName != "" || planPublisher != "" || planProduct != "" {
		props.PurchasePlan = &armcompute.ImagePurchasePlan{Name: strPtr(planName), Publisher: strPtr(planPublisher), Product: strPtr(planProduct)}
	}

	_, client, subID, err := newClients(cmd)
	if err != nil {
		return err
	}
	location, err := locationOrGroup(ctx, cmd, subID, rg)
	if err != nil {
		return err
	}

	image := armcompute.GalleryImage{Location: to.Ptr(location), Tags: tagsToPtrs(tags), Properties: props}
	poller, err := client.BeginCreateOrUpdate(ctx, rg, gallery, name, image, nil)
	if err != nil {
		return fmt.Errorf("failed to create image definition: %w", err)
	}
	if noWait {
		return nil
	}
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to complete image definition creation: %w", err)
	}
	return output.PrintJSON(cmd, result.GalleryImage)
}
