# Quyết định kiến trúc

Mười một quyết định định hình hệ thống này. Mỗi cái được ghi thành một ADR để sáu tháng sau câu hỏi
là "đánh đổi này còn đúng không" chứ không phải "vì sao nó lại thành ra thế này".

Định dạng: **Bối cảnh** → **Quyết định** → **Hệ quả** → **Phương án đã cân nhắc**.

| # | Quyết định | Vì sao | Trạng thái |
|---|---|---|---|
| [0001](/vi/adr/0001-no-orm) | Không ORM, SQL viết tay | Mục tiêu là học ClickHouse; ORM che execution plan và không hỗ trợ combinator hay materialized view | Đã chốt |
| 0002 | `ORDER BY (site_id, event_name, event_time)` | Khớp workload dashboard; truy cập theo `user_id` xử lý bằng projection | Tạm — Level 2 sẽ đo ba phương án |
| 0003 | Batch phía client thay vì `async_insert` | Kiểm soát được retry, đo được, và cơ chế này chính là thứ cần học | Đã chốt |
| 0004 | Kafka giữa ingest và ClickHouse từ Level 4 | Decoupling, replay, fan-out cho consumer tương lai | Đã chốt |
| 0005 | At-least-once cộng dedup ở tầng query | Đơn giản hơn exactly-once và đủ chính xác cho analytics | Đã chốt |
| 0006 | AggregatingMergeTree thay vì query thẳng raw | Yêu cầu 300 ms không thể đạt được bằng cách quét raw ở 100M event | Đã chốt |
| 0007 | Monorepo | Contract API và client của nó không lệch nhau; một lần CI phủ cả hai | Đã chốt |
| 0008 | Docker Compose thay vì Kubernetes | Một máy, một người. Kubernetes là chi phí vận hành không đem lại gì ở đây | Đã chốt |
| 0009 | ClickHouse một node, chưa replication | Bớt phức tạp lúc này; đường nâng cấp qua Keeper và ReplicatedMergeTree đã ghi lại | Đã chốt |
| 0010 | Trả `202 Accepted` cho ingest | Ingest không được phụ thuộc vào độ sẵn sàng của storage | Đã chốt |
| [0011](/vi/adr/0011-hand-written-event-validation) | Validate tự viết, không dùng `validator/v10` | Một nửa quy tắc ở PLAN §5.2 sửa giá trị chứ không loại event — struct tag không diễn đạt được | Đã chốt |

## Tạm nghĩa là tạm

ADR-0002 được đánh dấu tạm một cách có chủ ý. Thứ tự sắp xếp là một giả thuyết dựa trên hình
dạng các query; Level 2 dựng ba bảng với ba thứ tự trên cùng dữ liệu rồi đo thời gian,
`read_rows` và dung lượng đĩa trên tám query. Cái nào thắng thì thành migration, và ADR này được
viết lại kèm số liệu.

Một quyết định được ghi kèm phép đo tạo ra nó có giá trị bằng mười quyết định ghi kèm lý lẽ suông.

## Viết một cái mới

Mỗi quyết định một file trong `docs/adr/`, đặt tên `NNNN-short-slug.md`, cộng bản tiếng Việt
trong `docs/vi/adr/`. Thêm cả hai vào sidebar ở `.vitepress/config/en.mts` và `vi.mts`, và vào
bảng phía trên.

Viết ngắn. Một ADR đọc mất hai mươi phút sẽ không được đọc. Giá trị nằm ở chỗ ghi lại **cái gì
đã bị loại** và vì sao — đó là phần không ai nhớ được.
