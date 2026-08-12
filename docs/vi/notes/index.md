# Ghi chép kỹ thuật

Những tài liệu được sinh ra **bởi** công việc chứ không phải **trước** nó. Mỗi cái là output của
một level cụ thể và còn trống cho tới khi level đó chạy.

Luật chung cho cả ba: **một con số không có phép đo đứng sau thì không được vào đây.** Chép một
tuyên bố từ tài liệu chính thức không phải là ghi chép; đo nó trên dữ liệu của mình mới là.

| Ghi chép | Sinh ra bởi | Nội dung |
|---|---|---|
| [Ghi chép ClickHouse](/vi/notes/clickhouse-notes) | Level 2 | Tối thiểu 20 thí nghiệm — quan sát, số liệu, giải thích |
| [Kết quả benchmark](/vi/notes/benchmark-results) | Level 3 | ClickHouse vs PostgreSQL trên bốn mức dữ liệu |
| [Runbook](/vi/notes/runbook) | Level 6 | Làm gì lúc 3 giờ sáng khi có sự cố |

## Vì sao chúng là deliverable

Level 2 gần như không sinh ra code. Toàn bộ output của nó là `clickhouse-notes.md` — điều đó
biến ghi chép thành deliverable chứ không phải sản phẩm phụ. Một level mà số đo không được ghi
lại thì không thể đánh dấu hoàn thành, vì sáu tuần sau không ai còn nhớ projection có đáng hay
không.

Runbook cũng vậy. Nó được viết trong lúc xây, không phải trong lúc sự cố.
