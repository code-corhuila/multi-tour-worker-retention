# multi-tour-worker-retention

> Worker job 6: data retention and technical purge (every domain)

Part of the **Multi Tour** distributed system — team `multi-tour`, Grupo 2.
Governance and documentation live in [`multi-tour-docs`](https://github.com/code-corhuila/multi-tour-docs).

## What it is

Job 6 of ADR-004 D3: data retention and technical purge. Once a day (`RETENTION_INTERVAL`, default 24h), for each tenant it sweeps, it asks `commercial-reservation` for the reservations whose sensitive data is due (`GET /api/v1/commercial-reservation/internal/tenants/{tenantId}/sensitive-data-retention/due`, paged) and asks it to delete each one (`POST .../reservations/{reservationId}/sensitive-data-deletion`, with an `Idempotency-Key`). The service decides what is due and deletes; the worker holds no retention rule, has no database and exposes only `/health`. Active and inactive tenants are both swept.

## How to start it

Copy `.env.example`, fill `SERVICE_TOKEN` (never versioned) and, optionally, `TENANT_IDS`. When `TENANT_IDS` is empty the job lists the tenants from identity-audit (`IDENTITY_AUDIT_BASE_URL`, `GET /api/v1/identity-audit/internal/tenants?status=ALL`). The worker joins the external `platform` network and publishes no port, so `deploy/compose.yml` is run from the root composition of `multi-tour-infra-postgres`, which also provides commercial-reservation. The configuration is read at start and a bad variable stops the worker naming it.

## How to test it

`gofmt -l .` must print nothing, then `go vet ./...` and `go test ./...` (Go 1.26). The same run in `ci.yml` on pull requests to `develop` and `qa`. The unit tests use stubs of the service and need no database, no network and no Docker.

## Branching

Three permanent branches. **None of them accepts a direct commit** — you enter through a child
branch and leave through a Pull Request.

```
develop  <--PR--  feat/... fix/... chore/...
qa       <--PR--  qa/...
main     <--PR--  release/...  hotfix/...
```

Promotion happens **by re-application** (`git cherry-pick -x`), never by merging one permanent
branch into another: `merge develop -> qa` and `merge qa -> main` do not exist in this model.

`main` requires **1 approval from `ariel5253`**. On `develop` and `qa` the team sets its own review
rule.

Full policy: `00-governance/branching-policy.md` in `multi-tour-docs`.

## What it does

Job 6 of the platform, once a day (`RETENTION_INTERVAL`, 24 hours by default): per tenant (the active and the inactive ones, read from `identity-audit` unless `TENANT_IDS` is set) it asks `commercial-reservation` for the
reservations whose sensitive data is more than a year old and asks it to delete each one. The service decides what is due and
deletes; the job holds no retention rule. It reads its configuration from the environment (`.env.example`) and a bad value stops it
at start. The technical purge of other domains is not done here (declared debt).
