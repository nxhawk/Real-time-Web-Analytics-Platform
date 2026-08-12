# Lộ trình

Bảy level, 232 task, khoảng 207 giờ làm part-time. Mỗi level kết thúc bằng một thứ demo được —
không phải "code đã viết xong" mà là "đây, nó chạy".

| Level | Phạm vi | Task | Ước lượng | Tag | Trạng thái |
|---|---|---|---|---|---|
| **L0** | Khởi tạo: quy ước, skeleton, Docker, Makefile, CI | 25 | 12h | — | ✅ 22/25 |
| **L1** | MVP: ingest → query → dashboard | 40 | 30h | `v0.1.0` | ⬜ |
| **L2** | Đào sâu ClickHouse: codec, skip index, TTL, projection | 24 | 25h | — | ⬜ |
| **L3** | Batch insert, seeder, benchmark ClickHouse vs PostgreSQL | 32 | 35h | `v0.3.0` | ⬜ |
| **L4** | Kafka pipeline: producer, consumer group, DLQ, tách binary | 30 | 35h | `v0.4.0` | ⬜ |
| **L5** | Materialized View, funnel, retention, realtime, dashboard đầy đủ | 46 | 45h | — | ⬜ |
| **L6** | Observability, security, CD, tài liệu | 35 | 25h | `v1.0.0` | ⬜ |
| | **Tổng** | **232** | **~207h** | | |
| **AWS** | Hạ tầng production — thay cho L6.4 | 32 | 14h | — | ⬜ |
| | **Khi đi đường AWS** | **255** | **~214h** | | |

L0–L3 dựng monolith Phase 1; L4–L6 chuyển sang event pipeline Phase 2. Xem
[kiến trúc](/vi/guide/architecture).

## Mỗi level chứng minh điều gì

**L0 — nền móng.** `make up` chạy được trên máy sạch, CI xanh, hai binary phục vụ các endpoint
vận hành. Chưa có hành vi sản phẩm nào, và điều đó là chủ ý.

**L1 — đường đi trọn vẹn đầu tiên.** `curl` một event, thấy số đổi trên trang web. Insert cố ý
làm ngây thơ, một row mỗi request, để Level 3 có mốc mà vượt.

**L2 — hiểu, không phải tính năng.** Gần như không code. Ba bảng với ba thứ tự sắp xếp, so sánh
codec, skip index, projection, TTL. Deliverable là
[hai mươi ghi chép có số đo](/vi/notes/clickhouse-notes), và tiêu chí ra là trả lời được "vì sao
thứ tự sắp xếp này" bằng số của chính mình.

**L3 — đường ghi trưởng thành.** Batch writer có backpressure và write-ahead log, một seeder
sinh dữ liệu thực tế ở quy mô 100M, và [benchmark](/vi/notes/benchmark-results) mà cả dự án
hướng tới. Test cửa ra: kill ClickHouse giữa lúc bắn tải mà không mất gì.

**L4 — event-driven.** Kafka giữa ingest và storage, consumer chỉ commit offset sau khi
ClickHouse xác nhận, dead-letter queue, và ba bài chaos test. Demo: kill ClickHouse lúc đang tải,
bật lại, xem số khớp tuyệt đối.

**L5 — sản phẩm.** Năm materialized view, funnel, retention, realtime, revenue, dashboard đầy đủ
và tracking SDK. Golden test so view với raw là bài test quan trọng nhất dự án.

**L6 — vận hành được.** Metrics, bốn dashboard, bốn alert, security hardening, deploy tự động có
rollback, và một bản backup đã thực sự restore một lần.

## Thứ tự phụ thuộc

```
L0 ──▶ L1 ──┬──▶ L2 ──┐
            │         ├──▶ L3 ──▶ L4 ──▶ L5 ──▶ L6 ──▶ AWS
            └─────────┘
```

Một ngoại lệ có ghi rõ: **L2 cần ít nhất 10M event để thí nghiệm.** Làm seeder (task L3-01 đến
L3-07) trước hoặc song song với L2 là cách hợp lệ để có chúng.

## Nếu thiếu thời gian

Thứ tự ưu tiên, giá trị nhất trước:

1. **L0 → L1 → L2 → L3.** Nội tại storage, đường ghi, và benchmark. Bỏ chúng là bỏ luôn lý do dự
   án tồn tại.
2. **L5.1 và L5.2** — materialized view và funnel. Đây là chỗ chứng minh hiểu AggregatingMergeTree
   và analytical SQL.
3. **L4** — Kafka. Hoãn được nếu mục tiêu không phải kiến trúc event-driven, nhưng mục tiêu
   **chính là** kiến trúc event-driven.
4. **L6.1 và L6.5** — metrics và tài liệu. Rẻ, mà nâng giá trị dự án lên thấy rõ.
5. Còn lại chờ được.

Rút gọn chấp nhận được: benchmark ở 10M thay vì 100M nếu thiếu đĩa (ghi mức dữ liệu cạnh mọi con
số), dùng Redpanda thay Kafka ở môi trường dev, bỏ OpenTelemetry, bỏ storage phân tầng.

**Không rút gọn được:** golden test so materialized view với raw, các bài test "kill ClickHouse
không mất gì", và buổi diễn tập restore backup. Ba thứ đó là chỗ phân biệt một bản demo với một
hệ thống.

## Định nghĩa hoàn thành

Dự án xong khi cả mười ba điều dưới đây đúng:

1. `git clone && make up` chạy được dưới năm phút trên máy sạch
2. `make seed N=10000000` thành công dưới năm phút
3. Dashboard hiển thị bảy nhóm widget với dữ liệu thật
4. Mọi endpoint analytics p95 dưới 300 ms ở 100M event, có `system.query_log` làm bằng chứng
5. Ingest chịu 10.000 event/s trong mười phút, drop = 0, p99 dưới 50 ms
6. Kill ClickHouse năm phút không mất gì; số liệu khớp tuyệt đối sau đó
7. Golden test so materialized view với raw khớp tuyệt đối
8. CI xanh: lint, unit, integration, security scan, build image
9. Deploy bằng một tag, có rollback và smoke test
10. Grafana có bốn dashboard và alert hoạt động
11. `benchmark-results.md` đầy đủ kèm kết luận
12. `clickhouse-notes.md` có ít nhất hai mươi mục đo được
13. README có kiến trúc, quickstart, ảnh chụp, và phần đã học được gì

Chi tiết từng task nằm ở
[`TODO.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/TODO.md);
điều kiện vào/ra từng level ở
[`PHASES.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/PHASES.md).
