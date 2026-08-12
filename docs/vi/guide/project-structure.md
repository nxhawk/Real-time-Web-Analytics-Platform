# Cấu trúc dự án

Monorepo, để contract API và client tiêu thụ nó di chuyển cùng nhau, và một lần chạy CI phủ cả
hai. Phần Go theo layout quy ước `cmd/` + `internal/` + `pkg/`.

## Cây thư mục

```
pulse-analytics/
├── backend/                        # Go services — một module
│   ├── cmd/                        # mỗi binary một thư mục; main() chỉ wire dependency
│   │   ├── ingest-api/             #   đường ghi: nhận event qua HTTP              [L0 ✓]
│   │   ├── analytics-api/          #   đường đọc: trả lời dashboard query          [L0 ✓]
│   │   ├── consumer/               #   Kafka → ClickHouse sink                     [L4]
│   │   ├── migrate/                #   chạy migration                              [L1]
│   │   └── seeder/                 #   sinh event giả lập                          [L3]
│   ├── internal/                   # riêng tư với module này — compiler ép buộc
│   │   ├── config/                 #   package DUY NHẤT được đọc biến môi trường   [L0 ✓]
│   │   ├── logging/                #   cấu hình slog: JSON ở prod, text ở local    [L0 ✓]
│   │   ├── version/                #   metadata build nhúng qua -ldflags           [L0 ✓]
│   │   ├── metrics/                #   registry Prometheus duy nhất                [L0 ✓]
│   │   ├── httpx/                  #   middleware, envelope lỗi, server, engine    [L0 ✓]
│   │   ├── handler/                #   tầng HTTP: decode → service → encode        [L0 ✓]
│   │   ├── service/                #   nghiệp vụ                                   [L1]
│   │   ├── repository/
│   │   │   ├── clickhouse/         #     kết nối, repo, queries/*.sql nhúng        [L1]
│   │   │   └── postgres/           #     chỉ dùng để benchmark so sánh             [L3]
│   │   ├── model/                  #   kiểu domain dùng chung giữa các tầng        [L1]
│   │   ├── validate/               #   quy tắc validate event                      [L1]
│   │   ├── buffer/                 #   batch writer, backpressure, WAL fallback    [L3]
│   │   └── kafka/                  #   producer, consumer, DLQ                     [L4]
│   ├── pkg/                        # import được từ ngoài: wrapper geoip, uaparser
│   ├── migrations/                 # migration goose đánh số, .up.sql / .down.sql
│   ├── test/                       # integration test (testcontainers) + fixture
│   ├── Dockerfile                  # multi-stage → distroless, mỗi SERVICE một image
│   └── .golangci.yml               # cấu hình lint
│
├── frontend/                       # Dashboard Next.js                             [L1]
├── sdk/js/                         # Snippet pulse.js, < 2 KB gzip                 [L5]
│
├── deploy/                         # cấu hình runtime, không phải code ứng dụng
│   ├── caddy/                      #   reverse proxy + TLS tự động
│   ├── clickhouse/config.d/        #   thiết lập server: memory, log, Prometheus
│   ├── clickhouse/users.d/         #   profile và quota: guard query, user readonly
│   ├── kafka/ prometheus/ grafana/ #   phần còn lại của stack                      [L4, L6]
│   └── scripts/                    #   script hỗ trợ deploy
│
├── infra/                          # Terraform cho đường AWS                       [AWS]
├── loadtest/                       # script k6 + benchmark ClickHouse vs PostgreSQL
├── docs/                           # trang tài liệu này + contract API
│
├── docker-compose.yml              # stack dev: ClickHouse + hai API
├── docker-compose.prod.yml         # stack production                              [L6]
├── docker-compose.bench.yml        # thêm PostgreSQL cho benchmark                 [L3]
├── Makefile                        # mọi lệnh phát triển — chạy `make help`
└── .env.example                    # mọi biến cấu hình, có chú thích
```

## Luật phân tầng

Phụ thuộc chỉ đi một chiều — `cmd` → `handler` → `service` → `repository` — không bao giờ ngược.

| Tầng | Trách nhiệm | Không được |
|---|---|---|
| `cmd/` | Wire dependency, khởi động server, xử lý shutdown | Chứa nghiệp vụ |
| `handler/` | Decode request, gọi một service, encode response | Nói chuyện với storage hay dựng SQL |
| `service/` | Nghiệp vụ: validate, enrich, điều phối | Biết mình đang được gọi qua HTTP |
| `repository/` | Truy cập storage, SQL viết tay | Chứa nghiệp vụ |
| `httpx/` | Hạ tầng transport tái dùng cho mọi service | Biết gì về analytics |
| `config/` | Đọc và validate môi trường | Bị `os.Getenv` ở nơi khác đi vòng |

Khi đặt file mới, hỏi nó **làm gì**, không phải nó thuộc tính năng nào. Query funnel vào
`repository/clickhouse/`; luật "funnel tối đa 8 bước" vào `service/`; việc biến `?steps=a,b,c`
thành slice vào `handler/`.

## Điểm mở rộng

Ba mối nối đã đặt sẵn để các level sau chỉ thêm vào chứ không phải mổ lại.

### `handler.Prober` — readiness

Bất cứ thứ gì trả lời được câu "bây giờ mày dùng được không" đều gắn vào `/readyz` được mà
không đụng health handler:

```go
type Prober interface {
	Name() string
	Check(ctx context.Context) error
}
```

ClickHouse hiện thực nó ở Level 1, Kafka ở Level 4:

```go
router := handler.NewIngestRouter(cfg, log, clickhouseConn, kafkaProducer)
```

Probe fail thì `/readyz` thành `503` kèm lý do từng dependency, còn `/healthz` vẫn `200` —
liveness không được phụ thuộc storage, nếu không orchestrator sẽ restart một tiến trình khoẻ
mạnh mỗi lần ClickHouse chớp mắt.

### `httpx.Server.Run(ctx, hooks...)` — shutdown

Shutdown hook chạy sau khi listener HTTP đóng và trước khi tiến trình thoát. Đó là chỗ dành cho
việc flush batch writer, để không mất event đã nhận khi deploy:

```go
server := httpx.NewServer(serviceName, addr, router, cfg, log)
return server.Run(ctx, batchWriter.Close, kafkaProducer.Close)
```

Hook vẫn chạy ngay cả khi shutdown HTTP đã hết thời gian chờ — flush event trong buffer quan
trọng hơn vài kết nối không chịu đóng.

### `config.Config` — thêm thiết lập

Thêm một knob luôn là ba chỗ sửa: field kèm tag `env` và default, một kiểm tra trong
`Validate()`, và một mục có chú thích trong `.env.example`. `Validate()` báo **mọi** lỗi tìm
được chứ không dừng ở lỗi đầu, để một deployment sai cấu hình sửa được trong một lượt.

## Quy ước

`make check` chạy gofmt, `go vet`, golangci-lint (21 linter) và test có race detector — đúng bộ
CI chạy. Ngoài những gì linter thấy được:

- Lỗi bọc bằng `%w` kèm ngữ cảnh: `fmt.Errorf("insert batch: %w", err)`.
- **Hoặc** log **hoặc** trả lỗi, không làm cả hai.
- `context.Context` là tham số đầu tiên của mọi hàm có I/O.
- Nhãn metric và log dùng route pattern (`c.FullPath()`), không bao giờ dùng path thô — path
  thô làm nổ cardinality.
- Test dạng bảng kèm `t.Parallel()` là phong cách mặc định.
- Không `fmt.Println`, không `log.Printf`, và không bao giờ log payload của event.
