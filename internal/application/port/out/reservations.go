// Package out holds the ports the job needs from the outside world.
package out

import "context"

// DuePage is one page of the reservations whose sensitive data is due, as the service names them (ids only). Next is the cursor of the following page, empty at the end.
type DuePage struct {
	Reservations []string
	Next         string
}

// Outcome is what the service decided when asked to delete the sensitive data of a reservation: DELETED, NOT_DUE, NO_ANCHOR or ALREADY_DELETED.
type Outcome struct {
	Name string
}

// Reservations is commercial-reservation as the job sees it: the application never learns that it is HTTP.
type Reservations interface {
	// ListDue reads up to limit due reservations of a tenant after the cursor (empty for the first page). The service lists an inactive tenant too.
	ListDue(ctx context.Context, tenantID string, limit int, after string) (DuePage, error)
	// Delete asks the service to delete the sensitive data of one reservation. The service decides and rechecks the year; it is safe to repeat with the same key.
	Delete(ctx context.Context, tenantID, reservationID, idempotencyKey string) (Outcome, error)
}
