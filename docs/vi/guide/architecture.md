# Kiến trúc

Hệ thống xây theo hai phase. Phase 1 là monolith ghi thẳng vào ClickHouse, phủ Level 1 đến 3.
Phase 2 chèn Kafka vào giữa ingest và storage rồi tách binary; bắt đầu từ Level 4.

Cả hai phase tuân thủ cùng một luật: **đường ingest không bao giờ phụ thuộc vào việc ClickHouse
có sống hay không.**

::: tip Cách đọc sơ đồ
Mọi sơ đồ trong trang này là mã Mermaid, render ngay trên trình duyệt. Màu sắc mang ý nghĩa,
không phải trang trí: tím là client, xanh dương là service HTTP viết bằng Go, xanh ngọc là xử
lý bất đồng bộ, cam là message bus, hổ phách là storage, đỏ là đường lỗi.
:::

## Phase 1 — monolith

```mermaid
flowchart TB
    SDK(["Web / SDK<br/>pulse.js · sendBeacon"])
    IH["Ingest HTTP handler<br/>validate · enrich"]

    %% Khai báo chiều phản hồi trước chiều request để client giữ được hàng trên cùng.
    IH -.->|"202 Accepted"| SDK
    SDK -->|"POST /api/v1/events"| IH

    subgraph API["Go / Gin — đường ghi"]
        direction TB
        IH
        BW["Batch Writer<br/>worker pool"]
        IH -->|"chan có buffer"| BW
    end

    WAL[["WAL dự phòng<br/>segment NDJSON trên đĩa"]]
    CH[("ClickHouse<br/>events · events_hourly MV<br/>daily_users MV")]
    AA["Analytics API<br/>/api/v1/analytics/*"]
    DASH["Next.js Dashboard"]

    BW -->|"INSERT — BATCH_SIZE row hoặc FLUSH_INTERVAL_MS"| CH
    BW -.->|"hết 3 lần retry"| WAL
    WAL -.->|"replay"| CH
    CH -->|"SELECT ... GROUP BY"| AA
    AA -->|"JSON envelope"| DASH

    classDef client fill:#7c3aed,stroke:#5b21b6,color:#ffffff
    classDef api fill:#2563eb,stroke:#1d4ed8,color:#ffffff
    classDef proc fill:#0d9488,stroke:#0f766e,color:#ffffff
    classDef store fill:#a16207,stroke:#854d0e,color:#ffffff
    classDef ui fill:#334155,stroke:#1e293b,color:#ffffff
    classDef fallback fill:#b91c1c,stroke:#991b1b,color:#ffffff

    class SDK client
    class IH,AA api
    class BW proc
    class CH store
    class WAL fallback
    class DASH ui
    style API fill:none,stroke:#94a3b8,stroke-width:1px,stroke-dasharray:5 4
```

HTTP handler **không** ghi vào ClickHouse. Nó validate, enrich, đẩy vào channel có buffer rồi
trả `202`. Một worker pool rút channel đó và insert theo lô `BATCH_SIZE` row hoặc mỗi
`FLUSH_INTERVAL_MS`, cái nào tới trước.

Khi ClickHouse từ chối insert, writer retry ba lần với backoff luỹ thừa kèm jitter, rồi ghi
batch ra write-ahead log dạng NDJSON trên đĩa. Một tiến trình replay đọc lại các file đó sau.
Đó là thứ biến "kill ClickHouse năm phút mà không mất gì" thành một bài test thay vì một hy vọng.

### Một event, từ đầu đến cuối

Mũi tên `202` nét đứt ở sơ đồ trên chính là điểm mấu chốt: response rời đi trước khi row được
lưu xuống.

```mermaid
sequenceDiagram
    autonumber
    actor U as Trình duyệt (pulse.js)
    participant M as Middleware
    participant H as Ingest handler
    participant S as Event service
    participant B as Batch writer
    participant CH as ClickHouse

    U->>M: POST /api/v1/events (batch tối đa 500 event)

    rect rgba(37, 99, 235, 0.10)
        Note over M,H: request id · CORS · recover panic · giới hạn body 1 MiB
        M->>M: X-API-Key phân giải ra site_id
    end

    M->>H: request đã decode, site_id nằm trong context
    H->>S: validate và enrich

    rect rgba(13, 148, 136, 0.10)
        S->>S: validate theo từng event, không theo cả batch
        S->>S: tra GeoIP rồi bỏ IP đi
        S->>S: parse UA · session_id · ingested_at
    end

    S--)B: đẩy event hợp lệ vào chan có buffer
    H-->>U: 202 Accepted kèm số accepted và rejected

    Note over S,CH: flush khi đủ BATCH_SIZE row, hoặc mỗi FLUSH_INTERVAL_MS

    B->>CH: INSERT INTO events (batch)
    alt insert thành công
        CH-->>B: ok
    else hết 3 lần retry
        rect rgba(185, 28, 28, 0.10)
            B->>B: ghi batch vào WAL NDJSON, replay sau
        end
    end
```

