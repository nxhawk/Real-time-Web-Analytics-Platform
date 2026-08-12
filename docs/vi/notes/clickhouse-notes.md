# Ghi chép ClickHouse

<Badge type="warning" text="Sinh ra ở Level 2" />

::: info Chưa viết
Level 2 chưa chạy. Trang này mô tả định dạng và những câu hỏi mà ghi chép phải trả lời; các mục
sẽ được thêm vào khi thí nghiệm được thực hiện.
:::

## Định dạng

Mỗi phát hiện một mục, luôn đủ ba phần:

> **Quan sát** — nhìn thấy gì.
> **Số liệu** — con số đo được, kèm mức dữ liệu bên cạnh.
> **Giải thích** — vì sao ClickHouse hành xử như vậy.

Một mục không có số là một ý kiến. Một mục không có giải thích là một mẩu chuyện vặt.

### Ví dụ về hình dạng

> **Quan sát.** Đổi `country` từ `String` sang `LowCardinality(String)` làm cột nhỏ đi và
> `GROUP BY country` nhanh hơn.
>
> **Số liệu.** Ở 10M event: dung lượng cột 41 MB → 0,7 MB. `GROUP BY country` trên 30 ngày:
> 180 ms → 62 ms, `read_rows` không đổi.
>
> **Giải thích.** `LowCardinality` lưu một từ điển cho mỗi part cộng chỉ mục vị trí, nên khoảng
> 200 mã quốc gia được ghi một lần thay vì 10 triệu lần. Việc gom nhóm sau đó diễn ra trên số
> nguyên nhỏ thay vì chuỗi, và vector hoá tốt.

## Những câu hỏi ghi chép phải trả lời

Level chưa xong cho tới khi mỗi câu dưới đây có câu trả lời kèm số đo của chính bạn:

1. **Vì sao chọn `ORDER BY` này?** Ba bảng trên cùng dữ liệu với ba thứ tự, tám query benchmark
   mỗi bảng, so sánh thời gian, `read_rows` và dung lượng đĩa.
2. **`index_granularity` thay đổi điều gì?** 8192 vs 4096 vs 16384: kích thước mark file và tốc
   độ query điểm.
3. **`LowCardinality` tiết kiệm bao nhiêu?** Theo từng cột: dung lượng và tốc độ gom nhóm.
4. **Codec nào cho cột nào?** `ZSTD(1)` vs `ZSTD(3)` vs `ZSTD(9)` vs `LZ4` trên `page` và
   `properties`; `Delta+ZSTD` vs `DoubleDelta` trên `event_time`.
5. **Vì sao tránh `Nullable`?** `Nullable(String)` vs `String DEFAULT ''`, đo cụ thể.
6. **Skip index có giúp không?** `bloom_filter` trên `user_id` và `page`, `tokenbf_v1` cho tìm
   chuỗi con, `minmax` trên `ingested_at` — `read_rows` trước và sau.
7. **Projection có đáng không?** Ba con số: đĩa tăng bao nhiêu, insert chậm bao nhiêu, query
   nhanh mấy lần.
8. **TTL recompress tiết kiệm bao nhiêu?** Dung lượng trước và sau khi đẩy part 30 ngày tuổi
   sang `ZSTD(9)`.

## Kỷ luật đo đạc

- **Tối thiểu 10M event.** Kết luận rút ra từ bảng nhỏ chỉ là nhiễu, và hình dạng câu trả lời
  thay đổi theo quy mô.
- **Ghi mức dữ liệu bên cạnh mọi con số.** "62 ms" tự nó không có nghĩa gì.
- **Đo cold và warm riêng.** Xoá cache trước khi đo cold:
  ```sql
  SYSTEM DROP MARK CACHE;
  SYSTEM DROP UNCOMPRESSED CACHE;
  ```
- **Ba lần chạy, lấy trung vị.** Một lần chạy chỉ đo tâm trạng của cái máy.
- **Đọc execution plan, đừng chỉ nhìn đồng hồ.** `EXPLAIN indexes = 1` cho thấy bao nhiêu
  granule bị loại; `EXPLAIN PIPELINE` cho thấy bao nhiêu luồng làm việc. Một query nhanh lên vì
  lý do bạn không gọi tên được rồi sẽ chậm lại.

## Query soi bảng hữu ích

Những câu này nằm trong `docs/queries-ops.sql` khi Level 2 bắt đầu:

```sql
-- Số part, dung lượng, số dòng theo từng bảng
SELECT table, count() AS parts, formatReadableSize(sum(bytes_on_disk)) AS size, sum(rows)
FROM system.parts WHERE active AND database = 'analytics'
GROUP BY table ORDER BY sum(bytes_on_disk) DESC;

-- Tỉ lệ nén theo từng cột: con số thường gây bất ngờ nhất
SELECT column,
       formatReadableSize(sum(column_data_compressed_bytes))   AS compressed,
       formatReadableSize(sum(column_data_uncompressed_bytes)) AS uncompressed,
       round(sum(column_data_uncompressed_bytes) / sum(column_data_compressed_bytes), 2) AS ratio
FROM system.parts_columns WHERE active AND table = 'events'
GROUP BY column ORDER BY sum(column_data_compressed_bytes) DESC;

-- Query chậm nhất một giờ qua, lấy từ log của chính server
SELECT query_duration_ms, read_rows, formatReadableSize(memory_usage), substring(query, 1, 120)
FROM system.query_log
WHERE type = 'QueryFinish' AND event_time > now() - INTERVAL 1 HOUR
ORDER BY query_duration_ms DESC LIMIT 20;
```
