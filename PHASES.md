# Pulse Analytics — PHASES: Chi tiết các giai đoạn triển khai

> Kế hoạch triển khai theo **phase = level (L0 → L6)** + **phase phụ AWS**.
> Đây là tài liệu **điều phối**: nó nối `PLAN.md` (thiết kế) với `TODO.md` (checklist)
> và `DEPLOY-AWS.md` (hạ tầng thật), trả lời câu hỏi *"làm theo thứ tự nào, khi nào
> được coi là xong, xong thì có gì trong tay"*.
> Phiên bản tài liệu: 1.0 — 2026-08-12

---

## Bản đồ tài liệu

| File | Vai trò | Khi nào đọc |
|---|---|---|
| [`README.md`](README.md) | Giới thiệu, quickstart, tổng quan API | Người mới vào repo |
| [`PLAN.md`](PLAN.md) | **Đặc tả kỹ thuật** — kiến trúc, schema, DDL, query, contract, ADR | Trước khi code một hạng mục |
| [`PHASES.md`](PHASES.md) | **Giai đoạn triển khai** — thứ tự, entry/exit, deliverable, rủi ro | Đầu mỗi phase & khi review |
| [`TODO.md`](TODO.md) | **Checklist thực thi** — task ID, ước lượng, "Done khi" | Hằng ngày, khi tick việc |
| [`DEPLOY-AWS.md`](DEPLOY-AWS.md) | Hạ tầng production: Vercel + EC2 + Terraform | Phase AWS (thay L6.4) |
| [`CLAUDE.md`](CLAUDE.md) | Quy ước cho AI coding agent | Khi dùng agent sinh code |

Thứ tự ưu tiên khi tài liệu mâu thuẫn:
**`DEPLOY-AWS.md` (phần deploy)** → **`PLAN.md`** → **`PHASES.md`** → **`TODO.md`** → code.
Phát hiện mâu thuẫn thì sửa tài liệu cấp cao trước, đừng để code "âm thầm đúng".

---

## Mục lục

