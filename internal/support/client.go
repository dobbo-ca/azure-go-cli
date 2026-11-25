package support

import (
	"context"
	"errors"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"

	"github.com/cdobbyn/azure-go-cli/pkg/azure"
	"github.com/cdobbyn/azure-go-cli/pkg/config"
)

// Client is the set of support API calls the commands and the TUI use. It has
// a subscription-scoped and a tenant-scoped (no-subscription) implementation.
type Client interface {
	CheckName(ctx context.Context, name string, typ armsupport.Type) error // nil = available; else errors.New(*Message)
	CreateTicket(ctx context.Context, name string, t armsupport.TicketDetails) (armsupport.TicketDetails, error)
	GetTicket(ctx context.Context, name string) (armsupport.TicketDetails, error)
	ListTickets(ctx context.Context, filter string) ([]*armsupport.TicketDetails, error) // all pages; "" = no filter
	UpdateTicket(ctx context.Context, name string, u armsupport.UpdateSupportTicket) (armsupport.TicketDetails, error)
	CheckCommunicationName(ctx context.Context, ticket, name string) error
	CreateCommunication(ctx context.Context, ticket, name string, c armsupport.CommunicationDetails) (armsupport.CommunicationDetails, error)
	GetCommunication(ctx context.Context, ticket, name string) (armsupport.CommunicationDetails, error)
	ListCommunications(ctx context.Context, ticket, filter string) ([]*armsupport.CommunicationDetails, error) // all pages
	GetChatTranscript(ctx context.Context, ticket, name string) (armsupport.ChatTranscriptDetails, error)
	ListChatTranscripts(ctx context.Context, ticket string) ([]*armsupport.ChatTranscriptDetails, error)
	CreateFileWorkspace(ctx context.Context, name string) (armsupport.FileWorkspaceDetails, error)
	GetFileWorkspace(ctx context.Context, name string) (armsupport.FileWorkspaceDetails, error)
	CreateFile(ctx context.Context, workspace, name string, f armsupport.FileDetails) (armsupport.FileDetails, error)
	UploadChunk(ctx context.Context, workspace, name string, index int32, content string) error
	GetFile(ctx context.Context, workspace, name string) (armsupport.FileDetails, error)
	ListFiles(ctx context.Context, workspace string) ([]*armsupport.FileDetails, error)
	ListServices(ctx context.Context) ([]*armsupport.Service, error)
	GetService(ctx context.Context, name string) (armsupport.Service, error)
	ListProblemClassifications(ctx context.Context, service string) ([]*armsupport.ProblemClassification, error)
	GetProblemClassification(ctx context.Context, service, name string) (armsupport.ProblemClassification, error)
}

// typeFileWorkspaces is not in the SDK enum; the Python extension uses it.
const typeFileWorkspaces = armsupport.Type("Microsoft.Support/fileWorkspaces")

// collect drains every page of a pager.
func collect[R, T any](ctx context.Context, p *runtime.Pager[R], items func(R) []*T) ([]*T, error) {
	var out []*T
	for p.More() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, items(page)...)
	}
	return out, nil
}

// available turns a name-availability result into nil or the service's message.
func available(out armsupport.CheckNameAvailabilityOutput, err error) error {
	if err != nil {
		return err
	}
	if out.NameAvailable != nil && !*out.NameAvailable {
		msg := "name is not available"
		if out.Message != nil {
			msg = *out.Message
		}
		return errors.New(msg)
	}
	return nil
}

// catalog holds the service and problem-classification clients, which are not
// subscription scoped.
type catalog struct {
	services *armsupport.ServicesClient
	problems *armsupport.ProblemClassificationsClient
}

func newCatalog(cred azcore.TokenCredential, opts *arm.ClientOptions) (catalog, error) {
	s, err := armsupport.NewServicesClient(cred, opts)
	if err != nil {
		return catalog{}, err
	}
	p, err := armsupport.NewProblemClassificationsClient(cred, opts)
	return catalog{s, p}, err
}

func (c catalog) ListServices(ctx context.Context) ([]*armsupport.Service, error) {
	return collect(ctx, c.services.NewListPager(nil), func(r armsupport.ServicesClientListResponse) []*armsupport.Service { return r.Value })
}

func (c catalog) GetService(ctx context.Context, name string) (armsupport.Service, error) {
	r, err := c.services.Get(ctx, name, nil)
	return r.Service, err
}

func (c catalog) ListProblemClassifications(ctx context.Context, service string) ([]*armsupport.ProblemClassification, error) {
	return collect(ctx, c.problems.NewListPager(service, nil), func(r armsupport.ProblemClassificationsClientListResponse) []*armsupport.ProblemClassification {
		return r.Value
	})
}

