# Cấu hình

Cấu hình nằm ở hai nơi, và chỉ hai nơi này:

```
backend/config/<APP_ENV>.config.yml   default được commit, review được — nguồn sự thật
.env và biến môi trường của tiến trình  secret và giá trị riêng từng máy
```

`backend/internal/config` là package duy nhất được phép chạm vào `os.Getenv`. Mọi package
khác nhận `*config.Config` qua constructor.

## Một giá trị được quyết định thế nào

`APP_ENV` chọn file. `APP_ENV=staging` nạp `backend/config/staging.config.yml`; không đặt thì
mặc định là `development`.

Trong file đó, mọi thứ viết dạng `${VAR}` hoặc `${VAR:-fallback}` được thay bằng giá trị từ
môi trường lúc khởi động:

```yaml
ingest:
  batch_size: "${BATCH_SIZE:-5000}"   # lấy BATCH_SIZE nếu có, không thì 5000
clickhouse:
  dsn: "${CLICKHOUSE_DSN}"            # bắt buộc — không có fallback
```

Dạng thứ hai chính là lý do tách hai nơi. Placeholder không có fallback là **bắt buộc**: biến
không được đặt thì tiến trình từ chối khởi động và nêu đích danh tên biến.
`production.config.yml` dùng dạng này cho `CLICKHOUSE_DSN` và `CORS_ALLOWED_ORIGINS`, nên một
bản production không thể âm thầm thừa hưởng default localhost rồi trông thì khỏe mạnh mà
không phục vụ gì cả.

```
invalid configuration: unset environment variables with no fallback:
CLICKHOUSE_DSN (required by clickhouse.dsn); CORS_ALLOWED_ORIGINS (required by http.cors_allowed_origins)
```

`.env` được nạp trước, nên nó có thể cung cấp các biến đó mà không cần export ra shell. Biến
môi trường thật luôn thắng `.env`. Thiếu `.env` là chuyện bình thường — trong container thì
không có — nhưng `.env` sai cú pháp là lỗi khởi động.

::: tip Loader tìm file ở đâu
Loader đi ngược lên từ thư mục làm việc để tìm `config/`, nhờ vậy `make run` chạy được từ
`backend/` và `go test ./...` chạy được từ bất kỳ đâu trong cây thư mục. `CONFIG_DIR` ghi đè
cơ chế đó; image Docker đặt `CONFIG_DIR=/config` vì binary chạy từ `/` và không có cây source
nào xung quanh. `ENV_FILE` làm điều tương tự cho `.env`.
:::

## Thứ tự ưu tiên

Bốn tầng quyết định một giá trị. Cao nhất trước:

| # | Nguồn | Thắng cái gì |
|---|---|---|
| 1 | Biến môi trường thật của tiến trình | tất cả những cái dưới |
| 2 | Cùng biến đó trong `.env` | fallback và literal |
| 3 | Giá trị sau `:-` trong YAML | phần còn lại |
| 4 | Literal viết trần trong YAML | — không ai override được, xem bên dưới |

Theo trình tự nạp, tiến trình làm thế này:

```
loadDotEnv()           .env được đọc trước — godotenv không bao giờ ghi đè một biến đã
                       có sẵn, đó là lý do tầng 1 thắng tầng 2
      ▼
os.Getenv("APP_ENV")   đọc SAU .env, nên .env cũng chọn được file
      ▼
config/<APP_ENV>.config.yml
      ▼
expandEnvVars()        ${VAR} → os.LookupEnv; không có thì lấy phần sau :-;
                       không có mà cũng không có :- là lỗi, kèm tên biến
      ▼
Unmarshal → app.env phải khớp tên file → Validate()
```

::: danger Biến môi trường chỉ có tác dụng nếu YAML có khai báo
Đây là điểm hành vi thay đổi khi cấu hình rời khỏi struct tag `env`. Một key viết trần thì
hoàn toàn không override được:

```yaml
ingest:
  batch_size: 5000                       # BATCH_SIZE=100 không có tác dụng gì
  batch_size: "${BATCH_SIZE:-5000}"      # BATCH_SIZE=100 chạy đúng
```

Knob nào cần chỉnh được theo từng bản deploy thì phải có placeholder — ở **cả bốn** file.
:::