1. [Quy ước](#1-quy-ước)
2. [Bảng số liệu chuẩn](#2-bảng-số-liệu-chuẩn)
3. [Tổng quan các phase](#3-tổng-quan-các-phase)
4. [Phase L0 — Khởi tạo & nền tảng](#4-phase-l0--khởi-tạo--nền-tảng)
5. [Phase L1 — MVP: ingest → query → dashboard](#5-phase-l1--mvp-ingest--query--dashboard)
6. [Phase L2 — Đào sâu ClickHouse](#6-phase-l2--đào-sâu-clickhouse)
7. [Phase L3 — Batch insert, seeder, benchmark](#7-phase-l3--batch-insert-seeder-benchmark)
8. [Phase L4 — Kafka pipeline](#8-phase-l4--kafka-pipeline)
9. [Phase L5 — Analytics nâng cao + dashboard đầy đủ](#9-phase-l5--analytics-nâng-cao--dashboard-đầy-đủ)
10. [Phase L6 — Observability, security, CD, docs](#10-phase-l6--observability-security-cd-docs)
11. [Phase AWS — Hạ tầng production](#11-phase-aws--hạ-tầng-production)
12. [Ma trận truy vết](#12-ma-trận-truy-vết)
13. [Đường tắt khi thiếu thời gian](#13-đường-tắt-khi-thiếu-thời-gian)
14. [Definition of Done toàn dự án](#14-definition-of-done-toàn-dự-án)

---

## 1. Quy ước

### 1.1 Cấu trúc mỗi phase

Mỗi phase dưới đây được mô tả theo đúng 8 mục, không thiếu mục nào:

| Mục | Ý nghĩa |
|---|---|
| **Mục tiêu** | Kết quả cuối cùng, viết bằng 1–3 câu |
| **Điều kiện vào (Entry)** | Phải đúng *trước khi* bắt đầu — nếu chưa đúng thì đang làm sai thứ tự |
| **Phạm vi công việc** | Bảng nhóm việc ↔ task ID trong `TODO.md` ↔ mục tham chiếu trong `PLAN.md` |
| **Deliverable** | File/artifact cụ thể tồn tại trong repo sau phase |
| **Tiêu chí ra (Exit)** | Điều kiện **đo được** để tick phase là xong |
| **Chỉ số cần đo** | Con số phải ghi lại, dùng để so sánh ở phase sau |
| **Rủi ro** | Cạm bẫy hay gặp + cách phòng, tham chiếu [`PLAN.md` §21](PLAN.md#21-rủi-ro--cạm-bẫy-thường-gặp) |
| **Milestone** | Tag git + demo chứng minh |

### 1.2 Trạng thái

Dùng chung với `TODO.md`: `[ ]` chưa làm · `[~]` đang làm · `[x]` xong · `[-]` bỏ qua có chủ ý.

### 1.3 Task ID

- `L<n>-<nn>` — task trong `TODO.md`, ví dụ `L3-08`.
- `AWS-<nn>` — task trong [`DEPLOY-AWS.md` §17](DEPLOY-AWS.md#17-checklist-thay-thế-l64).
- `X-<nn>` — backlog mở rộng, không tính vào tiến độ.

### 1.4 Nguyên tắc chuyển phase

1. **Không nhảy phase để né việc khó.** L2 và L3 là phần "đắt giá" nhất về kiến thức; bỏ chúng thì project chỉ còn là CRUD.
2. **Không mở phase mới khi phase trước còn nợ deliverable tài liệu.** Số đo không ghi lại = không tồn tại.
3. **Mỗi phase kết thúc bằng một tag git.** Tag là mốc rollback và là mốc để viết bài tổng kết.
4. **Mỗi phase phải demo được cho người ngoài.** Không demo được nghĩa là chưa xong.

---

## 2. Bảng số liệu chuẩn

> Đây là **nguồn sự thật duy nhất** cho các con số lặp lại ở nhiều file.
> Khi đổi một giá trị ở đây, phải sửa đồng thời trong `PLAN.md`, `TODO.md`, `README.md`.

### 2.1 Version công nghệ

| Thành phần | Version chốt | Tham chiếu |
|---|---|---|
| Go | **1.26** | [`PLAN.md` §3](PLAN.md#3-tech-stack--version) — 1.27 vẫn ở mức rc tại thời điểm chốt |
| ClickHouse | **26.3 LTS** | như trên |
| Kafka | **4.x (KRaft)** — dev có thể thay Redpanda | như trên |
| Next.js / React | **16.3 / 19** | như trên |
| Migration tool | **goose** (đã chốt, không dùng golang-migrate) | [`TODO.md` L1-06](TODO.md#l12--migration--bảng-events-5h) |
| Kafka client Go | `twmb/franz-go` | [`PLAN.md` §3](PLAN.md#3-tech-stack--version) |
| Registry image | **GHCR** cho dev/CI · **ECR** cho production AWS | [`DEPLOY-AWS.md` §10](DEPLOY-AWS.md#10-cicd-github-actions--ecr--ssm) |
| Node.js | **22 LTS** | dashboard và trang tài liệu |
| Trang tài liệu | **VitePress 1.6** | `docs/`, song ngữ EN (mặc định) + VI, deploy GitHub Pages |

### 2.2 Ngưỡng hiệu năng (hard requirement)

| Chỉ số | Ngưỡng | Đo ở phase |
|---|---|---|
| Dashboard query p95 @ 100M events | **< 300 ms** | L5 |
| `/analytics/overview` sau MV @ 100M | **< 100 ms** | L5 |
| `/analytics/realtime` | **< 200 ms** | L5 |
| Ingest throughput chịu tải | **10.000 event/s trong 10 phút, drop = 0** | L3 |
| Ingest p99 latency API | **< 50 ms** | L3 |
| End-to-end lag p99 (`ingested_at − event_time`) @ 5k ev/s | **< 5 s** | L4 |
| Seed 10M event | **< 5 phút** | L3 |
| `make up` trên máy sạch | **< 5 phút**, ClickHouse healthy < 60 s | L0 |
| Kích thước MV so với raw | **≤ 15%** | L5 |
| SDK `pulse.js` | **< 2 KB gzip** | L5 |

### 2.3 Hạn mức API

| Hạn mức | Giá trị | Tham chiếu |
|---|---|---|
| Batch ingest | 1–500 event, body ≤ 1 MB | [`PLAN.md` §5.2](PLAN.md#52-quy-tắc-validate) |
| `properties` | ≤ 8 KB sau serialize | như trên |
| Range truy vấn tối đa | 400 ngày | [`PLAN.md` §12.5](PLAN.md#125-giới-hạn) |
| `limit` tối đa | 1000 | như trên |
| Rate limit ingest | 1000 req/phút/API key | như trên |
| Rate limit analytics | 120 req/phút/IP | như trên |
| Guard mỗi query | `max_execution_time = 15`, `max_memory_usage = 4GB` | như trên |
| Timezone mặc định | `Asia/Ho_Chi_Minh` (dữ liệu lưu UTC) | [`PLAN.md` §12.2](PLAN.md#122-analytics-đều-yêu-cầu-x-api-key-hoặc-session-cookie-của-dashboard) |
| `event` (tên event) | <= 64 ký tự, `^[a-z0-9_]{1,64}$` | [`PLAN.md` §5.2](PLAN.md#52-quy-tắc-validate) |
| `user_id`, `session_id` | <= 128 ký tự | như trên |
| `page`, `referrer` | <= 2048 ký tự | như trên |
| `city` | <= 128 ký tự | `internal/validate` (L1.1) |
| `os`, `browser`, `utm_*` | <= 64 ký tự (bảo vệ từ điển `LowCardinality`) | như trên |
| `revenue` | `Decimal(18, 4)` — <= 14 chữ số nguyên, <= 4 chữ số thập phân | [`PLAN.md` §6.1](PLAN.md#61-bảng-raw-events) |

### 2.4 Tham số vận hành mặc định

| Tham số | Giá trị | Ghi chú |
|---|---|---|
| `BATCH_SIZE` | 5000 | Tinh chỉnh lại ở L3.3 dựa trên số đo thật |
| `FLUSH_INTERVAL_MS` | 500 | như trên |
| Consumer poll batch | 10.000 record / 500 ms | L4 |
| Kafka `events.raw` | 6 partition, retention 7 ngày, nén zstd | L4 |
| Kafka `events.dlq` | 1 partition, retention 30 ngày | L4 |
| Retry batch writer | 3 lần — 1 s, 4 s, 16 s + jitter | L3 |
| TTL raw events | DELETE sau 180 ngày, RECOMPRESS ZSTD(9) sau 30 ngày | L2 |
| Polling dashboard | 10 s (trang thường) · 5 s (trang `/realtime`) | L1 / L5 |

### 2.5 Phân phối dữ liệu của seeder

Dùng thống nhất ở L3 (seeder), L5 (kiểm chứng funnel/retention) và mọi bảng benchmark:

| Chiều | Phân phối chốt |
|---|---|
| Page | Zipf trên ~500 page |
| Device | **desktop 62% · mobile 35% · tablet 3%** |
| Country | top-10 + đuôi dài |
| Giờ cao điểm | 20–22h; cuối tuần thấp hơn 30% |
| Session | 1–5 session/user/ngày, 1–15 event/session |
| Funnel | view→product 72% · →cart 29% · →checkout 38% · →purchase 65% |
| Retention | D1 ~30% · D7 ~12% · D30 ~5% |

---

## 3. Tổng quan các phase

### 3.1 Bảng tổng hợp

| Phase | Tên | Task | Ước lượng | Kiến trúc | Tag khi xong |
|---|---|---|---|---|---|
| **L0** | Khởi tạo & nền tảng | 25 | 12h | — | — |
| **L1** | MVP: ingest → query → dashboard | 40 | 30h | Phase 1 (monolith) | `v0.1.0` |
| **L2** | Đào sâu ClickHouse | 24 | 25h | Phase 1 | — |
| **L3** | Batch insert, seeder, benchmark | 32 | 35h | Phase 1 | `v0.3.0` |
| **L4** | Kafka pipeline | 30 | 35h | Phase 2 (event-driven) | `v0.4.0` |
| **L5** | Analytics nâng cao + dashboard đầy đủ | 46 | 45h | Phase 2 | — |
| **L6** | Observability, security, CD, docs | 35 | 25h | Phase 2 | `v1.0.0` |
| | **Tổng lộ trình chính** | **232** | **~207h** | | |
| **AWS** | Hạ tầng production (thay L6.4) | 32 | 14h | Phase 2 | — |
| | **Tổng khi đi đường AWS** | **255** | **~214h** | | |
| **DOC** | Trang tài liệu VitePress — song song, không tính vào tổng | 8 | 6h | — | — |

> Đường AWS **thay thế** L6-20 → L6-28 (9 task, 7h) bằng 32 task AWS (14h):
> `232 − 9 + 32 = 255 task`, `207 − 7 + 14 = 214 giờ`.

### 3.2 Hai kiến trúc, bảy phase

```
   Phase 1 — Monolith                        Phase 2 — Event pipeline
   (PLAN §2.1)                               (PLAN §2.2)
   ├── L0  nền tảng                          ├── L4  Kafka + tách binary
   ├── L1  MVP                               ├── L5  MV + analytics nâng cao
   ├── L2  ClickHouse deep dive              └── L6  observability + CD + docs
   └── L3  batch insert + benchmark               └── AWS  hạ tầng thật
```

### 3.3 Sơ đồ phụ thuộc

```
L0 ──▶ L1 ──┬──▶ L2 ──┐
            │         ├──▶ L3 ──▶ L4 ──▶ L5 ──▶ L6 ──▶ AWS
            └─────────┘
                 ▲                        │
                 └── L2 có thể chạy song song với L3.1 (seeder)
                     vì L2 cần dữ liệu thật để thí nghiệm
```

Ràng buộc cứng:

- **L2 cần dữ liệu** → làm `L3-01…L3-07` (seeder) *trước* hoặc *song song* với L2 nếu muốn thí nghiệm ở mức 10M+.
- **L4 cần BatchWriter của L3** → `internal/buffer` được consumer tái sử dụng.
- **L5 cần MV** → mọi endpoint nâng cao đọc từ MV; không làm L5 khi L2 chưa chốt `ORDER BY`.
- **AWS cần L6.1–L6.3** → deploy khi chưa có metrics/security là deploy mù.

---

## 4. Phase L0 — Khởi tạo & nền tảng

**Ước lượng:** 12h · **Task:** `L0-01` → `L0-25` · **Checklist:** [`TODO.md` LEVEL 0](TODO.md#level-0--khởi-tạo--nền-tảng-12h)

### Mục tiêu

Dựng bộ khung để mọi phase sau chỉ việc thêm code: repo có quy ước, backend chạy được, Docker Compose lên được ClickHouse, CI xanh. Chưa có nghiệp vụ nào.

### Điều kiện vào

- Đã đọc [`PLAN.md` §1–§4](PLAN.md#1-mục-tiêu--phạm-vi) và hiểu mục tiêu học tập quan trọng hơn mục tiêu sản phẩm.
- Máy có Docker + Compose v2, Go 1.26, Node 22+, `make`.

### Phạm vi công việc

| Nhóm | Task | Tham chiếu PLAN | Giờ |
|---|---|---|---|
| L0.1 Repository & quy ước | `L0-01`…`L0-09` | [§4 Cấu trúc repository](PLAN.md#4-cấu-trúc-repository) | 2h |
| L0.2 Backend skeleton | `L0-10`…`L0-16` | [§9.1 Luồng](PLAN.md#91-luồng), [§9.4 Graceful shutdown](PLAN.md#94-graceful-shutdown) | 3h |
| L0.3 Docker & Makefile | `L0-17`…`L0-22` | [§4](PLAN.md#4-cấu-trúc-repository) | 4h |
| L0.4 CI khung | `L0-23`…`L0-25` | [§17.2 `ci-backend.yml`](PLAN.md#172-ci-backendyml) | 3h |

### Deliverable

- Cây thư mục khớp [`PLAN.md` §4](PLAN.md#4-cấu-trúc-repository), kể cả `infra/` (rỗng, cho phase AWS).
- `README.md`, `PLAN.md`, `TODO.md`, `PHASES.md`, `DEPLOY-AWS.md`, `CLAUDE.md` đã nằm trong repo.
- `Makefile` với 15 target ở [`README.md` § Common Make targets](README.md#common-make-targets).
- `docker-compose.yml` (ClickHouse + ingest-api + analytics-api), `.env.example` đầy đủ.
- `.github/workflows/ci-backend.yml`, `.golangci.yml`, `dependabot.yml`.

### Tiêu chí ra

- [ ] `make up` trên máy sạch → ClickHouse healthy trong **< 60 s**.
- [ ] `make ch-cli` → `SELECT version()` trả `26.x`.
- [ ] `go run ./cmd/ingest-api` → `curl localhost:8080/healthz` trả `{"status":"ok"}`.
- [ ] `GET /readyz` trả 503 khi ClickHouse tắt, 200 khi bật.
- [ ] `GET /version` trả commit sha + build time nhúng bằng ldflags.
- [ ] PR đầu tiên có CI xanh, badge hiện trong README.
- [ ] Branch protection `main` đã bật: yêu cầu PR + CI pass + cấm force-push.

### Chỉ số cần đo

| Chỉ số | Ghi vào |
|---|---|
| Thời gian `make up` từ đầu | `docs/runbook.md` |
| Kích thước image backend sau multi-stage | `docs/runbook.md` |

### Rủi ro

| Rủi ro | Phòng ngừa |
|---|---|
| Cấu trúc thư mục đẻ ra tuỳ tiện ở các phase sau | Tạo đủ thư mục + `.gitkeep` ngay từ L0-04 |
| CI chậm dần vì không cache | Cache Go modules + Docker layer ngay từ đầu (`L0-24`) |
| `.env` bị commit | `.gitignore` + `gitleaks` (bật ở L6, nhưng thêm rule sớm) |

### Milestone

Không tag. Bằng chứng: ảnh chụp CI xanh + output `make up`.

---

## 5. Phase L1 — MVP: ingest → query → dashboard

**Ước lượng:** 30h · **Task:** `L1-01` → `L1-40` · **Checklist:** [`TODO.md` LEVEL 1](TODO.md#level-1--mvp-ingest--query--dashboard-30h)

### Mục tiêu

Đường đi trọn vẹn đầu tiên: `curl` gửi event → ClickHouse lưu → API trả số → dashboard hiển thị. Cố tình làm **naive** (insert từng row) để L3 có mốc so sánh.

### Điều kiện vào

- L0 đã đạt toàn bộ tiêu chí ra.
- Đã đọc [`PLAN.md` §5 Event schema](PLAN.md#5-event-schema--contract) và [§6.1 Bảng raw `events`](PLAN.md#61-bảng-raw-events).

### Phạm vi công việc

| Nhóm | Task | Tham chiếu PLAN | Giờ |
|---|---|---|---|
| L1.1 Model & validation | `L1-01`…`L1-05` | [§5.1](PLAN.md#51-payload-từ-client), [§5.2](PLAN.md#52-quy-tắc-validate) | 4h |
| L1.2 Migration & bảng `events` | `L1-06`…`L1-11` | [§6.1](PLAN.md#61-bảng-raw-events) | 5h |
| L1.3 Repository & insert naive | `L1-12`…`L1-16` | [§9.2](PLAN.md#92-ingest-handler-rút-gọn), [§10.1](PLAN.md#101-vì-sao-không-insert-từng-row) | 4h |
| L1.4 Ingest endpoint | `L1-17`…`L1-22` | [§12.1](PLAN.md#121-ingest), [§5.3](PLAN.md#53-enrichment-phía-server) | 4h |
| L1.5 Analytics API cơ bản | `L1-23`…`L1-30` | [§8.1–8.4](PLAN.md#8-analytics-query-cookbook), [§12.2](PLAN.md#122-analytics-đều-yêu-cầu-x-api-key-hoặc-session-cookie-của-dashboard) | 6h |
| L1.6 Dashboard tối giản | `L1-31`…`L1-38` | [§13.1–13.3](PLAN.md#13-frontend-design-nextjs) | 7h |
| L1.7 Milestone | `L1-39`, `L1-40` | — | — |

### Deliverable

- Migration `0001_create_database`, `0002_events` (bản tối giản, **chưa** codec/skip index — để L2 chứng minh giá trị).
- Endpoint hoạt động: `POST /api/v1/events`, `GET /api/v1/pixel.gif`, `GET /analytics/overview|timeseries|pages|devices|countries`.
- Dashboard `/` với 3 StatCard + time series + top pages + devices bar.
- `frontend/Dockerfile`, `ci-frontend.yml`.
- Ảnh chụp màn hình dashboard cho README.

### Tiêu chí ra

- [ ] Gửi batch 100 event có 3 event hỏng → nhận **97**, trả `202` kèm `rejected: [...]` ([`PLAN.md` §5.2](PLAN.md#52-quy-tắc-validate)).
- [ ] Sai `X-API-Key` → 401; body > 1 MB → 413; vượt rate limit → 429.
- [ ] `go test ./internal/model/... ./internal/validate/...` xanh, coverage **> 85%**.
- [ ] `make migrate-up` → `SHOW CREATE TABLE analytics.events` khớp file migration; `up → down → up` sạch.
- [ ] Mọi endpoint analytics trả đúng shape JSON ở [`PLAN.md` §12.2](PLAN.md#122-analytics-đều-yêu-cầu-x-api-key-hoặc-session-cookie-của-dashboard), **không endpoint nào > 1 s ở 1M event**.
- [ ] Mở `localhost:3000`, gửi event bằng curl, refresh → số tăng.
- [ ] Date range picker đồng bộ với URL query string (share link được, back/forward hoạt động).

### Chỉ số cần đo

| Chỉ số | Vì sao quan trọng | Ghi vào |
|---|---|---|
| Throughput insert naive (event/s) | Mốc gốc để L3 so sánh — con số "trước" của bài "từ X lên Y ev/s" | `docs/benchmark-results.md` |
| Số part sinh ra sau 100k event insert từng row | Chứng cứ cho lỗi `Too many parts` | `docs/clickhouse-notes.md` |
| Thời gian mỗi endpoint ở 1M event | Mốc so sánh cho L5 (sau MV) | `docs/benchmark-results.md` |

### Rủi ro

| Rủi ro | Dấu hiệu | Phòng ngừa |
|---|---|---|
| Nhảy sang tối ưu quá sớm | Muốn viết BatchWriter ngay ở L1 | Giữ `InsertOne` sau cờ `INSERT_MODE=single\|batch` (`L1-15`) — mốc so sánh quý hơn tốc độ |
| Timezone lẫn lộn | Số liệu lệch 7 tiếng | Lưu UTC tuyệt đối, convert bằng `toTimeZone` ở tầng query |
| Validation lỏng | Event rác vào bảng | Table-driven test ≥ 25 case (`L1-05`) |
| `SELECT *` trong query analytics | Chậm gấp nhiều lần | Luôn liệt kê cột — review checklist ở PR |

### Milestone

Tag **`v0.1.0`** + CHANGELOG + demo GIF: `curl` → dashboard đổi số.

---

## 6. Phase L2 — Đào sâu ClickHouse

**Ước lượng:** 25h · **Task:** `L2-01` → `L2-24` · **Checklist:** [`TODO.md` LEVEL 2](TODO.md#level-2--đào-sâu-clickhouse-25h)

### Mục tiêu

Phase **ít code, nhiều thực nghiệm**. Mục tiêu không phải tính năng mà là *hiểu bằng số liệu của chính mình*: `ORDER BY` nên thế nào, `LowCardinality` tiết kiệm bao nhiêu, projection có đáng không.

### Điều kiện vào

- L1 xong, bảng `events` có dữ liệu.
- **Có ít nhất 10M event** để thí nghiệm — nếu chưa, làm trước `L3-01`…`L3-07` (seeder). Đây là ngoại lệ hợp lệ duy nhất của thứ tự phase.
- Đã đọc [`PLAN.md` §6](PLAN.md#6-thiết-kế-clickhouse) và [§7.1](PLAN.md#71-nguyên-lý-cần-hiểu-trước-khi-viết).

### Phạm vi công việc

| Nhóm | Task | Nội dung thí nghiệm | Giờ |
|---|---|---|---|
| L2.1 Hiểu storage | `L2-01`…`L2-05` | `system.parts`, `index_granularity` 8192/4096/16384, `EXPLAIN indexes=1`, `EXPLAIN PIPELINE` | 5h |
| L2.2 Thí nghiệm ORDER BY | `L2-06`…`L2-09` | 3 bảng cùng dữ liệu, 3 `ORDER BY` khác nhau, 8 query benchmark | 5h |
| L2.3 Kiểu dữ liệu & nén | `L2-10`…`L2-14` | `LowCardinality`, `DateTime64(3)`, codec ZSTD/Delta/DoubleDelta, `Nullable` vs DEFAULT | 4h |
| L2.4 Skip index & projection | `L2-15`…`L2-19` | `bloom_filter`, `tokenbf_v1`, `minmax`, `PROJECTION prj_by_user` | 5h |
| L2.5 TTL & vòng đời | `L2-20`…`L2-22` | TTL DELETE 180d, RECOMPRESS 30d, (optional) TO VOLUME cold | 3h |
| L2.6 Ghi chép | `L2-23`, `L2-24` | `docs/clickhouse-notes.md` ≥ 20 mục | 3h |

### Deliverable

- `docs/clickhouse-notes.md` — **≥ 20 mục**, mỗi mục theo cấu trúc *quan sát → số liệu → giải thích*. Không copy tài liệu chính thức.
- `docs/queries-ops.sql` — 5 query "soi bảng".
- Migration mới áp codec tốt nhất (`MODIFY COLUMN`, **không sửa file migration cũ**).
- ADR-0002 cập nhật với `ORDER BY` chốt cuối cùng; ADR mới về projection (giữ hay bỏ).
- Nếu kết luận khác giả định ban đầu → cập nhật [`PLAN.md` §6](PLAN.md#6-thiết-kế-clickhouse) (`L2-24`).

### Tiêu chí ra

Trả lời được **bằng số liệu tự đo**, không bằng cảm tính:

- [ ] "Vì sao `ORDER BY` là thứ tự này?" — có bảng so sánh 3 phương án × 8 query (thời gian + `read_rows` + dung lượng đĩa).
- [ ] "`LowCardinality` tiết kiệm bao nhiêu?" — có % dung lượng và % tốc độ GROUP BY.
- [ ] "Projection đáng giá không?" — có 3 số: dung lượng tăng %, insert chậm %, query theo user nhanh mấy lần.
- [ ] "Vì sao tránh `Nullable`?" — có số đo `Nullable(String)` vs `String DEFAULT ''`.
- [ ] TTL DELETE và RECOMPRESS đã test thật (chèn dữ liệu cũ + `OPTIMIZE ... FINAL`).
- [ ] `docs/clickhouse-notes.md` đạt ≥ 20 mục có số liệu.

### Chỉ số cần đo

| Thí nghiệm | Số phải ghi |
|---|---|
| `index_granularity` | Kích thước mark file, tốc độ query điểm |
| 3 phương án `ORDER BY` | Thời gian, `read_rows`, dung lượng đĩa từng bảng |
| Codec | Dung lượng từng cột trước/sau, tốc độ đọc |
| Skip index | `read_rows` trước/sau, số granule bị loại |
| Projection | +% đĩa, −% tốc độ insert, ×lần tốc độ query |
| TTL RECOMPRESS | Dung lượng tiết kiệm |

### Rủi ro

| Rủi ro | Phòng ngừa |
|---|---|
| Đo trên dữ liệu quá nhỏ → kết luận sai | Tối thiểu 10M event; ghi rõ mức dữ liệu bên cạnh mỗi số |
| Đo warm cache tưởng là cold | `SYSTEM DROP MARK CACHE; SYSTEM DROP UNCOMPRESSED CACHE` trước mỗi lần đo cold |
| Sửa file migration cũ | Luôn viết migration mới (`L2-14`) — file cũ là lịch sử, không phải bản nháp |
| Sa đà vô tận vào tinh chỉnh | Đóng hộp 25h; điều gì chưa xong thì đẩy sang backlog `X-*` |

### Milestone

Không tag. Bằng chứng: `docs/clickhouse-notes.md` đủ 20 mục + bảng so sánh `ORDER BY`.

---

## 7. Phase L3 — Batch insert, seeder, benchmark

**Ước lượng:** 35h · **Task:** `L3-01` → `L3-32` · **Checklist:** [`TODO.md` LEVEL 3](TODO.md#level-3--batch-insert-seeder-benchmark-35h)

### Mục tiêu

Biến đường ghi naive thành đường ghi production: batch + backpressure + WAL fallback. Sinh dữ liệu quy mô thật và trả lời câu hỏi lớn nhất của project — **ClickHouse nhanh hơn PostgreSQL bao nhiêu, và vì sao**.

### Điều kiện vào

- L1 xong; L2 đã chốt `ORDER BY` và codec (nếu chưa, số benchmark sẽ phải đo lại).
- Đã đọc [`PLAN.md` §10](PLAN.md#10-batch-insert--backpressure) và [§18](PLAN.md#18-benchmark-plan-postgresql-vs-clickhouse).
- Có đủ đĩa cho mức dữ liệu định benchmark (100M event ≈ vài chục GB tuỳ codec).

### Phạm vi công việc

| Nhóm | Task | Tham chiếu PLAN | Giờ |
|---|---|---|---|
| L3.1 Seeder | `L3-01`…`L3-07` | [§18.2](PLAN.md#182-dataset) + [§2.5 bảng này](#25-phân-phối-dữ-liệu-của-seeder) | 6h |
| L3.2 BatchWriter | `L3-08`…`L3-17` | [§10.2](PLAN.md#102-thiết-kế-batchwriter), [§10.3](PLAN.md#103-backpressure--3-mức) | 8h |
| L3.3 Benchmark insert | `L3-18`…`L3-22` | [§10.4](PLAN.md#104-so-sánh-với-async_insert-của-clickhouse) | 6h |
| L3.4 Benchmark query CH vs PG | `L3-23`…`L3-31` | [§18](PLAN.md#18-benchmark-plan-postgresql-vs-clickhouse) | 11h |
| L3.5 Milestone | `L3-32` | — | 4h |

### Deliverable

- `cmd/seeder` với đầy đủ flag, phân phối theo [§2.5](#25-phân-phối-dữ-liệu-của-seeder).
- `internal/buffer/writer.go` — non-blocking enqueue, worker pool, retry backoff+jitter, graceful flush, WAL NDJSON rotate 64 MB.
- `cmd/wal-replay` (hoặc goroutine nền) quét WAL và replay.
- `docker-compose.bench.yml` với PostgreSQL 17 + `repository/postgres/`.
- `loadtest/k6/ingest.js`, `loadtest/bench/run_bench.go`.
- **`docs/benchmark-results.md`** — bảng [`PLAN.md` §18.3](PLAN.md#183-bảng-kết-quả-cần-điền) điền đủ + biểu đồ PNG + phần "Vì sao ClickHouse nhanh" / "ClickHouse dở ở đâu".

### Tiêu chí ra

- [ ] `make seed N=10000000` chạy **< 5 phút**.
- [ ] Seed 10M rồi chạy funnel query → khớp tỉ lệ cài sẵn **sai số < 2%**.
- [ ] Test "kill ClickHouse giữa chừng → WAL có file → bật lại → replay → `count()` khớp" **xanh** (testcontainers, `L3-17`).
- [ ] Enqueue quá tải **không block** handler; `events_dropped_total` tăng đúng, ưu tiên giữ `purchase`/`signup`.
- [ ] Graceful shutdown flush hết buffer, **không mất event**.
- [ ] k6 ramp 0 → 10k ev/s trong 10 phút: **drop = 0, p99 API < 50 ms, không lỗi 252**.
- [ ] Bảng benchmark ở [`PLAN.md` §18.3](PLAN.md#183-bảng-kết-quả-cần-điền) điền đủ **10 dòng**, có cả cold/warm và dung lượng đĩa.
- [ ] `BATCH_SIZE` / `FLUSH_INTERVAL_MS` mặc định đã cập nhật theo số đo (`L3-22`) — nếu khác 5000/500 thì sửa [§2.4](#24-tham-số-vận-hành-mặc-định), `README.md` và `.env.example`.

### Chỉ số cần đo

| Kịch bản | Số phải ghi |
|---|---|
| 6 kịch bản insert (1 row, batch 100/1k/10k, `async_insert`) | throughput, p99 API, part/phút, CPU/RAM CH, có lỗi 252 không |
| Query CH vs PG × 4 biến thể | cold/warm, median của 3 lần |
| Insert 10M rows | thời gian, dung lượng đĩa hai bên |

### Rủi ro

| Rủi ro | Dấu hiệu | Phòng ngừa |
|---|---|---|
| `Too many parts` | Insert lỗi code 252 | Batch ≥ 10k row, ≤ 1 insert/s/bảng — tăng `parts_to_throw_insert` chỉ là băng dán |
| Benchmark không công bằng | PG thua "quá đẹp" | Cùng máy, cùng RAM, `VACUUM ANALYZE` trước, có biến thể partition, đo 3 lần lấy trung vị |
| Buffer in-memory mất khi restart | Mất event lúc deploy | Flush trong graceful shutdown + WAL file |
| Đĩa đầy giữa lúc seed 100M | CH dừng ghi | Kiểm dung lượng trước; alert ở 75% |
| Số benchmark bị nhiễu vì chạy chung máy | Dao động > 30% giữa các lần | Ghi rõ cấu hình máy vào `benchmark-results.md`; xem [`DEPLOY-AWS.md` §2](DEPLOY-AWS.md#2-quyết-định--đánh-đổi) |

### Milestone

Tag **`v0.3.0`** + bài viết "Batch insert: từ X ev/s lên Y ev/s" (X lấy từ số đo L1).

---

## 8. Phase L4 — Kafka pipeline

**Ước lượng:** 35h · **Task:** `L4-01` → `L4-30` · **Checklist:** [`TODO.md` LEVEL 4](TODO.md#level-4--kafka-pipeline-35h)

### Mục tiêu

Chuyển từ kiến trúc **Phase 1** sang **Phase 2** ([`PLAN.md` §2.2](PLAN.md#22-phase-2--event-pipeline-với-kafka-level-4)): Kafka đứng giữa ingest và ClickHouse để decouple, replay được, fan-out được. Tách `ingest-api` và `analytics-api` thành hai binary scale độc lập.

### Điều kiện vào

- L3 xong: `internal/buffer` ổn định (consumer sẽ tái dùng), WAL đã chứng minh không mất event.
- Đã đọc [`PLAN.md` §11](PLAN.md#11-kafka-pipeline) và ADR-0004, ADR-0005.

### Phạm vi công việc

| Nhóm | Task | Tham chiếu PLAN | Giờ |
|---|---|---|---|
| L4.1 Hạ tầng Kafka | `L4-01`…`L4-04` | [§11.1 Topic design](PLAN.md#111-topic-design) | 4h |
| L4.2 Producer | `L4-05`…`L4-11` | [§11.1](PLAN.md#111-topic-design), [§11.4](PLAN.md#114-vì-sao-chèn-kafka-vào-giữa) | 6h |
| L4.3 Consumer | `L4-12`…`L4-20` | [§11.2 at-least-once](PLAN.md#112-consumer-loop-at-least-once-đúng-cách), [§11.3 Dedup](PLAN.md#113-dedup) | 10h |
| L4.4 Tách service & chaos test | `L4-21`…`L4-28` | [§2.3 nguyên tắc 4](PLAN.md#23-nguyên-tắc-kiến-trúc) | 9h |
| L4.5 Milestone | `L4-29`, `L4-30` | — | 6h |

### Deliverable

- Kafka KRaft (hoặc Redpanda) trong compose + Kafka UI + `make kafka-topics`, `make kafka-lag`.
- Topic `events.raw` (6 partition, 7d, zstd) và `events.dlq` (1 partition, 30d).
- `internal/kafka/producer.go` — acks=1, linger 50 ms, idempotence, key = `site_id|session_id`.
- `cmd/consumer` — consumer group `clickhouse-sink`, **tắt auto-commit**, commit **sau** khi ClickHouse ack.
- `cmd/dlq-replay`.
- `SINK=direct|kafka` để bật/tắt Kafka mà không đổi code.
- Sơ đồ kiến trúc trong `README.md` cập nhật sang Phase 2 (`L4-29`).
- ADR-0004, ADR-0005 hoàn chỉnh.

### Tiêu chí ra

- [ ] **Chaos 1:** kill ClickHouse 5 phút giữa lúc bắn 5k ev/s → bật lại → `count()` **khớp 100%** số đã gửi.
- [ ] **Chaos 2:** kill 1 consumer → rebalance → không mất event, trùng lặp trong ngưỡng dedup chấp nhận được.
- [ ] **Chaos 3:** gửi event hỏng → vào `events.dlq` kèm header `error`, `retry_count`, `original_partition`, `original_offset` → **không chặn partition**.
- [ ] Kafka down → API **vẫn trả 202**, WAL có file (`L4-11`).
- [ ] End-to-end lag p99 **< 5 s** ở 5k ev/s.
- [ ] `ingest-api` không còn import package analytics (`L4-21`); compose chạy 2 replica ingest, 2 consumer, 1 analytics.
- [ ] Rebalance an toàn: `OnPartitionsRevoked` flush + commit trước khi nhả partition.
- [ ] Metric `kafka_consumer_lag` (per partition), `kafka_records_processed_total`, `kafka_dlq_total` xuất hiện ở `/metrics`.
- [ ] Bảng benchmark L3 có thêm dòng "Kafka + consumer batch 10k" (`L4-27`).

### Chỉ số cần đo

| Chỉ số | Ghi vào |
|---|---|
| End-to-end lag p50/p95/p99 ở 5k ev/s | `docs/benchmark-results.md` |
| Throughput consumer (record/s) theo kích thước batch | như trên |
| Thời gian rebalance | `docs/runbook.md` |
| So sánh consumer tự viết vs ClickHouse `Kafka` table engine (`L4-28`) | `docs/clickhouse-notes.md` |

### Rủi ro

| Rủi ro | Dấu hiệu | Phòng ngừa |
|---|---|---|
| Commit offset trước khi insert | Mất data khi crash | Commit **sau** khi ClickHouse ack (`L4-15`) |
| Consumer retry vô hạn | Lag phình, partition bị chặn | Giới hạn 3 lần → DLQ → rồi mới commit (`L4-16`) |
| Rebalance làm mất batch đang xử lý | Thiếu event rải rác | Flush + commit trong `OnPartitionsRevoked` (`L4-17`) |
| Trùng lặp do at-least-once | Số liệu phồng nhẹ | Dedup theo `event_id` ở tầng query (ADR-0005) |
| Kafka JVM tranh CPU với ClickHouse | Benchmark dao động | Ghi rõ vào `benchmark-results.md`; xem [`DEPLOY-AWS.md` §2](DEPLOY-AWS.md#2-quyết-định--đánh-đổi) |

### Milestone

Tag **`v0.4.0`** + demo: kill ClickHouse khi đang bắn tải → ingest vẫn 202 → bật lại → số khớp.

---

## 9. Phase L5 — Analytics nâng cao + dashboard đầy đủ

**Ước lượng:** 45h · **Task:** `L5-01` → `L5-46` · **Checklist:** [`TODO.md` LEVEL 5](TODO.md#level-5--analytics-nâng-cao--dashboard-đầy-đủ-45h)

### Mục tiêu

Phase lớn nhất. Đưa dashboard từ "tối giản" lên "đủ dùng thật": Materialized View để mọi query < 300 ms ở 100M, funnel, retention, realtime, revenue, và SDK nhúng.

### Điều kiện vào

- L4 xong, dữ liệu chảy ổn định qua Kafka.
- Đã có **100M event** trong ClickHouse để đo ngưỡng hiệu năng.
- Đã đọc [`PLAN.md` §7 (MV)](PLAN.md#7-materialized-view--pre-aggregation) và [§8 (query cookbook)](PLAN.md#8-analytics-query-cookbook).

### Phạm vi công việc

| Nhóm | Task | Tham chiếu PLAN | Giờ |
|---|---|---|---|
| L5.1 Materialized Views | `L5-01`…`L5-10` | [§7.2](PLAN.md#72-events_hourly), [§7.3](PLAN.md#73-events_daily-dauwaumau), [§7.4](PLAN.md#74-sessions), [§7.5](PLAN.md#75-user_first_seen-cohort) | 10h |
| L5.2 Funnel | `L5-11`…`L5-16` | [§8.5 `windowFunnel`](PLAN.md#85-funnel--windowfunnel) | 6h |
| L5.3 Retention & cohort | `L5-17`…`L5-23` | [§8.6](PLAN.md#86-retention--cohort-matrix) | 7h |
| L5.4 Endpoint còn lại | `L5-24`…`L5-31` | [§8.7](PLAN.md#87-realtime-30-phút-gần-nhất), [§12.2](PLAN.md#122-analytics-đều-yêu-cầu-x-api-key-hoặc-session-cookie-của-dashboard) | 6h |
| L5.5 Dashboard đầy đủ | `L5-32`…`L5-44` | [§13](PLAN.md#13-frontend-design-nextjs) | 14h |
| L5.6 Tracking SDK | `L5-45`, `L5-46` | [§13.4](PLAN.md#134-tracking-sdk-sdkjs) | 2h |

### Deliverable

**5 Materialized View** (migration `0003` → `0007`, thêm `page_stats_hourly`):

| MV | Mục đích | Lưu ý |
|---|---|---|
| `events_hourly` | Overview + timeseries | Chỉ cột cardinality thấp trong GROUP BY |
| `events_daily` | DAU/WAU/MAU | |
| `page_stats_hourly` | Top pages | Tách riêng vì `page` cardinality cao |
| `sessions` | Bounce rate, avg duration, entry/exit page | `argMinState`/`argMaxState` |
| `user_first_seen` | Cohort/retention | |

Cùng với: script backfill có `--dry-run` chạy theo từng tháng; toàn bộ endpoint ở [`PLAN.md` §12.2](PLAN.md#122-analytics-đều-yêu-cầu-x-api-key-hoặc-session-cookie-của-dashboard); 7 trang dashboard; `sdk/js/pulse.js`; `types/api.ts` sinh từ `openapi.yaml`.

### Tiêu chí ra

- [ ] **Golden test MV vs raw**: insert 50k event cố định → mọi metric khớp **tuyệt đối**. Đây là test quan trọng nhất project ([`PLAN.md` §16](PLAN.md#16-testing-strategy)).
- [ ] `/analytics/overview` **< 100 ms** ở 100M events.
- [ ] Mọi endpoint analytics p95 **< 300 ms** ở 100M events, có ảnh chụp `system.query_log` làm bằng chứng.
- [ ] `/analytics/realtime` **< 200 ms**.
- [ ] Kích thước tất cả MV **≤ 15%** raw — nếu vượt, xem lại cardinality của GROUP BY.
- [ ] Funnel khớp tỉ lệ seeder **±2%**; retention khớp D1 ~30% / D7 ~12% / D30 ~5%.
- [ ] Backfill không double-count (kiểm bằng cách so `count()` raw và MV trên cùng khoảng).
- [ ] `FilterBar` toàn cục áp đúng cho mọi widget và đồng bộ URL.
- [ ] Playwright E2E xanh: seed → dashboard → đổi date range → funnel 5 bước.
- [ ] Lighthouse: performance **> 90**, a11y **> 95**.
- [ ] `pulse.js` **< 2 KB gzip**, chạy được trên trang HTML tĩnh.
- [ ] `types/api.ts` sinh từ OpenAPI, **không còn type viết tay**.

### Chỉ số cần đo

| Chỉ số | Ghi vào |
|---|---|
| overview/timeseries/pages: trước MV vs sau MV ở 100M | `docs/benchmark-results.md` |
| Dung lượng từng MV / dung lượng raw | `docs/clickhouse-notes.md` |
| Cohort matrix: thời gian query JOIN vs hàm `retention()` (`L5-18`) | như trên |
| Kích thước gzip của `pulse.js` | `sdk/js/README.md` |

### Rủi ro

| Rủi ro | Dấu hiệu | Phòng ngừa |
|---|---|---|
| MV dùng cột non-aggregate | Số liệu sai **âm thầm** sau merge | `SimpleAggregateFunction`/`AggregateFunction` + golden test (`L5-03`) |
| Quên backfill sau khi tạo MV | Dashboard thiếu dữ liệu cũ | `INSERT ... SELECT` backfill, chú ý biên thời gian |
| Cardinality nổ ở MV | `events_hourly` to gần bằng raw | Tách `page_stats_hourly` riêng; kiểm ngưỡng 15% (`L5-10`) |
| `FINAL` lọt vào hot path | Query chậm khủng khiếp | Cấm `FINAL` ở endpoint; review PR |
| Dashboard query không giới hạn range | 1 request quét cả năm, OOM | Ép max 400 ngày + `max_execution_time` + `max_memory_usage` |
| Gửi 100k điểm về browser | FE đơ | Bucket ở server; ECharts + downsampling khi > 5.000 điểm (`L5-39`) |
| Bot traffic làm phồng DAU | Số liệu sai | Phân loại bot khi enrich, mặc định lọc |

### Milestone

Không tag riêng (gộp vào `v1.0.0` ở L6). Bằng chứng: ảnh chụp toàn bộ 7 trang + `system.query_log` chứng minh < 300 ms.

---

## 10. Phase L6 — Observability, security, CD, docs

**Ước lượng:** 25h (→ **32h** nếu đi đường AWS) · **Task:** `L6-01` → `L6-35` · **Checklist:** [`TODO.md` LEVEL 6](TODO.md#level-6--observability-security-cd-docs-25h)

### Mục tiêu

Biến "chạy được trên máy mình" thành "vận hành được": nhìn thấy hệ thống (metrics/log/alert), bịt lỗ hổng, deploy tự động có rollback, và viết lại toàn bộ hiểu biết thành tài liệu.

### Điều kiện vào

- L5 xong, dashboard đầy đủ và đạt ngưỡng hiệu năng.
- Đã đọc [`PLAN.md` §14](PLAN.md#14-observability), [§15](PLAN.md#15-security-privacy--multi-tenant), [§17](PLAN.md#17-cicd).

### Phạm vi công việc

| Nhóm | Task | Tham chiếu | Giờ |
|---|---|---|---|
| L6.1 Metrics & Grafana | `L6-01`…`L6-08` | [`PLAN.md` §14.1](PLAN.md#141-metrics-prometheus-bắt-buộc-có), [§14.2](PLAN.md#142-grafana-dashboards), [§14.4](PLAN.md#144-alert-alertmanager-hoặc-chỉ-grafana-alert) | 6h |
| L6.2 Logging & tracing | `L6-09`…`L6-11` | [`PLAN.md` §14.3](PLAN.md#143-logging) | 3h |
| L6.3 Security hardening | `L6-12`…`L6-19` | [`PLAN.md` §15](PLAN.md#15-security-privacy--multi-tenant), [§17.3](PLAN.md#173-securityyml) | 5h |
| L6.4 CD & môi trường thật | `L6-20`…`L6-28` | [`PLAN.md` §17.4](PLAN.md#174-cd-productionyml) — **hoặc** [Phase AWS](#11-phase-aws--hạ-tầng-production) | 7h → 14h |
| L6.5 Tài liệu & tổng kết | `L6-29`…`L6-35` | [`PLAN.md` §22](PLAN.md#22-definition-of-done) | 4h |

> **Rẽ nhánh L6.4.** Có hai đường:
> - **Đường VPS** (mô tả trong [`PLAN.md` §17.4–17.5](PLAN.md#174-cd-productionyml)): 1 VPS ≥ 4 vCPU / 16 GB / SSD 200 GB, deploy qua SSH + docker compose. 7h.
> - **Đường AWS** (khuyến nghị, mô tả trong [`DEPLOY-AWS.md`](DEPLOY-AWS.md)): Vercel + EC2 `r7g.xlarge` + Terraform + ECR + SSM. 14h, thay `L6-20`→`L6-28` bằng `AWS-01`→`AWS-32`.
>
> Chọn **một** đường, đánh dấu đường còn lại là `[-]` (bỏ qua có chủ ý) trong `TODO.md`.

### Deliverable

- `internal/metrics/metrics.go` với đủ 10 metric ở [`PLAN.md` §14.1](PLAN.md#141-metrics-prometheus-bắt-buộc-có).
- **4 Grafana dashboard**: Ingest health · ClickHouse internals · Kafka · API RED — export JSON vào `deploy/grafana/dashboards/`.
- **4 alert rule** theo [`PLAN.md` §14.4](PLAN.md#144-alert-alertmanager-hoặc-chỉ-grafana-alert).
- `security.yml`: govulncheck, gosec, trivy (fail HIGH/CRITICAL), gitleaks, npm audit — PR + weekly.
- Auth dashboard (JWT/session cookie httpOnly+SameSite, argon2id), API key lưu **hash** + rotate + `revoked_at`.
- `docker-compose.prod.yml`, `deploy/caddy/Caddyfile`, `cd-staging.yml`, `cd-production.yml`.
- `docs/runbook.md`, `docs/api/openapi.yaml` khớp 100% implementation, **10 ADR**, `CONTRIBUTING.md`.
- `README.md` hoàn chỉnh + bài viết tổng kết.

### Tiêu chí ra

- [ ] `/metrics` xuất đủ metric ở [`PLAN.md` §14.1](PLAN.md#141-metrics-prometheus-bắt-buộc-có); label theo **route pattern**, không phải raw path (tránh nổ cardinality).
- [ ] 4 Grafana dashboard hiển thị dữ liệu thật; 4 alert **đã kích hoạt thử** ít nhất 1 lần.
- [ ] Log JSON có `request_id`, `site_id`, `trace_id`; có test chứng minh **payload không lọt vào log mức info**.
- [ ] 100% query parameterized; param `Identifier` có whitelist.
- [ ] Container non-root, read-only rootfs, `cap_drop: [ALL]`, `no-new-privileges`.
- [ ] Security scan xanh, không HIGH/CRITICAL.
- [ ] CD: deploy bằng **1 tag**, thứ tự `migrate → deploy → healthcheck → rollback nếu fail`; đã **thử rollback thật** bằng cách deploy image hỏng.
- [ ] Smoke test sau deploy: gửi 1 event thật và query lại thấy nó.
- [ ] Backup chạy theo lịch, giữ 7 bản, và **đã restore thử ít nhất 1 lần** — ghi RTO thực tế vào `docs/runbook.md`.
- [ ] `openapi.yaml` khớp implementation (kiểm bằng schemathesis).
- [ ] 10 ADR hoàn chỉnh theo format Context → Decision → Consequences → Alternatives.

### Chỉ số cần đo

| Chỉ số | Ghi vào |
|---|---|
| RTO thực tế khi restore | `docs/runbook.md` |
| Thời gian deploy end-to-end (tag → healthy) | `docs/runbook.md` |
| Kích thước image sau hardening | `docs/runbook.md` |

### Rủi ro

| Rủi ro | Phòng ngừa |
|---|---|
| Cardinality metric nổ vì label raw path | Dùng route pattern (`L6-02`) |
| Backup chưa từng restore | Bắt buộc diễn tập restore (`L6-26` / `AWS-28`) |
| Migration phá tương thích ngược | Luôn tương thích ngược một bước: add column → đọc/ghi → xoá ở release sau ([`PLAN.md` §17.4](PLAN.md#174-cd-productionyml)) |
| Secret lọt vào repo | gitleaks trong `security.yml` + `.env` chỉ tạo trên máy đích |
| Deploy mù vì chưa có metrics | Làm L6.1 **trước** L6.4 |

### Milestone

Tag **`v1.0.0`** + bài viết tổng kết + README có ảnh dashboard và mục "những gì tôi học được".

---

## 11. Phase AWS — Hạ tầng production

**Ước lượng:** 14h · **Task:** `AWS-01` → `AWS-32` · **Chi tiết:** [`DEPLOY-AWS.md`](DEPLOY-AWS.md) · **Checklist:** [`DEPLOY-AWS.md` §17](DEPLOY-AWS.md#17-checklist-thay-thế-l64)

### Mục tiêu

Đưa hệ thống lên hạ tầng thật: **Next.js trên Vercel** + **1 EC2 `r7g.xlarge` ở `ap-southeast-1`** chạy toàn bộ backend bằng Docker Compose, hạ tầng khai báo bằng Terraform, TLS tự động bằng Caddy, deploy qua ECR + SSM.

Phase này **thay thế** `L6-20` → `L6-28`.

### Điều kiện vào

- L6.1 (metrics), L6.2 (logging), L6.3 (security) đã xong — không deploy khi chưa nhìn thấy hệ thống.
- Đã có domain và tài khoản AWS.
- **Đã đặt AWS Budget trước khi tạo bất kỳ tài nguyên nào** (`AWS-02`).

### Phạm vi công việc

| Nhóm | Task | Tham chiếu | Giờ |
|---|---|---|---|
| AWS.1 Chuẩn bị tài khoản | `AWS-01`…`AWS-04` | [§3](DEPLOY-AWS.md#3-chuẩn-bị-aws-account) | 2h |
| AWS.2 Terraform | `AWS-05`…`AWS-11` | [§4](DEPLOY-AWS.md#4-terraform--cấu-trúc), [§5](DEPLOY-AWS.md#5-terraform--code), [§6](DEPLOY-AWS.md#6-bootstrap-ec2-cloud-init) | 5h |
| AWS.3 Chạy stack | `AWS-12`…`AWS-17` | [§7](DEPLOY-AWS.md#7-phân-bổ-ram--docker-composeprodyml), [§8](DEPLOY-AWS.md#8-caddy--reverse-proxy--tls), [§9](DEPLOY-AWS.md#9-tinh-chỉnh-clickhouse--kafka-cho-1-máy) | 3h |
| AWS.4 CI/CD | `AWS-18`…`AWS-21` | [§10](DEPLOY-AWS.md#10-cicd-github-actions--ecr--ssm) | 2h |
| AWS.5 Vercel | `AWS-22`…`AWS-26` | [§11](DEPLOY-AWS.md#11-vercel-setup), [§12](DEPLOY-AWS.md#12-cors--authentication) | 1h |
| AWS.6 Vận hành | `AWS-27`…`AWS-32` | [§13](DEPLOY-AWS.md#13-backup--restore), [§14](DEPLOY-AWS.md#14-giám-sát-chi-phí), [§15](DEPLOY-AWS.md#15-runbook-sự-cố-đặc-thù-aws) | 1h |

### Deliverable

- `infra/` — 13 file Terraform theo [`DEPLOY-AWS.md` §4](DEPLOY-AWS.md#4-terraform--cấu-trúc), state trên S3 + lock DynamoDB.
- EC2 `r7g.xlarge` (4 vCPU / 32 GB, arm64) + EBS gp3 500 GB / 6000 IOPS, mount `/data`, `prevent_destroy`.
- Caddy phục vụ `api.pulse.dev` và `grafana.pulse.dev` với TLS tự động.
- `cd-production.yml` qua GitHub OIDC → ECR → SSM (không lưu AWS key trong Secrets).
- DLM snapshot policy, AWS Budget $300 + Cost Anomaly Detection, CloudWatch alarm.
- Vercel project (root `frontend`, region `sin1`) trỏ `app.pulse.dev`.

### Tiêu chí ra

- [ ] `terraform plan` lần 2 báo **"No changes"** (`prevent_destroy` + `ignore_changes = [ami]` có hiệu lực).
- [ ] `aws ssm start-session` vào được — **không cần SSH key, không mở port 22**.
- [ ] `/data` đã mount đúng dung lượng; sysctl + THP `madvise` đã áp.
- [ ] Toàn bộ service `docker compose ps` healthy; Caddy lấy được chứng chỉ cho `api.*` và `grafana.*`.
- [ ] `curl https://api.pulse.dev/healthz` trả 200 từ máy ngoài.
- [ ] **Không** truy cập được `:8123`, `:9000`, `:9092` từ Internet (kiểm bằng `nmap` từ máy khác).
- [ ] Cố tình deploy image hỏng → smoke test fail → **rollback tự động chạy**.
- [ ] Dashboard trên Vercel gọi API không lỗi CORS/CSP; preview deployment hành xử đúng thiết kế.
- [ ] DLM đã tạo snapshot đầu tiên; **đã diễn tập restore**, RTO ghi vào `docs/runbook.md`.
- [ ] Đã ghi vào `docs/benchmark-results.md`: cấu hình all-in-one, Kafka/Go dùng chung CPU với ClickHouse → **số liệu là bi quan** (`AWS-31`).

### Chỉ số cần đo

| Chỉ số | Ghi vào |
|---|---|
| Chi phí thực tháng đầu vs ước tính ~$267 | `docs/runbook.md` |
| RTO restore từ snapshot | `docs/runbook.md` |
| Benchmark trên EC2 vs trên máy local | `docs/benchmark-results.md` |

### Rủi ro

| Rủi ro | Phòng ngừa |
|---|---|
| Hóa đơn nhảy vọt | Budget + Cost Anomaly Detection **trước** khi tạo tài nguyên; kiểm hóa đơn hằng tuần tháng đầu (`AWS-32`) |
| Volume mồ côi / EIP không gắn máy sau `terraform destroy` | Chạy đủ 4 lệnh kiểm sót ở [§16 Teardown](DEPLOY-AWS.md#16-teardown) |
| ClickHouse OOM-kill | Docker limit cao hơn `max_server_memory_usage` ~10% |
| Insert chậm dần | Theo dõi `VolumeQueueLength`; nới IOPS nóng bằng `terraform apply` |
| Tranh tài nguyên làm benchmark nhiễu | Ghi chú rõ trong báo cáo; cân nhắc tách máy khi gặp dấu hiệu ở [§2](DEPLOY-AWS.md#2-quyết-định--đánh-đổi) |

### Milestone

Hệ thống chạy thật trên domain có TLS, có Grafana, có backup đã diễn tập restore.

---

## 12. Ma trận truy vết

| Phase | Task ID | PLAN.md | TODO.md | DEPLOY-AWS.md | Deliverable chính |
|---|---|---|---|---|---|
| L0 | `L0-01`…`L0-25` | §4, §9.1, §9.4, §17.2 | LEVEL 0 | — | Skeleton + compose + CI |
| L1 | `L1-01`…`L1-40` | §5, §6.1, §8.1–8.4, §9, §12, §13.1–13.3 | LEVEL 1 | — | MVP end-to-end, tag `v0.1.0` |
| L2 | `L2-01`…`L2-24` | §6, §7.1 | LEVEL 2 | — | `docs/clickhouse-notes.md` |
| L3 | `L3-01`…`L3-32` | §10, §18 | LEVEL 3 | — | `docs/benchmark-results.md`, tag `v0.3.0` |
| L4 | `L4-01`…`L4-30` | §2.2, §11 | LEVEL 4 | — | Kafka pipeline, tag `v0.4.0` |
| L5 | `L5-01`…`L5-46` | §7, §8, §12.2, §13 | LEVEL 5 | — | 5 MV + dashboard đầy đủ + SDK |
| L6 | `L6-01`…`L6-35` | §14, §15, §16, §17, §22 | LEVEL 6 | — | Observability + CD + docs, tag `v1.0.0` |
| AWS | `AWS-01`…`AWS-32` | (thay §17.4–17.5) | (thay L6.4) | toàn bộ | Hạ tầng production |

---

## 13. Đường tắt khi thiếu thời gian

Thứ tự giữ lại theo mức "đắt giá về kiến thức", khớp [`PLAN.md` §19](PLAN.md#19-roadmap-theo-level):

| Ưu tiên | Phase | Lý do giữ / bỏ |
|---|---|---|
| 1 | **L0 → L1 → L2 → L3** | Lõi kiến thức: ClickHouse storage + write path + benchmark. Bỏ là mất phần giá trị nhất |
| 2 | **L5.1 (MV) + L5.2 (funnel)** | Chứng minh hiểu AggregatingMergeTree và analytical SQL |
| 3 | **L4 (Kafka)** | Có thể lùi lại nếu mục tiêu không phải event-driven; nhưng đã đặt mục tiêu này thì đừng bỏ |
| 4 | **L6.1 + L6.5** | Metrics và tài liệu — rẻ mà tăng giá trị project rõ rệt |
| 5 | **L5.3–L5.6, L6.2–L6.4, AWS** | Hoãn được nếu hết thời gian |

Rút gọn chấp nhận được:

- Benchmark PG vs CH dừng ở mức **10M** thay vì 100M nếu không đủ đĩa — ghi rõ mức dữ liệu bên cạnh mọi con số.
- Thay Kafka bằng **Redpanda** ở dev để nhẹ máy (giữ nguyên giao diện client).
- Bỏ OpenTelemetry (`L6-11`) — đã đánh dấu optional sẵn.
- Bỏ TTL `TO VOLUME 'cold'` (`L2-22`) — optional.

**Không được rút gọn:** golden test MV vs raw (`L5-03`), test "kill ClickHouse không mất event" (`L3-17`, `L4-23`), diễn tập restore backup (`L6-26` / `AWS-28`).

---

## 14. Definition of Done toàn dự án

Bản đầy đủ ở [`PLAN.md` §22](PLAN.md#22-definition-of-done) và [`TODO.md` § Checklist nghiệm thu cuối](TODO.md#checklist-nghiệm-thu-cuối-copy-từ-plan-22). Tóm tắt theo phase để dễ đối chiếu:

| # | Tiêu chí | Phase chịu trách nhiệm |
|---|---|---|
| 1 | `git clone && make up` chạy trong < 5 phút trên máy sạch | L0 |
| 2 | `make seed N=10000000` thành công trong < 5 phút | L3 |
| 3 | Dashboard đủ 7 nhóm widget với dữ liệu thật | L5 |
| 4 | Mọi endpoint analytics p95 < 300 ms ở 100M events (có `system.query_log` làm bằng) | L5 |
| 5 | Ingest 10.000 ev/s trong 10 phút, drop = 0, p99 API < 50 ms | L3 |
| 6 | Kill ClickHouse 5 phút → không mất event → số liệu khớp 100% | L3 + L4 |
| 7 | Golden test MV vs raw khớp tuyệt đối | L5 |
| 8 | CI xanh: lint, unit, integration, security scan, build image | L0 + L6 |
| 9 | CD deploy bằng 1 tag, có rollback, có smoke test | L6 / AWS |
| 10 | Grafana 4 dashboard + alert hoạt động | L6 |
| 11 | `docs/benchmark-results.md` đầy đủ bảng PG vs CH + kết luận | L3 |
| 12 | `docs/clickhouse-notes.md` ≥ 20 ghi chú thực nghiệm | L2 |
| 13 | README có kiến trúc, quickstart, ảnh dashboard, "những gì tôi học được" | L6 |

---

*Tài liệu này điều phối các phase. Thiết kế chi tiết ở [`PLAN.md`](PLAN.md); checklist từng task ở [`TODO.md`](TODO.md); hạ tầng production ở [`DEPLOY-AWS.md`](DEPLOY-AWS.md).*
