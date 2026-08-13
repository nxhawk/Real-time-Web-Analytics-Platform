# Cấu hình triển khai

<Badge type="tip" text="Level 0 / stack local" />

Trang này giải thích chi tiết những gì nằm trong [`deploy/`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/tree/main/deploy)
và từng khối của [`docker-compose.yml`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/docker-compose.yml)
thực sự làm gì — kèm lý do chọn giá trị đó và chuyện gì hỏng nếu bạn đổi nó.

[Cấu hình](./configuration) nói về các biến mà code Go đọc. Trang này nói về tầng bên dưới:
container, mount và các file cấu hình của ClickHouse server.

## Trong `deploy/` có gì

`deploy/` chứa cấu hình được **mount vào container của bên thứ ba**. Nó không phải code ứng
dụng và không bao giờ được biên dịch — file chỉ đơn giản xuất hiện bên trong container ở đúng
đường dẫn mà image đó vốn đã tìm đến.

```
deploy/
├── caddy/                     .gitkeep   → Level 6: Caddyfile, kết thúc TLS
├── clickhouse/
│   ├── config.d/pulse.xml     ✔ đang dùng → override cấp server
│   └── users.d/pulse.xml      ✔ đang dùng → user, profile, quota
├── grafana/dashboards/        .gitkeep   → Level 6: dashboard provisioning
├── kafka/                     .gitkeep   → Level 4: cấu hình broker và topic
├── prometheus/                .gitkeep   → Level 6: prometheus.yml, scrape job
└── scripts/                   .gitkeep   → script deploy / backup / restore
```

Hiện chỉ hai file ClickHouse là thật sự hoạt động. Phần còn lại là `.gitkeep` giữ chỗ, để level
sau chỉ việc thêm file chứ không phải tự nghĩ ra cấu trúc thư mục — cùng lý do mà
`docker-compose.yml` đã đặt sẵn tên cổng cho Kafka và Grafana.

::: tip Vì sao tách `config.d/` và `users.d/`
Image ClickHouse đọc `/etc/clickhouse-server/config.xml` và `users.xml`, rồi merge đè mọi file
`*.xml` trong `config.d/` và `users.d/` lên trên, theo thứ tự bảng chữ cái. Nhờ vậy thả một file
overlay nhỏ vào là an toàn qua các lần nâng image; sửa thẳng `config.xml` gốc thì không, vì tag
image kế tiếp mang theo bản `config.xml` của riêng nó.
:::

## `docker-compose.yml`, từng khối một

File này mô tả stack **development**: ClickHouse cộng hai service Go. Kafka đến ở Level 4,
dashboard Next.js ở Level 1, Prometheus và Grafana ở Level 6.

### Tên project

```yaml
name: pulse
```

Đặt tên project cho Compose thay vì để nó lấy mặc định theo tên thư mục. Mọi thứ Compose tạo ra
đều mang tiền tố này: network là `pulse_default`, volume là `pulse_clickhouse-data` và
`pulse_clickhouse-logs`. Đổi tên thư mục checkout thì dữ liệu vẫn còn, vì tên volume không đổi.

### Ba anchor YAML

Compose hỗ trợ anchor (`&name`) và merge key (`<<: *name`). Các key bắt đầu bằng `x-` là
extension field: Compose bỏ qua chúng, nên đó là chỗ hợp lệ để đặt một mảnh cấu hình dùng lại
về sau.

```yaml
x-api-common: &api-common     # runtime và security dùng chung cho cả hai API
x-api-env:    &api-env        # biến môi trường dùng chung cho cả hai API
x-build-args: &build-args     # build argument dùng chung cho cả hai image
```

Mục đích là để `ingest-api` và `analytics-api` không thể lệch nhau. Siết chặt một cái là siết
chặt cả hai, vì chỉ có một định nghĩa chứ không phải hai bản sao.

#### `x-api-common`

| Key | Giá trị | Tác dụng |
|---|---|---|
| `restart` | `unless-stopped` | Khởi động lại khi crash và khi Docker daemon start, nhưng nằm im sau khi `docker compose stop` |
| `depends_on.clickhouse.condition` | `service_healthy` | Chưa khởi động API cho tới khi healthcheck của ClickHouse pass — không phải chỉ tới khi container tồn tại |
| `security_opt` | `no-new-privileges:true` | Chặn leo thang qua `setuid` bên trong container |
| `read_only` | `true` | Root filesystem của container ở chế độ chỉ đọc |
| `cap_drop` | `ALL` | Bỏ toàn bộ Linux capability. Một HTTP server Go nghe cổng > 1024 không cần cái nào |
| `logging` | `json-file`, `10m` × 3 | Chặn log ở ~30 MB mỗi container thay vì để nó ăn hết ổ đĩa |

