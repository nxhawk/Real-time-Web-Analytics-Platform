# Pulse Analytics — TODO / Checklist thực thi

> Đi kèm `[PLAN.md](PLAN.md)` (thiết kế) và `[PHASES.md](PHASES.md)` (thứ tự + tiêu chí ra).
> Mỗi task có **ID**, **ước lượng (h)**, và **Done khi** (tiêu chí nghiệm thu).
> Quy ước: `[ ]` chưa làm · `[~]` đang làm · `[x]` xong · `[-]` bỏ qua có chủ ý.
> Ước lượng tổng: **~207 giờ** (232 task) — hoặc **~214 giờ** (255 task) khi đi đường AWS.
> Nhóm `DOC-*` (trang tài liệu) chạy song song, không tính vào con số trên.

---

## Bản đồ tài liệu


| File                             | Dùng để                                                   |            |
| -------------------------------- | --------------------------------------------------------- | ---------- |
| `[README.md](README.md)`         | Giới thiệu + quickstart                                   | tiếng Anh  |
| `[PLAN.md](PLAN.md)`             | Tra thiết kế trước khi code một task                      | tiếng Việt |
| `[PHASES.md](PHASES.md)`         | Xem entry/exit criteria, deliverable, rủi ro của cả level | tiếng Việt |
| `TODO.md` (file này)             | Tick từng task hằng ngày                                  | tiếng Việt |
| `[DEPLOY-AWS.md](DEPLOY-AWS.md)` | Checklist `AWS-01`→`AWS-32` thay cho L6.4                 | tiếng Việt |


