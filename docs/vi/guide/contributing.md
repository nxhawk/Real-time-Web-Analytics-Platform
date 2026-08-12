# Đóng góp

Bản đầy đủ nằm ở
[`CONTRIBUTING.md`](https://github.com/nxhawk/Real-time-Web-Analytics-Platform/blob/main/CONTRIBUTING.md).
Trang này là bản rút gọn cộng phần riêng cho tài liệu.

## Luật ngôn ngữ

**Mọi thứ viết vào codebase đều bằng tiếng Anh** — code, comment, tên định danh, log, chuỗi
lỗi, commit message, nội dung pull request, test.

Bốn tài liệu kế hoạch (`PLAN.md`, `PHASES.md`, `TODO.md`, `DEPLOY-AWS.md`) viết bằng tiếng Việt
và giữ nguyên như vậy. Khi hiện thực một task mô tả bằng tiếng Việt, hãy dịch **ý** sang code
tiếng Anh:

```go
// GOOD: Flush the buffer when it is full or the flush interval elapses.
// BAD:  Đẩy buffer khi đầy hoặc hết thời gian flush.
```

Riêng trang tài liệu này là song ngữ: tiếng Anh mặc định, tiếng Việt là bản song hành.

## Nhánh và commit

`main` được bảo vệ — chỉ vào bằng pull request, CI phải xanh, cấm force push.

Tên nhánh: `feat/<slug>`, `fix/<slug>`, `chore/<slug>`, `docs/<slug>`.

Commit theo [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(ingest): accept batches of up to 500 events
fix(clickhouse): map error 252 to a domain error
docs(guide): document the readiness prober
```

Ghi task id từ `TODO.md` vào nội dung pull request — `Closes L1-17` — để checklist và lịch sử
git không lệch nhau.

## Trước khi mở pull request

```bash
make check
```

Đó là gofmt, `go vet`, golangci-lint và test có race detector: đúng những gì CI chạy. Thêm hoặc
sửa test cho mọi thay đổi hành vi.

## Quy trình làm một task

1. Đọc task trong `TODO.md` và phase của nó trong `PHASES.md` — điều kiện vào, điều kiện ra,
   deliverable.
2. Đọc mục `PLAN.md` được tham chiếu **trước khi** viết code. DDL, hình dạng query và contract
   API đã được quyết ở đó rồi.
3. Hiện thực, tuân thủ [luật phân tầng](/vi/guide/project-structure#luật-phân-tầng).
4. Test. `make check` phải sạch.
5. Tick `[x]` trong `TODO.md`, và sửa tài liệu nào bị hiện thực phản bác.

## Các con số dùng chung

Giá trị xuất hiện ở nhiều tài liệu — version công cụ, ngưỡng hiệu năng, hạn mức API, phân phối
seeder — thuộc quyền sở hữu của `PHASES.md` §2. Sửa ở đó trước, rồi lan sang `PLAN.md`,
`README.md`, `.env.example`, trang này và code. Không bao giờ sửa lẻ một chỗ.

## Tài liệu

Trang này dùng [VitePress](https://vitepress.dev). Nội dung nằm trong `docs/`.

```bash
cd docs
npm ci
npm run dev      # http://localhost:5173, hot reload
npm run build    # build production; fail nếu có link chết
```

### Trang mới đặt ở đâu

| Thư mục | Nội dung |
|---|---|
| `docs/guide/` | Diễn giải: làm việc gì đó thế nào, cái gì đó hoạt động ra sao |
| `docs/reference/` | Tra cứu: endpoint, schema, tên metric |
| `docs/notes/` | Số đo và kiến thức vận hành do một level sinh ra |
| `docs/adr/` | Mỗi file một quyết định: bối cảnh, quyết định, hệ quả, phương án khác |
| `docs/vi/**` | Bản tiếng Việt — cùng cây thư mục, cùng tên file |

### Thêm một trang

1. Tạo trang tiếng Anh, ví dụ `docs/guide/caching.md`.
2. Tạo bản tiếng Việt tại `docs/vi/guide/caching.md`.
3. Thêm cả hai vào sidebar: `docs/.vitepress/config/en.mts` và `.../vi.mts`.
4. Chạy `npm run build` — link chết sẽ làm fail build.

Một trang chỉ có ở một ngôn ngữ là chấp nhận được khi bản dịch còn đang chờ; thêm một dòng ghi
chú ở đầu trỏ sang ngôn ngữ kia. Thứ **không** chấp nhận được là mục sidebar trỏ tới file không
tồn tại — cái đó làm vỡ build.

### Phong cách viết

- Đưa câu trả lời trước, giải thích sau.
- Ưu tiên bảng thay vì danh sách gạch đầu dòng dạng cặp.
- Mọi code block phải chạy được, hoặc ghi rõ là trích đoạn.
- Đánh dấu tính năng chưa có bằng badge: `<Badge type="warning" text="Level 3" />`.
- Link tới `PLAN.md` cho phần đặc tả thay vì chép lại — chép lại thì sẽ lệch.

### Triển khai

Push lên `main` có thay đổi trong `docs/` sẽ kích hoạt `.github/workflows/docs.yml`, build trang
và đẩy lên GitHub Pages. Pull request cũng build nhưng không deploy — nhờ vậy build hỏng bị bắt
trước khi merge.