`depends_on: service_healthy` chính là thứ khiến `make up` chạy ổn định. Không có nó, hai API sẽ
đua với ~20 giây khởi động của ClickHouse, kết nối lần đầu thất bại rồi restart liên tục cho tới
khi may mắn thắng.

::: warning `read_only: true` và write-ahead log
Root filesystem chỉ đọc nghĩa là process không ghi được vào đâu trừ chỗ có volume hoặc tmpfs.
`WAL_DIR` mặc định là `./data/wal` (Level 3), mà file này chưa mount gì cho nó cả. Khi đụng tới
đường WAL, hãy thêm cho service `tmpfs: [/tmp]` kèm `WAL_DIR=/tmp/wal`, hoặc một named volume —
nếu không, lần ghi fallback đầu tiên sẽ chết với `read-only file system`.
:::

#### `x-api-env`

```yaml
APP_ENV:              ${APP_ENV:-development}
LOG_LEVEL:            ${LOG_LEVEL:-info}
LOG_FORMAT:           ${LOG_FORMAT:-json}
CLICKHOUSE_DSN:       clickhouse://pulse:${CLICKHOUSE_PASSWORD:-pulse}@clickhouse:9000/analytics
CORS_ALLOWED_ORIGINS: ${CORS_ALLOWED_ORIGINS:-http://localhost:3000}
```

`${VAR:-default}` đọc `VAR` từ shell hoặc từ file `.env` nằm cạnh `docker-compose.yml`, không có
thì lấy giá trị sau `:-`. Nhờ vậy stack chạy được ngay cả khi chưa có `.env`, và
`cp .env.example .env` chỉ thay đổi đúng những gì bạn thật sự sửa.

Hai chi tiết cần nhớ kỹ:

- Host trong DSN là **`clickhouse`**, tên service, do DNS nội bộ của Compose phân giải. Từ máy
  bạn thì cùng database đó lại là `localhost` — đây là nguyên nhân phổ biến nhất của lỗi
  `dial tcp: connection refused` khi chạy lệnh nhầm chỗ.
- Cổng **9000** là native protocol. `8123` là HTTP, dành cho `clickhouse-client` và debug tay.
  Insert luôn đi qua native protocol.

Chú ý thứ **không** có ở đây: `BATCH_SIZE`, `INGEST_WORKERS`, `SINK`… đều vắng mặt, nên mỗi
service dùng default biên dịch sẵn trong `internal/config`. Muốn đổi theo môi trường thì sửa
`.env`, đừng sửa file này.

#### `x-build-args`

```yaml
COMMIT:  ${GIT_COMMIT:-dev}
VERSION: ${GIT_TAG:-dev}
```

Makefile export cả hai:

```make
export GIT_COMMIT := $(shell git rev-parse --short HEAD)
export GIT_TAG    := $(shell git describe --tags --always --dirty)
```

Chúng được truyền vào Dockerfile dưới dạng `ARG` rồi đóng dấu vào binary qua
`-ldflags -X .../internal/version.Tag=...`, và đó chính là thứ `/healthz` trả về. Build bằng
`docker compose build` trần thay vì `make up` thì bạn nhận được `dev` — image vẫn chạy, chỉ là
nó không nói được mình là commit nào.

### Service `clickhouse`

```yaml
image: clickhouse/clickhouse-server:26.3-alpine
```

Ghim theo minor version, không dùng `latest`. ClickHouse đổi giá trị mặc định giữa các bản phát
hành — ngữ nghĩa của `background_pool_size` trong file cấu hình bên dưới phụ thuộc phiên bản —
nên tag thả nổi đồng nghĩa với stack hôm nay và stack ngày mai là hai hệ thống khác nhau.

**Environment.** `CLICKHOUSE_DB`, `CLICKHOUSE_USER` và `CLICKHOUSE_PASSWORD` được entrypoint của
image đọc **chỉ ở lần boot đầu tiên**: nó tạo database và user, rồi ghi ra
`/etc/clickhouse-server/users.d/default-user.xml`. Một khi `pulse_clickhouse-data` đã tồn tại,
đổi `CLICKHOUSE_PASSWORD` trong `.env` sẽ không có tác dụng gì cho tới khi bạn `make nuke` hoặc
tự tay `ALTER USER`.

