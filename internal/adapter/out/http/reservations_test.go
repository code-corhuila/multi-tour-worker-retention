package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/code-corhuila/multi-tour-worker-runtime/config"
	"github.com/code-corhuila/multi-tour-worker-runtime/correlation"
	"github.com/code-corhuila/multi-tour-worker-runtime/httpclient"

	reservationshttp "github.com/code-corhuila/multi-tour-worker-retention/internal/adapter/out/http"
)

const (
	tenant      = "11111111-1111-4111-8111-111111111111"
	reservation = "33333333-3333-4333-8333-333333333333"
	cursor      = "44444444-4444-4444-8444-444444444444"
	token       = "service-token-value"
	base        = "/api/v1/commercial-reservation/internal/tenants/" + tenant
)

type seen struct {
	method, path, query, auth, correlation, key string
}

// stub is commercial-reservation: it records each request and answers with the next scripted reply.
type stub struct {
	mu      sync.Mutex
	seen    []seen
	replies []func(w http.ResponseWriter)
}

func (s *stub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.seen = append(s.seen, seen{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"),
		r.Header.Get(correlation.Header), r.Header.Get("Idempotency-Key")})
	reply := s.replies[min(len(s.seen), len(s.replies))-1]
	s.mu.Unlock()
	reply(w)
}

func write(status int, body string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func adapter(t *testing.T, replies ...func(http.ResponseWriter)) (*reservationshttp.Reservations, *stub) {
	t.Helper()
	s := &stub{replies: replies}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	client := httpclient.New(srv.URL, config.Secret(token), httpclient.WithAttempts(3),
		httpclient.WithSleeper(func(context.Context, time.Duration) error { return nil }))
	return reservationshttp.NewReservations(client), s
}

func run() context.Context { return correlation.WithID(context.Background(), "run-0000-aaaa") }

const dueList = `{"data":[{"reservationId":"` + reservation + `","customerId":"` + cursor + `","anchorBasis":"SERVICE_END","anchorAt":"2025-01-10T10:00:00Z"}],"next":"` + cursor + `"}`

// HU-WRK-006: it reads the {data, next} list of the tenant of the path with the token and the run's correlation id, and follows the cursor it is given.
func TestListDue_reads_the_due_list_and_the_cursor_with_the_token_and_the_correlation_id(t *testing.T) {
	a, s := adapter(t, write(200, dueList), write(200, `{"data":[]}`))

	first, err := a.ListDue(run(), tenant, 50, "")
	second, err2 := a.ListDue(run(), tenant, 50, cursor)

	if err != nil || err2 != nil || len(first.Reservations) != 1 || first.Reservations[0] != reservation || first.Next != cursor || len(second.Reservations) != 0 || second.Next != "" {
		t.Fatalf("got %+v %v / %+v %v", first, err, second, err2)
	}
	r := s.seen[0]
	if r.method != "GET" || r.path != base+"/sensitive-data-retention/due" || r.query != "limit=50" {
		t.Errorf("request = %+v", r)
	}
	if s.seen[1].query != "limit=50&after="+cursor {
		t.Errorf("second query = %q", s.seen[1].query)
	}
	if r.auth != "Bearer "+token || r.correlation != "run-0000-aaaa" {
		t.Errorf("credentials = %q, correlation %q", r.auth, r.correlation)
	}
}

// HU-WRK-006: the deletion is a POST with an Idempotency-Key and the token, and its answer says what the service decided.
func TestDelete_posts_with_the_key_and_reads_what_the_service_decided(t *testing.T) {
	a, s := adapter(t, write(200, `{"outcome":"DELETED","companionRowsDeleted":2,"customerDataDeleted":true}`), write(200, `{"outcome":"ALREADY_DELETED","companionRowsDeleted":0,"customerDataDeleted":false}`))

	first, err1 := a.Delete(run(), tenant, reservation, "retention-key-0001")
	second, err2 := a.Delete(run(), tenant, reservation, "retention-key-0001")

	if err1 != nil || err2 != nil || first.Name != "DELETED" || second.Name != "ALREADY_DELETED" {
		t.Fatalf("got %+v %v / %+v %v", first, err1, second, err2)
	}
	r := s.seen[0]
	if r.method != "POST" || r.path != base+"/reservations/"+reservation+"/sensitive-data-deletion" || r.key != "retention-key-0001" {
		t.Errorf("request = %+v", r)
	}
	if r.auth != "Bearer "+token || r.correlation != "run-0000-aaaa" {
		t.Errorf("credentials = %q, correlation %q", r.auth, r.correlation)
	}
}

// Annex D: 5xx and 429 are retried up to the limit with the same key; any other 4xx is not retried.
func TestDelete_retries_a_503_with_the_same_key_and_does_not_retry_a_4xx(t *testing.T) {
	a, s := adapter(t, write(503, `{"error":"UNAVAILABLE","message":"x","traceId":"t1"}`), write(429, `{"error":"TOO_MANY","message":"x","traceId":"t2"}`),
		write(200, `{"outcome":"DELETED"}`))

	got, err := a.Delete(run(), tenant, reservation, "retention-key-0001")

	if err != nil || got.Name != "DELETED" || len(s.seen) != 3 || s.seen[0].key != s.seen[2].key {
		t.Fatalf("got %+v %v after %d calls", got, err, len(s.seen))
	}
	for _, status := range []int{401, 403, 404} {
		b, seenByB := adapter(t, write(status, `{"error":"X","message":"no","traceId":"t"}`))
		if _, err := b.Delete(run(), tenant, reservation, "retention-key-0001"); err == nil || len(seenByB.seen) != 1 {
			t.Errorf("%d: error %v after %d calls, want one call and a failure", status, err, len(seenByB.seen))
		}
	}
}

// An id that is not a UUID never reaches a URL path, and a body that is not the contract is an error that does not echo it.
func TestAdapter_refuses_an_id_that_is_not_a_uuid_and_a_body_that_is_not_the_contract(t *testing.T) {
	a, s := adapter(t, write(200, dueList))

	_, err1 := a.Delete(run(), tenant, "../../x", "retention-key-0001")
	_, err2 := a.ListDue(run(), "acme", 10, "")
	_, err3 := a.ListDue(run(), tenant, 0, "")
	_, err4 := a.ListDue(run(), tenant, 10, "not-a-cursor")

	if err1 == nil || err2 == nil || err3 == nil || err4 == nil || len(s.seen) != 0 {
		t.Errorf("errors %v %v %v %v, calls %d, want four refusals and no call", err1, err2, err3, err4, len(s.seen))
	}
	bad, _ := adapter(t, write(200, `{"data":[{"reservationId":"nope"}]}`))
	if _, err := bad.ListDue(run(), tenant, 10, ""); err == nil || strings.Contains(err.Error(), "nope") {
		t.Errorf("error = %v, want a refusal that does not echo the body", err)
	}
}
