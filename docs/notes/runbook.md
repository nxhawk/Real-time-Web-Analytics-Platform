# Runbook

<Badge type="warning" text="Filled in during Level 6" />

::: info Partly written
The failure modes below are known from the design; the exact commands and the measured
recovery times are filled in when the system runs in production.
:::

What to do when something breaks. Written before the incident, not during it.

## Triage order

1. `/healthz` on both services — is the process alive?
2. `/readyz` — which dependency is unhappy?
3. Grafana **Ingest health** — are events still arriving?
4. `pulse_end_to_end_lag_seconds` — is the pipeline behind, and by how much?
5. Disk usage — ClickHouse stops accepting writes when it fills, and recovery is slow.

## Failure modes

### `Too many parts` — inserts fail with error 252

**Cause.** Too many small inserts. ClickHouse merges parts in the background and refuses new
inserts when it cannot keep up.

**Fix.** Increase `BATCH_SIZE`, or `FLUSH_INTERVAL_MS`, so each insert carries at least 10k
rows. Confirm with `SELECT table, count() FROM system.parts WHERE active GROUP BY table`.
Raising `parts_to_throw_insert` buys time but does not fix the cause.

### Consumer lag climbing

**Cause.** Consumers are slower than producers — a slow ClickHouse, a large merge, a poison
message being retried, or simply not enough consumers.

**Check.** `pulse_kafka_consumer_lag` per partition. Lag on one partition means a poison
message; lag on all of them means throughput.

**Fix.** Add consumer replicas up to the partition count. If one partition is stuck, find the
offset, and let the retry limit push the message to the DLQ rather than removing the limit.

### Disk full

**Cause.** Raw events plus projections plus Kafka retention plus Docker logs.

**Fix, in order of speed.** Reduce Kafka retention, `docker system prune -af`, check
`/data/caddy/access.log`, verify the TTL is actually applying. Alert fires at 75% precisely so
this is done before writes stop.

### ClickHouse OOM-killed

**Cause.** `max_server_memory_usage` set higher than the container's memory limit, so the
kernel kills the process before ClickHouse can refuse the query.

**Fix.** Keep the Docker limit about 10% above `max_server_memory_usage`.

### Events accepted but not visible

**Where to look, in order.** Is the WAL directory filling up (ClickHouse unreachable)? Is
`kafka_dlq_total` climbing (validation rejecting them)? Is the materialized view missing a
backfill (raw has the data, the view does not)?

### A query suddenly slow

Check `system.query_log` for `read_rows`. A jump usually means a filter stopped matching the
sort order, or a materialized view was bypassed. `EXPLAIN indexes = 1` shows whether granules
are still being skipped.

## Recovery procedures

### Replaying the WAL

The write-ahead log holds newline-delimited JSON batches that ClickHouse refused. The replay
process scans `WAL_DIR`, re-inserts, and deletes each file on success. Verify with a count
comparison before and after.

### Replaying the DLQ

`cmd/dlq-replay` reads `events.dlq`, lets you filter or fix, and produces back to
`events.raw`. Fix the cause first — replaying into the same bug just refills the DLQ.

### Restoring from a snapshot

Documented in
[`DEPLOY-AWS.md` §13](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/DEPLOY-AWS.md).
Create a volume from the snapshot, attach it to a temporary instance, compare row counts, then
swap.

::: danger Rehearse it once
A backup that has never been restored is not a backup. Level 6 requires one rehearsal, with
the **actual recovery time** recorded here. That number is what you will be asked for during a
real incident.
:::

## Operational numbers

Filled in from real measurements, not estimates:

| Measurement | Value |
|---|---|
| Restore RTO from snapshot | *(record after the rehearsal)* |
| Deploy time, tag to healthy | |
| `make up` on a clean machine | |
| Backend image size | |

## AWS-specific incidents

Symptom-to-cause table for the deployment environment — EBS queue length, SSM connectivity,
Caddy certificates, Terraform wanting to replace the instance, sudden bill increases — is in
[`DEPLOY-AWS.md` §15](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/DEPLOY-AWS.md).

## Getting a shell

```bash
# On the production host, without SSH
aws ssm start-session --target i-xxxxx

# Port-forward ClickHouse HTTP to your machine without opening the security group
aws ssm start-session --target i-xxxxx \
  --document-name AWS-StartPortForwardingSession \
  --parameters '{"portNumber":["8123"],"localPortNumber":["8123"]}'
curl localhost:8123/ping
```