**Ports.**

| Mapping | Dùng để |
|---|---|
| `${CLICKHOUSE_HTTP_PORT:-8123}:8123` | Giao diện HTTP — `clickhouse-client`, `curl .../ping`, debug |
| `${CLICKHOUSE_NATIVE_PORT:-9000}:9000` | Native protocol — driver Go dùng cổng này |

Cả hai đều cho phép override để máy nào đã chạy sẵn thứ gì đó trên 8123 hay 9000 (một ClickHouse
local, hoặc Portainer ở 9000) không buộc bạn phải sửa file đang được git theo dõi.

**Volumes.**

```yaml
- clickhouse-data:/var/lib/clickhouse                                             # dữ liệu
- clickhouse-logs:/var/log/clickhouse-server                                      # log
- ./deploy/clickhouse/config.d/pulse.xml:/etc/clickhouse-server/config.d/pulse.xml:ro
- ./deploy/clickhouse/users.d/pulse.xml:/etc/clickhouse-server/users.d/pulse.xml:ro
```

Hai cái đầu là named volume, nên `make down` giữ nguyên dữ liệu và chỉ `make nuke` (`down -v`)
mới xoá. Hai cái sau là bind mount chỉ đọc của các file trong `deploy/`.

::: warning Mount file, đừng mount thư mục
Rất dễ bị cám dỗ mount cả `./deploy/clickhouse/users.d` lên `/etc/clickhouse-server/users.d`.
Đừng. Entrypoint cần *ghi* `default-user.xml` vào thư mục đó, và bind thư mục ở chế độ chỉ đọc
làm container chết ngay lúc khởi động. Mount đúng một file thì thư mục vẫn ghi được. Cờ `:ro`
trên file thì vẫn đúng: không thứ gì bên trong container được phép sửa cấu hình bạn giữ trong git.
:::

**`ulimits.nofile: 262144`.** ClickHouse giữ một file descriptor cho mỗi file part, và một bảng
MergeTree đang insert liên tục thì có rất nhiều part. Mặc định 1024 hết rất nhanh, và nó hiện ra
dưới dạng `Too many open files` giữa lúc chạy benchmark chứ không phải lúc khởi động.

**Healthcheck.**

```yaml
test: ["CMD-SHELL", "wget --no-verbose --tries=1 --spider http://127.0.0.1:8123/ping || exit 1"]
interval: 5s   timeout: 3s   retries: 20   start_period: 20s
```

Dùng `wget` vì image Alpine có BusyBox chứ không có `curl`. `--spider` chỉ lấy header.
`start_period: 20s` là khoảng ân hạn: thất bại trong lúc đó không tính vào `retries`, nên một lần
boot chậm (nạp schema, xoay log) không bị hiểu nhầm là server hỏng. Xấu nhất thì service bị coi
là unhealthy sau `20 + 20 × 5 = 120` giây.

### Service `ingest-api` và `analytics-api`

```yaml
ingest-api:
  <<: *api-common
  build:
    context: ./backend
    args:
      <<: *build-args
      SERVICE: ingest-api
  environment:
    <<: *api-env
    HTTP_ADDR: ":8080"
  ports:
    - "${INGEST_PORT:-8080}:8080"
```

Một Dockerfile build cả hai image; `SERVICE` chọn `./cmd/${SERVICE}` ở bước `go build`. Vì thế
hai service chỉ khác nhau đúng ba dòng: build arg, biến địa chỉ lắng nghe (`HTTP_ADDR` so với
`ANALYTICS_ADDR`) và cổng publish.

Địa chỉ lắng nghe phải để trong nháy — `:8080` không nháy là YAML sai, vì dấu hai chấm mở đầu
một scalar sẽ bị hiểu thành mapping.

::: tip Vì sao hai API không khai báo healthcheck
Image runtime là `gcr.io/distroless/static-debian12:nonroot`: một binary tĩnh, không shell,
không `wget`, không `curl`. Bên trong chẳng có gì để chạy probe, mà thêm shell chỉ để chạy probe
thì mất luôn lý do dùng distroless.

Nên readiness được kiểm tra từ **bên ngoài**, đúng như load balancer làm trên production:
`make health` gọi `http://localhost:8080/healthz` và `:8081/healthz` tối đa 30 lần, cách nhau 1
giây. Nếu vẫn muốn probe bên trong container, cách thường dùng là biên dịch một subcommand
`/healthcheck` vào chính binary đó rồi đặt `CMD ["/app", "healthcheck"]`.
:::

