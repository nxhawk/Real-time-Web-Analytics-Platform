# Kết quả benchmark

<Badge type="warning" text="Sinh ra ở Level 3" />

::: info Chưa đo
Các bảng dưới đây là hình dạng mà kết quả sẽ có. Level 3 sẽ điền vào.
:::

## So sánh cái gì

PostgreSQL và ClickHouse trên cùng dữ liệu, cùng máy, cùng ngân sách bộ nhớ. Mục đích không
phải chứng minh ClickHouse thắng — nó sẽ thắng, trên những query này — mà là định lượng **thắng
bao nhiêu** và gọi tên những trường hợp nó thua.

## Luật công bằng

Bỏ qua bất kỳ điều nào dưới đây thì con số trở thành quảng cáo chứ không phải phép đo:

- Cùng máy, cùng dataset, cùng lượng bộ nhớ cấp phát.
- PostgreSQL được cho một schema tử tế: bảng tương đương, index trên
  `(event_name, event_time)`, index BRIN trên `event_time`, và một biến thể partition theo tháng.
- Chạy `VACUUM ANALYZE` trước khi đo.
- Ba lần chạy, báo cáo trung vị.
- Đo cold và warm riêng. Cold nghĩa là xoá page cache của OS **và** cache của ClickHouse.
- Ghi lại dung lượng đĩa — thường là con số ấn tượng nhất của cả bộ.

## Các mức dữ liệu

| Mức | Số event | Mục đích |
|---|---|---|
| S | 1.000.000 | Smoke test, chạy được trên laptop |
| M | 10.000.000 | Khác biệt bắt đầu hiện rõ |
| L | 100.000.000 | Quy mô mục tiêu, nơi kiểm chứng yêu cầu 300 ms |
| XL | 500.000.000 | Tuỳ chọn, cần đĩa lớn |

Sinh bởi `cmd/seeder` với phân phối thực tế: Zipf trên ~500 page, device chia 62/35/3, top-10
country cộng đuôi dài, giờ cao điểm 20:00–22:00, cuối tuần thấp hơn 30%. Tỉ lệ chuyển đổi và
retention được cài sẵn để query funnel và retention kiểm chứng được với đáp án đã biết.

## So sánh query

| Query | PG cold/warm | PG partition | CH cold/warm | CH + MV | Tỉ lệ |
|---|---|---|---|---|---|
| `COUNT(*)` toàn bảng | | | | | |
| `COUNT` 7 ngày gần nhất | | | | | |
| `GROUP BY country` | | | | | |
| `GROUP BY toStartOfHour`, 30 ngày | | | | | |
| `uniq(user_id)`, 30 ngày | | | | | |
| Top 10 pages | | | | | |
| Funnel 5 bước | | | | | |
| Retention 30 ngày | | | | | |
| **Insert 10M rows** | | | | | |
| **Dung lượng đĩa** | | | | | |

## Chiến lược insert

Sáu kịch bản ở phía ClickHouse, vì đường ghi là chỗ mà một hiện thực ngây thơ gục trước tiên:

| Chiến lược | Throughput (ev/s) | p99 API | Part/phút | CPU / RAM | Có lỗi 252? |
|---|---|---|---|---|---|
| Một row mỗi INSERT | | | | | |
| Batch 100 | | | | | |
| Batch 1.000 | | | | | |
| Batch 10.000 | | | | | |
| `async_insert` | | | | | |
| Kafka + consumer batch 10k | | | | | |

Dòng đầu tiên là lý do Level 1 cố ý hiện thực `InsertOne` sau cờ `INSERT_MODE=single`. Không có
mốc so sánh thì "batch làm nó nhanh hơn" chỉ là tuyên bố chứ không phải kết quả.

## Kết luận cần rút ra

Ba câu hỏi mà phần viết phải trả lời, bằng văn xuôi, dựa trên số liệu ở trên:

**Vì sao column store nhanh ở đây.** Chỉ đọc cột có trong query; dữ liệu cột đã sắp xếp nén tốt
hơn hàng chục lần; thực thi được vector hoá; sparse index trên dữ liệu đã sắp xếp bỏ qua nguyên
cả granule thay vì đi bộ trên B-tree.

**ClickHouse yếu ở đâu.** Point lookup theo primary key, UPDATE và DELETE, JOIN lớn, transaction.
Nói thẳng ra — một benchmark chỉ khoe người thắng thì không đáng tin.

**Khi nào chọn cái nào.** Một cái bảng, không phải một cảm giác.

## Ghi lại môi trường

Mọi kết quả đều vô nghĩa nếu thiếu thông tin về cái máy đã chạy nó. Ghi lại CPU, RAM, loại đĩa,
và có dịch vụ nào khác đang tranh tài nguyên không.

::: warning Số liệu ở production là bi quan
Trên deployment AWS mọi thứ dùng chung một `r7g.xlarge`: JVM của Kafka và các service Go tranh
CPU với page cache của ClickHouse. Số đo ở đó tệ hơn so với một máy chuyên dụng. Hãy ghi rõ điều
này cạnh bảng — người đọc mặc định nghĩ tới máy riêng sẽ rút ra kết luận sai.
:::
