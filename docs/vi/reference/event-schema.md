# Event schema

## Payload

`POST /api/v1/events` nhận một event đơn lẻ hoặc cả batch, cùng một hình dạng.

```jsonc
{
  "site_id": "site_abc",              // bắt buộc, phải khớp API key
  "events": [
    {
      "event_id": "0192f8a1-...",     // UUIDv7 do client sinh, dùng để dedup
      "event": "page_view",           // bắt buộc, snake_case, <= 64 ký tự
      "user_id": "u_123",             // id ẩn danh khi người dùng chưa đăng nhập
      "session_id": "s_456",          // server tự suy ra nếu bỏ trống
      "timestamp": "2026-08-11T14:20:00.123Z",  // ISO 8601 UTC
      "page": "/products/123",
      "referrer": "https://google.com/",
      "utm": { "source": "google", "medium": "cpc", "campaign": "summer" },
      "device": "desktop",            // desktop | mobile | tablet | bot | unknown
      "os": "macOS",
      "browser": "Chrome",
      "screen": "1920x1080",
      "country": "VN",                // server enrich từ IP nếu bỏ trống
      "city": "Ho Chi Minh City",
      "revenue": 199000,              // chỉ với event mua hàng
      "currency": "VND",
      "properties": { "product_id": "123", "category": "shoes" }
    }
  ]
}
```

Chỉ `site_id` và `event` là bắt buộc. Mọi thứ còn lại hoặc tuỳ chọn, hoặc do server điền.

## Quy tắc validate

Validate theo từng event, không bao giờ theo cả batch.

Dưới một chữ "validate" có hai việc khác nhau. Có lỗi phải trả bằng cả event; có lỗi chỉ trả
bằng chính giá trị đó. Cách chia là có chủ đích: loại nhóm thứ hai là vứt đi traffic thật vì
những tình huống mà người dùng không chọn, còn nhận nhóm thứ nhất một cách âm thầm là làm hỏng
chính những con số dashboard báo cáo.

### Lỗi làm event bị loại

Event đó quay về trong `rejected: [{index, reason}]`, phần còn lại của batch vẫn được lưu.

| Trường | Quy tắc | `reason` |
|---|---|---|
| bản thân phần tử | Phải decode được thành object JSON | `malformed_event` |
| `event` | Bắt buộc | `missing_event_name` |
| `event` | `^[a-z0-9_]{1,64}$` | `invalid_event_name` |
| `event_id` | Là UUID nếu có gửi | `invalid_event_id` |
| `timestamp` | Parse được ISO 8601 nếu có gửi | `invalid_timestamp` |
| `properties` | Là **object** JSON, không phải mảng hay scalar | `invalid_properties` |
| `properties` | ≤ 8 KB sau serialize | `properties_too_large` |
| `revenue` | Vừa `Decimal(18, 4)`: ≤ 14 chữ số phần nguyên, ≤ 4 chữ số thập phân, không dạng mũ | `invalid_revenue` |

### Lỗi chỉ sửa giá trị

Event vẫn được lưu. Mỗi phép sửa tăng `pulse_events_field_repaired_total{field,repair}`, nên ở
đây không có gì diễn ra âm thầm.

| Trường | Quy tắc | Phép sửa |
|---|---|---|
| `event_id` | Thiếu | Server sinh một UUIDv7 |
| `timestamp` | Thiếu | Đặt bằng `now()` |
| `timestamp` | Lệch quá 24h tương lai hoặc 30 ngày quá khứ | Ghi đè bằng `now()`, tăng `pulse_events_clock_skew_total{direction}` |
| `user_id`, `session_id` | ≤ 128 ký tự | Cắt bớt, đếm theo ký tự chứ không theo byte |
| `page`, `referrer` | ≤ 2048 ký tự; bỏ fragment và query param nhạy cảm | Làm sạch |
| `country` | Không phải mã ISO 3166-1 alpha-2 | Xoá trắng để GeoIP điền vào |
| `city` | ≤ 128 ký tự | Cắt bớt |
| `device` | Không thuộc `desktop`, `mobile`, `tablet`, `bot`, `unknown` | Chuẩn hoá về `unknown` |
| `os`, `browser`, `utm_*` | ≤ 64 ký tự | Cắt bớt — đây là cột `LowCardinality`, mỗi part giữ một từ điển riêng |
| `currency` | Không phải mã ISO 4217 | Về `VND`, giá trị mặc định của cột |

### Lỗi làm cả request bị loại

| Quy tắc | Mã trạng thái |
|---|---|
| Body có `site_id` khác với site mà API key cho phép | `401` |
| Không có event nào | `400` |
| Quá 500 event | `413` |
| Body quá 1 MiB | `413` |

