# ADR-0011 — Pipeline validate tự viết, không dùng `go-playground/validator`

**Trạng thái:** Chấp nhận · **Ngày:** 2026-08-18

## Bối cảnh

`PLAN.md` §3 chốt `go-playground/validator/v10` làm thư viện validate. Khi bắt tay viết các
quy tắc ở §5.2 thì lộ ra: chỉ một phần trong đó là "validate" theo nghĩa của thư viện này.

Đọc lại bảng quy tắc, nó tách làm hai nhóm:

| Quy tắc | Hành vi khi vi phạm |
|---|---|
| `event` không khớp `^[a-z0-9_]{1,64}$` | **Loại** event đó |
| `properties` không phải object, hoặc quá 8 KB | **Loại** event đó |
| `revenue` không vừa `Decimal(18, 4)` | **Loại** event đó |
| `user_id` dài quá 128 ký tự | **Cắt bớt** |
| `page` có query param `token` | **Strip** param đó |
| `timestamp` lệch 40 giờ về tương lai | **Ghi đè** bằng `now()` |
| `device` là `"smart_fridge"` | **Chuẩn hoá** về `unknown` |

Chỉ nhóm đầu là một vị từ đúng/sai. Nhóm sau *sửa giá trị rồi giữ lại event* — đây là quyết
định sản phẩm có chủ đích: thiết bị sai giờ là chuyện thường gặp, và traffic của nó là thật.
Validate bằng struct tag không diễn đạt được một phép sửa: validator trả về phán quyết, nó
không ghi lại struct mà nó nhận.

Ràng buộc thứ ba: partial success. Batch 100 event có 3 event hỏng phải nhận 97 và trả về 3
lỗi **kèm index**. Điều đó buộc mỗi phần tử phải được decode riêng, và đó là chuyện của cách
parse request chứ không phải của bất kỳ validator nào.

## Quyết định

**Tự viết quy tắc, không dùng thư viện validate.**

`internal/validate` là một danh sách quy tắc có thứ tự. Mỗi quy tắc phụ trách vài trường, ghi
chúng vào `model.ValidatedEvent`, rồi trả về `ReasonNone` hoặc lý do nó không làm được:

```go
var eventRules = []eventRule{
    {"event_name", ruleEventName},
    {"event_id",   ruleEventID},
    {"timestamp",  ruleTimestamp},
    // ...
}
```

Ba hệ quả đi kèm với hình dạng này:

- **`model.Event` → `model.ValidatedEvent`.** Chỉ package này tạo ra kiểu thứ hai, nên một
  event chưa kiểm tra không thể tới được repository. Bảo đảm nằm ở kiểu dữ liệu, không nằm ở
  quy ước ai đó phải nhớ.
- **`IngestRequest.Events` là `[]json.RawMessage`.** Một phần tử hỏng chỉ tốn đúng một index.
- **Các phép sửa được báo qua một `Observer`,** không phải qua biến đếm ở mức package, nên
  package không giữ trạng thái và `internal/metrics` cung cấp bản Prometheus mà hai package
  không phải import lẫn nhau.

`PLAN.md` §3 đã được sửa trong cùng thay đổi này. Cấu trúc thu được, và việc phải làm khi cần
thêm một rule, nằm ở [hướng dẫn pipeline validate](/vi/guide/validation).

## Hệ quả

**Tốt**

- Phép sửa và phép loại nằm cạnh nhau và đọc như cùng một loại việc — đúng bản chất của chúng.
- Không reflection trên hot path. Mục tiêu ingest là 10.000 event/s, mà validator theo tag
  phải duyệt struct bằng `reflect` cho từng event một.
- Thêm một quy tắc = một hàm cộng một dòng. Bảng test duyệt chính danh sách đó và fail nếu có
  quy tắc không có case, nên registry không thể mọc thêm bước mà không ai kiểm.
- Mọi ngưỡng là một field của `validate.Limits`, nên test thu nhỏ ngưỡng thay vì phải dựng một
  chuỗi 8 KB, và ngưỡng theo gói dịch vụ sau này là một giá trị chứ không phải một lần viết
  lại.

**Xấu**

- Nhiều code hơn struct tag. Khoảng 250 dòng quy tắc so với chừng 20 dòng tag — dù tag cũng
  chỉ phủ được chừng một nửa bảng.
- Quy tắc là của mình, mình tự chịu trách nhiệm đúng sai. Thư viện thì đã có người khác vấp
  các ca biên trước.
- Hai kiểu dữ liệu thay vì một, nên thêm một trường vào payload phải đi qua cả hai.

**Trung tính**

- `go-playground/validator/v10` vẫn là dependency gián tiếp qua phần binding của Gin. Quyết
  định này nói về đường ingest, không phải về việc gỡ nó khỏi module graph.

## Phương án đã cân nhắc

**Dùng `go-playground/validator/v10` cho nhóm quy tắc "loại", tự viết phần còn lại.** Bám sát
`PLAN.md` §3 theo đúng chữ. Bị loại vì nó xé một bảng quy tắc ra hai cơ chế: người đọc muốn
biết "`user_id` quá dài thì sao?" phải đoán xem nên tìm ở nửa nào, và mã lỗi sẽ đến từ hai
nguồn khác nhau.

**Mỗi quy tắc một struct tag, viết custom validator cho mọi thứ.** Có thể đăng ký custom
validator cho nhóm "loại", nhưng một validator đi cắt chuỗi đầu vào là một validator nói dối
về việc nó làm. Nó cũng biến lý do trả cho client thành một câu thông báo có định dạng thay vì
một tập đóng — mà tập đó đang là label của một metric Prometheus.

**`ozzo-validation` hoặc thư viện kiểu builder khác.** Gần hình dạng đúng hơn — quy tắc là giá
trị, không phải tag — nhưng vẫn mô hình hoá quy tắc như một vị từ, nên phần sửa giá trị vẫn
phải nằm ngoài nó. Lợi ích còn lại chỉ là một cách viết tám câu `if` cho đẹp hơn.