Mọi thứ trước `202` xảy ra bên trong request. Mọi thứ sau đó là việc của writer, và ClickHouse
chậm hay chết cũng không đổi được response trả về cho trình duyệt.

## Phase 2 — event pipeline

```mermaid
flowchart TB
    SDK(["Web / SDK"])
    ING["Go Ingest API<br/>validate · enrich"]
    K{{"Kafka — events.raw<br/>6 partition · retention 7d"}}
    C1["Consumer<br/>group ch-sink<br/>batch 10k / 500 ms"]
    C2["Consumer<br/>group alerting"]
    ML["Tương lai<br/>ML / ETL"]
    DLQ{{"events.dlq"}}
    CH[("ClickHouse")]
    AN["Go Analytics API"]
    RD[("Redis cache<br/>tuỳ chọn")]
    DASH["Next.js Dashboard"]

    SDK -->|"POST /api/v1/events"| ING
    ING -.->|"202 Accepted ngay"| SDK
    ING -->|"produce — async, acks=1"| K
    K --> C1
    K --> C2
    K -.-> ML
    C1 -->|"INSERT batch rồi commit offset"| CH
    C2 -->|"không xử lý được"| DLQ
    CH --> AN
    AN -.->|"cache read-through"| RD
    AN -->|"JSON envelope"| DASH

    classDef client fill:#7c3aed,stroke:#5b21b6,color:#ffffff
    classDef api fill:#2563eb,stroke:#1d4ed8,color:#ffffff
    classDef proc fill:#0d9488,stroke:#0f766e,color:#ffffff
    classDef queue fill:#c2410c,stroke:#9a3412,color:#ffffff
    classDef store fill:#a16207,stroke:#854d0e,color:#ffffff
    classDef ui fill:#334155,stroke:#1e293b,color:#ffffff
    classDef fallback fill:#b91c1c,stroke:#991b1b,color:#ffffff
    classDef future fill:#64748b,stroke:#475569,color:#ffffff,stroke-dasharray:4 3

    class SDK client
    class ING,AN api
    class C1,C2 proc
    class K queue
    class DLQ fallback
    class CH store
    class DASH ui
    class ML,RD future
```

Kafka mang lại ba thứ mà ghi trực tiếp không có: ingest API thôi quan tâm ClickHouse có tới
được không, event replay được sau khi sửa schema, và một consumer group thứ hai đọc cùng luồng
mà không đụng vào đường ghi.

Cái giá là at-least-once. Consumer commit offset **sau khi** ClickHouse xác nhận insert, không
bao giờ trước, nên crash chỉ gây gửi lại chứ không mất. Trùng lặp được khử ở tầng query bằng
`event_id`. Xem ADR-0005 ở [trang quyết định](/vi/adr/).

### Offset được commit ở đâu

```mermaid
sequenceDiagram
    autonumber
    actor U as Trình duyệt (pulse.js)
    participant I as Ingest API
    participant K as Kafka events.raw
    participant C as Consumer ch-sink
    participant CH as ClickHouse
    participant D as events.dlq

    U->>I: POST /api/v1/events
    I->>I: validate và enrich
    I--)K: produce (async, acks=1, key là site_id)
    I-->>U: 202 Accepted

    loop mỗi 10k message hoặc 500 ms
        K->>C: fetch record
        C->>CH: INSERT batch
        alt insert thành công
            rect rgba(13, 148, 136, 0.10)
                CH-->>C: ok
                C->>K: commit offset — sau ack, không bao giờ trước
            end
        else lỗi có thể retry
            C->>C: backoff rồi thử lại chính batch đó
            Note over C,CH: offset chưa commit nên crash chỉ gây gửi lại
        else không xử lý được
            rect rgba(185, 28, 28, 0.10)
                C->>D: đẩy sang events.dlq kèm lý do
                C->>K: commit offset
            end
        end
    end
```

At-least-once là một đánh đổi có chủ đích: một row trùng thì khử ở tầng query rất rẻ, còn một
event đã mất thì không lấy lại được.

## Vòng đời một request

Chuyện gì xảy ra với một event, từ đầu đến cuối:

1. **Middleware.** Gắn request id UUIDv7 (hoặc tái dùng `X-Request-ID` từ ngoài vào), panic
   được chuyển thành `500` đúng envelope chuẩn, áp CORS, giới hạn body 1 MiB.
2. **Xác thực.** Header `X-API-Key` phân giải ra `site_id`, đặt vào context. Mọi query sau đó
   đều bị giới hạn theo nó — không có chuyện đọc chéo tenant.