`site_id` lấy từ API key, không bao giờ lấy từ body. Mọi truy vấn analytics đều lọc theo nó,
nên một body tự đặt được `site_id` là một lần ghi xuyên tenant.

**Lệch giờ được sửa chứ không bị loại.** Thiết bị sai giờ là chuyện thường, và loại event của
nó là âm thầm đánh mất traffic thật. Counter làm cho việc sửa đó nhìn thấy được. Một timestamp
không parse được lại là chuyện khác — đó là bug của client chứ không phải đồng hồ sai — nên nó
được báo ngược lại để sửa.

**Trường lạ vẫn được nhận.** Client chạy SDK mới hơn server là trạng thái bình thường trong lúc
rollout, và từ chối event của nó sẽ biến một payload tương thích tiến thành một sự cố.

**`screen` được nhận nhưng không được lưu.** Payload có ghi trường này còn `analytics.events`
thì chưa có cột tương ứng. Gửi nó trong `properties` cho tới khi có cột.

## Enrich phía server

1. **IP → country và city** qua MaxMind GeoLite2. Sau đó IP thô bị **bỏ đi** — không bao giờ
   ghi xuống storage. Chỉ lưu hash nếu có yêu cầu cụ thể.
2. **User-Agent → device, OS, browser, cờ bot.** Bỏ qua nếu client đã gửi.
3. **Ghép session.** `session_id` rỗng thì thành `hash(user_id + date + cửa sổ 30 phút)`.
4. **`ingested_at = now()`** để đo độ trễ đầu-cuối và phát hiện backlog.

## Riêng tư

Hai luật này thuộc về cấu trúc, không phải tuỳ chọn cấu hình:

- **Không bao giờ lưu IP thô.** Nó chỉ tồn tại trong bộ nhớ đủ lâu để tra GeoIP rồi bị bỏ.
- **Query param nhạy cảm bị strip khỏi `page` và `referrer` trước khi lưu.** Denylist mặc định
  gồm `token`, `email`, `password`, `secret`, `otp`, `api_key`, `apikey`, `access_token`,
  `refresh_token`, `id_token`, `authorization`, `passwd` và `pwd`. URL trong analytics là chỗ
  rò rỉ token reset mật khẩu phổ biến, và người dùng bấm một link *rời khỏi* trang đó sẽ gửi
  nguyên URL ấy làm referrer của event kế tiếp — nên cả hai trường được xử lý như nhau.

  Danh sách này cộng thêm được qua `ingest.sensitive_query_params`
  ([cấu hình](/vi/guide/configuration)) cho những tên mà ứng dụng này coi là bí mật còn ứng
  dụng khác thì không, ví dụ `code` hay `ref`. Không có cách nào gỡ một mục khỏi danh sách mặc
  định: tắt việc strip mật khẩu không được phép chỉ cách một lỗi gõ nhầm.

- **Fragment của URL luôn bị bỏ.** Trong một lần tải trang thật nó vốn không tới được server,
  nên lưu lại thứ client tự đặt vào đó chẳng được gì.

Traffic bot được phân loại lúc enrich và mặc định lọc khỏi số user, nếu không thì DAU sẽ phồng
lên một cách âm thầm.

## Khử trùng lặp

Mỗi event mang `event_id` (UUIDv7) do client sinh. Vì pipeline Kafka là at-least-once, một lần
retry có thể gửi cùng event hai lần; việc khử trùng lặp diễn ra ở tầng query dựa trên `event_id`
thay vì cố làm cho đường ghi trở thành exactly-once. Xem ADR-0005 ở
[trang quyết định](/vi/adr/).

Chọn UUIDv7 thay v4 vì nó có thứ tự theo thời gian, giúp insert vào column store đã sắp xếp rẻ
hơn và làm id hữu ích khi phá hoà trong cursor pagination.

## Snippet tracking

<Badge type="warning" text="Level 5" />

```html
<script defer src="https://cdn.example.com/pulse.js" data-site="site_abc"></script>
```

Dưới 2 KB gzip. Tự động page view kể cả SPA route change nhờ vá `history.pushState`,
`pulse('event_name', {props})` cho event tuỳ chỉnh, gom 10 event hoặc 3 giây, dùng
`navigator.sendBeacon` khi `visibilitychange`, tôn trọng `navigator.doNotTrack`, và hàng đợi
localStorage để retry khi offline.

## Đặc tả đầy đủ

Toàn bộ quy tắc, kể cả denylist chính xác và DDL mà payload ánh xạ vào, nằm ở
[`PLAN.md` §5](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/PLAN.md).

Pipeline được lắp ráp ra sao, và checklist khi thêm một rule vào đó:
[hướng dẫn pipeline validate](/vi/guide/validation). Vì sao quy tắc được viết tay thay vì
dùng struct tag: [ADR-0011](/vi/adr/0011-hand-written-event-validation).
