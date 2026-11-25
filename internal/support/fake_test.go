package support

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
)

type chunk struct {
	index   int32
	content string
}

// fakeClient is an in-memory Client. calls records "Method:arg" in order.
type fakeClient struct {
	tickets    map[string]armsupport.TicketDetails
	comms      map[string][]*armsupport.CommunicationDetails
	workspaces map[string]bool
	files      map[string][]*armsupport.FileDetails
	chunks     []chunk
	services   []*armsupport.Service
	problems   map[string][]*armsupport.ProblemClassification
	chats      map[string][]*armsupport.ChatTranscriptDetails

	mu         sync.Mutex // guards calls (loadCatalog calls the fake concurrently)
	calls      []string
	lastFilter string
	takenNames map[string]bool
	err        map[string]error // method name -> error to return
}

var _ Client = (*fakeClient)(nil)

func newFake() *fakeClient {
	return &fakeClient{
		tickets:    map[string]armsupport.TicketDetails{},
		comms:      map[string][]*armsupport.CommunicationDetails{},
		workspaces: map[string]bool{},
		files:      map[string][]*armsupport.FileDetails{},
		problems:   map[string][]*armsupport.ProblemClassification{},
		chats:      map[string][]*armsupport.ChatTranscriptDetails{},
		takenNames: map[string]bool{},
		err:        map[string]error{},
	}
}

func (f *fakeClient) rec(method string, args ...string) error {
	f.mu.Lock()
	f.calls = append(f.calls, method+":"+strings.Join(args, "/"))
	f.mu.Unlock()
	return f.err[method]
}

func (f *fakeClient) CheckName(_ context.Context, name string, typ armsupport.Type) error {
	if err := f.rec("CheckName", name); err != nil {
		return err
	}
	if f.takenNames[name] {
		return fmt.Errorf("Name taken")
	}
	return nil
}

func (f *fakeClient) CreateTicket(_ context.Context, name string, t armsupport.TicketDetails) (armsupport.TicketDetails, error) {
	if err := f.rec("CreateTicket", name); err != nil {
		return armsupport.TicketDetails{}, err
	}
	t.Name = &name
	f.tickets[name] = t
	return t, nil
}

func (f *fakeClient) GetTicket(_ context.Context, name string) (armsupport.TicketDetails, error) {
	if err := f.rec("GetTicket", name); err != nil {
		return armsupport.TicketDetails{}, err
	}
	t, ok := f.tickets[name]
	if !ok {
		return t, fmt.Errorf("ticket %s not found", name)
	}
	return t, nil
}

func (f *fakeClient) ListTickets(_ context.Context, filter string) ([]*armsupport.TicketDetails, error) {
	f.lastFilter = filter
	if err := f.rec("ListTickets", filter); err != nil {
		return nil, err
	}
	var out []*armsupport.TicketDetails
	for _, t := range f.tickets {
		t := t
		out = append(out, &t)
	}
	return out, nil
}

func (f *fakeClient) UpdateTicket(_ context.Context, name string, u armsupport.UpdateSupportTicket) (armsupport.TicketDetails, error) {
	if err := f.rec("UpdateTicket", name); err != nil {
		return armsupport.TicketDetails{}, err
	}
	t := f.tickets[name]
	if t.Properties == nil {
		t.Properties = &armsupport.TicketDetailsProperties{}
	}
	if u.Severity != nil {
		t.Properties.Severity = u.Severity
	}
	if u.Status != nil {
		st := string(*u.Status)
		t.Properties.Status = &st
	}
	if u.ContactDetails != nil {
		t.Properties.ContactDetails = &armsupport.ContactProfile{}
	}
	f.tickets[name] = t
	return t, nil
}

func (f *fakeClient) CheckCommunicationName(_ context.Context, ticket, name string) error {
	if err := f.rec("CheckCommunicationName", ticket, name); err != nil {
		return err
	}
	if f.takenNames[name] {
		return fmt.Errorf("Name taken")
	}
	return nil
}

func (f *fakeClient) CreateCommunication(_ context.Context, ticket, name string, c armsupport.CommunicationDetails) (armsupport.CommunicationDetails, error) {
	if err := f.rec("CreateCommunication", ticket, name); err != nil {
		return armsupport.CommunicationDetails{}, err
	}
	c.Name = &name
	f.comms[ticket] = append(f.comms[ticket], &c)
	return c, nil
}

func (f *fakeClient) GetCommunication(_ context.Context, ticket, name string) (armsupport.CommunicationDetails, error) {
	if err := f.rec("GetCommunication", ticket, name); err != nil {
		return armsupport.CommunicationDetails{}, err
	}
	for _, c := range f.comms[ticket] {
		if c.Name != nil && *c.Name == name {
			return *c, nil
		}
	}
	return armsupport.CommunicationDetails{}, fmt.Errorf("communication %s not found", name)
}

