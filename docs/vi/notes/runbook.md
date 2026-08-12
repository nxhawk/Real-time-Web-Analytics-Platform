# Runbook

<Badge type="warning" text="Điền dần ở Level 6" />

::: info Viết một phần
Các kiểu hỏng dưới đây đã biết từ khâu thiết kế; lệnh cụ thể và thời gian khôi phục đo được sẽ
điền khi hệ thống chạy thật.
:::

Làm gì khi có sự cố. Viết trước khi sự cố xảy ra, không phải trong lúc nó xảy ra.

## Thứ tự phân loại

1. `/healthz` trên cả hai service — tiến trình còn sống không?
2. `/readyz` — dependency nào đang không ổn?
3. Grafana **Ingest health** — event còn vào không?
4. `pulse_end_to_end_lag_seconds` — pipeline có tụt lại không, tụt bao nhiêu?
5. Dung lượng đĩa — ClickHouse dừng nhận ghi khi đầy, và khôi phục thì chậm.

## Các kiểu hỏng

### `Too many parts` — insert lỗi 252

**Nguyên nhân.** Quá nhiều insert nhỏ. ClickHouse merge part ở nền và từ chối insert mới khi
không theo kịp.

**Xử lý.** Tăng `BATCH_SIZE` hoặc `FLUSH_INTERVAL_MS` để mỗi insert mang ít nhất 10k row. Kiểm
chứng bằng `SELECT table, count() FROM system.parts WHERE active GROUP BY table`. Tăng
`parts_to_throw_insert` mua được thời gian nhưng không sửa nguyên nhân.

### Consumer lag tăng dần

**Nguyên nhân.** Consumer chậm hơn producer — ClickHouse chậm, một lần merge lớn, một message
độc bị retry mãi, hoặc đơn giản là thiếu consumer.

**Kiểm tra.** `pulse_kafka_consumer_lag` theo từng partition. Lag ở một partition nghĩa là
message độc; lag ở tất cả nghĩa là thiếu throughput.

**Xử lý.** Thêm replica consumer, tối đa bằng số partition. Nếu một partition kẹt, tìm offset,
và để giới hạn retry đẩy message vào DLQ thay vì gỡ giới hạn đi.

### Đầy đĩa

**Nguyên nhân.** Event thô cộng projection cộng retention Kafka cộng log Docker.

**Xử lý, theo thứ tự nhanh nhất.** Giảm retention Kafka, `docker system prune -af`, kiểm
`/data/caddy/access.log`, xác nhận TTL có thực sự đang áp. Cảnh báo bắn ở 75% chính là để làm
việc này trước khi ghi bị dừng.

### ClickHouse bị OOM-kill

**Nguyên nhân.** `max_server_memory_usage` đặt cao hơn giới hạn bộ nhớ của container, nên kernel
giết tiến trình trước khi ClickHouse kịp từ chối query.

**Xử lý.** Giữ giới hạn Docker cao hơn `max_server_memory_usage` khoảng 10%.

### Event đã nhận nhưng không thấy đâu

**Tìm theo thứ tự.** Thư mục WAL có đang phình lên không (ClickHouse không tới được)?
`kafka_dlq_total` có tăng không (validate đang loại chúng)? Materialized View có thiếu backfill
không (raw có dữ liệu, view thì không)?

### Một query đột nhiên chậm

Kiểm `system.query_log` xem `read_rows`. Nó nhảy vọt thường nghĩa là một bộ lọc không còn khớp
thứ tự sắp xếp, hoặc materialized view bị bỏ qua. `EXPLAIN indexes = 1` cho biết granule còn
được loại bỏ hay không.

## Quy trình khôi phục

### Replay WAL

Write-ahead log chứa các batch NDJSON mà ClickHouse đã từ chối. Tiến trình replay quét `WAL_DIR`,
insert lại, và xoá từng file khi thành công. Kiểm chứng bằng cách so số dòng trước và sau.

### Replay DLQ

`cmd/dlq-replay` đọc `events.dlq`, cho phép lọc hoặc sửa, rồi produce lại vào `events.raw`. Sửa
nguyên nhân trước — replay vào đúng cái bug đó chỉ làm DLQ đầy lại.

### Restore từ snapshot

Ghi trong
[`DEPLOY-AWS.md` §13](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/DEPLOY-AWS.md).
Tạo volume từ snapshot, gắn vào một instance tạm, đối chiếu số dòng, rồi tráo.

::: danger Diễn tập một lần
Backup chưa từng restore thì chưa phải backup. Level 6 yêu cầu diễn tập một lần, với **thời gian
khôi phục thực tế** ghi lại ở đây. Con số đó là thứ bạn sẽ bị hỏi trong một sự cố thật.
:::

## Số liệu vận hành

Điền từ phép đo thật, không phải ước lượng:

| Phép đo | Giá trị |
|---|---|
| RTO restore từ snapshot | *(ghi lại sau khi diễn tập)* |
| Thời gian deploy, từ tag tới healthy | |
| `make up` trên máy sạch | |
| Kích thước image backend | |

## Sự cố đặc thù AWS

Bảng triệu chứng → nguyên nhân cho môi trường deploy — độ dài hàng đợi EBS, kết nối SSM, chứng
chỉ Caddy, Terraform đòi thay instance, hoá đơn nhảy vọt — nằm ở
[`DEPLOY-AWS.md` §15](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/DEPLOY-AWS.md).

## Lấy shell

```bash
# Vào máy production mà không cần SSH
aws ssm start-session --target i-xxxxx

# Port-forward ClickHouse HTTP về máy mình mà không cần mở security group
aws ssm start-session --target i-xxxxx \
  --document-name AWS-StartPortForwardingSession \
  --parameters '{"portNumber":["8123"],"localPortNumber":["8123"]}'
curl localhost:8123/ping
```