Cộng với `read_only`, `cap_drop: ALL`, `no-new-privileges` và `USER nonroot` (uid 65532) của
image, một handler bị chiếm quyền chỉ có: process không phải root, không capability, không
filesystem ghi được và không đường leo thang.

### `volumes`

```yaml
volumes:
  clickhouse-data:
  clickhouse-logs:
```

Khai báo không kèm option, nên Docker tự quản chúng trong `/var/lib/docker/volumes` với tên
`pulse_clickhouse-data` và `pulse_clickhouse-logs`. Xem chi tiết bằng
`docker volume inspect pulse_clickhouse-data`.

## `deploy/clickhouse/config.d/pulse.xml`

Override cấp server. Tinh thần chung: **đừng để ClickHouse hành xử như thể nó sở hữu cả máy.**
Mặc định nó tính cache và pool theo tổng RAM và toàn bộ core — đúng trên máy chủ chuyên dụng,
sai trên laptop đang chạy song song Go, Docker và trình duyệt. Giá trị production nằm ở
`DEPLOY-AWS.md` §9.

| Cấu hình | Giá trị | Vì sao |
|---|---|---|
| `logger.level` | `warning` | Mặc định `trace`/`debug` ghi hàng trăm dòng mỗi query. Đổi sang `debug` khi cần điều tra rồi đổi lại |
| `logger.console` | `true` | Đẩy log ra stdout để `make logs` nhìn thấy |
| `logger.size` / `count` | `200M` / `3` | Trần xoay vòng log bên trong container |
| `max_server_memory_usage` | `4294967296` (4 GiB) | Trần cứng cho toàn server. Không có nó, ClickHouse mặc định lấy ~90% RAM máy và sớm muộn OOM killer sẽ chọn một nạn nhân |
| `mark_cache_size` | `536870912` (512 MiB) | Mark file định vị granule trong part; đây là cache thật sự đáng tiền |
| `uncompressed_cache_size` | `0` | Tắt. Nó chỉ có ích cho point lookup lặp lại trên cùng một dải nhỏ; scan analytics chỉ làm nó bị evict |
| `background_pool_size` | `8` | Thread merge. Merge là thứ ngốn CPU nền nhiều nhất |
| `background_schedule_pool_size` | `8` | Tác vụ định kỳ (TTL, dọn dẹp, replication) |
| `merge_tree.number_of_free_entries_in_pool_to_execute_mutation` | `4` | Xem cảnh báo dưới |
| `merge_tree.number_of_free_entries_in_pool_to_execute_optimize_entire_partition` | `4` | Cùng lý do, cho `OPTIMIZE` toàn partition |

::: warning Ràng buộc khó đoán duy nhất
ClickHouse 26.x từ chối khởi động nếu
`number_of_free_entries_in_pool_to_execute_mutation` (mặc định **20**) lớn hơn
`background_pool_size × concurrency_ratio` — ở đây là `8 × 2 = 16`. Thu nhỏ pool mà quên hạ giá
trị này thì container biến thành vòng lặp boot. Nếu sau này nâng `background_pool_size` lên lại,
hai dòng này có thể trả về mặc định.
:::

**Bảng log hệ thống.** `query_log` và `metric_log` được giữ chứ không tắt — Level 2 đọc
`query_log` để truy nguyên query chậm, tắt đi là mất luôn bằng chứng. Thay vào đó chúng bị chặn
bằng TTL:

| Bảng | Flush | TTL |
|---|---|---|
| `system.query_log` | mỗi 7,5 s | `event_date + INTERVAL 14 DAY DELETE` |
| `system.metric_log` | mỗi 7,5 s, lấy mẫu mỗi 1 s | `event_date + INTERVAL 7 DAY DELETE` |

`metric_log` lấy mẫu mọi metric của server mỗi giây; không có TTL thì trong vòng một tuần nó
thường là bảng lớn nhất trên máy dev.

**Endpoint Prometheus.**

```xml
<prometheus>
  <endpoint>/metrics</endpoint>
  <port>9363</port>
  <metrics>true</metrics> <events>true</events> <asynchronous_metrics>true</asynchronous_metrics>
</prometheus>
```

ClickHouse tự expose metric của nó — không cần exporter sidecar. Level 6 trỏ một scrape job vào
`clickhouse:9363/metrics`.

