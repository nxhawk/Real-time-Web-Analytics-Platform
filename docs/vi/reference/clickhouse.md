# Schema ClickHouse

<Badge type="warning" text="Từ Level 1 trở đi" />

DDL chính thức nằm ở
[`PLAN.md` §6–§7](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/PLAN.md)
và các file migration trong `backend/migrations/`. Trang này giải thích **vì sao** schema lại
như vậy.

## Các bảng

| Bảng | Engine | Mục đích | Level |
|---|---|---|---|
| `events` | MergeTree | Event thô, nguồn sự thật | L1 |
| `events_hourly` | AggregatingMergeTree | Overview và time series | L5 |
| `events_daily` | AggregatingMergeTree | DAU / WAU / MAU | L5 |
| `page_stats_hourly` | AggregatingMergeTree | Top pages — tách riêng vì `page` cardinality cao | L5 |
| `sessions` | AggregatingMergeTree | Bounce rate, thời lượng session, entry/exit page | L5 |
| `user_first_seen` | ReplacingMergeTree | Gán cohort cho retention | L5 |

## Thứ tự sắp xếp

`ORDER BY` của một bảng MergeTree là lựa chọn có hệ quả lớn nhất trong schema: nó quyết định
sparse primary index, tỉ lệ nén, và query nào bỏ qua được granule.

Đề xuất khởi điểm là `(site_id, event_name, event_time)` — khớp với workload dashboard, vốn
luôn lọc theo site, thường theo tên event, và luôn theo khoảng thời gian.

Đó là **giả thuyết**, không phải kết luận. Level 2 dựng ba bảng trên cùng dữ liệu với ba thứ tự
khác nhau, chạy tám query benchmark trên mỗi bảng, so sánh thời gian, `read_rows` và dung lượng
đĩa. Cái nào thắng thì thành migration và cập nhật ADR-0002. Query theo `user_id`, thứ mà không
thứ tự nào ở đây phục vụ tốt, sẽ dùng projection — nếu số đo cho thấy projection đáng giá.

## Kiểu dữ liệu và codec

Ba luật, mỗi luật có một phép đo đứng sau ở Level 2:

- **`LowCardinality(String)`** cho `country`, `device`, `browser`, `event_name`. Một từ điển cho
  mỗi part thay vì lặp lại chuỗi: nhỏ hơn trên đĩa và group by nhanh hơn.
- **Không bao giờ `Nullable`.** Cột nullable lưu thêm một bitmap và làm hỏng vài tối ưu hoá.
  Giá trị mặc định mang cùng ý nghĩa với chi phí thấp hơn.
- **Codec khớp với dữ liệu.** `Delta` hoặc `DoubleDelta` trước `ZSTD` cho timestamp tăng dần,
  `ZSTD` thuần cho văn bản. `ZSTD(1)` cho dữ liệu nóng, `ZSTD(9)` sau TTL recompress.

## Materialized View

Materialized View trong ClickHouse là một **trigger insert**, không phải query được cache: khi
một row rơi vào `events`, câu `SELECT` của view chạy trên đúng block đó và kết quả được ghi vào
bảng đích. Đó là lý do chúng rẻ, và cũng là lý do có hai cái bẫy:

**Lưu aggregate state, không phải aggregate value.** Bảng đích lưu giá trị `-State`, thứ merge
đúng khi các part merge, và query đọc lại bằng `-Merge`. Đặt một cột non-aggregate vào bảng đích
kiểu AggregatingMergeTree sẽ cho ra số liệu đúng lúc đầu rồi âm thầm sai sau khi merge. Golden
test ở Level 5 (task L5-03) tồn tại chính là để bắt lỗi này: insert 50k event cố định rồi so
từng metric giữa raw và view — phải khớp tuyệt đối.

**View chỉ thấy insert xảy ra sau khi nó tồn tại.** Dữ liệu lịch sử cần một lệnh
`INSERT ... SELECT` backfill tường minh, chạy theo từng tháng để không hết bộ nhớ, và chú ý biên
thời gian để không đếm hai lần.

**Cardinality sẽ nổ nếu bạn để nó nổ.** Chỉ cột cardinality thấp mới được vào `GROUP BY` của
view. `page` có quá nhiều giá trị khác nhau, nên `page_stats_hourly` là bảng riêng. Nếu tổng các
view vượt 15% bảng raw thì `GROUP BY` đang sai.

## Vòng đời dữ liệu

| Tuổi | Điều gì xảy ra |
|---|---|
| 0–30 ngày | Nóng, `ZSTD(1)` |
| 30 ngày | `TTL RECOMPRESS` sang `ZSTD(9)` — đọc chậm hơn, nhỏ hơn nhiều |
| 180 ngày | `TTL DELETE` |

Có thể thêm storage policy phân tầng để đẩy part nguội xuống đĩa chậm.

## Guard cho query

Mọi query analytics chạy dưới `max_execution_time = 15` và `max_memory_usage = 4GB`, cộng với
giới hạn 400 ngày ở tầng API. Chúng được đặt trên **profile user** của ClickHouse trong
`deploy/clickhouse/users.d/pulse.xml`, không phải trong mệnh đề `SETTINGS` từng query — mệnh đề
thì dễ quên, còn profile áp cả với query gõ tay.

Có sẵn user `dashboard` với `readonly = 2` để từ Level 6, analytics API không thể sửa dữ liệu
ngay cả khi một handler có bug.

## Những cái bẫy nên biết trước khi dẫm phải

| Bẫy | Triệu chứng | Cách tránh |
|---|---|---|
| `Too many parts` | Insert bắt đầu lỗi code 252 | Batch ít nhất 10k row, tối đa một insert mỗi giây mỗi bảng. Tăng `parts_to_throw_insert` chỉ là băng dán |
| `FINAL` trong query nóng | Chậm khủng khiếp | Đừng đặt ReplacingMergeTree ở hot path |
| `SELECT *` trên column store | Chậm gấp chục lần cần thiết | Luôn liệt kê cột |
| Lẫn lộn timezone | Số liệu lệch bảy tiếng | Lưu UTC tuyệt đối, convert bằng `toTimeZone` lúc query, hiển thị theo timezone của site |
| Khoảng thời gian không giới hạn | Một request quét cả năm rồi OOM | Ép giới hạn range cộng với guard ở profile |
| Đầy đĩa vì raw cộng projection | ClickHouse dừng nhận ghi | TTL, giám sát, cảnh báo ở 75% |

## Học bằng cách đo

Level 2 cố ý ít code và nhiều thí nghiệm. Mọi thứ học được ghi vào
[ghi chép ClickHouse](/vi/notes/clickhouse-notes) theo dạng *quan sát → số liệu → giải thích*,
tối thiểu hai mươi mục, không mục nào chép từ tài liệu chính thức.

Tiêu chí ra là trả lời được, bằng số của chính mình: vì sao thứ tự sắp xếp này,
`LowCardinality` tiết kiệm bao nhiêu, và projection có đáng không.