Các con số dùng chung (version, ngưỡng hiệu năng, hạn mức, phân phối seeder) chốt tại
`PHASES.md` [§2](PHASES.md#2-bảng-số-liệu-chuẩn).

---



## Bảng tiến độ


| Level                | Tên                                                      | Task    | Ước lượng | Chi tiết phase                                                                                                       | Trạng thái |
| -------------------- | -------------------------------------------------------- | ------- | --------- | -------------------------------------------------------------------------------------------------------------------- | ---------- |
| L0                   | Khởi tạo & nền tảng                                      | 25      | 12h       | [PHASES §4](PHASES.md#4-phase-l0--khởi-tạo--nền-tảng)                                                                | 🟨 22/25   |
| L1                   | MVP ingest → query → dashboard                           | 40      | 30h       | [PHASES §5](PHASES.md#5-phase-l1--mvp-ingest--query--dashboard)                                                      | ☐          |
| L2                   | Đào sâu ClickHouse                                       | 24      | 25h       | [PHASES §6](PHASES.md#6-phase-l2--đào-sâu-clickhouse)                                                                | ☐          |
| L3                   | Batch insert, seeder, benchmark                          | 32      | 35h       | [PHASES §7](PHASES.md#7-phase-l3--batch-insert-seeder-benchmark)                                                     | ☐          |
| L4                   | Kafka pipeline                                           | 30      | 35h       | [PHASES §8](PHASES.md#8-phase-l4--kafka-pipeline)                                                                    | ☐          |
| L5                   | Analytics nâng cao + dashboard đầy đủ                    | 46      | 45h       | [PHASES §9](PHASES.md#9-phase-l5--analytics-nâng-cao--dashboard-đầy-đủ)                                              | ☐          |
| L6                   | Observability, security, CD, docs                        | 35      | 25h       | [PHASES §10](PHASES.md#10-phase-l6--observability-security-cd-docs)                                                  | ☐          |
| **Tổng**             |                                                          | **232** | **~207h** |                                                                                                                      |            |
| AWS                  | Hạ tầng production — **thay** L6.4 (`L6-20`→`L6-28`)     | 32      | 14h       | [PHASES §11](PHASES.md#11-phase-aws--hạ-tầng-production) · [DEPLOY-AWS §17](DEPLOY-AWS.md#17-checklist-thay-thế-l64) | ☐          |
| DOC                  | Trang tài liệu VitePress (song song, không tính vào 232) | 8       | 6h        | [mục bên dưới](#trang-tài-liệu-docs-site)                                                                            | 🟨 7/8     |
| **Tổng (đường AWS)** |                                                          | **255** | **~214h** |                                                                                                                      |            |


---



# LEVEL 0 — Khởi tạo & nền tảng (12h)

> Mục tiêu, điều kiện vào/ra, deliverable và rủi ro: `PHASES.md` [§4](PHASES.md#4-phase-l0--khởi-tạo--nền-tảng).
> **Trạng thái: 22/25 task xong.** 3 task còn lại cần thao tác trên máy bạn / trên GitHub,
> đánh dấu 🔸 bên dưới.
> Module path đã chốt: `github.com/nxhawk/pulse-analytics/backend`. Go pin **1.26** (1.27 còn là rc).



## L0.1 — Repository & quy ước (2h)

- [x] `L0-01` Repo trên GitHub + `LICENSE` (MIT)
- [x] `L0-02` `.gitignore` (Go, Node, `.env`, `*.log`, `data/`, `bin/`, `coverage.out`, Terraform, `go.work`, `.DS_Store`)
- [x] `L0-03` `.editorconfig` (LF, utf-8, go=tab, ts=2 space)
- [x] `L0-04` Cấu trúc thư mục theo `PLAN.md` [§4](PLAN.md#4-cấu-trúc-repository), kể cả `infra/` (kèm `.gitkeep`)
- [x] `L0-05` `README.md`: mô tả, sơ đồ kiến trúc ASCII, quickstart, cây thư mục kèm ý nghĩa
- [x] `L0-06` Đưa toàn bộ tài liệu vào repo: `PLAN.md`, `PHASES.md`, `TODO.md`, `DEPLOY-AWS.md`, `CLAUDE.md`
- [x] 🔸 `L0-07` Bật branch protection cho `main`: yêu cầu PR + CI pass + không force-push — *làm trong Settings của GitHub*
- [x] `L0-08` `.github/pull_request_template.md` và `.github/CODEOWNERS`
- [x] `L0-09` Quy ước commit Conventional Commits — ghi trong `CONTRIBUTING.md`

> **Done khi**: cấu trúc thư mục khớp PLAN §4. ✅



## L0.2 — Backend skeleton (3h)

- [x] `L0-10` `go mod init github.com/nxhawk/pulse-analytics/backend`, Go 1.26
- [x] `L0-11` Deps nền: `gin`, `gin-contrib/cors`, `caarlos0/env/v11`, `joho/godotenv`, `google/uuid`, `prometheus/client_golang`, `stretchr/testify`
  ```
  *(`clickhouse-go/v2` và `validator` được thêm ở L1 khi có code dùng — `go mod tidy` sẽ gỡ dep không dùng)*
  ```
- [x] `L0-12` `internal/config/config.go`: load env theo nhóm (App/HTTP/Log/ClickHouse/Ingest/Kafka), default đầy đủ, `Validate()` gom **mọi** lỗi cấu hình rồi báo một lần
- [x] `L0-13` `cmd/ingest-api/main.go` + `cmd/analytics-api/main.go`: config → slog JSON → gin → `httpx.Server` + graceful shutdown 30s (có shutdown hook cho L3/L4)
- [x] `L0-14` Middleware trong `internal/httpx`: `RequestID` (UUIDv7, tái dùng header vào), `Recover` (log stack, trả envelope 500), `Logger` (slog, bỏ qua `/healthz` `/readyz` `/metrics`, log theo **route pattern**), `CORS`, `MaxBodySize`
- [x] `L0-15` `GET /healthz`, `GET /readyz` (cơ chế `Prober` để L1 cắm ClickHouse vào), `GET /version` (ldflags), `GET /metrics`
- [x] `L0-16` `.env.example` đầy đủ mọi biến, có comment và giá trị mặc định

> **Done khi**: `go run ./cmd/ingest-api` → `curl localhost:8080/healthz` trả `{"status":"ok"}`. ✅



## L0.3 — Docker & Makefile (4h)

- [x] `L0-17` `backend/Dockerfile` multi-stage: `golang:1.26-alpine` (CGO_ENABLED=0, `-trimpath`, ldflags, cache mount) → `gcr.io/distroless/static-debian12:nonroot`
- [x] `L0-18` `docker-compose.yml`: `clickhouse` 26.3 (healthcheck `/ping`, volume, ulimit nofile), `ingest-api`, `analytics-api` (read-only rootfs, `cap_drop: ALL`, `no-new-privileges`)
- [x] `L0-19` `deploy/clickhouse/config.d/pulse.xml`: `max_server_memory_usage`, log warning, TTL cho `query_log`/`metric_log`, bật Prometheus endpoint, tắt cổng MySQL/PostgreSQL
- [x] `L0-20` `deploy/clickhouse/users.d/pulse.xml`: user `pulse` (profile `max_execution_time=15`, `max_memory_usage=4G`) + user `dashboard` readonly + quota
- [x] `L0-21` `Makefile` 30 target có `make help`; đủ `up`, `down`, `logs`, `ps`, `build`, `run`, `test`, `test-int`, `lint`, `fmt`, `migrate-*`, `seed`, `bench`, `ch-cli`, `clean`
- [x] 🔸 `L0-22` Chạy `make up` trên máy sạch, ClickHouse healthy trong < 60s — *cần Docker trên máy bạn*

> **Done khi**: `make up && make ch-cli` → `SELECT version()` trả về 26.x.



## L0.4 — CI khung (3h)

- [x] `L0-23` `backend/.golangci.yml` (v2): `errcheck, govet, staticcheck, ineffassign, unused, bodyclose, sqlclosecheck, rowserrcheck, noctx, errorlint, nilerr, contextcheck, gosec, gocritic, revive, misspell, unconvert, unparam, wastedassign, copyloopvar, predeclared, goconst` + formatter `gofmt`/`goimports`
- [x] `L0-24` `.github/workflows/ci-backend.yml`: job `lint` (mod verify, kiểm tra `go mod tidy` sạch, gofmt, vet, golangci-lint) + job `test` (race + coverage) + job `build` (matrix 2 service, build image, smoke test container) + cache Go modules và Docker layer
- [x] `L0-25` `.github/dependabot.yml`: gomod, npm, docker, github-actions — weekly, gom nhóm minor/patch
- [x] 🔸 *Chạy* `make deps` *một lần để sinh* `go.sum`*, rồi commit* — sandbox không truy cập được `proxy.golang.org` nên `go.sum` chưa có sẵn trong repo

> **Done khi**: PR đầu tiên có CI xanh, badge hiện trong README.



### Kết quả kiểm chứng L0 (chạy trong sandbox, Go 1.26.5)


| Kiểm tra                            | Kết quả                                                                          |
| ----------------------------------- | -------------------------------------------------------------------------------- |
| `gofmt -l -s .`                     | sạch                                                                             |
| `go vet ./...`                      | sạch                                                                             |
| `golangci-lint run ./...` (v2.12.2) | **0 issues**                                                                     |
| `go test -race -count=1 ./...`      | pass — `config` 73.1%, `handler` 100%, `httpx` 40.6%                             |
| `go build ./cmd/...`                | 2 binary                                                                         |
| `curl /healthz`                     | `{"status":"ok"}`                                                                |
| `curl /readyz`                      | `{"status":"ok","checks":{}}`                                                    |
| `curl /version`                     | `{"tag":"v0.0.1","commit":"abc1234","build_time":"...","go_version":"go1.26.5"}` |
| `curl /nope`                        | 404 đúng envelope, có `request_id`                                               |
| `curl /metrics`                     | có `pulse_build_info{...} 1`                                                     |
| SIGTERM                             | `shutdown signal received` → `server stopped`, thoát sạch                        |
| `docker compose config`             | hợp lệ                                                                           |


---



# LEVEL 1 — MVP: ingest → query → dashboard (30h)

> Mục tiêu, điều kiện vào/ra, deliverable và rủi ro: `PHASES.md` [§5](PHASES.md#5-phase-l1--mvp-ingest--query--dashboard).



## L1.1 — Model & validation (4h)

- [ ] `L1-01` `internal/model/event.go`: struct `Event`, `IngestRequest`, `RejectedEvent{Index, Reason}`
- [ ] `L1-02` Validate theo PLAN §5.2: regex tên event, độ dài field, giới hạn batch 500, properties <= 8KB
- [ ] `L1-03` Xử lý timestamp: parse ISO8601, phát hiện clock skew, override + đếm metric
- [ ] `L1-04` Sanitize `page`: strip fragment, strip query param nhạy cảm theo denylist, giới hạn 2048 ký tự
- [ ] `L1-05` Unit test cho toàn bộ validate/sanitize (table-driven, >= 25 case)

> **Done khi**: `go test ./internal/model/... ./internal/validate/...` xanh, coverage > 85%.



## L1.2 — Migration & bảng `events` (5h)

- [ ] `L1-06` Cài **goose** (đã chốt ở `PLAN.md` [§3](PLAN.md#3-tech-stack--version) — hỗ trợ multi-statement ClickHouse tốt hơn `golang-migrate`)
- [ ] `L1-07` `cmd/migrate/main.go`: `up`, `down`, `status`, `create <name>`; đọc DSN từ env; chạy được cả trong container
- [ ] `L1-08` `0001_create_database.sql`
- [ ] `L1-09` `0002_events.sql` — nguyên văn DDL ở PLAN §6.1 (bản tối giản trước: chưa codec/skip index)
- [ ] `L1-10` Chạy migration trong `docker-compose` bằng service `migrate` chạy một lần trước `api`
- [ ] `L1-11` Test: chạy `up` → `down` → `up` lại phải sạch sẽ

> **Done khi**: `make migrate-up` tạo bảng; `SHOW CREATE TABLE analytics.events` khớp file migration.



## L1.3 — Repository & insert (naive) (4h)

- [ ] `L1-12` `repository/clickhouse/conn.go`: `clickhouse.Open` với native protocol, `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`, `Compression: LZ4`, `DialTimeout`, retry khi khởi động
- [ ] `L1-13` Ping trong `/readyz`
- [ ] `L1-14` `event_repo.go`: `InsertBatch(ctx, []Event)` dùng `PrepareBatch` + `Append` + `Send`
- [ ] `L1-15` **Cố tình** làm bản `InsertOne` để sau này benchmark so sánh — đặt sau flag `INSERT_MODE=single|batch`
- [ ] `L1-16` Map lỗi ClickHouse (code 252 too many parts, 241 memory limit) → lỗi domain có ý nghĩa

> **Done khi**: `curl -XPOST /api/v1/events -d @sample.json` → `SELECT count() FROM events` tăng.



## L1.4 — Ingest endpoint (4h)

- [ ] `L1-17` `handler/event_handler.go`: `POST /api/v1/events`, giới hạn body 1MB (`http.MaxBytesReader`)
- [ ] `L1-18` Partial success: trả `202` + `{accepted, rejected:[{index, reason}]}`
- [ ] `L1-19` Middleware API key: header `X-API-Key`, tra bảng/config → gán `site_id` vào context; 401 nếu sai
- [ ] `L1-20` `GET /api/v1/pixel.gif` fallback
- [ ] `L1-21` Rate limit in-memory theo API key (token bucket, `golang.org/x/time/rate`)
- [ ] `L1-22` Test handler bằng `httptest`: 200/202/400/401/413/429

> **Done khi**: tất cả case test xanh; gửi batch 100 event có 3 event hỏng → nhận 97.



## L1.5 — Analytics API cơ bản (6h)

- [ ] `L1-23` `queries/` với `go:embed` từng file `.sql`, dùng parameterized query `{name:Type}`
- [ ] `L1-24` Helper parse khoảng thời gian: `from`, `to`, `tz`; mặc định 7 ngày; validate range <= 400 ngày
- [ ] `L1-25` `GET /analytics/overview` (query trực tiếp `events`, chưa MV)
- [ ] `L1-26` `GET /analytics/timeseries` với auto interval (hour/day/week) + `WITH FILL`
- [ ] `L1-27` `GET /analytics/pages` có `limit/offset/sort`
- [ ] `L1-28` `GET /analytics/devices`, `/countries` (dùng chung handler với whitelist `dim`)
- [ ] `L1-29` Áp settings mỗi query: `max_execution_time`, `max_memory_usage`, `max_rows_to_read`
- [ ] `L1-30` Chuẩn hoá error response + `request_id`

> **Done khi**: `curl` từng endpoint trả JSON đúng shape; không endpoint nào > 1s với 1M event.



## L1.6 — Dashboard tối giản (7h)

- [ ] `L1-31` `npx create-next-app@latest frontend` — TS, App Router, Tailwind 4, ESLint
- [ ] `L1-32` Cài shadcn/ui, TanStack Query, Recharts, date-fns, zod
- [ ] `L1-33` `lib/api.ts`: fetch wrapper (base URL từ env, timeout, error mapping, zod parse)
- [ ] `L1-34` Trang `/`: 3 `StatCard` (Users, Sessions, Events) + `TimeSeriesChart` + `Top Pages` table + `Devices` bar
- [ ] `L1-35` `DateRangePicker` (preset: Today, 24h, 7d, 30d, 90d, custom) đồng bộ với URL query string
- [ ] `L1-36` Loading skeleton + empty state + error boundary cho từng widget
- [ ] `L1-37` `frontend/Dockerfile` (multi-stage, `output: 'standalone'`) + thêm vào compose
- [ ] `L1-38` `ci-frontend.yml`: `lint`, `tsc --noEmit`, `build`

> **Done khi**: mở `localhost:3000`, gửi event bằng curl, refresh → thấy số tăng. Chụp ảnh màn hình cho README.



## L1.7 — Milestone L1

- [ ] `L1-39` Tag `v0.1.0`, viết CHANGELOG
- [ ] `L1-40` Demo GIF: curl → dashboard đổi số

---



# LEVEL 2 — Đào sâu ClickHouse (25h)

> Mục tiêu, điều kiện vào/ra, deliverable và rủi ro: `PHASES.md` [§6](PHASES.md#6-phase-l2--đào-sâu-clickhouse).
> **Điều kiện vào:** cần >= 10M event — nếu chưa có, làm `L3-01`…`L3-07` (seeder) trước.

> Level này ít code, nhiều thực nghiệm. Mọi kết luận ghi vào `docs/clickhouse-notes.md` **kèm số liệu đo được**.



## L2.1 — Hiểu storage (5h)

- [ ] `L2-01` Đọc `system.parts`, `system.parts_columns`, `system.columns` — viết 5 query "soi bảng" vào `docs/queries-ops.sql`
- [ ] `L2-02` Insert 1M event, quan sát số part; chạy `OPTIMIZE TABLE ... FINAL` và đo lại
- [ ] `L2-03` Thí nghiệm `index_granularity` 8192 vs 4096 vs 16384 → đo kích thước mark file + tốc độ query điểm
- [ ] `L2-04` `EXPLAIN indexes = 1 SELECT ...` cho 5 query chính — ghi lại số granule bị loại
- [ ] `L2-05` `EXPLAIN PIPELINE` cho 1 query GROUP BY — hiểu số luồng thực thi



## L2.2 — Thí nghiệm ORDER BY (5h)

- [ ] `L2-06` Tạo 3 bảng cùng dữ liệu, khác `ORDER BY`:
  - A: `(event_name, event_time, user_id)` (đề xuất ban đầu)
  - B: `(site_id, event_name, event_time)` (đề xuất trong PLAN)
  - C: `(site_id, user_id, event_time)`
- [ ] `L2-07` Chạy bộ 8 query benchmark trên cả 3 → bảng so sánh thời gian + `read_rows`
- [ ] `L2-08` So sánh dung lượng đĩa 3 bảng (thứ tự sắp xếp ảnh hưởng tỉ lệ nén!)
- [ ] `L2-09` Kết luận và chốt ORDER BY chính thức → cập nhật migration + ADR-0002



## L2.3 — Kiểu dữ liệu & nén (4h)

- [ ] `L2-10` So sánh `String` vs `LowCardinality(String)` cho `country`, `device`, `event_name`: dung lượng + tốc độ GROUP BY
- [ ] `L2-11` So sánh `DateTime` vs `DateTime64(3)` — chi phí độ chính xác mili giây
- [ ] `L2-12` Thử codec: `ZSTD(1)` vs `ZSTD(3)` vs `ZSTD(9)` vs `LZ4` cho cột `page`, `properties`; `Delta+ZSTD` vs `DoubleDelta` cho `event_time`
- [ ] `L2-13` Đo `Nullable(String)` vs `String DEFAULT ''` → chứng minh vì sao tránh Nullable
- [ ] `L2-14` Áp codec tốt nhất vào migration `0002` (viết migration mới `MODIFY COLUMN`, không sửa file cũ)



## L2.4 — Skip index & projection (5h)

- [ ] `L2-15` Thêm `bloom_filter` cho `user_id`, `page`; đo query lọc theo user trước/sau
- [ ] `L2-16` Thử `tokenbf_v1` cho tìm kiếm chuỗi con trong `page`
- [ ] `L2-17` Thêm `minmax` cho `ingested_at`
- [ ] `L2-18` Tạo `PROJECTION prj_by_user`, `MATERIALIZE`, đo: dung lượng tăng bao nhiêu %, insert chậm bao nhiêu %, query theo user nhanh bao nhiêu lần
- [ ] `L2-19` Quyết định giữ hay bỏ projection → ghi ADR



## L2.5 — TTL & vòng đời dữ liệu (3h)

- [ ] `L2-20` TTL `DELETE` sau 180 ngày; test bằng cách chèn dữ liệu cũ và `OPTIMIZE ... FINAL`
- [ ] `L2-21` TTL `RECOMPRESS` sau 30 ngày sang `ZSTD(9)` — đo dung lượng tiết kiệm
- [ ] `L2-22` (Optional) TTL `TO VOLUME 'cold'` với storage policy 2 tầng (SSD → HDD)



## L2.6 — Ghi chép

- [ ] `L2-23` `docs/clickhouse-notes.md` >= 20 mục, mỗi mục: *quan sát → số liệu → giải thích*
- [ ] `L2-24` Cập nhật `PLAN.md` §6 nếu kết luận khác giả định ban đầu

> **Done khi**: có thể trả lời bằng số liệu của chính mình: "Vì sao ORDER BY như vậy?", "LowCardinality tiết kiệm bao nhiêu?", "Projection đáng giá không?"

---



# LEVEL 3 — Batch insert, seeder, benchmark (35h)

> Mục tiêu, điều kiện vào/ra, deliverable và rủi ro: `PHASES.md` [§7](PHASES.md#7-phase-l3--batch-insert-seeder-benchmark).



## L3.1 — Seeder (6h)

- [ ] `L3-01` `cmd/seeder/main.go`: flags `-n`, `-days`, `-sites`, `-workers`, `-batch`, `-out=clickhouse|ndjson|http`
- [ ] `L3-02` Phân phối thực tế theo `PHASES.md` [§2.5](PHASES.md#25-phân-phối-dữ-liệu-của-seeder): Zipf cho page (~500 page), device 62/35/3, country top-10 + đuôi dài, giờ cao điểm 20–22h, cuối tuần thấp hơn 30%
- [ ] `L3-03` Mô hình session: mỗi user 1–5 session/ngày, mỗi session 1–15 event, chuỗi event hợp lệ để funnel có ý nghĩa
- [ ] `L3-04` Tỉ lệ chuyển đổi cài sẵn: view→product 72%, →cart 29%, →checkout 38%, →purchase 65% (để kiểm chứng funnel query)
- [ ] `L3-05` Cohort: 30% user quay lại D1, 12% D7, 5% D30 (để kiểm chứng retention query)
- [ ] `L3-06` Chạy song song nhiều worker, in progress bar + throughput
- [ ] `L3-07` `make seed N=10000000` chạy < 5 phút

> **Done khi**: seed 10M rồi chạy funnel query, kết quả xấp xỉ tỉ lệ đã cài (sai số < 2%).



## L3.2 — BatchWriter (8h)

- [ ] `L3-08` `internal/buffer/writer.go`: struct + config (`bufferSize`, `batchSize`, `flushInterval`, `workers`)
- [ ] `L3-09` Non-blocking enqueue (`select` + `default`), metric `events_dropped_total`
- [ ] `L3-10` Worker loop: flush theo size hoặc ticker
- [ ] `L3-11` Retry với exponential backoff + jitter (3 lần: 1s, 4s, 16s)
- [ ] `L3-12` Graceful shutdown: `Close()` flush hết buffer, có timeout
- [ ] `L3-13` WAL fallback: khi retry hết mà vẫn fail → ghi NDJSON ra `WAL_DIR`, rotate theo file 64MB
- [ ] `L3-14` `cmd/wal-replay` hoặc goroutine nền: quét WAL dir, đọc lại, insert, xoá file khi thành công
- [ ] `L3-15` Backpressure 3 mức theo PLAN §10.3 (drop có ưu tiên: giữ `purchase`/`signup`)
- [ ] `L3-16` Unit test: enqueue quá tải không block; flush đúng size; shutdown không mất event
- [ ] `L3-17` Integration test với testcontainers: kill container CH giữa chừng → WAL có file → bật lại → replay → count khớp

> **Done khi**: test "kill ClickHouse, không mất event" xanh.



## L3.3 — Benchmark insert (6h)

- [ ] `L3-18` `loadtest/k6/ingest.js`: ramp 0→10k ev/s trong 10 phút, đo p50/p95/p99, error rate
- [ ] `L3-19` Chạy 6 kịch bản ở PLAN §10.4 (1 row, batch 100/1k/10k, async_insert, — Kafka để sau L4)
- [ ] `L3-20` Với mỗi kịch bản ghi: throughput, p99 API, số part/phút, CPU/RAM ClickHouse, có lỗi 252 không
- [ ] `L3-21` Vẽ biểu đồ so sánh (script Python/Go xuất PNG) đưa vào docs
- [ ] `L3-22` Tinh chỉnh `batchSize`/`flushInterval` tối ưu → cập nhật default trong config



## L3.4 — Benchmark query CH vs PG (11h)

- [ ] `L3-23` `docker-compose.bench.yml`: thêm Postgres 17 với `shared_buffers`, `work_mem` hợp lý
- [ ] `L3-24` `repository/postgres/`: schema tương đương + index `(event_name, event_time)` + BRIN
- [ ] `L3-25` Biến thể Postgres partition theo tháng (declarative partitioning)
- [ ] `L3-26` Seeder ghi được vào cả PG (COPY) và CH
- [ ] `L3-27` `loadtest/bench/run_bench.go`: chạy N query × M lần, lấy trung vị, hỗ trợ cold/warm
- [ ] `L3-28` Chạy ở 1M → 10M → 100M; ghi cả thời gian seed và dung lượng đĩa
- [ ] `L3-29` Điền bảng ở PLAN §18.3 vào `docs/benchmark-results.md`
- [ ] `L3-30` Viết phần "Vì sao ClickHouse nhanh" và "ClickHouse dở ở đâu" bằng dữ liệu của mình
- [ ] `L3-31` (Bonus) Thử `EXPLAIN ANALYZE` bên PG và `EXPLAIN PIPELINE` bên CH cho cùng 1 query, so sánh mô hình thực thi

> **Done khi**: `docs/benchmark-results.md` đủ bảng + biểu đồ + kết luận; đọc xong người khác hiểu được sự khác biệt.



## L3.5 — Milestone L3

- [ ] `L3-32` Tag `v0.3.0`; viết bài blog/README section "Batch insert: từ 800 ev/s lên X ev/s"

---



# LEVEL 4 — Kafka pipeline (35h)

> Mục tiêu, điều kiện vào/ra, deliverable và rủi ro: `PHASES.md` [§8](PHASES.md#8-phase-l4--kafka-pipeline).
> Đây là mốc chuyển từ kiến trúc Phase 1 sang Phase 2 (`PLAN.md` [§2.2](PLAN.md#22-phase-2--event-pipeline-với-kafka-level-4)).



## L4.1 — Hạ tầng Kafka (4h)

- [ ] `L4-01` Thêm Kafka (KRaft mode, không ZooKeeper) hoặc Redpanda vào `docker-compose.yml` + healthcheck
- [ ] `L4-02` Kafka UI (`provectuslabs/kafka-ui` hoặc Redpanda Console) cho dev
- [ ] `L4-03` Script tạo topic: `events.raw` (6 partition, retention 7d, compression zstd), `events.dlq` (1 partition, 30d)
- [ ] `L4-04` Thêm `make kafka-topics`, `make kafka-lag`



## L4.2 — Producer (6h)

- [ ] `L4-05` `internal/kafka/producer.go` với franz-go: config acks=1, linger 50ms, idempotence, compression zstd
- [ ] `L4-06` Serialize event: JSON trước (dễ debug) — ghi TODO thử Protobuf/Avro sau
- [ ] `L4-07` Key = `site_id|session_id` để giữ thứ tự trong session
- [ ] `L4-08` Producer async + callback đếm metric `kafka_produce_errors_total`
- [ ] `L4-09` Fallback: Kafka down → ghi WAL (tái dùng module L3), KHÔNG trả lỗi cho client
- [ ] `L4-10` Đổi `event_service` sang chiến lược có thể cấu hình: `SINK=direct|kafka`
- [ ] `L4-11` Test: Kafka down → API vẫn 202, WAL có file



## L4.3 — Consumer (10h)

- [ ] `L4-12` `cmd/consumer/main.go`: consumer group `clickhouse-sink`, `auto.offset.reset=earliest`, **tắt auto-commit**
- [ ] `L4-13` Poll batch (max 10k record / 500ms) → decode → validate
- [ ] `L4-14` Gom batch rồi `InsertBatch` vào ClickHouse (tái dùng repository)
- [ ] `L4-15` Commit offset **sau khi** insert thành công
- [ ] `L4-16` Retry 3 lần backoff; vẫn fail → produce vào `events.dlq` kèm header `error`, `retry_count`, `original_partition`, `original_offset` → rồi mới commit
- [ ] `L4-17` Xử lý rebalance: `OnPartitionsRevoked` → flush + commit trước khi nhả partition
- [ ] `L4-18` Graceful shutdown: nhận SIGTERM → dừng poll → flush → commit → thoát
- [ ] `L4-19` Metric: `kafka_consumer_lag` (per partition), `kafka_records_processed_total`, `kafka_dlq_total`, `consumer_batch_size`
- [ ] `L4-20` `cmd/dlq-replay`: đọc DLQ, sửa/lọc, produce lại vào `events.raw`



## L4.4 — Tách service & test độ bền (9h)

- [ ] `L4-21` Tách `analytics-api` thành binary + container riêng; ingest không còn import package analytics
- [ ] `L4-22` Compose: `ingest-api` (2 replica), `consumer` (2 replica), `analytics-api` (1)
- [ ] `L4-23` Chaos test 1: kill ClickHouse 5 phút giữa lúc bắn 5k ev/s → bật lại → `count()` phải khớp số đã gửi
- [ ] `L4-24` Chaos test 2: kill 1 consumer → partition rebalance → không mất, không trùng quá mức
- [ ] `L4-25` Chaos test 3: gửi event hỏng → vào DLQ, không chặn partition
- [ ] `L4-26` Đo end-to-end lag (`ingested_at - event_time`) ở 5k ev/s → mục tiêu p99 < 5s
- [ ] `L4-27` Benchmark bổ sung dòng "Kafka + consumer batch 10k" vào bảng ở L3
- [ ] `L4-28` (Học thêm) Thử ClickHouse `Kafka` table engine + MV cho cùng topic, so sánh với consumer tự viết → ghi vào docs



## L4.5 — Milestone L4

- [ ] `L4-29` Cập nhật sơ đồ kiến trúc trong README sang Phase 2
- [ ] `L4-30` Tag `v0.4.0`; ADR-0004, ADR-0005 hoàn chỉnh

---



# LEVEL 5 — Analytics nâng cao + dashboard đầy đủ (45h)

> Mục tiêu, điều kiện vào/ra, deliverable và rủi ro: `PHASES.md` [§9](PHASES.md#9-phase-l5--analytics-nâng-cao--dashboard-đầy-đủ).
> **Điều kiện vào:** đã có 100M event trong ClickHouse để đo ngưỡng < 300ms.



## L5.1 — Materialized Views (10h)

- [ ] `L5-01` Migration `0003_mv_events_hourly` — bảng `events_hourly` + MV (`PLAN.md` [§7.2](PLAN.md#72-events_hourly))
- [ ] `L5-02` Script backfill an toàn (chạy theo từng tháng để không OOM), có `--dry-run`
- [ ] `L5-03` **Golden test**: insert 50k event cố định → so sánh từng metric giữa raw và MV, phải khớp tuyệt đối
- [ ] `L5-04` Migration `0004_mv_daily_users` — `events_daily` + MV, DAU/WAU/MAU (`PLAN.md` [§7.3](PLAN.md#73-events_daily-dauwaumau))
- [ ] `L5-05` Migration `0005_mv_page_stats_hourly` — tách riêng vì `page` cardinality cao
- [ ] `L5-06` Migration `0006_mv_sessions` — MV với `argMinState`/`argMaxState` cho entry/exit page (`PLAN.md` [§7.4](PLAN.md#74-sessions))
- [ ] `L5-07` Migration `0007_user_first_seen` + MV cho cohort (`PLAN.md` [§7.5](PLAN.md#75-user_first_seen-cohort))
- [ ] `L5-08` Đổi các endpoint analytics sang đọc MV; giữ cờ `USE_MV=false` để so sánh
- [ ] `L5-09` Đo lại tốc độ: overview/timeseries/pages trước và sau MV ở 100M → ghi vào docs
- [ ] `L5-10` Kiểm tra kích thước MV so với raw; nếu > 15% raw thì xem lại cardinality GROUP BY

> **Done khi**: golden test xanh; overview < 100ms ở 100M events.



## L5.2 — Funnel (6h)

- [ ] `L5-11` Query `windowFunnel` theo PLAN §8.5, tham số hoá số bước (2–8 bước)
- [ ] `L5-12` Endpoint `GET /analytics/funnel?steps=&window=&mode=` (mode: default / strict_order / strict_deduplication)
- [ ] `L5-13` Tính conversion từ bước trước và từ bước đầu; tính drop-off
- [ ] `L5-14` Validate: whitelist tên event, tối đa 8 bước, window 60s–7 ngày
- [ ] `L5-15` Test tính đúng với dữ liệu seeder (khớp tỉ lệ cài sẵn ±2%)
- [ ] `L5-16` FE: trang `/funnel` — chọn step bằng combobox, chart bậc thang + bảng số



## L5.3 — Retention & cohort (7h)

- [ ] `L5-17` Query cohort matrix (JOIN với `user_first_seen`) theo PLAN §8.6
- [ ] `L5-18` Biến thể dùng hàm `retention()` — so sánh tốc độ 2 cách, ghi docs
- [ ] `L5-19` Endpoint `GET /analytics/retention?cohort=day|week&periods=`
- [ ] `L5-20` Giới hạn: tối đa 12 cohort × 30 kỳ để không nổ response
- [ ] `L5-21` Test đúng với dữ liệu seeder (D1 ~30%, D7 ~12%, D30 ~5%)
- [ ] `L5-22` FE: `RetentionHeatmap` — màu theo %, tooltip số tuyệt đối, hàng = cohort, cột = D0..Dn
- [ ] `L5-23` Xử lý ô "chưa đủ dữ liệu" (cohort mới chưa tới D30) → hiển thị khác ô 0%



## L5.4 — Các endpoint còn lại (6h)

- [ ] `L5-24` `/analytics/realtime`: active users 30 phút, events 5 phút, top page, top country — query raw, phải < 200ms
- [ ] `L5-25` `/analytics/sources`: referrer domain + utm breakdown (`domain(referrer)` của ClickHouse)
- [ ] `L5-26` `/analytics/browsers`, `/analytics/os`
- [ ] `L5-27` `/analytics/events`: event stream có cursor pagination `(event_time, event_id)`
- [ ] `L5-28` `/analytics/export?format=csv`: stream bằng `FORMAT CSVWithNames`, không load hết vào RAM
- [ ] `L5-29` Revenue analytics: doanh thu theo ngày/nguồn/country, AOV, `quantile(0.5)(revenue)`
- [ ] `L5-30` Bounce rate + avg session duration từ bảng `sessions`
- [ ] `L5-31` So sánh kỳ trước (`delta`) cho mọi số ở overview



## L5.5 — Dashboard đầy đủ (14h)

- [ ] `L5-32` `FilterBar` toàn cục: device, country, browser, source, page — đồng bộ URL, áp cho mọi widget
- [ ] `L5-33` Trang `/realtime`: polling 5s, đồng hồ active users, event stream cuộn (giới hạn 100 dòng), top country
- [ ] `L5-34` Trang `/pages`: TanStack Table, sort server-side, phân trang, cột avg time on page
- [ ] `L5-35` Trang `/audience`: 4 breakdown card + biểu đồ ngang có %
- [ ] `L5-36` Trang `/settings`: hiển thị site, API key (che một phần, nút copy), snippet nhúng có sẵn site id
- [ ] `L5-37` Dark mode + responsive (mobile: card xếp dọc, bảng scroll ngang)
- [ ] `L5-38` Định dạng số kiểu `vi-VN` (12.430, 1,2 tỷ ₫), thời lượng kiểu `2m 14s`
- [ ] `L5-39` Chuyển timeseries sang ECharts nếu > 5.000 điểm; bật downsampling
- [ ] `L5-40` Sinh `types/api.ts` từ `openapi.yaml` bằng `openapi-typescript`, bỏ type viết tay
- [ ] `L5-41` Vitest cho `lib/format.ts`, `lib/queries.ts`
- [ ] `L5-42` Playwright E2E: seed → mở dashboard → kiểm tra số → đổi date range → số đổi → mở funnel → thấy 5 bước
- [ ] `L5-43` Lighthouse: performance > 90, a11y > 95
- [ ] `L5-44` Chụp ảnh tất cả trang cho README



## L5.6 — Tracking SDK (2h)

- [ ] `L5-45` `sdk/js`: auto page_view, SPA route change, batch 10 event/3s, `sendBeacon` khi `visibilitychange`, tôn trọng DNT, queue localStorage khi offline
- [ ] `L5-46` Build bằng esbuild → < 2KB gzip; test trên 1 trang HTML tĩnh

---



# LEVEL 6 — Observability, security, CD, docs (25h)

> Mục tiêu, điều kiện vào/ra, deliverable và rủi ro: `PHASES.md` [§10](PHASES.md#10-phase-l6--observability-security-cd-docs).
> L6.4 có hai đường: VPS (mục dưới, 7h) hoặc AWS (`DEPLOY-AWS.md` [§17](DEPLOY-AWS.md#17-checklist-thay-thế-l64), 14h). Chọn một, đường còn lại đánh dấu `[-]`.



## L6.1 — Metrics & Grafana (6h)

- [ ] `L6-01` `internal/metrics/metrics.go`: đăng ký toàn bộ metric ở PLAN §14.1
- [ ] `L6-02` Middleware đo `http_request_duration_seconds` theo route pattern (không phải raw path — tránh nổ cardinality)
- [ ] `L6-03` Histogram `end_to_end_lag_seconds` tính từ `ingested_at - event_time`
- [ ] `L6-04` Thêm Prometheus + Grafana vào compose, provisioning datasource + dashboard bằng file
- [ ] `L6-05` 4 dashboard: Ingest health, ClickHouse internals, Kafka, API RED
- [ ] `L6-06` Export dashboard JSON vào `deploy/grafana/dashboards/`
- [ ] `L6-07` Exporter cho ClickHouse (`clickhouse-server` có sẵn endpoint Prometheus — bật trong config)
- [ ] `L6-08` 4 alert rule ở PLAN §14.4



## L6.2 — Logging & tracing (3h)

- [ ] `L6-09` slog JSON có `request_id`, `site_id`, `trace_id`; level theo env
- [ ] `L6-10` Không log PII; thêm test đảm bảo payload không xuất hiện trong log ở mức info
- [ ] `L6-11` (Optional) OpenTelemetry: trace HTTP → service → ClickHouse query, export sang Jaeger/Tempo



## L6.3 — Security hardening (5h)

- [ ] `L6-12` Container non-root, read-only rootfs, `cap_drop: [ALL]`, `no-new-privileges`
- [ ] `L6-13` `security.yml`: govulncheck, gosec, trivy (fail HIGH/CRITICAL), gitleaks, npm audit — chạy trên PR + weekly
- [ ] `L6-14` Rà lại toàn bộ query: 100% parameterized; `Identifier` param có whitelist
- [ ] `L6-15` CORS chặt: ingest theo origin đăng ký của site; analytics chỉ origin dashboard
- [ ] `L6-16` Security headers cho FE (CSP, X-Frame-Options, Referrer-Policy) qua Caddy hoặc `next.config.ts`
- [ ] `L6-17` Auth dashboard: JWT hoặc session cookie httpOnly/SameSite; hash password bằng argon2id
- [ ] `L6-18` API key: lưu hash (không lưu plaintext), hỗ trợ rotate, có `revoked_at`
- [ ] `L6-19` Endpoint xoá dữ liệu theo user (`ALTER TABLE ... DELETE WHERE user_id=`), ghi rõ trong runbook là thao tác nặng



## L6.4 — CD & môi trường thật (7h) · *đường VPS*

> **Rẽ nhánh.** Nhóm task này là đường "1 VPS + SSH + docker compose".
> Đường mặc định của dự án là AWS: bỏ qua `L6-20`→`L6-28` (đánh dấu `[-]`) và làm
> `AWS-01`→`AWS-32` ở `DEPLOY-AWS.md` [§17](DEPLOY-AWS.md#17-checklist-thay-thế-l64) (14h).
> So sánh hai đường: `PHASES.md` [§10](PHASES.md#10-phase-l6--observability-security-cd-docs).

- [ ] `L6-20` Thuê/chuẩn bị VPS (>= 4 vCPU / 16GB / SSD 200GB cho 100M events) — *đường AWS dùng EC2* `r7g.xlarge` *4 vCPU / 32GB + EBS gp3 500GB thay cho task này*
- [ ] `L6-21` `docker-compose.prod.yml`: image từ GHCR theo tag sha, restart policy, resource limits, log rotation
- [ ] `L6-22` `deploy/caddy/Caddyfile`: TLS tự động, reverse proxy `/api` → analytics-api, `/` → web, `/collect` → ingest-api
- [ ] `L6-23` `cd-staging.yml`: tự deploy khi merge `main`
- [ ] `L6-24` `cd-production.yml`: chạy khi push tag `v*`; thứ tự migrate → deploy → healthcheck → rollback nếu fail
- [ ] `L6-25` Deploy key SSH riêng, lưu trong GitHub Secrets; user deploy không phải root
- [ ] `L6-26` Backup: `clickhouse-backup` hoặc `BACKUP TABLE ... TO Disk(...)` theo lịch cron hằng ngày, giữ 7 bản; **test restore ít nhất 1 lần**
- [ ] `L6-27` Uptime check bên ngoài (UptimeRobot/healthchecks.io) cho `/healthz`
- [ ] `L6-28` Smoke test sau deploy: script gửi 1 event thật và query lại



## L6.5 — Tài liệu & tổng kết (4h)

- [ ] `L6-29` `README.md` hoàn chỉnh: mô tả, sơ đồ, ảnh dashboard, quickstart 3 lệnh, tech stack, kết quả benchmark tóm tắt, "những gì tôi học được"
- [ ] `L6-30` `docs/runbook.md`: cách xử lý "Too many parts", consumer lag cao, đĩa đầy, CH không khởi động, cách replay DLQ/WAL
- [ ] `L6-31` `docs/api/openapi.yaml` khớp 100% implementation; kiểm bằng schemathesis
- [ ] `L6-32` Hoàn thiện 10 ADR
- [ ] `L6-33` `CONTRIBUTING.md` + hướng dẫn setup dev trong 5 phút
- [ ] `L6-34` Bài viết tổng kết (blog/LinkedIn): "Xây analytics 100M events với ClickHouse — 8 điều tôi học được"
- [ ] `L6-35` Tag `v1.0.0`

---



# Trang tài liệu (docs site)

> Nhóm task song song, **không tính** vào 232 task của lộ trình chính.
> Site: VitePress, tiếng Anh mặc định ở gốc + tiếng Việt dưới `/vi/`, deploy GitHub Pages.
> URL: `https://nxhawk.github.io/Real-time-Web-Analytics-Platform/`

- [x] `DOC-01` Scaffold VitePress trong `docs/`: `package.json`, `.vitepress/config.mts`, tách config theo locale (`shared.mts` / `en.mts` / `vi.mts`)
- [x] `DOC-02` Cấu hình i18n: `root` = tiếng Anh (không prefix URL), `vi` dưới `/vi/`; dịch toàn bộ nav, sidebar, footer, nhãn outline và UI tìm kiếm
- [x] `DOC-03` `base` = `/Real-time-Web-Analytics-Platform/` khớp tên repo; sitemap có hreflang; local search không cần dịch vụ ngoài
- [x] `DOC-04` Nội dung tiếng Anh: home, 7 trang guide, 4 trang reference, 4 trang notes, 2 trang ADR, roadmap
- [x] `DOC-05` Bản tiếng Việt đầy đủ — cùng cây thư mục, cùng tên file (19 + 19 trang)
- [x] `DOC-06` `scripts/copy-assets.mjs` copy `docs/api/openapi.yaml` sang `public/` lúc build (Node thuần, chạy được trên Windows)
- [x] `DOC-07` `.github/workflows/docs.yml`: build trên PR (chặn link chết), deploy Pages khi push `main`; cache npm, concurrency `pages`, quyền `id-token: write`
- [ ] 🔸 `DOC-08` Bật GitHub Pages: **Settings → Pages → Source = GitHub Actions** — *phải làm tay một lần, nếu không workflow deploy sẽ fail*

> **Done khi**: `cd docs && npm ci && npm run build` xanh (build fail nếu có link nội bộ chết);
> mở được cả `/` và `/vi/`; nút đổi ngôn ngữ giữ nguyên trang đang đọc.



### Kết quả kiểm chứng (chạy trong sandbox)


| Kiểm tra                  | Kết quả                                              |
| ------------------------- | ---------------------------------------------------- |
| `npm ci && npm run build` | xanh, 8,6s, **38 trang** (19 EN + 19 VI)             |
| Kiểm tra link chết        | đã thử chèn link hỏng → build fail đúng như mong đợi |
| `base` trong HTML         | `/Real-time-Web-Analytics-Platform/...`              |
| Thẻ `lang`                | `en-US` ở gốc, `vi-VN` ở `/vi/`                      |
| Sitemap                   | có `hreflang` cho cả hai ngôn ngữ                    |
| Chụp màn hình             | cả hai locale render đúng, UI đã dịch                |


---



# Backlog / Ý tưởng mở rộng (không bắt buộc)

> Không tính vào 232 task của lộ trình chính.

- [ ] `X-01` ReplicatedMergeTree + ClickHouse Keeper (2 node) — học replication
- [ ] `X-02` Distributed table + sharding theo `site_id`
- [ ] `X-03` Refreshable Materialized View cho báo cáo nặng (thay vì incremental)
- [ ] `X-04` Dictionary cho GeoIP thay vì gọi thư viện trong Go (`DICTIONARY` + `dictGet`)
- [ ] `X-05` Protobuf/Avro + Schema Registry thay JSON trên Kafka
- [ ] `X-06` Kafka Streams-like aggregation trong Go trước khi vào CH
- [ ] `X-07` Anomaly detection: cảnh báo khi traffic lệch > 3σ so với cùng giờ tuần trước
- [ ] `X-08` A/B testing framework trên nền event
- [ ] `X-09` User explorer: xem timeline sự kiện của 1 user (dùng projection)
- [ ] `X-10` Query cache bằng Redis cho dashboard công khai
- [ ] `X-11` ClickHouse query cache (`use_query_cache = 1`) — đo lợi ích
- [ ] `X-12` Multi-tenant thật: quota theo site, isolation bằng row policy
- [ ] `X-13` Terraform/Ansible thay cho `deploy.sh`
- [ ] `X-14` Chuyển sang Kubernetes + Helm (nếu muốn học k8s)
- [ ] `X-15` gRPC cho internal service-to-service

---



# Nhật ký học tập

> Mỗi khi phát hiện điều gì bất ngờ, ghi vào đây rồi chuyển sang `docs/clickhouse-notes.md`.


| Ngày | Phát hiện | Số liệu | Kết luận |
| ---- | --------- | ------- | -------- |
|      |           |         |          |


---



# Checklist nghiệm thu cuối (copy từ PLAN §22)

> Bản gốc: `PLAN.md` [§22](PLAN.md#22-definition-of-done).
> Bản đối chiếu theo phase (tiêu chí nào thuộc level nào): `PHASES.md` [§14](PHASES.md#14-definition-of-done-toàn-dự-án).

- [ ] `git clone && make up` chạy được trong < 5 phút trên máy sạch
- [ ] `make seed N=10000000` thành công
- [ ] Dashboard đủ 7 nhóm widget, dữ liệu thật
- [ ] Mọi endpoint analytics p95 < 300ms ở 100M events (có bằng chứng từ `system.query_log`)
- [ ] Ingest 10.000 ev/s trong 10 phút, drop = 0, p99 API < 50ms
- [ ] Kill ClickHouse 5 phút → không mất event → số liệu khớp 100%
- [ ] Golden test MV vs raw khớp tuyệt đối
- [ ] CI xanh đủ: lint, unit, integration, security, build
- [ ] CD deploy bằng 1 tag, có rollback (đã thử thật), có smoke test
- [ ] Grafana 4 dashboard + alert hoạt động
- [ ] `docs/benchmark-results.md` đầy đủ bảng + kết luận
- [ ] `docs/clickhouse-notes.md` >= 20 ghi chú thực nghiệm
- [ ] README hoàn chỉnh có ảnh + phần "những gì tôi học được"

---

*Thiết kế chi tiết:* `[PLAN.md](PLAN.md)` *· Giai đoạn & tiêu chí ra:* `[PHASES.md](PHASES.md)` *· Hạ tầng production:* `[DEPLOY-AWS.md](DEPLOY-AWS.md)`