::: tip Cổng 9363 chưa publish ra host
`docker-compose.yml` mới map 8123 và 9000. Prometheus sẽ vào được qua network của Compose, nhưng
muốn mở bằng trình duyệt ngay bây giờ thì thêm `- "9363:9363"` vào mục `ports`.
:::

**Tính năng bị gỡ.**

```xml
<mysql_port remove="remove"/>
<postgresql_port remove="remove"/>
```

`remove="remove"` xoá hẳn node thừa kế từ `config.xml` gốc chứ không phải ghi đè giá trị.
ClickHouse nói được cả giao thức dây của MySQL lẫn PostgreSQL; dự án này không dùng cái nào, nên
bớt đi hai socket đang lắng nghe khỏi bề mặt tấn công.

## `deploy/clickhouse/users.d/pulse.xml`

Hai profile, hai user. **Profile** là một gói cấu hình có tên, áp cho mọi query mà user đó chạy —
kể cả query bạn gõ tay trong `clickhouse-client`. Đó chính là lý do các chốt chặn nằm ở đây thay
vì trong mệnh đề `SETTINGS` từng query, thứ rất dễ quên.

### Profile `pulse_app` — ứng dụng

| Cấu hình | Giá trị | Tác dụng |
|---|---|---|
| `max_execution_time` | `15` | Query bị giết sau 15 giây. Khớp với `CLICKHOUSE_QUERY_TIMEOUT` phía client, để hai bên bỏ cuộc cùng lúc |
| `max_memory_usage` | `4000000000` (~3,7 GiB) | Trần cho mỗi query; query chết chứ server không chết |
| `max_bytes_before_external_group_by` | `2000000000` (~1,9 GiB) | Vượt ngưỡng này, `GROUP BY` đổ ra đĩa thay vì fail. Chậm, nhưng chạy xong |
| `max_concurrent_queries_for_user` | `32` | Từ chối query thứ 33 ngay lập tức thay vì xếp hàng. Fail nhanh hơn là một hàng đợi không ai theo dõi |
| `network_compression_method` | `LZ4` | Nén luồng insert trên đường truyền. LZ4 thay ZSTD: rẻ CPU hơn nhiều, mà mạng không phải nút thắt khi chạy local |
| `insert_distributed_sync` | `0` | Không bao giờ chờ ack phân tán |
| `async_insert` | `0` | **Cố ý tắt** — Level 3 so batching phía client với `async_insert` phía server, phép so chỉ có nghĩa khi baseline tắt tính năng này |
| `log_queries` | `1` | Ghi vào `system.query_log`; phân tích ở Level 2 phụ thuộc vào đó |

::: warning `max_memory_usage` quá sát `max_server_memory_usage`
Trần mỗi query (~3,7 GiB) gần bằng trần toàn server (4 GiB). Một query nặng có thể ăn hết ngân
sách của cả server và bỏ đói các query chạy song song. Đây là đánh đổi có chủ ý cho môi trường
dev — thà một query lớn chạy xong còn hơn fail — nhưng trên máy dùng chung hoặc production thì
hãy đặt trần mỗi query bằng một phần của trần server.
:::

### Profile `pulse_readonly` — dashboard

Cùng giới hạn thời gian và bộ nhớ, cộng thêm `readonly=2`. ClickHouse có ba mức: `0` toàn quyền,
`1` chỉ đọc kể cả setting, `2` chỉ đọc nhưng được đổi setting cho phiên hiện tại. `2` đúng với
nhu cầu của dashboard — nó truyền được `max_threads` cho một query, còn `INSERT`, `ALTER` và
`DROP` bị chính server từ chối chứ không phải bị code ứng dụng chặn.

### Users

| User | Profile | Quota | Mật khẩu | Mục đích |
|---|---|---|---|---|
| `pulse` | `pulse_app` | `default` | lấy từ `CLICKHOUSE_PASSWORD`, qua `default-user.xml` do entrypoint sinh | Cả hai service Go: đọc và ghi |
| `dashboard` | `pulse_readonly` | `dashboard` | rỗng trong git — **không dùng được cho tới khi được cấp** | Level 6: đường đọc của analytics API |

`access_management=0` trên `pulse` nghĩa là nó không tạo được user và không cấp được quyền. Tài
khoản ứng dụng không nên có khả năng tự viết lại quyền của chính mình.

`<password></password>` rỗng ở `dashboard` là cố ý chứ không phải sót: trong ClickHouse mật khẩu
rỗng nghĩa là *không có credential hợp lệ*, nên user tồn tại nhưng không đăng nhập được cho tới
khi một mật khẩu thật được tiêm vào lúc deploy. Commit một placeholder tình cờ dùng được chính là
cách placeholder lọt lên production.

