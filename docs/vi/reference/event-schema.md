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

| Trường | Quy tắc | Hành vi khi vi phạm |
|---|---|---|
| `site_id` | Bắt buộc, phải khớp API key | `401` cho cả request |
| `event` | Bắt buộc, `^[a-z0-9_]{1,64}$` | Chỉ loại event đó |
| `timestamp` | ISO 8601; lệch quá 24h về tương lai hoặc 30 ngày về quá khứ | Ghi đè bằng `now()`, tăng counter `events_clock_skew_total` |
| `user_id` | ≤ 128 ký tự | Cắt bớt |
| `page` | ≤ 2048 ký tự; strip query param nhạy cảm | Làm sạch |
| `properties` | Object JSON, ≤ 8 KB sau serialize | Loại event đó |
| Kích thước batch | ≤ 500 event, body ≤ 1 MiB | `413` cho cả request |

**Lệch giờ được sửa chứ không bị loại.** Thiết bị sai giờ là chuyện thường, và loại event của
nó là âm thầm đánh mất traffic thật. Counter làm cho việc sửa đó nhìn thấy được.

## Enrich phía server

1. **IP → country và city** qua MaxMind GeoLite2. Sau đó IP thô bị **bỏ đi** — không bao giờ
   ghi xuống storage. Chỉ lưu hash nếu có yêu cầu cụ thể.
2. **User-Agent → device, OS, browser, cờ bot.** Bỏ qua nếu client đã gửi.
3. **Ghép session.** `session_id` rỗng thì thành `hash(user_id + date + cửa sổ 30 phút)`.
4. **`ingested_at = now()`** để đo độ trễ đầu-cuối và phát hiện backlog.

## Riêng tư

Hai luật này thuộc về cấu trúc, không phải tuỳ chọn cấu hình:

- **Không bao giờ lưu IP thô.** Nó chỉ tồn tại trong bộ nhớ đủ lâu để tra GeoIP rồi bị bỏ.
- **Query param nhạy cảm bị strip khỏi `page` trước khi lưu** — `token`, `email`, `password` và
  mọi thứ khác trong denylist. URL trong analytics là chỗ rò rỉ token reset mật khẩu phổ biến.

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
