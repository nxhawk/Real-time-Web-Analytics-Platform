# HTTP API

Base path `/api/v1`. Mọi response đều là JSON, `Content-Type: application/json; charset=utf-8`.

Contract dạng máy đọc là `docs/api/openapi.yaml`, và đó là nguồn sự thật duy nhất — type của
frontend sinh ra từ nó và không bao giờ sửa tay. Trang này là bản tóm tắt cho người.

::: info Mức độ sẵn sàng
Hiện chỉ có các endpoint vận hành. Ingest và analytics vào ở Level 1, các endpoint analytics
nâng cao ở Level 5. Mỗi dòng dưới đây đều có đánh dấu.
:::

## Vận hành

Cả hai binary đều phục vụ.

| Method | Path | Mô tả | Trạng thái |
|---|---|---|---|
| `GET` | `/healthz` | Liveness — `200` chừng nào tiến trình còn sống. Không bao giờ chạm dependency | ✅ |
| `GET` | `/readyz` | Readiness — ping mọi dependency đã đăng ký; `503` kèm lý do từng cái nếu có lỗi | ✅ |
| `GET` | `/metrics` | Prometheus exposition | ✅ |
| `GET` | `/version` | Tag, commit sha, thời điểm build, phiên bản Go | ✅ |

```bash
$ curl -s localhost:8080/readyz | jq
{
  "status": "ok",
  "checks": {}
}

$ curl -s localhost:8080/version | jq
{
  "tag": "v0.1.0",
  "commit": "abc1234",
  "build_time": "2026-08-12T10:00:00Z",
  "go_version": "go1.26.5"
}
```

`healthz` và `readyz` trả lời hai câu hỏi khác nhau. Liveness hỏi "có nên restart tôi không";
readiness hỏi "có nên gửi traffic cho tôi không". ClickHouse chết không được làm restart một
tiến trình khoẻ, nên chỉ `/readyz` xuống cấp.

## Ingest

| Method | Path | Auth | Mô tả | Trạng thái |
|---|---|---|---|---|
| `POST` | `/events` | `X-API-Key` | Nhận 1–500 event, thành công một phần | <Badge type="warning" text="L1" /> |
| `GET` | `/pixel.gif` | `?k=` | Pixel dự phòng khi không có JavaScript | <Badge type="warning" text="L1" /> |

**Thành công một phần.** Batch 100 event có 3 event sai thì nhận 97 và trả `202` kèm mảng
`rejected`. Một event hỏng không bao giờ làm mất cả lô.

```json
{
  "accepted": 97,
  "rejected": [
    { "index": 12, "reason": "event name must match ^[a-z0-9_]{1,64}$" },
    { "index": 40, "reason": "properties exceed 8KB" },
    { "index": 71, "reason": "page exceeds 2048 characters" }
  ]
}
```

Hình dạng payload nằm ở trang [event schema](/vi/reference/event-schema).

## Analytics

Mọi endpoint analytics đều cần `X-API-Key` hoặc session cookie của dashboard. API key quyết định
`site_id`, và `site_id` được áp trong mọi query — không có chuyện đọc chéo tenant.

**Tham số dùng chung:** `from`, `to` (ISO date hoặc datetime), `tz` (mặc định
`Asia/Ho_Chi_Minh`; dữ liệu lưu ở UTC), `filter[device]`, `filter[country]`, `filter[page]`,
`filter[event]`.

| Method | Path | Trả về | Trạng thái |
|---|---|---|---|
| `GET` | `/analytics/overview` | users, sessions, events, pageviews, revenue, bounce rate, session trung bình, delta | <Badge type="warning" text="L1" /> |
| `GET` | `/analytics/timeseries` | `{series:[{ts,value}], interval}`; `metric=users\|events\|sessions\|revenue`, `interval=hour\|day\|week` | <Badge type="warning" text="L1" /> |
| `GET` | `/analytics/pages` | `{items:[{page,views,users,avg_time_sec}], total}` | <Badge type="warning" text="L1" /> |
| `GET` | `/analytics/devices` | `{items:[{name,users,events,pct}]}` | <Badge type="warning" text="L1" /> |
| `GET` | `/analytics/countries` | như trên | <Badge type="warning" text="L1" /> |
| `GET` | `/analytics/browsers` | như trên | <Badge type="warning" text="L5" /> |
| `GET` | `/analytics/os` | như trên | <Badge type="warning" text="L5" /> |
| `GET` | `/analytics/sources` | breakdown theo referrer domain và UTM | <Badge type="warning" text="L5" /> |
| `GET` | `/analytics/funnel` | `{steps:[{name,users,conv_from_prev,conv_from_first}]}`; 2–8 bước, window 60s–7 ngày | <Badge type="warning" text="L5" /> |
| `GET` | `/analytics/retention` | `{cohorts:[{date,size,values:[…]}]}`; `cohort=day\|week` | <Badge type="warning" text="L5" /> |
| `GET` | `/analytics/realtime` | `{active_users, events_last_5m, top_pages, by_country}` | <Badge type="warning" text="L5" /> |
| `GET` | `/analytics/events` | luồng event thô, phân trang bằng cursor `(event_time, event_id)` | <Badge type="warning" text="L5" /> |
| `GET` | `/analytics/export?format=csv` | CSV dạng stream qua `FORMAT CSVWithNames` của ClickHouse | <Badge type="warning" text="L5" /> |

## Lỗi

Một envelope duy nhất, ở mọi nơi:

```json
{
  "error": {
    "code": "invalid_range",
    "message": "from must be before to",
    "details": {}
  },
  "request_id": "0192f8a1-0000-7000-8000-000000000001"
}
```

`request_id` là UUIDv7 sinh cho mỗi request, echo lại trong header `X-Request-ID`, và có mặt
trong mọi dòng log của request đó. Hãy trích nó khi báo lỗi.

| Mã | HTTP | Ý nghĩa |
|---|---|---|
| `invalid_json` | 400 | Body không phải JSON hợp lệ |
| `validation_failed` | 400 | Payload đúng dạng nhưng vi phạm quy tắc |
| `unauthorized` | 401 | Thiếu hoặc sai API key |
| `not_found` | 404 | Route không tồn tại, hoặc method không được phép |
| `payload_too_large` | 413 | Body vượt `HTTP_MAX_BODY_BYTES` |
| `rate_limited` | 429 | Vượt hạn mức theo key hoặc theo IP |
| `invalid_range` | 400 | `from` không đứng trước `to` |
| `range_too_large` | 400 | Khoảng thời gian vượt 400 ngày |
| `upstream_unavailable` | 503 | Một dependency chết và request không phục vụ được |
| `internal` | 500 | Bất kỳ lỗi ngoài dự kiến nào; chi tiết nằm trong log, không nằm trong response |

## Hạn mức

| Hạn mức | Giá trị |
|---|---|
| Event mỗi request | 1–500 |
| Body request | 1 MiB |
| `properties` mỗi event | 8 KB sau serialize |
| Khoảng thời gian query | 400 ngày |
| Tham số `limit` | 1000 |
| Tần suất ingest | 1000 request/phút mỗi API key |
| Tần suất analytics | 120 request/phút mỗi IP |
| Guard phía server | `max_execution_time = 15`, `max_memory_usage = 4GB` |

Hai guard cuối đặt ở **profile user** của ClickHouse chứ không phải trong mệnh đề `SETTINGS`
từng query — mệnh đề thì dễ quên, còn profile áp cả với query gõ tay trong `clickhouse-client`.