### Bắt buộc ở đây, tùy chọn ở kia

Tầng 3 chính là thứ làm việc tách hai nơi trở nên đáng giá: cùng một key có thể bắt buộc ở
môi trường này và có default ở môi trường kia, không cần thêm dòng Go nào.

```yaml
# development.config.yml — clone về là chạy, không cần .env
dsn: "${CLICKHOUSE_DSN:-clickhouse://pulse:pulse@localhost:9000/analytics}"

# production.config.yml — thiếu là tiến trình từ chối khởi động
dsn: "${CLICKHOUSE_DSN}"
```

### Đặt cái gì ở đâu

Hai câu hỏi khác nhau, hai câu trả lời khác nhau.

**Giá trị khác nhau giữa các môi trường nhưng không phải secret** thì thuộc về file
`*.config.yml` tương ứng. Đó là thứ bốn file mua về cho bạn: `log.level` là `debug` ở staging
và `info` ở production vì file ghi vậy, nằm trong một diff có người review.

**Secret hoặc giá trị riêng từng máy** thì dùng placeholder `${VAR}` và đi vào qua môi trường:

| Môi trường | Bơm biến bằng cách nào |
|---|---|
| Dev ở local | `.env` ở gốc repo |
| Docker Compose | block `environment:` trong `docker-compose.yml` (`x-api-env`) |
| CI | `env:` trong workflow, hoặc GitHub secrets |
| Server dùng systemd | `EnvironmentFile=/etc/pulse/pulse.env` |
| Docker thuần | `docker run --env-file /etc/pulse/pulse.env` |
| Kubernetes | `envFrom:` trỏ tới Secret hoặc ConfigMap |

::: warning Hai cơ chế `${VAR}` trông y hệt nhau
`docker-compose.yml` có cơ chế thay thế của riêng nó, và **không phải** cơ chế mô tả trong
trang này. Compose expand `${VAR}` trước khi container khởi động, đọc file `.env` nằm cạnh
`docker-compose.yml`. Loader Go expand `${VAR}` bên trong container, đọc môi trường của chính
container đó.

Nên `CLICKHOUSE_PASSWORD` trong `.env` đi qua hai chặng: Compose đọc nó rồi đặt
`CLICKHOUSE_DSN` vào môi trường container, sau đó `expandEnvVars` mới giải placeholder trong
`production.config.yml` từ đấy. Biến nào Compose không chuyển tiếp thì không tới được tiến
trình Go, dù nhìn trong `.env` có vẻ rất đúng.
:::

## Validate

Cấu hình được kiểm tra lúc khởi động, và `Validate()` gom **tất cả** lỗi trước khi trả về.
Thông báo nêu cả key trong YAML lẫn biến môi trường tương ứng:

```
invalid configuration in config/development.config.yml:
app.env (APP_ENV) must be one of development|staging|production|test, got "prod";
log.level (LOG_LEVEL) must be one of debug|info|warn|error, got "verbose";
ingest.batch_size (BATCH_SIZE) must be greater than 0
```

Tiến trình thoát hẳn thay vì chạy với cấu hình nửa vời. `app.env` còn phải khớp với tên file
đã nạp, để bắt trường hợp copy file rồi đặt sai tên.

## Bốn file

| `APP_ENV` | File | Khác nhau ở đâu |
|---|---|---|
| `development` *(mặc định)* | `development.config.yml` | Mọi thứ đều có fallback: clone về là chạy được, không cần `.env`. `log.format` là `text` |
| `test` | `test.config.yml` | Batch nhỏ, timeout ngắn để test không phải ngồi chờ lô 5000 dòng. Cũng là file CI smoke test dùng |
| `staging` | `staging.config.yml` | `clickhouse.dsn` bắt buộc. `log.level` mặc định `debug`, CORS là `*` |
| `production` | `production.config.yml` | `clickhouse.dsn` **và** `cors_allowed_origins` đều bắt buộc. `log.format` là `json` |

## Ứng dụng

