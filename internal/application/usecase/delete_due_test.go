package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	workerruntime "github.com/code-corhuila/multi-tour-worker-runtime"

	"github.com/code-corhuila/multi-tour-worker-retention/internal/application/port/out"
	"github.com/code-corhuila/multi-tour-worker-retention/internal/application/usecase"
)

const (
	tenantA = "11111111-1111-4111-8111-111111111111"
	tenantB = "22222222-2222-4222-8222-222222222222"
)

type deletion struct{ tenant, reservation, key string }

// fake is commercial-reservation as the job sees it: the due reservations of each tenant in order, read by cursor and limit, and the deletion.
type fake struct {
	mu       sync.Mutex
	due      map[string][]string
	listErr  map[string]error
	deleteBy func(reservation string) (out.Outcome, error)
	asked    []string // "tenant limit after"
	calls    []deletion
}

func (f *fake) ListDue(_ context.Context, tenant string, limit int, after string) (out.DuePage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, fmt.Sprintf("%s %d %s", tenant, limit, after))
	if err := f.listErr[tenant]; err != nil {
		return out.DuePage{}, err
	}
	all := f.due[tenant]
	from := 0
	if after != "" {
		for i, id := range all {
			if id == after {
				from = i + 1
			}
		}
	}
	rest := all[from:]
	if len(rest) <= limit {
		return out.DuePage{Reservations: rest}, nil
	}
	return out.DuePage{Reservations: rest[:limit], Next: rest[limit-1]}, nil
}

func (f *fake) Delete(_ context.Context, tenant, reservation, key string) (out.Outcome, error) {
	f.mu.Lock()
	f.calls = append(f.calls, deletion{tenant, reservation, key})
	f.mu.Unlock()
	if f.deleteBy != nil {
		return f.deleteBy(reservation)
	}
	return out.Outcome{Name: "DELETED"}, nil
}

func job(f *fake, tenants ...string) *usecase.DeleteDue {
	return usecase.NewDeleteDue(f, tenants, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

var _ workerruntime.Job = (*usecase.DeleteDue)(nil)

// HU-WRK-006: it asks for the due reservations of each tenant and asks the service to delete the sensitive data of each one.
func TestHU_WRK_006_asks_the_deletion_of_every_due_reservation(t *testing.T) {
	f := &fake{due: map[string][]string{tenantA: {"r1", "r2"}, tenantB: {"r3"}}}

	processed, failed := job(f, tenantA, tenantB).Run(context.Background(), 100)

	if processed != 3 || failed != 0 {
		t.Fatalf("processed=%d failed=%d, want 3 and 0", processed, failed)
	}
	if len(f.calls) != 3 || f.calls[0].tenant != tenantA || f.calls[2].tenant != tenantB {
		t.Fatalf("calls = %+v", f.calls)
	}
}

// The key names the reservation only: the record of the deletion is permanent, so a retry or a later run repeats the same call and the service answers it again.
func TestHU_WRK_006_the_key_names_the_reservation_and_is_the_same_in_every_run(t *testing.T) {
	f := &fake{due: map[string][]string{tenantA: {"r1"}}}
	j := job(f, tenantA)

	j.Run(context.Background(), 100)
	j.Run(context.Background(), 100)

	if len(f.calls) != 2 || f.calls[0].key != f.calls[1].key || !strings.Contains(f.calls[0].key, "r1") || len(f.calls[0].key) < 8 {
		t.Fatalf("keys = %+v", f.calls)
	}
}

// The list pages by cursor and the batch bounds the work: what does not fit waits for the next run.
func TestHU_WRK_006_follows_the_cursor_until_the_batch_is_full(t *testing.T) {
	var many []string
	for i := 1; i <= 150; i++ {
		many = append(many, fmt.Sprintf("r%d", i))
	}
	f := &fake{due: map[string][]string{tenantA: many}}

	processed, _ := job(f, tenantA).Run(context.Background(), 120)

	if processed != 120 || len(f.asked) != 2 || f.asked[0] != tenantA+" 100 " || f.asked[1] != tenantA+" 20 r100" {
		t.Fatalf("processed=%d asked=%v", processed, f.asked)
	}
}

// A service that answers NOT_DUE, NO_ANCHOR or ALREADY_DELETED is an answer, not a failure: the service decides.
func TestHU_WRK_006_a_service_answer_other_than_deleted_is_not_a_failure(t *testing.T) {
	f := &fake{due: map[string][]string{tenantA: {"r1", "r2"}}, deleteBy: func(r string) (out.Outcome, error) {
		if r == "r1" {
			return out.Outcome{Name: "NOT_DUE"}, nil
		}
		return out.Outcome{Name: "ALREADY_DELETED"}, nil
	}}

	processed, failed := job(f, tenantA).Run(context.Background(), 100)

	if processed != 2 || failed != 0 {
		t.Fatalf("processed=%d failed=%d", processed, failed)
	}
}

// One failure never stops the others, and a tenant whose list fails is counted and the next tenant is still swept.
func TestHU_WRK_006_a_failure_does_not_stop_the_batch_nor_the_other_tenants(t *testing.T) {
	f := &fake{due: map[string][]string{tenantA: {"r1", "r2"}, tenantB: {"r3"}},
		listErr: map[string]error{tenantA: errors.New("unavailable")}}

	processed, failed := job(f, tenantA, tenantB).Run(context.Background(), 100)
	if processed != 1 || failed != 1 {
		t.Fatalf("list failure: processed=%d failed=%d", processed, failed)
	}

	g := &fake{due: map[string][]string{tenantA: {"r1", "r2"}}, deleteBy: func(r string) (out.Outcome, error) {
		if r == "r1" {
			return out.Outcome{}, errors.New("boom")
		}
		return out.Outcome{Name: "DELETED"}, nil
	}}
	processed, failed = job(g, tenantA).Run(context.Background(), 100)
	if processed != 1 || failed != 1 {
		t.Fatalf("delete failure: processed=%d failed=%d", processed, failed)
	}
}

func TestHU_WRK_006_stops_when_the_context_is_done(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &fake{due: map[string][]string{tenantA: {"r1"}}}

	processed, failed := job(f, tenantA).Run(ctx, 100)

	if processed != 0 || failed != 0 || len(f.calls) != 0 {
		t.Fatalf("processed=%d failed=%d calls=%v", processed, failed, f.calls)
	}
}