func (c catalog) GetProblemClassification(ctx context.Context, service, name string) (armsupport.ProblemClassification, error) {
	r, err := c.problems.Get(ctx, service, name, nil)
	return r.ProblemClassification, err
}

type subscriptionClient struct {
	catalog
	tickets *armsupport.TicketsClient
	comms   *armsupport.CommunicationsClient
	chats   *armsupport.ChatTranscriptsClient
	spaces  *armsupport.FileWorkspacesClient
	files   *armsupport.FilesClient
}

func (c *subscriptionClient) CheckName(ctx context.Context, name string, typ armsupport.Type) error {
	r, err := c.tickets.CheckNameAvailability(ctx, armsupport.CheckNameAvailabilityInput{Name: &name, Type: &typ}, nil)
	return available(r.CheckNameAvailabilityOutput, err)
}

func (c *subscriptionClient) CreateTicket(ctx context.Context, name string, t armsupport.TicketDetails) (armsupport.TicketDetails, error) {
	p, err := c.tickets.BeginCreate(ctx, name, t, nil)
	if err != nil {
		return armsupport.TicketDetails{}, err
	}
	r, err := p.PollUntilDone(ctx, nil)
	return r.TicketDetails, err
}

func (c *subscriptionClient) GetTicket(ctx context.Context, name string) (armsupport.TicketDetails, error) {
	r, err := c.tickets.Get(ctx, name, nil)
	return r.TicketDetails, err
}

func (c *subscriptionClient) ListTickets(ctx context.Context, filter string) ([]*armsupport.TicketDetails, error) {
	o := &armsupport.TicketsClientListOptions{}
	if filter != "" {
		o.Filter = &filter
	}
	return collect(ctx, c.tickets.NewListPager(o), func(r armsupport.TicketsClientListResponse) []*armsupport.TicketDetails { return r.Value })
}

func (c *subscriptionClient) UpdateTicket(ctx context.Context, name string, u armsupport.UpdateSupportTicket) (armsupport.TicketDetails, error) {
	r, err := c.tickets.Update(ctx, name, u, nil)
	return r.TicketDetails, err
}

func (c *subscriptionClient) CheckCommunicationName(ctx context.Context, ticket, name string) error {
	typ := armsupport.TypeMicrosoftSupportCommunications
	r, err := c.comms.CheckNameAvailability(ctx, ticket, armsupport.CheckNameAvailabilityInput{Name: &name, Type: &typ}, nil)
	return available(r.CheckNameAvailabilityOutput, err)
}

func (c *subscriptionClient) CreateCommunication(ctx context.Context, ticket, name string, d armsupport.CommunicationDetails) (armsupport.CommunicationDetails, error) {
	p, err := c.comms.BeginCreate(ctx, ticket, name, d, nil)
	if err != nil {
		return armsupport.CommunicationDetails{}, err
	}
	r, err := p.PollUntilDone(ctx, nil)
	return r.CommunicationDetails, err
}

func (c *subscriptionClient) GetCommunication(ctx context.Context, ticket, name string) (armsupport.CommunicationDetails, error) {
	r, err := c.comms.Get(ctx, ticket, name, nil)
	return r.CommunicationDetails, err
}

func (c *subscriptionClient) ListCommunications(ctx context.Context, ticket, filter string) ([]*armsupport.CommunicationDetails, error) {
	o := &armsupport.CommunicationsClientListOptions{}
	if filter != "" {
		o.Filter = &filter
	}
	return collect(ctx, c.comms.NewListPager(ticket, o), func(r armsupport.CommunicationsClientListResponse) []*armsupport.CommunicationDetails {
		return r.Value
	})
}

func (c *subscriptionClient) GetChatTranscript(ctx context.Context, ticket, name string) (armsupport.ChatTranscriptDetails, error) {
	r, err := c.chats.Get(ctx, ticket, name, nil)
	return r.ChatTranscriptDetails, err
}

func (c *subscriptionClient) ListChatTranscripts(ctx context.Context, ticket string) ([]*armsupport.ChatTranscriptDetails, error) {
	return collect(ctx, c.chats.NewListPager(ticket, nil), func(r armsupport.ChatTranscriptsClientListResponse) []*armsupport.ChatTranscriptDetails {
		return r.Value
	})
}

func (c *subscriptionClient) CreateFileWorkspace(ctx context.Context, name string) (armsupport.FileWorkspaceDetails, error) {
	r, err := c.spaces.Create(ctx, name, nil)
	return r.FileWorkspaceDetails, err
}

func (c *subscriptionClient) GetFileWorkspace(ctx context.Context, name string) (armsupport.FileWorkspaceDetails, error) {
	r, err := c.spaces.Get(ctx, name, nil)
	return r.FileWorkspaceDetails, err
}

