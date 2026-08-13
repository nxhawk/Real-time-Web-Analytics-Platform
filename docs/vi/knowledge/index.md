# Kiến thức nền

<Badge type="tip" text="Đọc thêm" />

Phần còn lại của site này mô tả **hệ thống này**: schema, API, các quyết định của nó. Mục này thì
khác — nó giải thích các công nghệ bên dưới từ nguyên lý đầu tiên, để những trang kia được phép
ngắn gọn.

Đọc một trang ở đây khi bạn muốn hiểu **vì sao** một công cụ hoạt động như vậy. Đọc
[Tra cứu](/vi/reference/api) khi bạn muốn biết dự án này **làm gì** với nó.

## Các trang

| Trang | Nội dung |
|---|---|
| [ClickHouse giải thích chi tiết](/vi/knowledge/clickhouse) | Column store, MergeTree, `ORDER BY` và sparse index, skip index, projection, materialized view, TTL, codec — kèm phần khi nào nên dùng ClickHouse, và so sánh với PostgreSQL cùng Elasticsearch |

Sẽ có thêm trang khi dự án đi tới các level cần đến chúng — ngữ nghĩa phân phối và consumer group
của Kafka ở Level 4, các mẫu concurrency trong Go cho đường ingest, và stack observability ở
Level 6.

## Các trang này được viết theo nguyên tắc nào

- **Khái niệm trước cấu hình.** Nếu bạn không giải thích được vì sao một setting tồn tại thì
  setting đó là cargo cult.
- **Mọi khẳng định đều kiểm chứng được.** Con số luôn đi kèm kích thước tập dữ liệu, và bất cứ
  thứ gì đo trong dự án này đều dẫn về [Ghi chép](/vi/notes/).
- **So sánh trung thực theo cả hai chiều.** Mỗi câu "X tốt hơn ở điểm này" đều đi kèm một câu
  "và Y tốt hơn ở điểm kia".
