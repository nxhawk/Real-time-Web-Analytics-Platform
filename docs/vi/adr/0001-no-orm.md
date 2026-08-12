# ADR-0001 — Không dùng ORM, SQL viết tay

**Trạng thái:** Đã chốt · **Ngày:** 2026-08-11

## Bối cảnh

Backend viết bằng Go, và hệ sinh thái Go có nhiều cách nói chuyện với cơ sở dữ liệu: ORM như
GORM hay Ent, query builder như squirrel, bộ sinh code như sqlc, hoặc dùng thẳng driver với SQL
viết tay.

Mục tiêu chính của dự án là học ClickHouse thật sâu — nội tại MergeTree, aggregate combinator,
materialized view, skip index, projection. Mục tiêu phụ là một nền tảng mà mọi dashboard query
dưới 300 ms ở 100 triệu event.

ClickHouse cũng không phải loại cơ sở dữ liệu mà phần lớn ORM được xây cho. Phương ngữ của nó có
combinator `-State` và `-Merge`, `windowFunnel`, `retention`, `arrayJoin`, `PROJECTION`, và mệnh
đề `SETTINGS` cho từng query. Không thứ nào ánh xạ được vào mô hình hàng-và-quan-hệ của ORM.

## Quyết định

**Không dùng ORM.** Query viết tay bằng SQL, lưu thành file `.sql` dưới
`internal/repository/clickhouse/queries/`, và nhúng vào binary bằng `go:embed`. Dùng driver
chính thức `ClickHouse/clickhouse-go/v2` qua native protocol cổng 9000.

Mọi tham số bind bằng named parameter của ClickHouse (`{name:Type}`). Những định danh không
tham số hoá được — ví dụ tên cột do một query param chọn — phải đi qua whitelist tường minh.

## Hệ quả

**Tốt**

- `EXPLAIN` và `EXPLAIN PIPELINE` chạy trên đúng câu query đang chạy ở production. Đây chính là
  mấu chốt: một tối ưu hoá mà bạn không quan sát được là một tối ưu hoá bạn không học được gì từ nó.
- Combinator, `windowFunnel`, projection và setting theo từng query đều dùng được mà không phải
  vật lộn với một lớp trừu tượng.
- Một file `.sql` dán thẳng vào `clickhouse-client` được để profile.
- Chi phí của query nhìn thấy được ngay lúc review, trong diff.

**Xấu**

- Nhiều boilerplate hơn: scan row vào struct phải viết tay.
- Không có kiểm tra lúc biên dịch rằng query khớp với struct. Bù lại bằng integration test chạy
  trên ClickHouse thật qua testcontainers.
- Đổi tên một cột nghĩa là grep thư mục `queries/` chứ không phải rename một field.

**Trung tính**

- Mất tính khả chuyển sang cơ sở dữ liệu khác. Ở đây điều đó không phải chi phí: các query vốn
  đặc thù ClickHouse theo thiết kế, còn code PostgreSQL trong `repository/postgres/` chỉ tồn tại
  để so sánh benchmark ở Level 3.

## Phương án đã cân nhắc

**GORM.** ORM phổ biến nhất của Go, nhưng hỗ trợ ClickHouse là driver cộng đồng, migration không
hợp với DDL của ClickHouse, và nó chủ động che execution plan. Bị loại vì mục tiêu chính: nó sẽ
ngăn đúng thứ mà dự án tồn tại để học.

**sqlc.** Sinh code Go type-safe từ SQL, thật sự hấp dẫn — SQL vẫn nhìn thấy được và boilerplate
scan biến mất. Bị loại vì hỗ trợ ClickHouse chưa chín, đặc biệt với các kiểu aggregate state, và
bước sinh code sẽ làm mờ mối liên hệ giữa một file `.sql` và đoạn code thực sự chạy nó. Đáng xem
lại khi hỗ trợ đã tốt hơn.

**squirrel hoặc query builder khác.** Hữu ích khi query được lắp ráp động. Ở đây query biết
trước và phần lớn là tĩnh; builder sẽ thêm một lớp gián tiếp mà không loại bỏ nhu cầu hiểu SQL.
Phần động — một bộ lọc tuỳ chọn, một chiều được chọn — xử lý bằng whitelist và một chút ghép
chuỗi, có review kỹ.

## Liên quan

- [Schema ClickHouse](/vi/reference/clickhouse) — thứ mà các query chạy lên
- ADR-0006 — materialized view thay vì query thẳng dữ liệu thô
- [`PLAN.md` §8](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/PLAN.md) —
  query cookbook