func (c *subscriptionClient) CreateFile(ctx context.Context, workspace, name string, f armsupport.FileDetails) (armsupport.FileDetails, error) {
	r, err := c.files.Create(ctx, workspace, name, f, nil)
	return r.FileDetails, err
}

func (c *subscriptionClient) UploadChunk(ctx context.Context, workspace, name string, index int32, content string) error {
	_, err := c.files.Upload(ctx, workspace, name, armsupport.UploadFile{ChunkIndex: &index, Content: &content}, nil)
	return err
}

func (c *subscriptionClient) GetFile(ctx context.Context, workspace, name string) (armsupport.FileDetails, error) {
	r, err := c.files.Get(ctx, workspace, name, nil)
	return r.FileDetails, err
}

func (c *subscriptionClient) ListFiles(ctx context.Context, workspace string) ([]*armsupport.FileDetails, error) {
	return collect(ctx, c.files.NewListPager(workspace, nil), func(r armsupport.FilesClientListResponse) []*armsupport.FileDetails { return r.Value })
}

type tenantClient struct {
	catalog
	tickets *armsupport.TicketsNoSubscriptionClient
	comms   *armsupport.CommunicationsNoSubscriptionClient
	chats   *armsupport.ChatTranscriptsNoSubscriptionClient
	spaces  *armsupport.FileWorkspacesNoSubscriptionClient
	files   *armsupport.FilesNoSubscriptionClient
}

func (c *tenantClient) CheckName(ctx context.Context, name string, typ armsupport.Type) error {
	r, err := c.tickets.CheckNameAvailability(ctx, armsupport.CheckNameAvailabilityInput{Name: &name, Type: &typ}, nil)
	return available(r.CheckNameAvailabilityOutput, err)
}

func (c *tenantClient) CreateTicket(ctx context.Context, name string, t armsupport.TicketDetails) (armsupport.TicketDetails, error) {
	p, err := c.tickets.BeginCreate(ctx, name, t, nil)
	if err != nil {
		return armsupport.TicketDetails{}, err
	}
	r, err := p.PollUntilDone(ctx, nil)
	return r.TicketDetails, err
}

func (c *tenantClient) GetTicket(ctx context.Context, name string) (armsupport.TicketDetails, error) {
	r, err := c.tickets.Get(ctx, name, nil)
	return r.TicketDetails, err
}

func (c *tenantClient) ListTickets(ctx context.Context, filter string) ([]*armsupport.TicketDetails, error) {
	o := &armsupport.TicketsNoSubscriptionClientListOptions{}
	if filter != "" {
		o.Filter = &filter
	}
	return collect(ctx, c.tickets.NewListPager(o), func(r armsupport.TicketsNoSubscriptionClientListResponse) []*armsupport.TicketDetails {
		return r.Value
	})
}

func (c *tenantClient) UpdateTicket(ctx context.Context, name string, u armsupport.UpdateSupportTicket) (armsupport.TicketDetails, error) {
	r, err := c.tickets.Update(ctx, name, u, nil)
	return r.TicketDetails, err
}

func (c *tenantClient) CheckCommunicationName(ctx context.Context, ticket, name string) error {
	typ := armsupport.TypeMicrosoftSupportCommunications
	r, err := c.comms.CheckNameAvailability(ctx, ticket, armsupport.CheckNameAvailabilityInput{Name: &name, Type: &typ}, nil)
	return available(r.CheckNameAvailabilityOutput, err)
}

func (c *tenantClient) CreateCommunication(ctx context.Context, ticket, name string, d armsupport.CommunicationDetails) (armsupport.CommunicationDetails, error) {
	p, err := c.comms.BeginCreate(ctx, ticket, name, d, nil)
	if err != nil {
		return armsupport.CommunicationDetails{}, err
	}
	r, err := p.PollUntilDone(ctx, nil)
	return r.CommunicationDetails, err
}

func (c *tenantClient) GetCommunication(ctx context.Context, ticket, name string) (armsupport.CommunicationDetails, error) {
	r, err := c.comms.Get(ctx, ticket, name, nil)
	return r.CommunicationDetails, err
}

func (c *tenantClient) ListCommunications(ctx context.Context, ticket, filter string) ([]*armsupport.CommunicationDetails, error) {
	o := &armsupport.CommunicationsNoSubscriptionClientListOptions{}
	if filter != "" {
		o.Filter = &filter
	}
	return collect(ctx, c.comms.NewListPager(ticket, o), func(r armsupport.CommunicationsNoSubscriptionClientListResponse) []*armsupport.CommunicationDetails {
		return r.Value
	})
}

