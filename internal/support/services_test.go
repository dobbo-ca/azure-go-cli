package support

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/support/armsupport/v2"
	"github.com/stretchr/testify/require"
)

func catalogFake() *fakeClient {
	f := newFake()
	svc := func(name, display string) *armsupport.Service {
		return &armsupport.Service{Name: ptr(name), Properties: &armsupport.ServiceProperties{
			DisplayName: ptr(display), ResourceTypes: []*string{ptr("Microsoft.X/" + name)}}}
	}
	pc := func(svc, name, display string) *armsupport.ProblemClassification {
		return &armsupport.ProblemClassification{
			ID:         ptr("/providers/Microsoft.Support/services/" + svc + "/problemClassifications/" + name),
			Name:       ptr(name),
			Properties: &armsupport.ProblemClassificationProperties{DisplayName: ptr(display)},
		}
	}
	f.services = []*armsupport.Service{svc("b", "Beta"), svc("a", "Alpha")}
	f.problems["a"] = []*armsupport.ProblemClassification{pc("a", "p2", "Zed"), pc("a", "p1", "Able")}
	f.problems["b"] = []*armsupport.ProblemClassification{pc("b", "p1", "Slow"), pc("b", "p2", "Down")}
	return f
}

func TestLoadCatalog(t *testing.T) {
	rows, err := loadCatalog(context.Background(), catalogFake(), 2)
	require.NoError(t, err)
	var got []string
	for _, r := range rows {
		got = append(got, r.text())
	}
	require.Equal(t, []string{"Alpha › Able", "Alpha › Zed", "Beta › Down", "Beta › Slow"}, got)
	require.Equal(t, "/providers/Microsoft.Support/services/a/problemClassifications/p1", rows[0].ProblemID)
	require.Equal(t, "a", rows[0].ServiceName)
	require.Equal(t, []string{"Microsoft.X/a"}, rows[0].ResourceTypes)
}

func TestLoadCatalogPartialFailure(t *testing.T) {
	f := catalogFake()
	f.err["ListProblemClassifications"] = fmt.Errorf("boom")
	f.problems["a"] = nil
	rows, err := loadCatalog(context.Background(), f, 2)
	require.ErrorContains(t, err, "2 service(s)")
	require.Empty(t, rows)

	h := &partialFake{fakeClient: catalogFake()}
	rows, err = loadCatalog(context.Background(), h, 2)
	require.ErrorContains(t, err, "1 service(s)")
	require.Len(t, rows, 2)
}

// partialFake fails problem listing for service "b" only.
type partialFake struct{ *fakeClient }

func (p *partialFake) ListProblemClassifications(ctx context.Context, service string) ([]*armsupport.ProblemClassification, error) {
	if service == "b" {
		return nil, fmt.Errorf("boom")
	}
	return p.fakeClient.ListProblemClassifications(ctx, service)
}

func TestLoadCatalogServicesError(t *testing.T) {
	f := newFake()
	f.err["ListServices"] = fmt.Errorf("nope")
	rows, err := loadCatalog(context.Background(), f, 2)
	require.Nil(t, rows)
	require.EqualError(t, err, "nope")
}
