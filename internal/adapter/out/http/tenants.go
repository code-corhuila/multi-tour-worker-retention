package http

import (
	"context"
	"fmt"

	"github.com/code-corhuila/multi-tour-worker-runtime/httpclient"

	"github.com/code-corhuila/multi-tour-worker-retention/internal/application/port/out"
)

const (
	tenantsPath = "/api/v1/identity-audit/internal/tenants?status=ALL&page=%d&limit=%d"
	tenantsPage = 100
	maxPages    = 1000 // a guard against a service that never stops paging
)

// Tenants reads the tenants from the internal route of identity-audit, which answers each one with its id and its status.
type Tenants struct {
	client *httpclient.Client
}

var _ out.Tenants = (*Tenants)(nil)

// NewTenants returns the adapter over a client whose base URL is identity-audit.
func NewTenants(client *httpclient.Client) *Tenants { return &Tenants{client: client} }

type tenantItem struct {
	TenantID string `json:"tenantId"`
	Status   string `json:"status"`
}

// ListToSweep reads every page of GET .../internal/tenants?status=ALL and keeps the active and the inactive tenants: one that is pending or failed was never
// onboarded and holds no customer data. An id that is not a UUID or a status that is not one of the four is an error, never a skipped item.
func (t *Tenants) ListToSweep(ctx context.Context) ([]string, error) {
	var ids []string
	for page := 1; page <= maxPages; page++ {
		got, err := httpclient.List[tenantItem](ctx, t.client, fmt.Sprintf(tenantsPath, page, tenantsPage))
		if err != nil {
			return nil, fmt.Errorf("list the tenants: %w", err)
		}
		for _, item := range got.Data {
			if !uuid.MatchString(item.TenantID) {
				return nil, fmt.Errorf("list the tenants: an item has no tenant uuid")
			}
			switch item.Status {
			case "ACTIVE", "INACTIVE":
				ids = append(ids, item.TenantID)
			case "PENDING", "FAILED":
			default:
				return nil, fmt.Errorf("list the tenants: an item has no known status")
			}
		}
		if page >= got.Meta.TotalPages {
			return ids, nil
		}
	}
	return nil, fmt.Errorf("list the tenants: more than %d pages", maxPages)
}
