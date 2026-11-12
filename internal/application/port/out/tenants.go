package out

import "context"

// Tenants names the tenants the job sweeps when no list is configured: the active and the inactive ones of identity-audit (spec 001 AC-RT-01.9),
// because the sensitive data of an inactive tenant must go too.
type Tenants interface {
	// ListToSweep reads the id of every tenant that can hold customer data. A failure is an error, never a partial list.
	ListToSweep(ctx context.Context) ([]string, error)
}
