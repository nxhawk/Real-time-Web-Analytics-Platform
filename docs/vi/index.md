---
layout: home

hero:
  name: Pulse Analytics
  text: Web analytics real-time bạn tự host
  tagline: >-
    Go, ClickHouse, Kafka và Next.js. Dashboard trả lời dưới 300 ms ở 100 triệu event, và
    đường ingest vẫn trả 202 ngay cả khi storage chết.
  image:
    src: /logo.svg
    alt: Pulse Analytics
  actions:
    - theme: brand
      text: Bắt đầu
      link: /vi/guide/quick-start
    - theme: alt
      text: Vì sao có dự án này
      link: /vi/guide/introduction
    - theme: alt
      text: Xem trên GitHub
      link: https://github.com/nxhawk/Real-time-Web-Analytics-Platform

features:
  - icon: 🚀
    title: Thiết kế cho đường ghi
    details: >-
      Batch insert qua native protocol, backpressure có ưu tiên, và write-ahead log trên đĩa.
      Kill ClickHouse giữa lúc bắn tải mà không mất một event nào.
    link: /vi/reference/clickhouse
    linkText: Thiết kế storage
  - icon: ⚡
    title: Đọc nhanh ở 100 triệu event
    details: >-
      Materialized View kiểu AggregatingMergeTree, skip index và projection — chọn bằng số đo
      của chính mình, không phải theo bài blog. Mọi ngưỡng đều là yêu cầu cứng.
    link: /vi/notes/benchmark-results
    linkText: Kết quả benchmark
  - icon: 🔍
    title: Học công khai
    details: >-
      Mỗi quyết định thiết kế là một ADR, mỗi thí nghiệm ClickHouse được ghi lại kèm số liệu
      thật, và mỗi level kết thúc bằng một demo tái lập được.
    link: /vi/adr/
    linkText: Quyết định kiến trúc
  - icon: 🔒
    title: Riêng tư từ thiết kế
    details: >-
      Không bao giờ lưu IP thô — dùng để tra GeoIP rồi bỏ. Query param nhạy cảm bị strip khỏi
      URL trước khi ghi xuống.
    link: /vi/reference/event-schema
    linkText: Event schema
---

## Trạng thái dự án

**Level 0 đã xong.** Bộ khung backend chạy được: hai binary, cấu hình, structured logging,
middleware, các endpoint vận hành, Docker Compose, và CI lint + test + build image. Phần nhận
event, schema ClickHouse và dashboard nằm ở Level 1.

| Level | Phạm vi | Trạng thái |
|---|---|---|
| **L0** | Khởi tạo: quy ước, skeleton, Docker, Makefile, CI | ✅ 22/25 |
| **L1** | MVP: ingest → query → dashboard | ⬜ |
| **L2** | Đào sâu ClickHouse: codec, skip index, TTL, projection | ⬜ |
| **L3** | Batch insert, seeder, benchmark ClickHouse vs PostgreSQL | ⬜ |
| **L4** | Kafka pipeline: producer, consumer group, DLQ, tách binary | ⬜ |
| **L5** | Materialized View, funnel, retention, dashboard đầy đủ | ⬜ |
| **L6** | Observability, security, CD, tài liệu | ⬜ |

Lộ trình đầy đủ kèm ước lượng ở [trang lộ trình](/vi/roadmap).

## Hôm nay chạy được gì

```bash
git clone https://github.com/nxhawk/Real-time-Web-Analytics-Platform.git
cd Real-time-Web-Analytics-Platform
cp .env.example .env
make deps && make up

curl localhost:8080/healthz   # {"status":"ok"}
```

Hướng dẫn đầy đủ ở trang [chạy thử nhanh](/vi/guide/quick-start).
