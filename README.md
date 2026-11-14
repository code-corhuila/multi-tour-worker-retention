# multi-tour-worker-retention

> Worker job 6: data retention and technical purge (every domain)

Part of the **Multi Tour** distributed system — team `multi-tour`, Grupo 2.
Governance and documentation live in [`multi-tour-docs`](https://github.com/code-corhuila/multi-tour-docs).

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
