# ADR 0004 — Configuration Module trong Injel

- **Trạng thái:** Accepted
- **Ngày:** 29/10/2026
- **Người quyết định:** Nhóm phát triển Injel
- **Phạm vi:** Configuration package (`config`) & Injel facade

## Bối cảnh

Mọi ứng dụng backend hiện đại (theo nguyên tắc Twelve-Factor App) đều cần cấu hình linh hoạt thông qua biến môi trường (Environment Variables): cổng mạng, chuỗi kết nối cơ sở dữ liệu, secret key, timeout, debug flag,...

Nếu không có cơ chế quản lý cấu hình có cấu trúc:
1. Code đọc `os.Getenv` bị rải rác khắp nơi, dễ quên xử lý giá trị mặc định (`default`) hoặc quên kiểm tra biến bắt buộc (`required`).
2. Việc ép kiểu thủ công (`strconv.Atoi`, `time.ParseDuration`,...) tạo nhiều boilerplate lặp lại và dễ gây lỗi runtime mơ hồ.
3. Lỗi cấu hình (thiếu biến, sai định dạng) thường chỉ phát hiện muộn khi component thực sự sử dụng tới biến đó trong quá trình chạy request.
4. Kiểm thử gặp khó khăn nếu cơ chế đọc biến môi trường bị gắn chặt vào `os.Getenv` của máy host.

## Các phương án đã cân nhắc

### Phương án 1 — Sử dụng thư viện ngoài (Viper, envconfig, cleanenv)
- **Ưu điểm:** Có sẵn nhiều tính năng.
- **Nhược điểm:** Kéo theo nhiều dependency ngoài, tăng kích thước binary và rủi ro chuỗi cung ứng; không gắn kết tự nhiên với triết lý thiết kế core nhỏ của Injel (Nguyên tắc 5: *"Core nhỏ, module rời. Core không phụ thuộc thư viện ngoài"*).

### Phương án 2 — Đọc file cấu hình phức tạp (YAML, TOML, JSON) kết hợp remote config
- **Nhược điểm:** Quá phức tạp cho giai đoạn hiện tại. File configuration thường yêu cầu parser nặng và tạo ra nhiều tầng ưu tiên (precedence) phức tạp khi kết hợp với biến môi trường.

### Phương án 3 — Module Configuration dựa trên Struct Tag & Thư viện chuẩn Go (Được chọn)
Sử dụng Go reflection và struct tag `env` với thư viện chuẩn Go:
- Struct tag khai báo tường minh: `env:"KEY,opt1,opt2..."`.
- Không thêm dependency bên ngoài nào.
- Hỗ trợ hàm tra cứu tùy chỉnh (`LookupFunc`) phục vụ test isolation.
- Tích hợp mượt mà vào Injel DI container dưới dạng Provider.

## Thiết kế chi tiết

### 1. Cú pháp Struct Tag `env`

Tag có dạng:
```go
`env:"KEY_NAME[,options]"`
```

Các tùy chọn:
- `required`: Biến môi trường bắt buộc phải tồn tại và không được rỗng. Nếu thiếu hoặc rỗng, trả về `MissingRequiredError`.
- `default=VALUE`: Giá trị mặc định được sử dụng khi biến môi trường không tồn tại.
  - Ví dụ: `env:"PORT,default=8080"`
  - Ví dụ: `env:"HOST,default=127.0.0.1"`
  - Ví dụ: `env:"TIMEOUT,default=5s"`

### 2. Phân biệt biến không tồn tại và biến rỗng

- **Biến không tồn tại (`!exists`):**
  - Nếu có `required`: Báo lỗi `MissingRequiredError`.
  - Nếu có `default`: Gán giá trị mặc định.
  - Nếu không có cả hai: Giữ nguyên giá trị zero của struct field.
- **Biến tồn tại nhưng mang chuỗi rỗng (`exists && val == ""`):**
  - Nếu có `required`: Báo lỗi `MissingRequiredError` (biến bắt buộc không được rỗng).
  - Nếu không `required`: Coi `""` là giá trị người dùng chủ ý thiết lập; chuyển đổi `""` sang kiểu tương ứng (chuỗi rỗng cho string, hoặc báo lỗi nếu kiểu số không chấp nhận chuỗi rỗng).

### 3. Các kiểu dữ liệu hỗ trợ chuyển đổi

1. **Chuỗi:** `string`
2. **Boolean:** `bool` (`1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False`)
3. **Số nguyên:** `int, int8, int16, int32, int64`
4. **Số nguyên không dấu:** `uint, uint8, uint16, uint32, uint64`
5. **Số thực:** `float32, float64`
6. **Thời gian:** `time.Duration` (ví dụ `10s`, `5m`, `500ms`, `1h`)
7. **Danh sách (Slice):** `[]string`, `[]int`, `[]int64`, `[]bool`, `[]float64` (phân tách bởi dấu phẩy `,`)
8. **Kiểu tùy chỉnh:** Bất kỳ kiểu nào hiện thực `encoding.TextUnmarshaler` (`UnmarshalText([]byte) error`).
9. **Struct lồng nhau:** Hỗ trợ struct lồng (nested struct) để tổ chức cấu hình phân cấp.

### 4. Hệ thống Lỗi có Ngữ cảnh

Các loại lỗi cụ thể trong package `config`:
- `MissingRequiredError`: Tên biến môi trường, tên struct field, nguyên nhân.
- `ConversionError`: Tên biến môi trường, giá trị thô, kiểu dữ liệu mục tiêu, struct field, lỗi gốc (`Unwrap`).
- `InvalidTagError`: Tên struct field, nội dung tag sai cú pháp.
- `UnsupportedTypeError`: Tên struct field, kiểu dữ liệu không được hỗ trợ.
- `InvalidTargetError`: Đối tượng nạp cấu hình không phải là con trỏ struct hợp lệ.

### 5. Khả năng Test Độc lập (Isolation)

Cung cấp interface/functional option:
```go
type LookupFunc func(key string) (string, bool)

func WithLookup(fn LookupFunc) Option
func WithMap(envMap map[string]string) Option
```
Mặc định sử dụng `os.LookupEnv`. Khi chạy test, có thể truyền mock map mà không chạm vào môi trường hệ điều hành thật.

### 6. Tích hợp với Injel DI

Cung cấp hàm tiện ích trong `injel`:
```go
// ProvideConfig nạp cấu hình kiểu T và đăng ký singleton vào DI container.
func ProvideConfig[T any](app *App, opts ...config.Option) error
```
Khi `app.Start(ctx)` được gọi:
- Cấu hình được nạp và kiểm tra tính hợp lệ ngay lập tức.
- Nếu có lỗi thiếu biến hay sai kiểu, ứng dụng dừng và chuyển sang `StateFailed` trước khi phục vụ request.
- Các service/repository phụ thuộc vào `Config` nhận được singleton hợp lệ thông qua DI injection chuẩn.