func (c *tenantClient) GetChatTranscript(ctx context.Context, ticket, name string) (armsupport.ChatTranscriptDetails, error) {
	r, err := c.chats.Get(ctx, ticket, name, nil)
	return r.ChatTranscriptDetails, err
}

func (c *tenantClient) ListChatTranscripts(ctx context.Context, ticket string) ([]*armsupport.ChatTranscriptDetails, error) {
	return collect(ctx, c.chats.NewListPager(ticket, nil), func(r armsupport.ChatTranscriptsNoSubscriptionClientListResponse) []*armsupport.ChatTranscriptDetails {
		return r.Value
	})
}

func (c *tenantClient) CreateFileWorkspace(ctx context.Context, name string) (armsupport.FileWorkspaceDetails, error) {
	r, err := c.spaces.Create(ctx, name, nil)
	return r.FileWorkspaceDetails, err
}

func (c *tenantClient) GetFileWorkspace(ctx context.Context, name string) (armsupport.FileWorkspaceDetails, error) {
	r, err := c.spaces.Get(ctx, name, nil)
	return r.FileWorkspaceDetails, err
}

func (c *tenantClient) CreateFile(ctx context.Context, workspace, name string, f armsupport.FileDetails) (armsupport.FileDetails, error) {
	r, err := c.files.Create(ctx, workspace, name, f, nil)
	return r.FileDetails, err
}

func (c *tenantClient) UploadChunk(ctx context.Context, workspace, name string, index int32, content string) error {
	_, err := c.files.Upload(ctx, workspace, name, armsupport.UploadFile{ChunkIndex: &index, Content: &content}, nil)
	return err
}

func (c *tenantClient) GetFile(ctx context.Context, workspace, name string) (armsupport.FileDetails, error) {
	r, err := c.files.Get(ctx, workspace, name, nil)
	return r.FileDetails, err
}

func (c *tenantClient) ListFiles(ctx context.Context, workspace string) ([]*armsupport.FileDetails, error) {
	return collect(ctx, c.files.NewListPager(workspace, nil), func(r armsupport.FilesNoSubscriptionClientListResponse) []*armsupport.FileDetails {
		return r.Value
	})
}

// credentialFor returns a credential bound to tenantID, or the default
// credential when tenantID is empty.
func credentialFor(tenantID string) (azcore.TokenCredential, error) {
	if tenantID == "" {
		return azure.GetCredential()
	}
	profile, err := config.Load()
	if err != nil {
		return nil, err
	}
	var rec azidentity.AuthenticationRecord
	if profile.AuthenticationRecord != nil {
		rec = *profile.AuthenticationRecord
	}
	return azure.TenantCredential(tenantID, rec)
}

// NewClient builds the client for a scope: tenant-level when it has no subscription.
func NewClient(s Scope) (Client, error) {
	cred, err := credentialFor(s.TenantID)
	if err != nil {
		return nil, err
	}
	return newClient(s, cred, nil)
}

func newClient(s Scope, cred azcore.TokenCredential, opts *arm.ClientOptions) (Client, error) {
	cat, err := newCatalog(cred, opts)
	if err != nil {
		return nil, err
	}
	if s.TenantLevel() {
		c := &tenantClient{catalog: cat}
		if c.tickets, err = armsupport.NewTicketsNoSubscriptionClient(cred, opts); err != nil {
			return nil, err
		}
		if c.comms, err = armsupport.NewCommunicationsNoSubscriptionClient(cred, opts); err != nil {
			return nil, err
		}
		if c.chats, err = armsupport.NewChatTranscriptsNoSubscriptionClient(cred, opts); err != nil {
			return nil, err
		}
		if c.spaces, err = armsupport.NewFileWorkspacesNoSubscriptionClient(cred, opts); err != nil {
			return nil, err
		}
		if c.files, err = armsupport.NewFilesNoSubscriptionClient(cred, opts); err != nil {
			return nil, err
		}
		return c, nil
	}
	c := &subscriptionClient{catalog: cat}
	sub := s.SubscriptionID
	if c.tickets, err = armsupport.NewTicketsClient(sub, cred, opts); err != nil {
		return nil, err
	}
	if c.comms, err = armsupport.NewCommunicationsClient(sub, cred, opts); err != nil {
		return nil, err
	}
	if c.chats, err = armsupport.NewChatTranscriptsClient(sub, cred, opts); err != nil {
		return nil, err
	}
	if c.spaces, err = armsupport.NewFileWorkspacesClient(sub, cred, opts); err != nil {
		return nil, err
	}
	if c.files, err = armsupport.NewFilesClient(sub, cred, opts); err != nil {
		return nil, err
	}
	return c, nil
}