3. **Validate.** Theo từng event, không theo cả batch. Batch 100 event có 3 event hỏng thì nhận
   97 và trả 3 cái kia trong mảng `rejected`. Một event hỏng không bao giờ làm mất cả lô.
4. **Enrich.** IP → country/city qua MaxMind GeoLite2, rồi **bỏ IP đi**. User-Agent → device,
   OS, browser. Thiếu `session_id` thì sinh từ hash cửa sổ 30 phút. Đóng dấu `ingested_at` để
   đo độ trễ.
5. **Sink.** Buffer, hoặc Kafka producer, tuỳ biến `SINK`.
6. **Phản hồi.** `202 Accepted`, kèm số event đã nhận và danh sách bị từ chối.

## Đường đọc

Đường ghi được tinh chỉnh để không mất event. Đường đọc được tinh chỉnh cho đúng một con số:
một panel dashboard trả lời dưới 300 ms ở mốc 100M row.

```mermaid
sequenceDiagram
    autonumber
    actor A as Người phân tích
    participant N as Next.js dashboard
    participant Q as Analytics API
    participant R as Redis cache (tuỳ chọn)
    participant CH as ClickHouse

    A->>N: mở panel overview cho 7 ngày gần nhất
    N->>Q: GET /api/v1/analytics/overview kèm X-API-Key
    Q->>Q: rate limit 120 req/phút theo IP · site_id lấy từ API key

    opt bật cache
        Q->>R: đọc cửa sổ đã cache
        R-->>Q: nếu hit thì trả luôn, không đụng ClickHouse
    end

    rect rgba(161, 98, 7, 0.12)
        Q->>CH: SELECT từ events_hourly, lọc theo site_id
        Note over Q,CH: max_execution_time và max_memory_usage chặn mọi query
        CH-->>Q: các dòng đã tổng hợp sẵn
    end

    Q-->>N: JSON envelope, p95 dưới 300 ms
    N-->>A: biểu đồ đã render
```

Materialized view là thứ khiến con số đó khả thi: `/analytics/overview` đọc `events_hourly`,
nhỏ hơn `events` vài bậc độ lớn, nên query chạm vào số giờ chứ không phải số row thô. Xem
[schema ClickHouse](/vi/reference/clickhouse).

## Vì sao ClickHouse

Workload là append-only, đọc chủ yếu ở dạng tổng hợp, và mọi query đều có hình dạng "gom
khoảng thời gian này theo một chiều cardinality thấp". Đó đúng là hình dạng mà column store
sinh ra để phục vụ: chỉ đọc những cột có trong query, tỉ lệ nén trên dữ liệu cột đã sắp xếp
tốt hơn hàng chục lần so với row store, và thực thi được vector hoá.

ClickHouse cũng dở ở những thứ dự án này không cần: point lookup theo primary key, UPDATE và
DELETE, JOIN lớn, transaction. Level 3 đo cả hai mặt đó so với PostgreSQL và ghi số lại — xem
[kết quả benchmark](/vi/notes/benchmark-results).

## Ngưỡng hiệu năng

Mỗi dòng dưới đây đều được kiểm chứng bằng test hoặc phép đo, không phải tuyên bố suông:

| Mục tiêu | Giá trị | Kiểm chứng ở |
|---|---|---|
| Dashboard query p95 ở 100M event | < 300 ms | Level 5 |
| `/analytics/overview` sau materialized view | < 100 ms | Level 5 |
| `/analytics/realtime` | < 200 ms | Level 5 |
| Throughput ingest | 10.000 event/s trong 10 phút, drop = 0 | Level 3 |
| p99 latency của ingest API | < 50 ms | Level 3 |
| End-to-end lag p99 ở 5k event/s | < 5 s | Level 4 |

## Sửa các sơ đồ này

Sơ đồ là các khối code `mermaid` ngay trong file này, do `vitepress-plugin-mermaid` render.
Hai quy tắc giữ cho chúng đồng bộ:

- **Không bao giờ ghim `theme` của Mermaid.** Plugin tự chuyển sang theme tối theo site. Màu
  của node đến từ bảng `classDef` khai báo ở cuối mỗi flowchart, và các màu nền đó đi kèm chữ
  trắng nên đọc được trên cả nền sáng lẫn nền tối.
- **Dùng lại bảng màu.** `client`, `api`, `proc`, `queue`, `store`, `ui`, `fallback` và
  `future` mang cùng một ý nghĩa trên mọi sơ đồ trong tài liệu. Hộp mới chọn một class có sẵn
  thay vì thêm màu mới.

## Đọc thêm

- [Schema ClickHouse](/vi/reference/clickhouse) — bảng, materialized view, vì sao có từng cái
- [Event schema](/vi/reference/event-schema) — contract payload và quy tắc validate
- [Quyết định kiến trúc](/vi/adr/) — mười quyết định và đánh đổi của chúng