| Key YAML | Biến | Mặc định (development) | Mô tả |
|---|---|---|---|
| `app.env` | `APP_ENV` | `development` | Chọn file. Production và test đặt gin ở release mode |
| `app.shutdown_timeout` | `SHUTDOWN_TIMEOUT` | `30s` | Thời gian cho request đang chạy hoàn tất sau `SIGTERM` |

## HTTP

| Key YAML | Biến | Mặc định (development) | Mô tả |
|---|---|---|---|
| `http.ingest_addr` | `HTTP_ADDR` | `:8080` | Địa chỉ lắng nghe của ingest API |
| `http.analytics_addr` | `ANALYTICS_ADDR` | `:8081` | Địa chỉ lắng nghe của analytics API |
| `http.read_header_timeout` | `HTTP_READ_HEADER_TIMEOUT` | `5s` | Chống slowloris |
| `http.read_timeout` | `HTTP_READ_TIMEOUT` | `15s` | |
| `http.write_timeout` | `HTTP_WRITE_TIMEOUT` | `30s` | Nới lên trước khi làm CSV export dạng stream ở Level 5 |
| `http.idle_timeout` | `HTTP_IDLE_TIMEOUT` | `60s` | |
| `http.max_body_bytes` | `HTTP_MAX_BODY_BYTES` | `1048576` | 1 MiB, khớp contract API |
| `http.cors_allowed_origins` | `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Ngăn cách bằng dấu phẩy |

::: warning Hai trường hợp biên của CORS
Để rỗng nghĩa là **không** cho phép cross-origin — middleware trở thành no-op và không phát
header CORS nào, thay vì làm crash tiến trình. Đúng một dấu `*` thì cho phép mọi origin và tự
động tắt credentials, vì trình duyệt cũng từ chối cặp đó.
:::

## Logging

| Key YAML | Biến | Mặc định (development) | Mô tả |
|---|---|---|---|
| `log.level` | `LOG_LEVEL` | `info` | `debug` · `info` · `warn` · `error` |
| `log.format` | `LOG_FORMAT` | `text` | `json` ở staging và production, `text` để đọc ở local |

Mỗi dòng log mang `service`, `env` và `request_id`. Log request dùng route pattern, không dùng
path thô. `/healthz`, `/readyz` và `/metrics` không được log — chúng bị poll vài giây một lần
và sẽ nhấn chìm traffic thật.

## ClickHouse

| Key YAML | Biến | Mặc định (development) | Mô tả |
|---|---|---|---|
| `clickhouse.dsn` | `CLICKHOUSE_DSN` | `clickhouse://pulse:pulse@localhost:9000/analytics` | **Native protocol, cổng 9000.** Không bao giờ dùng cổng HTTP để insert. Bắt buộc ở staging và production |
| — | `CLICKHOUSE_PASSWORD` | `pulse` | Docker Compose dùng khi tạo user, code Go không đọc |
| `clickhouse.max_open_conns` | `CLICKHOUSE_MAX_OPEN_CONNS` | `16` | |
| `clickhouse.max_idle_conns` | `CLICKHOUSE_MAX_IDLE_CONNS` | `8` | |
| `clickhouse.conn_max_lifetime` | `CLICKHOUSE_CONN_MAX_LIFETIME` | `10m` | |
| `clickhouse.dial_timeout` | `CLICKHOUSE_DIAL_TIMEOUT` | `5s` | |
| `clickhouse.query_timeout` | `CLICKHOUSE_QUERY_TIMEOUT` | `15s` | Khớp với `max_execution_time` phía server |

Trong Docker Compose host là `clickhouse`; từ máy bạn là `localhost`.

## Ingest và đường ghi

| Key YAML | Biến | Mặc định (development) | Mô tả |
|---|---|---|---|
| `ingest.sink` | `SINK` | `direct` | `direct` ghi thẳng ClickHouse; `kafka` produce vào topic (Level 4+) |
| `ingest.insert_mode` | `INSERT_MODE` | `batch` | `single` tồn tại chỉ để benchmark Level 3 có mốc so sánh |
| `ingest.batch_size` | `BATCH_SIZE` | `5000` | Số row mỗi lần insert |
| `ingest.flush_interval_ms` | `FLUSH_INTERVAL_MS` | `500` | Thời gian chờ tối đa trước khi flush lô chưa đầy |
| `ingest.buffer_size` | `BUFFER_SIZE` | `100000` | Hàng đợi giữa handler và writer. Phải ≥ `batch_size` |
| `ingest.workers` | `INGEST_WORKERS` | `4` | Số worker của batch writer |
| `ingest.wal_dir` | `WAL_DIR` | `./data/wal` | Nơi ghi batch khi không tới được ClickHouse |
| `ingest.max_events_per_request` | `MAX_EVENTS_PER_REQUEST` | `500` | Contract API chặn ở 500 |
| `ingest.rate_limit_per_min` | `INGEST_RATE_LIMIT_PER_MIN` | `1000` | Theo từng API key |

