// Package usecase holds the jobs of the worker.
package usecase

import (
	"context"
	"fmt"
	"log/slog"

	workerruntime "github.com/code-corhuila/multi-tour-worker-runtime"

	"github.com/code-corhuila/multi-tour-worker-retention/internal/application/port/out"
)

// maxPageSize is the most the due list gives in one page.
const maxPageSize = 100

// DeleteDue is job 6: per tenant it asks the service for the reservations whose sensitive data is more than a year old and asks it to delete each one. The service
// decides what is due (the anchor, the year of the tenant zone, the other reservations of the customer) and deletes, so the job holds no retention rule.
type DeleteDue struct {
	reservations out.Reservations
	tenants      []string
	log          *slog.Logger
}

var _ workerruntime.Job = (*DeleteDue)(nil)

// NewDeleteDue wires the job over the tenants it sweeps.
func NewDeleteDue(reservations out.Reservations, tenants []string, log *slog.Logger) *DeleteDue {
	return &DeleteDue{reservations: reservations, tenants: tenants, log: log}
}

// Name is how the runner logs the job.
func (*DeleteDue) Name() string { return "retention" }

// Run takes at most batch due reservations of each tenant, following the cursor of the list page by page. What does not fit waits for the next run.
func (j *DeleteDue) Run(ctx context.Context, batch int) (processed, failed int) {
	for _, tenant := range j.tenants {
		done, bad := j.sweep(ctx, tenant, batch)
		processed, failed = processed+done, failed+bad
	}
	return processed, failed
}

func (j *DeleteDue) sweep(ctx context.Context, tenant string, batch int) (processed, failed int) {
	after, remaining := "", batch
	for remaining > 0 && ctx.Err() == nil {
		page, err := j.reservations.ListDue(ctx, tenant, min(remaining, maxPageSize), after)
		if err != nil {
			j.log.WarnContext(ctx, "the due reservations could not be listed, the next run asks again", "tenantId", tenant, "error", err)
			return processed, failed + 1
		}
		done, bad := workerruntime.Each(ctx, j.log, page.Reservations, func(ctx context.Context, id string) error { return j.delete(ctx, tenant, id) })
		processed, failed = processed+done, failed+bad
		j.log.InfoContext(ctx, "retention page swept", "tenantId", tenant, "listed", len(page.Reservations), "handled", done, "failed", bad)
		remaining -= len(page.Reservations)
		if page.Next == "" || len(page.Reservations) == 0 {
			break
		}
		after = page.Next
	}
	return processed, failed
}

// delete asks for the deletion. The key names the reservation only: the record of the deletion is permanent, so a retry or a later run asks the same thing and the
// service answers it again (a reservation not yet due today may be due tomorrow, and one already deleted answers ALREADY_DELETED).
func (j *DeleteDue) delete(ctx context.Context, tenant, reservation string) error {
	outcome, err := j.reservations.Delete(ctx, tenant, reservation, fmt.Sprintf("retention-%s", reservation))
	if err != nil {
		return fmt.Errorf("delete the sensitive data of reservation %s of tenant %s: %w", reservation, tenant, err)
	}
	j.log.InfoContext(ctx, "sensitive data deletion answered", "tenantId", tenant, "reservationId", reservation, "outcome", outcome.Name)
	return nil
}
