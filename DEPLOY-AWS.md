# Pulse Analytics — Deploy: Vercel (FE) + EC2 all-in-one (BE)

> Bổ sung cho `PLAN.md` §17 và `TODO.md` L6.4 — thay thế phương án "VPS + docker compose".
> Region: **ap-southeast-1 (Singapore)** · Instance: **1 × r7g.xlarge** · IaC: **Terraform**
> Phiên bản: 1.0 — 2026-08-11

---

## Mục lục

1. [Kiến trúc & DNS](#1-kiến-trúc--dns)
2. [Quyết định & đánh đổi](#2-quyết-định--đánh-đổi)
3. [Chuẩn bị AWS account](#3-chuẩn-bị-aws-account)
4. [Terraform — cấu trúc](#4-terraform--cấu-trúc)
5. [Terraform — code](#5-terraform--code)
6. [Bootstrap EC2 (cloud-init)](#6-bootstrap-ec2-cloud-init)
7. [Phân bổ RAM & docker-compose.prod.yml](#7-phân-bổ-ram--docker-composeprodyml)
8. [Caddy — reverse proxy & TLS](#8-caddy--reverse-proxy--tls)
9. [Tinh chỉnh ClickHouse & Kafka cho 1 máy](#9-tinh-chỉnh-clickhouse--kafka-cho-1-máy)
10. [CI/CD: GitHub Actions → ECR → SSM](#10-cicd-github-actions--ecr--ssm)
11. [Vercel setup](#11-vercel-setup)
12. [CORS & authentication](#12-cors--authentication)
13. [Backup & restore](#13-backup--restore)
14. [Giám sát chi phí](#14-giám-sát-chi-phí)
15. [Runbook sự cố đặc thù AWS](#15-runbook-sự-cố-đặc-thù-aws)
16. [Teardown](#16-teardown)
17. [Checklist thay thế L6.4](#17-checklist-thay-thế-l64)

---

## 1. Kiến trúc & DNS

```
                        ┌──────────────────────────────┐
                        │   Browser (người dùng VN)    │
                        └───┬──────────────────────┬───┘
             HTML/JS/CSS    │                      │  JSON API + event
                            ▼                      ▼
             ┌──────────────────────┐   ┌────────────────────────────────────┐
             │      VERCEL          │   │      AWS ap-southeast-1             │
             │  app.pulse.dev       │   │                                     │
             │  Next.js 16 SSR/CDN  │   │  ┌───────────────────────────────┐  │
             │  Preview mỗi PR      │   │  │ EC2 r7g.xlarge (public subnet)│  │
             └──────────────────────┘   │  │  Elastic IP → api.pulse.dev   │  │
                     │                  │  │                               │  │
                     │ chỉ build-time   │  │  :443 ┌─────────┐             │  │
                     │ (ISR/SSG)        │  │ ──────│  Caddy  │ TLS auto    │  │
                     └──────────────────┼──┼──────▶└────┬────┘             │  │
                                        │  │            │                  │  │
                                        │  │   /collect ▼      /api ▼      │  │
                                        │  │  ┌────────────┐ ┌───────────┐ │  │
                                        │  │  │ ingest-api │ │analytics- │ │  │
                                        │  │  │   :8080    │ │ api :8081 │ │  │
                                        │  │  └─────┬──────┘ └─────┬─────┘ │  │
                                        │  │        ▼              │       │  │
                                        │  │   ┌─────────┐         │       │  │
                                        │  │   │  Kafka  │         │       │  │
                                        │  │   │  :9092  │         │       │  │
                                        │  │   └────┬────┘         │       │  │
                                        │  │        ▼              │       │  │
                                        │  │   ┌──────────┐        │       │  │
                                        │  │   │ consumer │        │       │  │
                                        │  │   └────┬─────┘        │       │  │
                                        │  │        ▼              ▼       │  │
                                        │  │   ┌───────────────────────┐   │  │
                                        │  │   │  ClickHouse :9000     │   │  │
                                        │  │   │  /var/lib/clickhouse  │───┼──┼──▶ EBS gp3 500GB
                                        │  │   └───────────────────────┘   │  │    6000 IOPS
                                        │  │   Prometheus + Grafana        │  │
                                        │  └───────────────────────────────┘  │
                                        │   SG: chỉ mở 80/443 · SSH: qua SSM  │
                                        │   ECR · S3 (deploy) · DLM snapshot  │
                                        └─────────────────────────────────────┘
```

### DNS (Route53 hoặc Cloudflare)

| Bản ghi | Kiểu | Giá trị | Ghi chú |
|---|---|---|---|
| `app.pulse.dev` | CNAME | `cname.vercel-dns.com` | Dashboard |
| `pulse.dev` | A/ALIAS | Vercel | Landing (nếu có) |
| `api.pulse.dev` | A | Elastic IP của EC2 | Analytics API + ingest |
| `grafana.pulse.dev` | A | Elastic IP | Bảo vệ bằng basic auth ở Caddy |

> Nếu dùng Cloudflare đứng trước `api.pulse.dev`: bật proxy (đám mây cam) cho `/collect` để chống DDoS và cache `pixel.gif`, **tắt proxy** cho `/api` khi benchmark để không đo nhầm độ trễ Cloudflare.

---

## 2. Quyết định & đánh đổi

| Quyết định | Lý do | Đánh đổi phải chấp nhận |
|---|---|---|
| 1 EC2 all-in-one | Rẻ nhất, ít thứ phải vận hành, đủ cho 100M event | Kafka (JVM) và Go tranh CPU/page-cache với ClickHouse → **số benchmark sẽ bi quan hơn thực tế**. Phải ghi rõ điều này trong `benchmark-results.md` |
| Public subnet, không NAT Gateway | NAT GW tốn ~$35/tháng mà chẳng để làm gì ở đây | Instance có IP public — bù lại bằng SG chặt + SSM |
| Không SSH, dùng SSM Session Manager | Không cần quản lý key, không mở port 22, có audit log | Phải cài SSM agent (AL2023 có sẵn) và gắn IAM role |
| Caddy thay ALB | ALB ~$20/tháng + phí LCU; Caddy tự lo TLS, chạy trong compose | Không có health-check tự động ở tầng LB, không auto-scale |
| EBS gp3 provisioned thay instance store | Dữ liệu không mất khi stop/start instance | Chậm hơn NVMe local ~3–5 lần. `i4g`/`im4gn` có NVMe nhưng **mất sạch dữ liệu khi stop** |
| ECR thay GHCR | Pull trong cùng region không tốn egress, nhanh hơn | Thêm 1 dịch vụ phải quản; ECR lưu trữ ~$0.10/GB/tháng |
| arm64 (Graviton) | Rẻ hơn ~20%, ClickHouse/Kafka/Go đều hỗ trợ tốt | CI phải build multi-arch hoặc build thẳng arm64 |

### Khi nào nên tách máy

Chuyển sang 2 EC2 (theo `PLAN.md` ADR mới) khi gặp **một trong** các dấu hiệu:

- Benchmark query dao động > 30% giữa các lần chạy (dấu hiệu tranh tài nguyên)
- ClickHouse bị OOM-kill dù `max_server_memory_usage` đã đặt đúng
- Consumer lag tăng mỗi khi có merge lớn

---

## 3. Chuẩn bị AWS account

- [ ] Bật **MFA** cho root, tạo IAM user riêng cho việc thao tác tay
- [ ] **AWS Budgets**: ngân sách $300/tháng, alert ở 50% / 80% / 100% qua email — làm trước khi tạo bất kỳ tài nguyên nào
- [ ] Bật **Cost Anomaly Detection**
- [ ] Chọn region `ap-southeast-1`, đặt mặc định trong `~/.aws/config`
- [ ] Tạo S3 bucket + DynamoDB table cho Terraform state (hoặc dùng Terraform Cloud free tier)
- [ ] Đăng ký domain, trỏ nameserver về Route53 (hoặc giữ ở Cloudflare)

```bash
# Terraform backend bootstrap (chạy 1 lần, bằng tay)
aws s3api create-bucket --bucket pulse-tfstate-<random> \
  --region ap-southeast-1 --create-bucket-configuration LocationConstraint=ap-southeast-1
aws s3api put-bucket-versioning --bucket pulse-tfstate-<random> \
  --versioning-configuration Status=Enabled
aws dynamodb create-table --table-name pulse-tflock \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST --region ap-southeast-1
```

---

## 4. Terraform — cấu trúc

```
infra/
├── backend.tf          # S3 state + DynamoDB lock
├── providers.tf
├── variables.tf
├── terraform.tfvars    # KHÔNG commit (thêm vào .gitignore)
├── network.tf          # VPC, subnet, IGW, route table
├── security.tf         # Security Group
├── iam.tf              # instance profile + GitHub OIDC role
├── compute.tf          # EC2, EBS data volume, Elastic IP
├── ecr.tf
├── s3.tf               # bucket chứa file deploy
├── dns.tf              # Route53 records
├── backup.tf           # DLM snapshot policy
├── budget.tf
├── outputs.tf
└── user_data.sh.tftpl
```

---

## 5. Terraform — code

### `providers.tf` + `backend.tf`

```hcl
terraform {
  required_version = ">= 1.9"
  required_providers {
    aws = { source = "hashicorp/aws", version = "~> 5.60" }
  }
  backend "s3" {
    bucket         = "pulse-tfstate-<random>"
    key            = "prod/terraform.tfstate"
    region         = "ap-southeast-1"
    dynamodb_table = "pulse-tflock"
    encrypt        = true
  }
}

provider "aws" {
  region = var.region
  default_tags {
    tags = {
      Project     = "pulse-analytics"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}
```

### `variables.tf`

```hcl
variable "region"        { type = string  default = "ap-southeast-1" }
variable "environment"   { type = string  default = "prod" }
variable "instance_type" { type = string  default = "r7g.xlarge" }
variable "data_volume_gb"        { type = number default = 500 }
variable "data_volume_iops"      { type = number default = 6000 }
variable "data_volume_throughput"{ type = number default = 250 }
variable "domain"        { type = string }               # pulse.dev
variable "api_subdomain" { type = string default = "api" }
variable "github_repo"   { type = string }               # user/pulse-analytics
variable "alert_email"   { type = string }
variable "admin_cidr"    { type = string default = "0.0.0.0/0" } # dùng khi cần mở tạm
```

### `network.tf`

```hcl
resource "aws_vpc" "main" {
  cidr_block           = "10.20.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags = { Name = "pulse-vpc" }
}

resource "aws_internet_gateway" "igw" {
  vpc_id = aws_vpc.main.id
  tags   = { Name = "pulse-igw" }
}

data "aws_availability_zones" "available" { state = "available" }

resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.20.1.0/24"
  availability_zone       = data.aws_availability_zones.available.names[0]
  map_public_ip_on_launch = true
  tags = { Name = "pulse-public-a" }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.igw.id
  }
  tags = { Name = "pulse-rt-public" }
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}
```

### `security.tf`

```hcl
resource "aws_security_group" "app" {
  name        = "pulse-app"
  description = "Pulse Analytics - chi mo HTTP/HTTPS"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "HTTP (Caddy redirect + ACME)"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "pulse-app-sg" }
}
```

> **Không có rule cho 22, 8123, 9000, 9092.** ClickHouse và Kafka chỉ nghe trên `127.0.0.1` / docker network. Cần truy cập → `aws ssm start-session` rồi port-forward.

### `iam.tf`

```hcl
# --- Instance profile ---
resource "aws_iam_role" "ec2" {
  name = "pulse-ec2-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow", Action = "sts:AssumeRole",
      Principal = { Service = "ec2.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "ssm" {
  role       = aws_iam_role.ec2.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_role_policy_attachment" "cw" {
  role       = aws_iam_role.ec2.name
  policy_arn = "arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy"
}

resource "aws_iam_role_policy" "ec2_extra" {
  role = aws_iam_role.ec2.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = ["ecr:GetAuthorizationToken", "ecr:BatchGetImage",
                  "ecr:GetDownloadUrlForLayer", "ecr:BatchCheckLayerAvailability"]
        Resource = "*"
      },
      {
        Effect   = "Allow"
        Action   = ["s3:GetObject", "s3:ListBucket"]
        Resource = [aws_s3_bucket.deploy.arn, "${aws_s3_bucket.deploy.arn}/*"]
      }
    ]
  })
}

resource "aws_iam_instance_profile" "ec2" {
  name = "pulse-ec2-profile"
  role = aws_iam_role.ec2.name
}

# --- GitHub Actions OIDC (khong can luu AWS key trong Secrets) ---
resource "aws_iam_openid_connect_provider" "github" {
  url             = "https://token.actions.githubusercontent.com"
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1"]
}

resource "aws_iam_role" "github_actions" {
  name = "pulse-github-actions"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRoleWithWebIdentity"
      Principal = { Federated = aws_iam_openid_connect_provider.github.arn }
      Condition = {
        StringEquals = {
          "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
        }
        StringLike = {
          "token.actions.githubusercontent.com:sub" = "repo:${var.github_repo}:*"
        }
      }
    }]
  })
}

resource "aws_iam_role_policy" "github_actions" {
  role = aws_iam_role.github_actions.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = ["ecr:GetAuthorizationToken"]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = ["ecr:BatchCheckLayerAvailability", "ecr:CompleteLayerUpload",
                  "ecr:InitiateLayerUpload", "ecr:PutImage", "ecr:UploadLayerPart",
                  "ecr:BatchGetImage", "ecr:GetDownloadUrlForLayer"]
        Resource = aws_ecr_repository.backend.arn
      },
      {
        Effect   = "Allow"
        Action   = ["s3:PutObject", "s3:DeleteObject", "s3:ListBucket"]
        Resource = [aws_s3_bucket.deploy.arn, "${aws_s3_bucket.deploy.arn}/*"]
      },
      {
        Effect   = "Allow"
        Action   = ["ssm:SendCommand", "ssm:GetCommandInvocation", "ssm:ListCommandInvocations"]
        Resource = "*"
      }
    ]
  })
}
```

### `compute.tf`

```hcl
data "aws_ssm_parameter" "al2023_arm64" {
  name = "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64"
}

resource "aws_instance" "app" {
  ami                    = data.aws_ssm_parameter.al2023_arm64.value
  instance_type          = var.instance_type
  subnet_id              = aws_subnet.public.id
  vpc_security_group_ids = [aws_security_group.app.id]
  iam_instance_profile   = aws_iam_instance_profile.ec2.name

  root_block_device {
    volume_type = "gp3"
    volume_size = 30
    encrypted   = true
  }

  metadata_options {
    http_tokens   = "required"   # bắt buộc IMDSv2
    http_endpoint = "enabled"
  }

  user_data = templatefile("${path.module}/user_data.sh.tftpl", {
    region        = var.region
    deploy_bucket = aws_s3_bucket.deploy.bucket
  })
  user_data_replace_on_change = false   # đổi user_data KHÔNG thay instance

  lifecycle { ignore_changes = [ami] }  # tránh Terraform thay máy khi AWS ra AMI mới

  tags = { Name = "pulse-app" }
}

resource "aws_ebs_volume" "data" {
  availability_zone = aws_subnet.public.availability_zone
  size              = var.data_volume_gb
  type              = "gp3"
  iops              = var.data_volume_iops
  throughput        = var.data_volume_throughput
  encrypted         = true
  tags              = { Name = "pulse-data", Backup = "daily" }

  lifecycle { prevent_destroy = true }  # chống xoá nhầm dữ liệu
}

resource "aws_volume_attachment" "data" {
  device_name = "/dev/sdf"
  volume_id   = aws_ebs_volume.data.id
  instance_id = aws_instance.app.id
}

resource "aws_eip" "app" {
  instance = aws_instance.app.id
  domain   = "vpc"
  tags     = { Name = "pulse-eip" }
}
```

> **Hai dòng quan trọng nhất file này**: `prevent_destroy` trên EBS data và `ignore_changes = [ami]` trên instance. Thiếu chúng, một lần `terraform apply` vô ý có thể xoá sạch 100M event.

### `ecr.tf`, `s3.tf`, `dns.tf`

```hcl
resource "aws_ecr_repository" "backend" {
  name                 = "pulse-backend"
  image_tag_mutability = "IMMUTABLE"
  image_scanning_configuration { scan_on_push = true }
  encryption_configuration { encryption_type = "AES256" }
}

resource "aws_ecr_lifecycle_policy" "backend" {
  repository = aws_ecr_repository.backend.name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "Giu 20 image gan nhat"
      selection    = { tagStatus = "any", countType = "imageCountMoreThan", countNumber = 20 }
      action       = { type = "expire" }
    }]
  })
}

resource "random_id" "suffix" { byte_length = 4 }

resource "aws_s3_bucket" "deploy" {
  bucket = "pulse-deploy-${random_id.suffix.hex}"
}

resource "aws_s3_bucket_public_access_block" "deploy" {
  bucket                  = aws_s3_bucket.deploy.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_versioning" "deploy" {
  bucket = aws_s3_bucket.deploy.id
  versioning_configuration { status = "Enabled" }
}

data "aws_route53_zone" "main" { name = var.domain }

resource "aws_route53_record" "api" {
  zone_id = data.aws_route53_zone.main.zone_id
  name    = "${var.api_subdomain}.${var.domain}"
  type    = "A"
  ttl     = 60
  records = [aws_eip.app.public_ip]
}

resource "aws_route53_record" "grafana" {
  zone_id = data.aws_route53_zone.main.zone_id
  name    = "grafana.${var.domain}"
  type    = "A"
  ttl     = 300
  records = [aws_eip.app.public_ip]
}
```

### `backup.tf` + `budget.tf`

```hcl
resource "aws_iam_role" "dlm" {
  name = "pulse-dlm-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "sts:AssumeRole",
                   Principal = { Service = "dlm.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy_attachment" "dlm" {
  role       = aws_iam_role.dlm.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSDataLifecycleManagerServiceRole"
}

resource "aws_dlm_lifecycle_policy" "daily" {
  description        = "Snapshot EBS data hang ngay, giu 7 ban"
  execution_role_arn = aws_iam_role.dlm.arn
  state              = "ENABLED"

  policy_details {
    resource_types = ["VOLUME"]
    target_tags    = { Backup = "daily" }

    schedule {
      name = "daily-03-00-ict"
      create_rule { interval = 24, interval_unit = "HOURS", times = ["20:00"] } # 20:00 UTC = 03:00 ICT
      retain_rule { count = 7 }
      copy_tags = true
    }
  }
}

resource "aws_budgets_budget" "monthly" {
  name         = "pulse-monthly"
  budget_type  = "COST"
  limit_amount = "300"
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  dynamic "notification" {
    for_each = [50, 80, 100]
    content {
      comparison_operator        = "GREATER_THAN"
      threshold                  = notification.value
      threshold_type             = "PERCENTAGE"
      notification_type          = "ACTUAL"
      subscriber_email_addresses = [var.alert_email]
    }
  }
}
```

### `outputs.tf`

```hcl
output "instance_id"   { value = aws_instance.app.id }
output "public_ip"     { value = aws_eip.app.public_ip }
output "ecr_url"       { value = aws_ecr_repository.backend.repository_url }
output "deploy_bucket" { value = aws_s3_bucket.deploy.bucket }
output "gha_role_arn"  { value = aws_iam_role.github_actions.arn }
output "ssm_connect"   { value = "aws ssm start-session --target ${aws_instance.app.id}" }
```

---

## 6. Bootstrap EC2 (cloud-init)

`infra/user_data.sh.tftpl`:

```bash
#!/bin/bash
set -euxo pipefail

dnf update -y
dnf install -y docker git jq amazon-cloudwatch-agent

# --- Docker + compose plugin ---
systemctl enable --now docker
mkdir -p /usr/local/lib/docker/cli-plugins
ARCH=$(uname -m)
curl -SL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-$${ARCH}" \
  -o /usr/local/lib/docker/cli-plugins/docker-compose
chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

# --- Mount EBS data volume ---
DEV=$(lsblk -ndo NAME,SIZE | awk '$2!="30G" && $1 ~ /^nvme/ {print "/dev/"$1; exit}')
if ! blkid "$DEV"; then
  mkfs.xfs -f "$DEV"           # XFS: ClickHouse khuyen dung
fi
mkdir -p /data
UUID=$(blkid -s UUID -o value "$DEV")
grep -q "$UUID" /etc/fstab || echo "UUID=$UUID /data xfs defaults,noatime,nofail 0 2" >> /etc/fstab
mount -a

mkdir -p /data/clickhouse /data/kafka /data/prometheus /data/grafana /data/wal /data/caddy
chown -R 101:101 /data/clickhouse    # uid clickhouse trong image chinh thuc

# --- Kernel tuning cho ClickHouse ---
cat >/etc/sysctl.d/99-clickhouse.conf <<'EOF'
vm.max_map_count = 1048576
vm.swappiness = 1
fs.file-max = 2097152
net.core.somaxconn = 4096
net.ipv4.tcp_max_syn_backlog = 4096
EOF
sysctl --system

# Tat transparent hugepages (ClickHouse khuyen dung 'madvise')
echo madvise > /sys/kernel/mm/transparent_hugepage/enabled || true
cat >/etc/systemd/system/disable-thp.service <<'EOF'
[Unit]
Description=Disable THP
After=sysinit.target
[Service]
Type=oneshot
ExecStart=/bin/sh -c "echo madvise > /sys/kernel/mm/transparent_hugepage/enabled"
RemainAfterExit=yes
[Install]
WantedBy=multi-user.target
EOF
systemctl enable disable-thp.service

# --- ulimit cho docker ---
mkdir -p /etc/systemd/system/docker.service.d
cat >/etc/systemd/system/docker.service.d/limits.conf <<'EOF'
[Service]
LimitNOFILE=1048576
LimitNPROC=unlimited
EOF
systemctl daemon-reload && systemctl restart docker

# --- Docker log rotation (khong co la day dia sau 2 tuan) ---
cat >/etc/docker/daemon.json <<'EOF'
{ "log-driver": "json-file", "log-opts": { "max-size": "50m", "max-file": "3" } }
EOF
systemctl restart docker

# --- Thu muc ung dung ---
mkdir -p /opt/pulse
aws s3 sync "s3://${deploy_bucket}/deploy" /opt/pulse --region ${region} || true

echo "bootstrap done" > /var/log/pulse-bootstrap.done
```

> Lưu ý cú pháp: trong `templatefile`, biến shell phải escape thành `$${VAR}`, còn `${region}` là biến Terraform.

---

## 7. Phân bổ RAM & docker-compose.prod.yml

### Ngân sách 32GB trên r7g.xlarge

| Thành phần | Giới hạn | Lý do |
|---|---|---|
| ClickHouse | **18GB** (`max_server_memory_usage`) | Chừa page cache cho chính nó |
| Kafka (JVM) | **3GB** (heap 2GB) | Kafka dựa vào page cache OS, heap không cần lớn |
| ingest-api ×1 | 1GB | Buffer 200k event ≈ 200MB |
| consumer ×1 | 1.5GB | Batch 10k × 2 |
| analytics-api | 1GB | |
| Caddy + Prometheus + Grafana | 2GB | |
| **Còn lại cho OS + page cache** | **~5.5GB** | Đây là phần khiến ClickHouse nhanh — đừng ăn hết |

### `docker-compose.prod.yml`

```yaml
name: pulse

x-logging: &default-logging
  driver: json-file
  options: { max-size: "50m", max-file: "3" }

services:
  clickhouse:
    image: clickhouse/clickhouse-server:26.3
    restart: unless-stopped
    logging: *default-logging
    volumes:
      - /data/clickhouse:/var/lib/clickhouse
      - ./clickhouse/config.d:/etc/clickhouse-server/config.d:ro
      - ./clickhouse/users.d:/etc/clickhouse-server/users.d:ro
    environment:
      CLICKHOUSE_DB: analytics
      CLICKHOUSE_USER: pulse
      CLICKHOUSE_PASSWORD: ${CLICKHOUSE_PASSWORD}
    ulimits:
      nofile: { soft: 262144, hard: 262144 }
    deploy:
      resources:
        limits: { memory: 20G, cpus: '3.0' }
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8123/ping"]
      interval: 10s
      timeout: 5s
      retries: 12
    # KHONG publish port ra host

  kafka:
    image: apache/kafka:4.0.0
    restart: unless-stopped
    logging: *default-logging
    volumes:
      - /data/kafka:/var/lib/kafka/data
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093
      KAFKA_LISTENERS: PLAINTEXT://kafka:9092,CONTROLLER://kafka:9093
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_MIN_ISR: 1
      KAFKA_NUM_PARTITIONS: 6
      KAFKA_LOG_RETENTION_HOURS: 168
      KAFKA_COMPRESSION_TYPE: zstd
      KAFKA_HEAP_OPTS: "-Xmx2g -Xms2g"
    deploy:
      resources:
        limits: { memory: 3G, cpus: '1.0' }

  migrate:
    image: ${ECR_URL}:${IMAGE_TAG}
    command: ["/app/migrate", "up"]
    restart: "no"
    environment:
      CLICKHOUSE_DSN: clickhouse://pulse:${CLICKHOUSE_PASSWORD}@clickhouse:9000/analytics
    depends_on:
      clickhouse: { condition: service_healthy }

  ingest-api:
    image: ${ECR_URL}:${IMAGE_TAG}
    command: ["/app/ingest-api"]
    restart: unless-stopped
    logging: *default-logging
    env_file: [.env]
    volumes:
      - /data/wal:/var/lib/pulse/wal
    deploy:
      resources:
        limits: { memory: 1G, cpus: '1.0' }
    healthcheck:
      test: ["CMD", "/app/ingest-api", "-healthcheck"]
      interval: 15s
      retries: 5
    depends_on:
      kafka: { condition: service_started }

  consumer:
    image: ${ECR_URL}:${IMAGE_TAG}
    command: ["/app/consumer"]
    restart: unless-stopped
    logging: *default-logging
    env_file: [.env]
    deploy:
      resources:
        limits: { memory: 1536M, cpus: '1.0' }
    depends_on:
      clickhouse: { condition: service_healthy }
      migrate:    { condition: service_completed_successfully }

  analytics-api:
    image: ${ECR_URL}:${IMAGE_TAG}
    command: ["/app/analytics-api"]
    restart: unless-stopped
    logging: *default-logging
    env_file: [.env]
    deploy:
      resources:
        limits: { memory: 1G, cpus: '1.0' }
    depends_on:
      clickhouse: { condition: service_healthy }

  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    logging: *default-logging
    ports: ["80:80", "443:443", "443:443/udp"]
    volumes:
      - ./caddy/Caddyfile:/etc/caddy/Caddyfile:ro
      - /data/caddy:/data
    environment:
      API_DOMAIN: ${API_DOMAIN}
      GRAFANA_DOMAIN: ${GRAFANA_DOMAIN}
      GRAFANA_BASICAUTH_HASH: ${GRAFANA_BASICAUTH_HASH}

  prometheus:
    image: prom/prometheus:latest
    restart: unless-stopped
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - /data/prometheus:/prometheus
    command: ["--config.file=/etc/prometheus/prometheus.yml", "--storage.tsdb.retention.time=15d"]
    deploy:
      resources: { limits: { memory: 1G } }

  grafana:
    image: grafana/grafana:latest
    restart: unless-stopped
    volumes:
      - /data/grafana:/var/lib/grafana
      - ./grafana/provisioning:/etc/grafana/provisioning:ro
    environment:
      GF_SECURITY_ADMIN_PASSWORD: ${GRAFANA_PASSWORD}
      GF_SERVER_ROOT_URL: https://${GRAFANA_DOMAIN}
    deploy:
      resources: { limits: { memory: 512M } }
```

> Không service nào ngoài `caddy` publish port ra host. Đây là lớp phòng thủ thứ hai sau Security Group.

---

## 8. Caddy — reverse proxy & TLS

`deploy/caddy/Caddyfile`:

```caddyfile
{
    email admin@pulse.dev
    servers {
        protocols h1 h2 h3
    }
}

{$API_DOMAIN} {
    encode zstd gzip

    # --- Ingest: CORS mo cho moi origin dang ky, rate limit o tang app ---
    handle /collect/* {
        reverse_proxy ingest-api:8080 {
            header_up X-Real-IP {remote_host}
            header_up X-Forwarded-For {remote_host}
        }
    }

    handle /pixel.gif {
        header Cache-Control "no-store, no-cache, must-revalidate"
        reverse_proxy ingest-api:8080
    }

    # --- Analytics API ---
    handle /api/* {
        reverse_proxy analytics-api:8081 {
            header_up X-Real-IP {remote_host}
        }
    }

    handle /healthz {
        reverse_proxy analytics-api:8081
    }

    # --- Metrics chi cho localhost (Prometheus goi truc tiep qua docker network) ---
    handle /metrics {
        respond 404
    }

    handle {
        respond "Pulse Analytics API" 200
    }

    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        X-Content-Type-Options "nosniff"
        Referrer-Policy "no-referrer"
        -Server
    }

    log {
        output file /data/access.log {
            roll_size 100mb
            roll_keep 5
        }
        format json
    }
}

{$GRAFANA_DOMAIN} {
    basic_auth {
        admin {$GRAFANA_BASICAUTH_HASH}
    }
    reverse_proxy grafana:3000
}
```

Sinh hash cho basic auth: `docker run --rm caddy:2-alpine caddy hash-password --plaintext '<mật khẩu>'`.

---

## 9. Tinh chỉnh ClickHouse & Kafka cho 1 máy

`deploy/clickhouse/config.d/10-memory.xml`:

```xml
<clickhouse>
    <max_server_memory_usage>19327352832</max_server_memory_usage> <!-- 18 GiB -->
    <max_concurrent_queries>32</max_concurrent_queries>
    <background_pool_size>8</background_pool_size>
    <background_merges_mutations_concurrency_ratio>2</background_merges_mutations_concurrency_ratio>
    <mark_cache_size>2147483648</mark_cache_size>            <!-- 2 GiB -->
    <uncompressed_cache_size>1073741824</uncompressed_cache_size>
    <logger>
        <level>warning</level>
        <size>200M</size>
        <count>3</count>
    </logger>
    <listen_host>0.0.0.0</listen_host>
    <prometheus>
        <endpoint>/metrics</endpoint>
        <port>9363</port>
        <metrics>true</metrics>
        <events>true</events>
        <asynchronous_metrics>true</asynchronous_metrics>
    </prometheus>
</clickhouse>
```

`deploy/clickhouse/users.d/10-profiles.xml`:

```xml
<clickhouse>
    <profiles>
        <default>
            <max_execution_time>15</max_execution_time>
            <max_memory_usage>4000000000</max_memory_usage>
            <max_bytes_before_external_group_by>2000000000</max_bytes_before_external_group_by>
            <max_result_rows>100000</max_result_rows>
            <readonly>0</readonly>
        </default>
        <dashboard>
            <max_execution_time>10</max_execution_time>
            <max_memory_usage>2000000000</max_memory_usage>
            <readonly>1</readonly>
        </dashboard>
    </profiles>
    <quotas>
        <dashboard_quota>
            <interval>
                <duration>3600</duration>
                <queries>10000</queries>
                <execution_time>1800</execution_time>
            </interval>
        </dashboard_quota>
    </quotas>
</clickhouse>
```

### Lưu ý riêng cho EBS

- gp3 mặc định **3000 IOPS / 125 MB/s** bất kể dung lượng. ClickHouse merge sẽ nghẹt → phải provision `iops = 6000, throughput = 250` (đã có trong Terraform). Chi phí thêm ~$25/tháng, xứng đáng.
- Theo dõi CloudWatch metric `VolumeQueueLength` (nếu > 10 liên tục là I/O đang là nút thắt) và `BurstBalance` (chỉ áp dụng gp2, không có với gp3).
- Không đặt WAL fallback (`/data/wal`) và ClickHouse trên cùng volume nếu có thể — nhưng với 1 máy thì chấp nhận, chỉ cần alert khi đĩa > 75%.
- **Khi mở rộng volume**: `terraform apply` đổi `data_volume_gb` → sau đó phải `xfs_growfs /data` trên máy, EBS không tự nới filesystem.

### Kafka trên cùng máy

- Heap 2GB là đủ; Kafka dựa vào page cache của OS. Đừng cấp heap lớn — nó ăn mất phần page cache mà ClickHouse cần.
- `log.retention.hours=168` với 6 partition ở 5k ev/s ≈ 250GB. **Cân nhắc giảm xuống 48h** trên máy này, hoặc dùng `log.retention.bytes` để chặn cứng.

---

## 10. CI/CD: GitHub Actions → ECR → SSM

### Biến cần đặt trong GitHub → Settings → Variables/Secrets

| Tên | Loại | Giá trị |
|---|---|---|
| `AWS_ROLE_ARN` | Variable | output `gha_role_arn` của Terraform |
| `AWS_REGION` | Variable | `ap-southeast-1` |
| `ECR_URL` | Variable | output `ecr_url` |
| `EC2_INSTANCE_ID` | Variable | output `instance_id` |
| `DEPLOY_BUCKET` | Variable | output `deploy_bucket` |

**Không cần `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`** — đó là điểm mấu chốt của OIDC.

### `.github/workflows/cd-production.yml`

```yaml
name: CD Production

on:
  push:
    tags: ['v*']
  workflow_dispatch:
    inputs:
      image_tag: { description: 'SHA hoac tag de deploy', required: true }

permissions:
  id-token: write      # bat buoc cho OIDC
  contents: read

concurrency:
  group: production
  cancel-in-progress: false    # KHONG huy deploy dang chay

env:
  AWS_REGION: ${{ vars.AWS_REGION }}

jobs:
  build:
    runs-on: ubuntu-latest
    outputs:
      image_tag: ${{ steps.meta.outputs.tag }}
    steps:
      - uses: actions/checkout@v4

      - id: meta
        run: echo "tag=${GITHUB_SHA::12}" >> "$GITHUB_OUTPUT"

      - uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ vars.AWS_ROLE_ARN }}
          aws-region: ${{ env.AWS_REGION }}

      - uses: aws-actions/amazon-ecr-login@v2

      - uses: docker/setup-qemu-action@v3
      - uses: docker/setup-buildx-action@v3

      - name: Build & push (arm64 cho Graviton)
        uses: docker/build-push-action@v6
        with:
          context: ./backend
          platforms: linux/arm64
          push: true
          tags: |
            ${{ vars.ECR_URL }}:${{ steps.meta.outputs.tag }}
            ${{ vars.ECR_URL }}:latest
          build-args: |
            VERSION=${{ github.ref_name }}
            COMMIT=${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

      - name: Upload deploy files len S3
        run: |
          aws s3 sync ./deploy "s3://${{ vars.DEPLOY_BUCKET }}/deploy" --delete
          aws s3 cp ./docker-compose.prod.yml "s3://${{ vars.DEPLOY_BUCKET }}/deploy/docker-compose.prod.yml"

  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment: production        # bat required reviewer neu muon duyet tay
    steps:
      - uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ vars.AWS_ROLE_ARN }}
          aws-region: ${{ env.AWS_REGION }}

      - name: Gui lenh deploy qua SSM
        id: ssm
        run: |
          CMD_ID=$(aws ssm send-command \
            --instance-ids "${{ vars.EC2_INSTANCE_ID }}" \
            --document-name "AWS-RunShellScript" \
            --comment "deploy ${{ needs.build.outputs.image_tag }}" \
            --timeout-seconds 900 \
            --parameters commands='[
              "set -euo pipefail",
              "cd /opt/pulse",
              "echo ${{ needs.build.outputs.image_tag }} > /opt/pulse/.image_tag_new",
              "cp -f /opt/pulse/.image_tag /opt/pulse/.image_tag_prev || true",
              "aws s3 sync s3://${{ vars.DEPLOY_BUCKET }}/deploy /opt/pulse --delete --exclude \".env\"",
              "aws ecr get-login-password --region ${{ env.AWS_REGION }} | docker login --username AWS --password-stdin ${{ vars.ECR_URL }}",
              "export IMAGE_TAG=${{ needs.build.outputs.image_tag }}",
              "export ECR_URL=${{ vars.ECR_URL }}",
              "docker compose -f docker-compose.prod.yml pull",
              "docker compose -f docker-compose.prod.yml run --rm migrate",
              "docker compose -f docker-compose.prod.yml up -d --no-deps ingest-api consumer analytics-api caddy",
              "mv /opt/pulse/.image_tag_new /opt/pulse/.image_tag",
              "docker image prune -af --filter until=168h"
            ]' \
            --query "Command.CommandId" --output text)
          echo "cmd_id=$CMD_ID" >> "$GITHUB_OUTPUT"

      - name: Cho SSM chay xong va in log
        run: |
          aws ssm wait command-executed \
            --command-id "${{ steps.ssm.outputs.cmd_id }}" \
            --instance-id "${{ vars.EC2_INSTANCE_ID }}" || true
          aws ssm get-command-invocation \
            --command-id "${{ steps.ssm.outputs.cmd_id }}" \
            --instance-id "${{ vars.EC2_INSTANCE_ID }}" \
            --query "{Status:Status,Out:StandardOutputContent,Err:StandardErrorContent}" \
            --output json | jq -r '.Out, .Err'
          STATUS=$(aws ssm get-command-invocation \
            --command-id "${{ steps.ssm.outputs.cmd_id }}" \
            --instance-id "${{ vars.EC2_INSTANCE_ID }}" --query Status --output text)
          [ "$STATUS" = "Success" ] || exit 1

      - name: Smoke test
        run: |
          for i in $(seq 1 30); do
            if curl -fsS "https://api.${{ vars.DOMAIN }}/healthz" >/dev/null; then
              echo "healthy"; exit 0
            fi
            sleep 3
          done
          echo "khong healthy sau 90s"; exit 1

      - name: Rollback neu smoke test fail
        if: failure()
        run: |
          aws ssm send-command --instance-ids "${{ vars.EC2_INSTANCE_ID }}" \
            --document-name "AWS-RunShellScript" \
            --parameters commands='[
              "cd /opt/pulse",
              "export IMAGE_TAG=$(cat .image_tag_prev)",
              "export ECR_URL=${{ vars.ECR_URL }}",
              "docker compose -f docker-compose.prod.yml up -d --no-deps ingest-api consumer analytics-api"
            ]'
```

### Điểm cần nhớ

- `permissions: id-token: write` — thiếu dòng này OIDC im lặng không hoạt động.
- `concurrency: cancel-in-progress: false` — hủy giữa chừng một deploy đang chạy migration là cách nhanh nhất để hỏng database.
- **Migration chạy trước khi đổi app**, và phải tương thích ngược một bước (xem `PLAN.md` §17.4).
- Rollback chỉ đổi lại image, **không rollback migration** — vì vậy migration phải luôn additive.
- File `.env` trên máy được loại khỏi `s3 sync --delete` để secret không bị xoá.

---

## 11. Vercel setup

### Import project

- Root Directory: `frontend`
- Framework: Next.js (tự nhận)
- Build Command / Output: để mặc định
- Region: **Singapore (sin1)** cho Serverless Functions

### Environment Variables

| Tên | Production | Preview | Ghi chú |
|---|---|---|---|
| `NEXT_PUBLIC_API_URL` | `https://api.pulse.dev` | `https://api-staging.pulse.dev` | Public — browser gọi thẳng |
| `NEXT_PUBLIC_SITE_ID` | `site_prod` | `site_staging` | |
| `AUTH_SECRET` | (random 32 byte) | | Chỉ server-side |

### `frontend/next.config.ts`

```ts
import type { NextConfig } from 'next'

const config: NextConfig = {
  output: 'standalone',
  poweredByHeader: false,
  async headers() {
    return [{
      source: '/(.*)',
      headers: [
        { key: 'X-Content-Type-Options', value: 'nosniff' },
        { key: 'Referrer-Policy', value: 'strict-origin-when-cross-origin' },
        {
          key: 'Content-Security-Policy',
          value: [
            "default-src 'self'",
            "script-src 'self' 'unsafe-inline'",
            "style-src 'self' 'unsafe-inline'",
            `connect-src 'self' ${process.env.NEXT_PUBLIC_API_URL}`,
            "img-src 'self' data:",
          ].join('; '),
        },
      ],
    }]
  },
}
export default config
```

> `connect-src` phải chứa domain API, nếu không browser chặn mọi fetch — lỗi này rất hay gặp và thông báo trong console không rõ ràng.

### `.github/workflows/ci-frontend.yml` (chỉ kiểm tra, không deploy)

```yaml
name: CI Frontend
on:
  pull_request: { paths: ['frontend/**'] }
jobs:
  check:
    runs-on: ubuntu-latest
    defaults: { run: { working-directory: frontend } }
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: 22, cache: npm, cache-dependency-path: frontend/package-lock.json }
      - run: npm ci
      - run: npm run lint
      - run: npx tsc --noEmit
      - run: npm run test -- --run
      - run: npm run build
```

Việc deploy do Vercel Git integration lo — mỗi PR tự có preview URL. Trong Vercel → Settings → Git, đặt **Ignored Build Step** để không build khi PR chỉ đụng `backend/`:

```bash
git diff --quiet HEAD^ HEAD -- ./frontend || exit 1
```

---

## 12. CORS & authentication

Vercel **không có dải IP cố định** (trừ Enterprise), nên không thể whitelist bằng Security Group. Bảo mật phải nằm ở tầng ứng dụng.

### CORS bên Go

```go
func CORS(cfg *config.Config) gin.HandlerFunc {
    allowed := map[string]bool{}
    for _, o := range cfg.CORSAllowedOrigins { // "https://app.pulse.dev"
        allowed[o] = true
    }
    previewRe := regexp.MustCompile(`^https://pulse-analytics-[a-z0-9-]+\.vercel\.app$`)

    return func(c *gin.Context) {
        origin := c.GetHeader("Origin")
        ok := allowed[origin] ||
            (cfg.AllowVercelPreviews && previewRe.MatchString(origin))

        if ok {
            c.Header("Access-Control-Allow-Origin", origin)  // echo lại, KHÔNG dùng "*"
            c.Header("Vary", "Origin")
            c.Header("Access-Control-Allow-Credentials", "true")
            c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
            c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
            c.Header("Access-Control-Max-Age", "86400")
        }
        if c.Request.Method == http.MethodOptions {
            c.AbortWithStatus(http.StatusNoContent)
            return
        }
        c.Next()
    }
}
```

Ba điểm dễ sai:

1. `Access-Control-Allow-Origin: *` **không dùng được** cùng `Allow-Credentials: true`. Phải echo lại origin cụ thể.
2. Thiếu header `Vary: Origin` → CDN/Cloudflare cache nhầm response CORS của origin này cho origin khác.
3. Regex preview `*.vercel.app` chỉ nên bật ở staging (`AllowVercelPreviews=false` ở production) — nếu không, bất kỳ ai deploy lên Vercel cũng gọi được API của bạn.

### Endpoint ingest thì khác

`/collect` cần cho phép origin của **website khách hàng** (bất kỳ site nào đã đăng ký), không phải chỉ dashboard. Tra `site_id` từ API key → lấy danh sách origin đã đăng ký của site đó → so khớp. Nếu site chưa cấu hình origin thì cho `*` nhưng **không** kèm credentials.

### Auth dashboard

- Login ở Next.js Route Handler (server-side) → gọi `analytics-api` → nhận JWT → set cookie `httpOnly; Secure; SameSite=Lax; Domain=.pulse.dev`.
- Domain cookie phải là `.pulse.dev` để `app.pulse.dev` gửi được cookie sang `api.pulse.dev`. **Vercel preview `*.vercel.app` sẽ không nhận được cookie này** — ở preview dùng token trong header thay vì cookie.

---

## 13. Backup & restore

| Lớp | Cơ chế | Tần suất | Giữ |
|---|---|---|---|
| Volume | EBS snapshot qua DLM (đã có Terraform) | Hằng ngày 03:00 ICT | 7 bản |
| Logic | `BACKUP TABLE analytics.events TO S3(...)` | Hằng tuần | 4 bản |
| Cấu hình | Trong Git | Mỗi commit | ∞ |
| Secret | `.env` trên máy — sao lưu thủ công vào password manager | Khi đổi | |

### Backup logic ClickHouse ra S3

```sql
BACKUP DATABASE analytics
TO S3('https://pulse-backup-<suffix>.s3.ap-southeast-1.amazonaws.com/weekly/{date}', '<key>', '<secret>')
SETTINGS compression_method = 'zstd';
```

Ưu điểm so với snapshot: khôi phục được **một bảng** thay vì cả volume.

### Diễn tập restore (bắt buộc làm ít nhất 1 lần)

```bash
# 1. Tạo volume mới từ snapshot
aws ec2 create-volume --snapshot-id snap-xxx --availability-zone ap-southeast-1a --volume-type gp3

# 2. Attach vào 1 instance tạm, mount /mnt/restore
# 3. Đối chiếu
docker run --rm -v /mnt/restore/clickhouse:/var/lib/clickhouse clickhouse/clickhouse-server:26.3 \
  clickhouse-local --query "SELECT count() FROM ..."

# 4. Ghi lại RTO thực tế vào docs/runbook.md
```

> Backup chưa từng được restore thì chưa phải backup. Ghi lại **thời gian khôi phục thực tế (RTO)** — con số này mới có giá trị.

---

## 14. Giám sát chi phí

### Ước tính ap-southeast-1, on-demand

| Khoản | Đơn giá | Tháng |
|---|---|---|
| `r7g.xlarge` (4 vCPU / 32GB) | ~$0.238/h | ~$174 |
| EBS gp3 500GB | $0.096/GB | ~$48 |
| EBS IOPS thêm (6000 − 3000) | ~$0.006/IOPS | ~$18 |
| EBS throughput thêm (250 − 125) | ~$0.048/MBps | ~$6 |
| EBS snapshot 7 bản (~150GB) | $0.05/GB | ~$8 |
| Elastic IP (đang gắn máy) | $0.005/h | ~$3.6 |
| Data transfer out ~50GB | $0.12/GB | ~$6 |
| ECR + S3 + Route53 | | ~$3 |
| Vercel Hobby | | $0 |
| **Tổng** | | **~$267** |

Giá là ước tính và thay đổi theo thời điểm — kiểm tra lại bằng [AWS Pricing Calculator](https://calculator.aws) trước khi tạo.

### Cách giảm chi phí

| Cách | Tiết kiệm | Đánh đổi |
|---|---|---|
| Compute Savings Plan 1 năm, trả trước một phần | ~30% | Cam kết 1 năm |
| `r7g.large` (2 vCPU/16GB) ở giai đoạn L0–L2 | ~$87/tháng | Không đủ cho benchmark 100M |
| **Stop instance ngoài giờ học** (EventBridge Scheduler) | ~60% | EBS vẫn tính tiền; Elastic IP khi không gắn máy bị tính $0.005/h |
| Giảm EBS còn 200GB đến khi cần | ~$29/tháng | Phải nới sau, nhớ `xfs_growfs` |
| Spot Instance | ~70% | **Không dùng** — bị thu hồi giữa lúc merge là hỏng dữ liệu |

Script stop/start theo lịch:

```hcl
resource "aws_scheduler_schedule" "stop_night" {
  name                = "pulse-stop-2300-ict"
  schedule_expression = "cron(0 16 * * ? *)"   # 16:00 UTC = 23:00 ICT
  flexible_time_window { mode = "OFF" }
  target {
    arn      = "arn:aws:scheduler:::aws-sdk:ec2:stopInstances"
    role_arn = aws_iam_role.scheduler.arn
    input    = jsonencode({ InstanceIds = [aws_instance.app.id] })
  }
}
```

> Cảnh báo: `stop` instance **không** làm mất dữ liệu trên EBS (khác instance store), nhưng Kafka và ClickHouse cần vài phút để phục hồi sau khi start. Đừng stop giữa lúc đang chạy benchmark dài.

---

## 15. Runbook sự cố đặc thù AWS

| Triệu chứng | Nguyên nhân thường gặp | Xử lý |
|---|---|---|
| Insert chậm dần, `VolumeQueueLength` > 10 | gp3 chưa provision IOPS | Tăng `data_volume_iops` lên 9000, `terraform apply` (nới nóng, không cần restart) |
| ClickHouse bị OOM-kill | `max_server_memory_usage` cao hơn limit docker | Đặt limit docker cao hơn `max_server_memory_usage` ~10% |
| `terraform apply` muốn thay instance | AWS ra AMI mới | Đã có `ignore_changes = [ami]`; nếu vẫn xảy ra, kiểm tra `user_data_replace_on_change` |
| Không SSM vào được | SSM agent chết, hoặc thiếu IAM policy, hoặc không có route ra Internet | `aws ssm describe-instance-information` để kiểm; khởi động lại agent qua EC2 Serial Console |
| GitHub Actions "Not authorized to perform sts:AssumeRoleWithWebIdentity" | Sai `sub` trong trust policy, hoặc thiếu `id-token: write` | Kiểm tra `repo:<owner>/<repo>:*` khớp chính xác |
| Caddy không lấy được chứng chỉ | DNS chưa trỏ đúng, hoặc port 80 bị chặn | `dig api.pulse.dev`; kiểm SG có rule 80; xem log `docker compose logs caddy` |
| Đĩa đầy | Kafka retention 7 ngày + ClickHouse + docker log | Giảm `KAFKA_LOG_RETENTION_HOURS` xuống 48; `docker system prune -af`; kiểm tra `/data/caddy/access.log` |
| Nới EBS xong dung lượng không tăng | Chưa `xfs_growfs` | `sudo xfs_growfs /data` |
| Hóa đơn nhảy vọt | Data transfer out, hoặc snapshot tích tụ, hoặc quên tắt instance benchmark | Cost Explorer → group by Usage Type |

### Truy cập ClickHouse để debug

```bash
# Mở shell trên máy
aws ssm start-session --target i-xxxxx

# Hoặc port-forward ClickHouse HTTP về máy local (không cần mở SG)
aws ssm start-session --target i-xxxxx \
  --document-name AWS-StartPortForwardingSession \
  --parameters '{"portNumber":["8123"],"localPortNumber":["8123"]}'
# rồi: curl localhost:8123/ping
```

---

## 16. Teardown

Khi học xong, dọn sạch để không bị tính tiền âm thầm:

```bash
# 1. Backup lần cuối
aws ec2 create-snapshot --volume-id vol-xxx --description "final-before-teardown"

# 2. Gỡ prevent_destroy trong compute.tf (sửa tay), rồi:
cd infra && terraform destroy

# 3. Kiểm tra sót
aws ec2 describe-volumes --filters Name=status,Values=available    # volume mồ côi vẫn tính tiền
aws ec2 describe-addresses                                          # EIP không gắn máy tính $0.005/h
aws ec2 describe-snapshots --owner-ids self
aws ecr describe-repositories
aws s3 ls
```

> Volume "available" (không gắn máy) và Elastic IP không dùng là 2 khoản tính tiền âm thầm phổ biến nhất. Luôn kiểm 2 thứ này sau khi destroy.

---

## 17. Checklist thay thế L6.4

> Thay cho `TODO.md` L6-20 → L6-28. Ước lượng: **14h**.

### AWS.1 — Chuẩn bị tài khoản (2h)

- [ ] `AWS-01` Bật MFA root, tạo IAM user thao tác tay, cài AWS CLI v2
- [ ] `AWS-02` Tạo AWS Budget $300 + 3 mức alert + Cost Anomaly Detection — **làm trước mọi thứ khác**
- [ ] `AWS-03` Đăng ký domain, tạo Route53 hosted zone, trỏ nameserver
- [ ] `AWS-04` Bootstrap S3 bucket + DynamoDB table cho Terraform state

### AWS.2 — Terraform (5h)

- [ ] `AWS-05` Viết `infra/` theo §5; `terraform fmt` + `terraform validate`
- [ ] `AWS-06` `terraform plan` — đọc kỹ, xác nhận không có gì bất ngờ
- [ ] `AWS-07` `terraform apply`; lưu outputs
- [ ] `AWS-08` Xác minh `prevent_destroy` trên EBS data và `ignore_changes = [ami]` có hiệu lực (thử `terraform plan` lần 2 phải "No changes")
- [ ] `AWS-09` `aws ssm start-session` vào được, không cần SSH key
- [ ] `AWS-10` Kiểm tra `/data` đã mount, `lsblk` + `df -h` đúng dung lượng
- [ ] `AWS-11` Kiểm tra sysctl và THP đã áp: `cat /sys/kernel/mm/transparent_hugepage/enabled`

### AWS.3 — Chạy stack (3h)

- [ ] `AWS-12` Tạo `.env` trên máy (chmod 600), sinh `CLICKHOUSE_PASSWORD`, `GRAFANA_PASSWORD`, `JWT_SECRET`
- [ ] `AWS-13` Upload `deploy/` lên S3, sync về `/opt/pulse`
- [ ] `AWS-14` `docker compose -f docker-compose.prod.yml up -d`; tất cả healthy
- [ ] `AWS-15` Caddy lấy được chứng chỉ TLS cho `api.*` và `grafana.*`
- [ ] `AWS-16` `curl https://api.pulse.dev/healthz` trả 200 từ máy ngoài
- [ ] `AWS-17` Xác nhận **không** truy cập được `:8123`, `:9000`, `:9092` từ Internet (`nmap` từ máy khác)

### AWS.4 — CI/CD (2h)

- [ ] `AWS-18` Đặt GitHub Variables theo §10
- [ ] `AWS-19` Chạy `cd-production.yml` bằng `workflow_dispatch`, xem log SSM
- [ ] `AWS-20` Cố tình deploy image hỏng → xác nhận smoke test fail và rollback chạy
- [ ] `AWS-21` Bật `environment: production` với required reviewer (tuỳ chọn)

### AWS.5 — Vercel (1h)

- [ ] `AWS-22` Import project, root `frontend`, region `sin1`
- [ ] `AWS-23` Đặt env vars; deploy production; trỏ `app.pulse.dev`
- [ ] `AWS-24` Cấu hình Ignored Build Step để bỏ qua PR chỉ đụng backend
- [ ] `AWS-25` Kiểm tra CORS: mở dashboard, xem Network tab không có lỗi CORS/CSP
- [ ] `AWS-26` Mở 1 preview deployment, xác nhận hành vi CORS đúng như thiết kế (chặn ở prod, cho phép ở staging)

### AWS.6 — Vận hành (1h)

- [ ] `AWS-27` Xác nhận DLM đã tạo snapshot đầu tiên
- [ ] `AWS-28` **Diễn tập restore** từ snapshot, ghi RTO thực tế vào `docs/runbook.md`
- [ ] `AWS-29` CloudWatch alarm: CPU > 85% 15 phút, `VolumeQueueLength` > 10, disk `/data` > 75%
- [ ] `AWS-30` Uptime check ngoài (healthchecks.io) cho `https://api.pulse.dev/healthz`
- [ ] `AWS-31` Ghi vào `benchmark-results.md`: **cấu hình all-in-one, Kafka/Go dùng chung CPU với ClickHouse** — để người đọc hiểu số liệu là bi quan
- [ ] `AWS-32` Đặt lịch nhắc kiểm tra hóa đơn hằng tuần trong tháng đầu

---

*Tài liệu này thay thế phần deploy trong `PLAN.md` §17.4–17.5 và `TODO.md` L6.4.*