::: tip Các giá trị này là tạm thời
`batch_size` và `flush_interval_ms` sẽ được chỉnh lại ở Level 3 dựa trên throughput, số part
sinh ra và p99 đo được. Khi số đo nói khác, sửa `PHASES.md` §2.4 trước, rồi lan sang
`backend/config/*.config.yml` và trang này.
:::

## Kafka

Chưa dùng cho tới Level 4. Cứ để `KAFKA_BROKERS` rỗng — đặt `sink: kafka` mà không có broker
là lỗi khởi động, cố ý như vậy.

| Key YAML | Biến | Mặc định (development) | Mô tả |
|---|---|---|---|
| `kafka.brokers` | `KAFKA_BROKERS` | *(rỗng)* | Danh sách broker |
| `kafka.topic_raw` | `KAFKA_TOPIC_RAW` | `events.raw` | 6 partition, retention 7 ngày, nén zstd |
| `kafka.topic_dlq` | `KAFKA_TOPIC_DLQ` | `events.dlq` | 1 partition, retention 30 ngày |
| `kafka.group_id` | `KAFKA_GROUP_ID` | `clickhouse-sink` | Consumer group |
| `kafka.batch_size` | `KAFKA_CONSUMER_BATCH_SIZE` | `10000` | Số record mỗi lần poll trước khi insert |

## Cổng host cho Docker Compose

Đổi khi cổng đã bị chiếm: `INGEST_PORT`, `ANALYTICS_PORT`, `CLICKHOUSE_HTTP_PORT`,
`CLICKHOUSE_NATIVE_PORT`. Đây là biến `docker-compose.yml` đọc, không phải code Go, nên không
có key YAML tương ứng.

## Bí mật

Secret lấy từ môi trường hoặc secret store của hệ thống deploy, không bao giờ từ source. Các
file YAML được commit chính vì chúng không chứa secret — chỉ có default và placeholder. Trên
production, `.env` được tạo trên máy đích với quyền `600` và không bao giờ commit; từ Level 6
`gitleaks` chạy trên mọi pull request.

## Thêm một thiết lập mới

`backend/internal/config` chia mỗi section cấu hình thành một file — `app.go`, `http.go`,
`log.go`, `clickhouse.go`, `ingest.go`, `kafka.go` — nên một thiết lập được khai báo và kiểm
tra ở cùng một chỗ. `config.go` chỉ giữ struct `Config` tổng hợp và các quy tắc liên quan tới
nhiều section; `load.go` và `expand.go` giữ phần tìm file và phân giải `${VAR}`.

1. Thêm field vào đúng file section của nó, kèm tag `mapstructure`.
2. Thêm key vào **cả bốn** file trong `backend/config/`, dùng placeholder `${VAR:-fallback}`
   nếu muốn override được. Key có trong struct nhưng thiếu ở một file sẽ decode thành giá trị
   zero — `TestShippedFilesLoad` sinh ra để bắt đúng lỗi đó.
3. Thêm kiểm tra vào method `validate` của chính section đó — không phải `Config.Validate`,
   nơi chỉ chứa quy tắc bắc cầu giữa các section. Thiết lập nào có thể sai thì phải báo ngay
   lúc khởi động.
4. Ghi vào trang này, và nếu là thứ người ta thật sự hay override thì ghi thêm vào
   `.env.example`.

Thêm hẳn một section mới cũng theo đúng khuôn đó: tạo `<tên>.go` chứa struct và `validate`
của nó, thêm field vào `Config`, và khai báo trong `Config.sections`. Không phải sửa gì khác.
