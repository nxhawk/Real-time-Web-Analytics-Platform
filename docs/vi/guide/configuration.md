# Cấu hình

Mọi thứ đọc từ môi trường bởi `backend/internal/config`, package duy nhất được phép chạm vào
`os.Getenv`. File `.env` local được nạp trước nếu có; biến môi trường thật luôn thắng nó.

Mọi biến đều có default chạy được cho môi trường dev, nên `cp .env.example .env` rồi để nguyên
là một cấu hình hợp lệ.

## Validate

Cấu hình được kiểm tra lúc khởi động, và `Validate()` gom **tất cả** lỗi trước khi trả về:

```
invalid configuration: APP_ENV must be one of development|staging|production|test, got "prod";
LOG_LEVEL must be one of debug|info|warn|error, got "verbose"; BATCH_SIZE must be greater than 0
```

Tiến trình thoát hẳn thay vì chạy với cấu hình nửa vời.

## Ứng dụng

| Biến | Mặc định | Mô tả |
|---|---|---|
| `APP_ENV` | `development` | `development` · `staging` · `production` · `test`. Production và test đặt gin ở release mode |
| `SHUTDOWN_TIMEOUT` | `30s` | Thời gian cho request đang chạy hoàn tất sau `SIGTERM` |

## HTTP

| Biến | Mặc định | Mô tả |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Địa chỉ lắng nghe của ingest API |
| `ANALYTICS_ADDR` | `:8081` | Địa chỉ lắng nghe của analytics API |
| `HTTP_READ_HEADER_TIMEOUT` | `5s` | Chống slowloris |
| `HTTP_READ_TIMEOUT` | `15s` | |
| `HTTP_WRITE_TIMEOUT` | `30s` | Nới lên trước khi làm CSV export dạng stream ở Level 5 |
| `HTTP_IDLE_TIMEOUT` | `60s` | |
| `HTTP_MAX_BODY_BYTES` | `1048576` | 1 MiB, khớp contract API |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Ngăn cách bằng dấu phẩy |

::: warning Hai trường hợp biên của CORS
Để rỗng nghĩa là **không** cho phép cross-origin — middleware trở thành no-op và không phát
header CORS nào, thay vì làm crash tiến trình. Đúng một dấu `*` thì cho phép mọi origin và tự
động tắt credentials, vì trình duyệt cũng từ chối cặp đó.
:::

## Logging

| Biến | Mặc định | Mô tả |
|---|---|---|
| `LOG_LEVEL` | `info` | `debug` · `info` · `warn` · `error` |
| `LOG_FORMAT` | `json` | `json` cho production, `text` để đọc ở local |

Mỗi dòng log mang `service`, `env` và `request_id`. Log request dùng route pattern, không dùng
path thô. `/healthz`, `/readyz` và `/metrics` không được log — chúng bị poll vài giây một lần
và sẽ nhấn chìm traffic thật.

## ClickHouse

| Biến | Mặc định | Mô tả |
|---|---|---|
| `CLICKHOUSE_DSN` | `clickhouse://pulse:pulse@localhost:9000/analytics` | **Native protocol, cổng 9000.** Không bao giờ dùng cổng HTTP để insert |
| `CLICKHOUSE_PASSWORD` | `pulse` | Docker Compose dùng khi tạo user |
| `CLICKHOUSE_MAX_OPEN_CONNS` | `16` | |
| `CLICKHOUSE_MAX_IDLE_CONNS` | `8` | |
| `CLICKHOUSE_CONN_MAX_LIFETIME` | `10m` | |
| `CLICKHOUSE_DIAL_TIMEOUT` | `5s` | |
| `CLICKHOUSE_QUERY_TIMEOUT` | `15s` | Khớp với `max_execution_time` phía server |

Trong Docker Compose host là `clickhouse`; từ máy bạn là `localhost`.

## Ingest và đường ghi

| Biến | Mặc định | Mô tả |
|---|---|---|
| `SINK` | `direct` | `direct` ghi thẳng ClickHouse; `kafka` produce vào topic (Level 4+) |
| `INSERT_MODE` | `batch` | `single` tồn tại chỉ để benchmark Level 3 có mốc so sánh |
| `BATCH_SIZE` | `5000` | Số row mỗi lần insert |
| `FLUSH_INTERVAL_MS` | `500` | Thời gian chờ tối đa trước khi flush lô chưa đầy |
| `BUFFER_SIZE` | `100000` | Hàng đợi giữa handler và writer. Phải ≥ `BATCH_SIZE` |
| `INGEST_WORKERS` | `4` | Số worker của batch writer |
| `WAL_DIR` | `./data/wal` | Nơi ghi batch khi không tới được ClickHouse |
| `MAX_EVENTS_PER_REQUEST` | `500` | Contract API chặn ở 500 |
| `INGEST_RATE_LIMIT_PER_MIN` | `1000` | Theo từng API key |

::: tip Các giá trị này là tạm thời
`BATCH_SIZE` và `FLUSH_INTERVAL_MS` sẽ được chỉnh lại ở Level 3 dựa trên throughput, số part
sinh ra và p99 đo được. Khi số đo nói khác, sửa `PHASES.md` §2.4 trước, rồi lan sang code,
`.env.example` và trang này.
:::

## Kafka

Chưa dùng cho tới Level 4. Cứ để `KAFKA_BROKERS` rỗng — đặt `SINK=kafka` mà không có broker là
lỗi khởi động, cố ý như vậy.

| Biến | Mặc định | Mô tả |
|---|---|---|
| `KAFKA_BROKERS` | *(rỗng)* | Danh sách broker |
| `KAFKA_TOPIC_RAW` | `events.raw` | 6 partition, retention 7 ngày, nén zstd |
| `KAFKA_TOPIC_DLQ` | `events.dlq` | 1 partition, retention 30 ngày |
| `KAFKA_GROUP_ID` | `clickhouse-sink` | Consumer group |
| `KAFKA_CONSUMER_BATCH_SIZE` | `10000` | Số record mỗi lần poll trước khi insert |

## Cổng host cho Docker Compose

Đổi khi cổng đã bị chiếm: `INGEST_PORT`, `ANALYTICS_PORT`, `CLICKHOUSE_HTTP_PORT`,
`CLICKHOUSE_NATIVE_PORT`.

## Bí mật

Secret lấy từ môi trường hoặc secret store của hệ thống deploy, không bao giờ từ source. Trên
production, `.env` được tạo trên máy đích với quyền `600` và không bao giờ commit; từ Level 6
`gitleaks` chạy trên mọi pull request.

## Thêm một thiết lập mới

1. Thêm field vào struct phù hợp trong `backend/internal/config/config.go`, kèm tag `env` và
   `envDefault`.
2. Thêm kiểm tra vào `Validate()`. Thiết lập nào có thể sai thì phải báo ngay lúc khởi động.
3. Ghi vào `.env.example` **và** trang này.
