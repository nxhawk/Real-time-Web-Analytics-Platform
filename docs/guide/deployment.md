# Deployment

<Badge type="warning" text="Level 6 / AWS phase" />

Production is **Next.js on Vercel** plus **a single EC2 `r7g.xlarge`** in `ap-southeast-1`
running the whole backend under Docker Compose. Infrastructure is Terraform, TLS is automatic
via Caddy, images go through ECR, and deployment runs over SSM without an SSH key.

The complete guide — Terraform modules, cloud-init, RAM budget, ClickHouse and Kafka tuning
for one host, CI/CD, backup and restore, cost monitoring, teardown — is in
[`DEPLOY-AWS.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/DEPLOY-AWS.md).
This page is the summary.

## Topology

```
Browser ──┬─▶ Vercel            app.pulse.dev      Next.js dashboard, SSR + CDN
          │
          └─▶ EC2 r7g.xlarge    api.pulse.dev      Caddy :443, automatic TLS
                 ├── /collect ──▶ ingest-api  :8080
                 ├── /api     ──▶ analytics-api :8081
                 ├── Kafka :9092  ──▶ consumer
                 ├── ClickHouse :9000 ──▶ EBS gp3 500 GB, 6000 IOPS
                 └── Prometheus + Grafana        grafana.pulse.dev

Security group opens 80 and 443 only. Shell access is SSM, never SSH.
```

## The decisions and what they cost

| Decision | Why | What you accept |
|---|---|---|
| One EC2 for everything | Cheapest, least to operate, enough for 100M events | Kafka and Go compete with ClickHouse for CPU and page cache, so **benchmark numbers are pessimistic** — say so in the report |
| Public subnet, no NAT Gateway | A NAT Gateway costs ~$35/month for nothing here | A public IP, offset by a tight security group and SSM |
| SSM instead of SSH | No keys to manage, no port 22, audit logs | Requires the SSM agent and an IAM role |
| Caddy instead of an ALB | An ALB is ~$20/month plus LCU charges | No load-balancer health checks, no autoscaling |
| EBS gp3 instead of instance store | Data survives stop/start | 3–5× slower than local NVMe |
| ECR instead of GHCR | Same-region pulls, no egress charge | One more service, ~$0.10/GB/month |
| arm64 (Graviton) | ~20% cheaper, everything supports it | CI must build for arm64 |

## Cost

Roughly **$267/month** on-demand in `ap-southeast-1` — the instance is $174 of it. A one-year
compute savings plan cuts about 30%; stopping the instance outside working hours cuts about
60%. Spot is not an option: reclamation mid-merge corrupts data.

Set an AWS Budget with alerts **before creating any resource**. It is the first task in the
AWS checklist for a reason.

## Deployment flow

```
push tag v*  or  manual dispatch
        │
        ▼  GitHub OIDC → AWS role (no static credentials in Secrets)
  build arm64 image → push to ECR with an immutable sha tag
        │
        ▼  SSM send-command
  docker compose pull
  docker compose run --rm migrate up      ← migrations before the app changes
  docker compose up -d --no-deps ingest-api analytics-api consumer
  healthcheck loop: curl /readyz, 30 attempts, 2s apart
        │
        └─ failure ──▶ roll back to the previous tag
```

Migrations are always backward compatible by one step: add a column, then read and write it,
then drop the old one in a later release. In ClickHouse `ALTER TABLE ADD COLUMN` is
metadata-only and therefore cheap; `DROP COLUMN` and `MODIFY COLUMN` rewrite data.

## Before the first deployment

Do not deploy what you cannot see. Metrics (L6.1), logging (L6.2) and security hardening
(L6.3) come first — otherwise the first production incident is debugged blind.

The gate list, in short:

- `terraform plan` on a second run reports **no changes**
- SSM session works; ports 8123, 9000 and 9092 are unreachable from the internet
- Caddy has certificates for `api.*` and `grafana.*`
- A deliberately broken image fails the smoke test and **rolls back automatically**
- A snapshot has been restored at least once, with the real RTO written into
  `docs/runbook.md`

Backups that have never been restored are not backups.

## The alternative

`PLAN.md` §17.4–17.5 describes a plain "single VPS plus SSH plus Docker Compose" path, 7 hours
instead of 14. `DEPLOY-AWS.md` supersedes it. Pick one and mark the other `[-]` in `TODO.md`
so the checklist does not imply both are pending.
