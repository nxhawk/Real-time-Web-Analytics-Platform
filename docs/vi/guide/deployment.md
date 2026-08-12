# Triển khai

<Badge type="warning" text="Level 6 / phase AWS" />

Production là **Next.js trên Vercel** cộng **một EC2 `r7g.xlarge`** ở `ap-southeast-1` chạy
toàn bộ backend bằng Docker Compose. Hạ tầng khai báo bằng Terraform, TLS tự động nhờ Caddy,
image đi qua ECR, và deploy chạy qua SSM mà không cần khoá SSH.

Hướng dẫn đầy đủ — module Terraform, cloud-init, phân bổ RAM, tinh chỉnh ClickHouse và Kafka
cho một máy, CI/CD, backup và restore, giám sát chi phí, teardown — nằm ở
[`DEPLOY-AWS.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/DEPLOY-AWS.md).
Trang này là bản tóm tắt.

## Sơ đồ

```
Browser ──┬─▶ Vercel            app.pulse.dev      Dashboard Next.js, SSR + CDN
          │
          └─▶ EC2 r7g.xlarge    api.pulse.dev      Caddy :443, TLS tự động
                 ├── /collect ──▶ ingest-api  :8080
                 ├── /api     ──▶ analytics-api :8081
                 ├── Kafka :9092  ──▶ consumer
                 ├── ClickHouse :9000 ──▶ EBS gp3 500 GB, 6000 IOPS
                 └── Prometheus + Grafana        grafana.pulse.dev

Security group chỉ mở 80 và 443. Vào máy bằng SSM, không bao giờ SSH.
```

## Các quyết định và cái giá của chúng

| Quyết định | Vì sao | Phải chấp nhận |
|---|---|---|
| Một EC2 chạy tất cả | Rẻ nhất, ít thứ vận hành nhất, đủ cho 100M event | Kafka và Go tranh CPU với page cache của ClickHouse, nên **số benchmark sẽ bi quan hơn thực tế** — phải ghi rõ điều đó trong báo cáo |
| Public subnet, không NAT Gateway | NAT Gateway tốn ~$35/tháng mà chẳng để làm gì ở đây | Máy có IP public, bù lại bằng security group chặt và SSM |
| SSM thay SSH | Không phải quản khoá, không mở cổng 22, có audit log | Cần SSM agent và IAM role |
| Caddy thay ALB | ALB ~$20/tháng cộng phí LCU | Không có health check ở tầng load balancer, không autoscale |
| EBS gp3 thay instance store | Dữ liệu còn nguyên khi stop/start | Chậm hơn NVMe local 3–5 lần |
| ECR thay GHCR | Pull cùng region, không tốn egress | Thêm một dịch vụ phải quản, ~$0.10/GB/tháng |
| arm64 (Graviton) | Rẻ hơn ~20%, mọi thứ đều hỗ trợ | CI phải build cho arm64 |

## Chi phí

Khoảng **$267/tháng** giá on-demand ở `ap-southeast-1` — riêng instance đã $174. Compute
Savings Plan một năm giảm ~30%; tắt máy ngoài giờ làm giảm ~60%. Spot thì không: bị thu hồi
giữa lúc merge là hỏng dữ liệu.

Đặt AWS Budget kèm cảnh báo **trước khi tạo bất kỳ tài nguyên nào**. Đó là task đầu tiên trong
checklist AWS, và có lý do cho việc đó.

## Luồng deploy

```
push tag v*  hoặc  chạy tay
        │
        ▼  GitHub OIDC → AWS role (không lưu credential tĩnh trong Secrets)
  build image arm64 → push lên ECR với tag sha bất biến
        │
        ▼  SSM send-command
  docker compose pull
  docker compose run --rm migrate up      ← migration trước khi đổi app
  docker compose up -d --no-deps ingest-api analytics-api consumer
  vòng healthcheck: curl /readyz, 30 lần, cách nhau 2s
        │
        └─ thất bại ──▶ rollback về tag trước
```

Migration luôn tương thích ngược một bước: thêm cột, rồi đọc/ghi cột đó, rồi mới xoá cột cũ ở
release sau. Trong ClickHouse `ALTER TABLE ADD COLUMN` chỉ đụng metadata nên rẻ; `DROP COLUMN`
và `MODIFY COLUMN` phải ghi lại dữ liệu.

## Trước lần deploy đầu tiên

Đừng deploy thứ bạn không nhìn thấy. Metrics (L6.1), logging (L6.2) và security hardening
(L6.3) phải xong trước — nếu không, sự cố production đầu tiên sẽ phải debug mù.

Danh sách cổng chặn, rút gọn:

- `terraform plan` chạy lần hai báo **không có thay đổi**
- SSM vào được; các cổng 8123, 9000 và 9092 không truy cập được từ Internet
- Caddy đã lấy được chứng chỉ cho `api.*` và `grafana.*`
- Cố tình deploy image hỏng thì smoke test fail và **rollback tự chạy**
- Đã restore snapshot ít nhất một lần, với RTO thực tế ghi vào `docs/runbook.md`

Backup chưa từng restore thì chưa phải backup.

## Phương án còn lại

`PLAN.md` §17.4–17.5 mô tả đường "1 VPS + SSH + Docker Compose", 7 giờ thay vì 14.
`DEPLOY-AWS.md` thay thế nó. Chọn một đường và đánh dấu đường kia là `[-]` trong `TODO.md` để
checklist không ngụ ý cả hai đều đang chờ làm.
