# Chạy thử nhanh

## Yêu cầu

| Công cụ | Phiên bản | Để làm gì |
|---|---|---|
| Docker + Compose v2 | mới | ClickHouse và các service chạy trong container |
| Go | 1.26 | Build và chạy backend |
| Node.js | 22+ | Dashboard (Level 1) và trang tài liệu này |
| `make` | bất kỳ | Mọi lệnh đều đi qua Makefile |

::: tip Vì sao Go 1.26 mà không phải 1.27
1.27 vẫn đang ở mức release candidate. Dự án pin bản stable mới nhất. Chỗ pin nằm ở
`backend/go.mod`, `backend/Dockerfile` và `.github/workflows/ci-backend.yml` — sửa cả ba cùng lúc.
:::

## Chạy

```bash
git clone https://github.com/nxhawk/Real-time-Web-Analytics-Platform.git
cd Real-time-Web-Analytics-Platform

# 1. Cấu hình. Mọi biến đều có default chạy được, nên copy nguyên xi là đủ.
cp .env.example .env

# 2. Tải dependency Go. Chỉ lần đầu — lệnh này sinh ra backend/go.sum.
make deps

# 3. Khởi động ClickHouse và hai API, rồi chờ tới khi chúng trả lời.
make up
```

`make up` build image, khởi động stack và poll `/healthz` cho tới khi cả hai service phản hồi.
Trên máy sạch mất khoảng hai đến ba phút, phần lớn là thời gian pull image.

## Kiểm tra

```bash
curl localhost:8080/healthz    # {"status":"ok"}
curl localhost:8080/readyz     # {"status":"ok","checks":{}}
curl localhost:8080/version    # tag, commit, build time, phiên bản Go
curl localhost:8080/metrics    # Prometheus exposition
curl localhost:8081/healthz    # analytics API, cùng bộ endpoint vận hành
```

`checks` rỗng vì Level 0 chưa gắn dependency nào vào readiness. ClickHouse vào ở Level 1,
Kafka ở Level 4 — xem [cấu trúc dự án](/vi/guide/project-structure) để biết điểm mở rộng.

Mở shell ClickHouse:

```bash
make ch-cli
# rồi gõ: SELECT version()
```

## Làm việc với backend

Chạy API trên máy thay vì trong container, trỏ vào ClickHouse đang chạy bằng Docker:

```bash
make down-app   # dừng container API, giữ ClickHouse
make run        # go run ./cmd/ingest-api
```

Trước khi mở pull request:

```bash
make check      # fmt + vet + lint + test có race detector — đúng bộ CI chạy
```

## Những lệnh sẽ dùng hằng ngày

`make help` in ra tất cả.

| Lệnh | Làm gì |
|---|---|
| `make up` / `make down` | Bật / tắt stack |
| `make nuke` | Tắt và xoá volume — mất sạch dữ liệu local |
| `make ps` / `make logs` | Trạng thái service / xem log (`make logs S=ingest-api`) |
| `make health` | Poll `/healthz` trên cả hai API tới khi có phản hồi |
| `make build` | Build binary vào `backend/bin/` |
| `make test` | Unit test có race detector kèm tóm tắt coverage |
| `make lint` / `make fmt` / `make vet` | golangci-lint / gofmt + goimports / go vet |
| `make check` | Toàn bộ những gì CI chạy |
| `make migrate-up` | Chạy migration <Badge type="warning" text="Level 1" /> |
| `make seed N=10000000` | Sinh event giả lập <Badge type="warning" text="Level 3" /> |
| `make bench` | Benchmark ClickHouse vs PostgreSQL <Badge type="warning" text="Level 3" /> |
| `make tools` | Cài golangci-lint và goimports |

## Chạy trang tài liệu này

```bash
cd docs
npm ci
npm run dev      # http://localhost:5173
npm run build    # output tĩnh ở docs/.vitepress/dist
```

Build sẽ **fail** nếu có link nội bộ chết — đây là chủ ý, xem
[đóng góp](/vi/guide/contributing#tài-liệu).

## Xử lý sự cố

**Cổng đã bị chiếm.** Đổi cổng host trong `.env`: `INGEST_PORT`, `ANALYTICS_PORT`,
`CLICKHOUSE_HTTP_PORT`, `CLICKHOUSE_NATIVE_PORT`.

**`make up` treo ở bước chờ health.** Xem `make logs S=clickhouse`. Máy ít RAM thì giảm
`max_server_memory_usage` trong `deploy/clickhouse/config.d/pulse.xml`.

**`missing go.sum entry`.** Chạy `make deps` — repo cố ý chỉ ship `go.mod`, không ship `go.sum`.

**Tải dependency lỗi vì proxy.** Đặt biến `GOPROXY` trong shell trước khi chạy `make deps`.
