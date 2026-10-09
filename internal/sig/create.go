package sig

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/cdobbyn/azure-go-cli/pkg/output"
	"github.com/spf13/cobra"
)

func Create(ctx context.Context, cmd *cobra.Command) error {
	f := cmd.Flags()
	rg, _ := f.GetString("resource-group")
	name, _ := f.GetString("gallery-name")
	description, _ := f.GetString("description")
	tags, _ := f.GetStringToString("tags")
	permissions, _ := f.GetString("permissions")
	publisherURI, _ := f.GetString("publisher-uri")
	publisherEmail, _ := f.GetString("publisher-email")
	eula, _ := f.GetString("eula")
	prefix, _ := f.GetString("public-name-prefix")
	noWait, _ := f.GetBool("no-wait")

	props := &armcompute.GalleryProperties{}
	if description != "" {
		props.Description = to.Ptr(description)
	}
	if permissions != "" {
		p, err := enumValue("permissions", permissions, "Private", "Groups", "Community")
		if err != nil {
			return err
		}
		sp := &armcompute.SharingProfile{Permissions: to.Ptr(armcompute.GallerySharingPermissionTypes(p))}
		if p == "Community" {
			if publisherURI == "" || publisherEmail == "" || eula == "" || prefix == "" {
				return fmt.Errorf("sharing to the community requires --publisher-uri, --publisher-email, --eula and --public-name-prefix")
			}
		}
		props.SharingProfile = sp
	}
	if publisherURI != "" || publisherEmail != "" || eula != "" || prefix != "" {
		if props.SharingProfile == nil {
			props.SharingProfile = &armcompute.SharingProfile{}
		}
		props.SharingProfile.CommunityGalleryInfo = &armcompute.CommunityGalleryInfo{
			PublisherURI:     strPtr(publisherURI),
			PublisherContact: strPtr(publisherEmail),
			Eula:             strPtr(eula),
			PublicNamePrefix: strPtr(prefix),
		}
	}

	client, _, subID, err := newClients(cmd)
	if err != nil {
		return err
	}
	location, err := locationOrGroup(ctx, cmd, subID, rg)
	if err != nil {
		return err
	}

	gallery := armcompute.Gallery{Location: to.Ptr(location), Tags: tagsToPtrs(tags), Properties: props}
	poller, err := client.BeginCreateOrUpdate(ctx, rg, name, gallery, nil)
	if err != nil {
		return fmt.Errorf("failed to create gallery: %w", err)
	}
	if noWait {
		return nil
	}
	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to complete gallery creation: %w", err)
	}
	return output.PrintJSON(cmd, result.Gallery)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func tagsToPtrs(tags map[string]string) map[string]*string {
	if len(tags) == 0 {
		return nil
	}
	out := make(map[string]*string, len(tags))
	for k, v := range tags {
		out[k] = to.Ptr(v)
	}
	return out
}
