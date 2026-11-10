// Package http is the output adapter that calls commercial-reservation on its internal routes.
package http

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/code-corhuila/multi-tour-worker-runtime/httpclient"

	"github.com/code-corhuila/multi-tour-worker-retention/internal/application/port/out"
)

const (
	listPath     = "/api/v1/commercial-reservation/internal/tenants/%s/sensitive-data-retention/due?limit=%d"
	deletionPath = "/api/v1/commercial-reservation/internal/tenants/%s/reservations/%s/sensitive-data-deletion"
)

var uuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Reservations calls commercial-reservation through the client of the runtime.
type Reservations struct {
	client *httpclient.Client
}

var _ out.Reservations = (*Reservations)(nil)

// NewReservations returns the adapter over a client whose base URL is commercial-reservation.
func NewReservations(client *httpclient.Client) *Reservations { return &Reservations{client: client} }

type dueItem struct {
	ReservationID string `json:"reservationId"`
}

type dueList struct {
	Data []dueItem `json:"data"`
	Next string    `json:"next"`
}

type deletionResult struct {
	Outcome string `json:"outcome"`
}

// ListDue reads one page of GET .../sensitive-data-retention/due: ids and a cursor, never a datum.
func (r *Reservations) ListDue(ctx context.Context, tenantID string, limit int, after string) (out.DuePage, error) {
	if !uuid.MatchString(tenantID) || limit < 1 || (after != "" && !uuid.MatchString(after)) {
		return out.DuePage{}, fmt.Errorf("a tenant uuid, a positive limit and a uuid cursor are required")
	}
	path := fmt.Sprintf(listPath, tenantID, limit)
	if after != "" {
		path += "&after=" + after
	}
	var got dueList
	if err := r.client.Get(ctx, path, &got); err != nil {
		return out.DuePage{}, fmt.Errorf("list the due reservations: %w", err)
	}
	if got.Next != "" && !uuid.MatchString(got.Next) {
		return out.DuePage{}, fmt.Errorf("list the due reservations: the cursor is not a uuid")
	}
	page := out.DuePage{Reservations: make([]string, 0, len(got.Data)), Next: got.Next}
	for _, item := range got.Data {
		if !uuid.MatchString(item.ReservationID) {
			return out.DuePage{}, fmt.Errorf("list the due reservations: an item has no reservation uuid")
		}
		page.Reservations = append(page.Reservations, item.ReservationID)
	}
	return page, nil
}

// Delete is a POST with an Idempotency-Key, so the runtime may repeat it after a network failure, a 429 or a 5xx.
func (r *Reservations) Delete(ctx context.Context, tenantID, reservationID, idempotencyKey string) (out.Outcome, error) {
	if !uuid.MatchString(tenantID) || !uuid.MatchString(reservationID) {
		return out.Outcome{}, fmt.Errorf("a tenant uuid and a reservation uuid are required")
	}
	var result deletionResult
	err := r.client.Do(ctx, httpclient.Request{
		Method:         http.MethodPost,
		Path:           fmt.Sprintf(deletionPath, tenantID, reservationID),
		IdempotencyKey: idempotencyKey,
	}, &result)
	if err != nil {
		return out.Outcome{}, fmt.Errorf("delete the sensitive data: %w", err)
	}
	return out.Outcome{Name: result.Outcome}, nil
}