func (f *fakeClient) ListCommunications(_ context.Context, ticket, filter string) ([]*armsupport.CommunicationDetails, error) {
	f.lastFilter = filter
	if err := f.rec("ListCommunications", ticket, filter); err != nil {
		return nil, err
	}
	return f.comms[ticket], nil
}

func (f *fakeClient) GetChatTranscript(_ context.Context, ticket, name string) (armsupport.ChatTranscriptDetails, error) {
	if err := f.rec("GetChatTranscript", ticket, name); err != nil {
		return armsupport.ChatTranscriptDetails{}, err
	}
	for _, c := range f.chats[ticket] {
		if c.Name != nil && *c.Name == name {
			return *c, nil
		}
	}
	return armsupport.ChatTranscriptDetails{}, fmt.Errorf("chat transcript %s not found", name)
}

func (f *fakeClient) ListChatTranscripts(_ context.Context, ticket string) ([]*armsupport.ChatTranscriptDetails, error) {
	if err := f.rec("ListChatTranscripts", ticket); err != nil {
		return nil, err
	}
	return f.chats[ticket], nil
}

func (f *fakeClient) CreateFileWorkspace(_ context.Context, name string) (armsupport.FileWorkspaceDetails, error) {
	if err := f.rec("CreateFileWorkspace", name); err != nil {
		return armsupport.FileWorkspaceDetails{}, err
	}
	f.workspaces[name] = true
	return armsupport.FileWorkspaceDetails{Name: &name}, nil
}

func (f *fakeClient) GetFileWorkspace(_ context.Context, name string) (armsupport.FileWorkspaceDetails, error) {
	if err := f.rec("GetFileWorkspace", name); err != nil {
		return armsupport.FileWorkspaceDetails{}, err
	}
	if !f.workspaces[name] {
		return armsupport.FileWorkspaceDetails{}, fmt.Errorf("workspace %s not found", name)
	}
	return armsupport.FileWorkspaceDetails{Name: &name}, nil
}

func (f *fakeClient) CreateFile(_ context.Context, workspace, name string, d armsupport.FileDetails) (armsupport.FileDetails, error) {
	if err := f.rec("CreateFile", workspace, name); err != nil {
		return armsupport.FileDetails{}, err
	}
	d.Name = &name
	f.files[workspace] = append(f.files[workspace], &d)
	return d, nil
}

func (f *fakeClient) UploadChunk(_ context.Context, workspace, name string, index int32, content string) error {
	if err := f.rec("UploadChunk", workspace, name, fmt.Sprint(index)); err != nil {
		return err
	}
	f.chunks = append(f.chunks, chunk{index, content})
	return nil
}

func (f *fakeClient) GetFile(_ context.Context, workspace, name string) (armsupport.FileDetails, error) {
	if err := f.rec("GetFile", workspace, name); err != nil {
		return armsupport.FileDetails{}, err
	}
	for _, d := range f.files[workspace] {
		if d.Name != nil && *d.Name == name {
			return *d, nil
		}
	}
	return armsupport.FileDetails{}, fmt.Errorf("file %s not found", name)
}

func (f *fakeClient) ListFiles(_ context.Context, workspace string) ([]*armsupport.FileDetails, error) {
	if err := f.rec("ListFiles", workspace); err != nil {
		return nil, err
	}
	return f.files[workspace], nil
}

func (f *fakeClient) ListServices(_ context.Context) ([]*armsupport.Service, error) {
	if err := f.rec("ListServices"); err != nil {
		return nil, err
	}
	return f.services, nil
}

func (f *fakeClient) GetService(_ context.Context, name string) (armsupport.Service, error) {
	if err := f.rec("GetService", name); err != nil {
		return armsupport.Service{}, err
	}
	for _, s := range f.services {
		if s.Name != nil && *s.Name == name {
			return *s, nil
		}
	}
	return armsupport.Service{}, fmt.Errorf("service %s not found", name)
}

func (f *fakeClient) ListProblemClassifications(_ context.Context, service string) ([]*armsupport.ProblemClassification, error) {
	if err := f.rec("ListProblemClassifications", service); err != nil {
		return nil, err
	}
	return f.problems[service], nil
}

func (f *fakeClient) GetProblemClassification(_ context.Context, service, name string) (armsupport.ProblemClassification, error) {
	if err := f.rec("GetProblemClassification", service, name); err != nil {
		return armsupport.ProblemClassification{}, err
	}
	for _, p := range f.problems[service] {
		if p.Name != nil && *p.Name == name {
			return *p, nil
		}
	}
	return armsupport.ProblemClassification{}, fmt.Errorf("problem classification %s not found", name)
}