`<networks><ip>::/0</ip></networks>` cho phép kết nối từ mọi nơi — chấp nhận được vì đường duy
nhất tới cổng 9000 là network của Compose cộng với những gì firewall máy bạn cho qua. Trên
production, security group mới là biên giới (`DEPLOY-AWS.md`).

### Quota `dashboard`

```xml
<interval>
  <duration>3600</duration>              <!-- cửa sổ trượt 1 giờ -->
  <queries>10000</queries>               <!-- số query tối đa -->
  <errors>500</errors>                   <!-- số lỗi tối đa -->
  <execution_time>1800</execution_time>  <!-- tối đa 30 phút CPU -->
</interval>
```

Quota giới hạn mức tiêu thụ theo thời gian, còn profile giới hạn từng query. Nó là chốt chặn cho
trường hợp dashboard poll trong vòng lặp vì bug frontend — 10.000 query mỗi giờ là rất rộng rãi
với người dùng thật và rất dễ vượt với một `setInterval` quên cleanup.

`pulse` dùng quota `default` có sẵn, không giới hạn.

## Mọi thứ ráp lại

```
.env / biến môi trường shell
      │  thay thế ${VAR:-default}
      ▼
docker-compose.yml ──build args──▶ backend/Dockerfile ──▶ image distroless
      │                                                        │
      │  bind mount :ro                                        │ biến môi trường
      ▼                                                        ▼
deploy/clickhouse/config.d/pulse.xml   ┐              internal/config
deploy/clickhouse/users.d/pulse.xml    ├─ merge đè lên ─┐      │
entrypoint image → users.d/default-user.xml            │      │  clickhouse://pulse@clickhouse:9000
                                        config.xml gốc ┘      ▼
                                                          ClickHouse
```

Hai hệ cấu hình không bao giờ chạm nhau: biến môi trường cấu hình các process Go, file XML cấu
hình ClickHouse server. `CLICKHOUSE_PASSWORD` là giá trị duy nhất xuất hiện ở cả hai phía — đó là
lý do nó là một biến chứ không phải hai.

## Thao tác hằng ngày

| Việc | Lệnh |
|---|---|
| Chạy tất cả rồi chờ tới khi healthy | `make up` |
| Dừng, giữ dữ liệu | `make down` |
| Dừng và xoá sạch dữ liệu local | `make nuke` |
| Theo dõi log một service | `make logs S=clickhouse` |
| Kiểm tra readiness từ host | `make health` |
| Mở SQL shell trong container | `make ch-cli` |
| Áp dụng thay đổi XML | `docker compose restart clickhouse` |

Sau khi sửa một trong hai file XML, hãy kiểm tra xem ClickHouse có thật sự nhận không — một lỗi
gõ có thể khiến server bỏ qua file hoặc từ chối khởi động:

```sql
SELECT name, value FROM system.settings WHERE name = 'max_execution_time';
SELECT name, profile FROM system.users;
SELECT * FROM system.quota_usage;
```

::: warning Nếu ClickHouse restart liên tục
Đọc `make logs S=clickhouse` trước. Hai nguyên nhân phổ biến nhất là XML sai cú pháp trong
`deploy/` (server từ chối khởi động) và ràng buộc mutation pool nói ở trên. Bind cả thư mục
`users.d/` tạo ra nguyên nhân thứ ba: entrypoint không ghi được `default-user.xml`.
:::

## Những chỗ còn thiếu

- **`make bench` tham chiếu `docker-compose.bench.yml`** nhưng file đó chưa tồn tại. Overlay này
  đến cùng phép so ClickHouse với PostgreSQL ở Level 3.
- **`WAL_DIR` chưa có mount** trong khi `read_only: true` đang bật — xem cảnh báo phía trên.
- **User `dashboard` chưa có mật khẩu**, đúng như thiết kế. Phải cấp trước Level 6, nếu không
  analytics API không dùng được đường chỉ đọc.
- **Cổng 9363 chưa publish**, nên endpoint Prometheus của ClickHouse hiện chỉ truy cập được từ
  bên trong network của Compose.

## Xem thêm

- [Cấu hình](./configuration) — mọi biến môi trường mà code Go đọc
- [Triển khai](./deployment) — kiến trúc production trên AWS
- [Schema ClickHouse](../reference/clickhouse) — bảng, codec, phân vùng
- [Runbook](../notes/runbook) — làm gì khi có sự cố
