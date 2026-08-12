# Giới thiệu

Pulse Analytics là nền tảng web analytics tự host — một Google Analytics thu nhỏ. Nó nhận
event từ website và app qua HTTP, lưu vào ClickHouse, và phục vụ dashboard real-time: overview
metrics, time series, top pages, breakdown theo device và country, funnel, retention cohort.

## Hai mục tiêu, theo đúng thứ tự này

**1. Học.** Mục đích chính là đào sâu những thứ khó học qua tutorial: nội tại storage của
ClickHouse, đường ghi throughput cao trong Go, ngữ nghĩa consumer của Kafka, và phần vận hành
— metrics, alert, deploy, backup đã thực sự restore thử một lần.

**2. Sản phẩm.** Một nền tảng analytics chạy được, trả lời mọi dashboard query dưới 300 ms ở
100 triệu event, và vẫn nhận traffic khi ClickHouse không sẵn sàng.

Thứ tự này quan trọng. Khi một lối tắt làm sản phẩm tốt hơn nhưng không dạy được gì, dự án
chọn đường dài — Level 1 **cố ý** insert từng row một để Level 3 có mốc so sánh.

## Nguyên tắc kiến trúc

Năm điều này đã chốt, không thương lượng lại theo từng tính năng:

1. **Đường ingest không được phụ thuộc vào ClickHouse.** Storage chết vẫn phải trả
   `202 Accepted` — nhờ Kafka từ Level 4, nhờ buffer trên đĩa trước đó.
2. **Ghi nhiều, đọc ít, nhưng đọc phải nhanh.** Mọi dashboard query dưới 300 ms ở 100M event.
   Đây là yêu cầu cứng, không phải mong muốn.
3. **Không dùng ORM.** SQL viết tay để `EXPLAIN` và `EXPLAIN PIPELINE` còn ý nghĩa. Xem
   [ADR-0001](/vi/adr/0001-no-orm).
4. **Tách binary từ Level 4.** `ingest-api` nặng I/O, `analytics-api` nặng CPU; chúng scale
   độc lập.
5. **Idempotency.** Mỗi event mang `event_id` (UUIDv7) do client sinh, nên retry có thể dedup
   ở tầng query.

## Trong phạm vi

- Nhận event qua HTTP, đơn lẻ và theo batch
- Schema ClickHouse, migration và materialized view
- Analytics API: overview, timeseries, pages, devices, countries, funnel, retention, realtime
- Dashboard Next.js
- Kafka pipeline
- Bộ sinh dữ liệu và benchmark ClickHouse vs PostgreSQL
- CI/CD lên một máy chủ production

## Ngoài phạm vi

- Multi-region và replication ClickHouse — ghi chú đường nâng cấp, không làm
- Quản lý người dùng phức tạp hay billing; xác thực là API key theo từng site
- Session replay và heatmap
- SDK mobile native

## Tài liệu được tổ chức thế nào

| Ở đâu | Chứa gì |
|---|---|
| Trang này | Tài liệu diễn giải: hướng dẫn, tra cứu, ghi chép kỹ thuật, quyết định |
| [`PLAN.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/PLAN.md) | Đặc tả kỹ thuật — kiến trúc, DDL, query cookbook, contract |
| [`PHASES.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/PHASES.md) | Giai đoạn triển khai: entry/exit criteria, deliverable, rủi ro, bảng số liệu chuẩn |
| [`TODO.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/TODO.md) | Checklist 232 task trên bảy level |
| [`DEPLOY-AWS.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/DEPLOY-AWS.md) | Hạ tầng production: Vercel + một EC2, dựng bằng Terraform |

Bốn file kế hoạch trên viết bằng tiếng Việt theo chủ ý; toàn bộ code, các trang tiếng Anh của
site này, và mọi thứ còn lại trong repo đều bằng tiếng Anh.

Khi hai tài liệu mâu thuẫn, thứ tự ưu tiên là `DEPLOY-AWS.md` cho phần deploy, rồi `PLAN.md`,
rồi `PHASES.md`, rồi `TODO.md`, cuối cùng mới đến code.

## Tiếp theo

- [Chạy thử nhanh](/vi/guide/quick-start) — dựng lên trong khoảng năm phút
- [Kiến trúc](/vi/guide/architecture) — các mảnh ghép với nhau ra sao, ở cả hai phase
- [Cấu trúc dự án](/vi/guide/project-structure) — code đặt ở đâu và vì sao
