package support

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/spf13/cobra"

	"github.com/cdobbyn/azure-go-cli/pkg/output"
)

func newServicesCmd() *cobra.Command {
	list := &cobra.Command{
		Use:   "list",
		Short: "List all Azure services available for support",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, false)
			if err != nil {
				return err
			}
			items, err := c.ListServices(cmd.Context())
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, listOrEmpty(items))
		},
	}
	show := &cobra.Command{
		Use:   "show",
		Short: "Get a specific Azure service for support ticket creation",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, false)
			if err != nil {
				return err
			}
			name, _ := cmd.Flags().GetString("service-name")
			s, err := c.GetService(cmd.Context(), name)
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, s)
		},
	}
	show.Flags().String("service-name", "", "Name of the Azure service")
	_ = show.MarkFlagRequired("service-name")

	pcList := &cobra.Command{
		Use:   "list",
		Short: "List problem classifications for a service",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, false)
			if err != nil {
				return err
			}
			name, _ := cmd.Flags().GetString("service-name")
			items, err := c.ListProblemClassifications(cmd.Context(), name)
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, listOrEmpty(items))
		},
	}
	pcList.Flags().String("service-name", "", "Name of the Azure service")
	_ = pcList.MarkFlagRequired("service-name")

	pcShow := &cobra.Command{
		Use:   "show",
		Short: "Get a problem classification of a service",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := commandClientFn(cmd, false)
			if err != nil {
				return err
			}
			svc, _ := cmd.Flags().GetString("service-name")
			pc, _ := cmd.Flags().GetString("problem-classification-name")
			p, err := c.GetProblemClassification(cmd.Context(), svc, pc)
			if err != nil {
				return err
			}
			return output.PrintJSON(cmd, p)
		},
	}
	pcShow.Flags().String("service-name", "", "Name of the Azure service")
	pcShow.Flags().String("problem-classification-name", "", "Name of the problem classification")
	_ = pcShow.MarkFlagRequired("service-name")
	_ = pcShow.MarkFlagRequired("problem-classification-name")

	return group("services", "Manage support services",
		list, show, group("problem-classifications", "Manage problem classifications of a service", pcList, pcShow))
}

// catalogRow is one selectable service / problem type pair.
type catalogRow struct {
	ServiceName, ServiceDisplay string // ServiceName = service GUID (Service.Name)
	ProblemID, ProblemDisplay   string // ProblemID = full ProblemClassification.ID
	ResourceTypes               []string
}

func (r catalogRow) text() string { return r.ServiceDisplay + " › " + r.ProblemDisplay }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// loadCatalog lists every service and its problem classifications, fetching at
// most workers services at once. If only some services fail it returns their
// rows together with an error.
func loadCatalog(ctx context.Context, c Client, workers int) ([]catalogRow, error) {
	services, err := c.ListServices(ctx)
	if err != nil {
		return nil, err
	}
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		rows     []catalogRow
		failed   int
		firstErr error
		sem      = make(chan struct{}, workers)
	)
	for _, svc := range services {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			name := deref(svc.Name)
			pcs, err := c.ListProblemClassifications(ctx, name)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			var types []string
			if svc.Properties != nil {
				for _, t := range svc.Properties.ResourceTypes {
					types = append(types, deref(t))
				}
			}
			display := name
			if svc.Properties != nil && svc.Properties.DisplayName != nil {
				display = *svc.Properties.DisplayName
			}
			for _, pc := range pcs {
				row := catalogRow{ServiceName: name, ServiceDisplay: display, ProblemID: deref(pc.ID), ResourceTypes: types}
				if pc.Properties != nil {
					row.ProblemDisplay = deref(pc.Properties.DisplayName)
				}
				rows = append(rows, row)
			}
		}()
	}
	wg.Wait()
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ServiceDisplay != rows[j].ServiceDisplay {
			return rows[i].ServiceDisplay < rows[j].ServiceDisplay
		}
		return rows[i].ProblemDisplay < rows[j].ProblemDisplay
	})
	if failed > 0 {
		return rows, fmt.Errorf("could not load problem types for %d service(s): %w", failed, firstErr)
	}
	return rows, nil
}
