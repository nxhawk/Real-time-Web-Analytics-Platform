# Observability

## Hiện đã có gì

Cả hai binary phát `/metrics` trên cổng chính, với Go runtime collector, process collector và
một gauge build-info:

```
pulse_build_info{service="ingest-api",tag="v0.1.0",commit="abc1234",go_version="go1.26.5"} 1
```

`pulse_build_info` cho phép dashboard chú thích đồ thị latency bằng đúng bản build tạo ra nó —
đó là cách bạn biết p99 tăng gấp đôi lúc 14:32 là vì một lần deploy chứ không phải vì traffic.

Cả tiến trình chỉ có một registry, `metrics.Registry`. Nó cố ý **không** phải
`prometheus.DefaultRegisterer`, để một thư viện bên thứ ba không thể lặng lẽ thêm metric.

ClickHouse tự phát endpoint Prometheus riêng ở cổng 9363, bật trong
`deploy/clickhouse/config.d/pulse.xml`.

## Metric ứng dụng

<Badge type="warning" text="Level 6" />

| Metric | Kiểu | Nhãn |
|---|---|---|
| `pulse_events_received_total` | counter | site, event_name |
| `pulse_events_rejected_total` | counter | site, reason |
| `pulse_events_dropped_total` | counter | site |
| `pulse_buffer_size` | gauge | worker |
| `pulse_batch_size` | histogram | |
| `pulse_batch_flush_duration_seconds` | histogram | result |
| `pulse_clickhouse_query_duration_seconds` | histogram | endpoint |
| `pulse_kafka_consumer_lag` | gauge | topic, partition |
| `pulse_http_request_duration_seconds` | histogram | method, route, status |
| `pulse_end_to_end_lag_seconds` | histogram | |

`pulse_end_to_end_lag_seconds` là `ingested_at − event_time`. Đây là con số duy nhất nói lên
pipeline có theo kịp hay không; consumer lag nói Kafka có ứ hay không, còn end-to-end lag nói
dashboard có đang nói thật hay không.

::: danger Cardinality
Nhãn dùng **route pattern** — `/api/v1/sites/:id` — không bao giờ dùng path thô. Path thô biến
mỗi URL duy nhất thành một time series mới và sẽ hạ gục Prometheus trước cả khi hạ gục ứng dụng.
Luật đó cũng áp cho nhãn `site` khi có nhiều tenant.
:::

## Dashboard

Bốn cái, provision từ file trong `deploy/grafana/dashboards/` để chúng là code chứ không phải
những cú click:

1. **Ingest health** — event nhận và bị loại, độ sâu buffer, số bị drop, kích thước batch, thời
   gian flush.
2. **ClickHouse internals** — số part mỗi bảng, hoạt động merge, bộ nhớ, đĩa, thời gian query
   lấy từ `system.query_log`.
3. **Kafka** — consumer lag theo partition, số record xử lý, tỉ lệ DLQ, số lần rebalance.
4. **API RED** — rate, error và duration theo từng route.

## Cảnh báo

Bốn rule ánh xạ vào các kiểu hỏng thật, không phải vào ngưỡng tuỳ tiện:

| Cảnh báo | Vì sao quan trọng |
|---|---|
| Consumer lag tăng liên tục 10 phút | Pipeline đang tụt lại; dashboard đang hiển thị số cũ |
| Đĩa trên 75% | ClickHouse dừng nhận ghi khi đầy, và khôi phục thì chậm |
| Tỉ lệ lỗi ingest trên 1% | Event đang bị mất ngay lúc này |
| Không nhận event nào trong 5 phút | Hoặc traffic dừng, hoặc pipeline hỏng — cả hai đều cần người |

## Logging

`log/slog` với JSON handler ở production, text ở local. Mỗi dòng mang `service`, `env` và
`request_id`; log request thêm `method`, `route`, `status`, `bytes` và `duration`, cộng
`site_id` khi đã có xác thực.

`/healthz`, `/readyz` và `/metrics` bị loại khỏi log request — chúng bị poll vài giây một lần
và sẽ chôn vùi traffic thật.

Status code ánh xạ sang level: 5xx log ở error, 4xx ở warn, còn lại ở info. Panic được cứu sẽ
log ở error kèm stack trace đã cắt ngắn và trả về envelope `500` chuẩn.

**Không bao giờ log payload của event.** Level 6 thêm một test khẳng định không payload nào xuất
hiện ở output mức info (task L6-10) — một luật không ai kiểm tra là một luật sẽ mục nát.

## Tracing

<Badge type="info" text="Tuỳ chọn, Level 5" />

OpenTelemetry trải từ HTTP → service → query ClickHouse, export sang Jaeger hoặc Tempo. Đánh dấu
tuỳ chọn: với structured log mang request id và histogram theo từng endpoint, tracing ở đây thêm
ít giá trị hơn so với một hệ thống nhiều service.

## Endpoint health

| Endpoint | Câu hỏi nó trả lời | Hành vi khi ClickHouse chết |
|---|---|---|
| `/healthz` | Có nên restart tiến trình này không? | Vẫn `200` |
| `/readyz` | Có nên gửi traffic cho tiến trình này không? | `503`, kèm lý do từng dependency |

Làm ngược điều này — kiểm tra storage từ endpoint liveness — biến một cú vấp ngắn của ClickHouse
thành vòng lặp restart trên mọi replica.

Thêm một dependency vào readiness nghĩa là hiện thực hai method:

```go
type Prober interface {
	Name() string
	Check(ctx context.Context) error
}
```

rồi truyền vào constructor của router. Mỗi lần kiểm tra bị giới hạn bởi timeout hai giây.
