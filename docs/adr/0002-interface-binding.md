# ADR 0002 — Interface Binding trong Injel

- **Trạng thái:** Accepted
- **Ngày:** 28/10/2026
- **Tác giả & Project Maintainer:** Phạm Trương Thông (@TogPam)
- **Người quyết định:** Nhóm phát triển Injel
- **Phạm vi:** DI core & Application facade

## Bối cảnh

Trong các ứng dụng Go có kiến trúc phân tầng (Clean Architecture / Hexagonal Architecture), các tầng nghiệp vụ (Service/Use Case) thường phụ thuộc vào các interface (ví dụ `UserStore`, `Mailer`, `Logger`) thay vì phụ thuộc trực tiếp vào concrete struct (`*SQLStore`, `*SMTPMailer`).

Hiện tại, Injel hỗ trợ trường hợp provider trả về trực tiếp interface:
```go
func NewUserStore() UserStore { return &SQLStore{} }
```
Tuy nhiên, trong thực tế, các provider thường trả về concrete type:
```go
func NewSQLStore(db *sql.DB) *SQLStore { return &SQLStore{db: db} }
```
Khi đó, nếu một consumer khai báo:
```go
func NewUserService(store UserStore) *UserService
```
Container chỉ lưu trữ kiểu `*SQLStore` và sẽ báo lỗi `MissingDependencyError` cho kiểu `UserStore`. Đồng thời, nhiều trường hợp cần cả consumer nhận `UserStore` và consumer khác nhận `*SQLStore` cùng trỏ về một singleton instance duy nhất.

## Các phương án đã cân nhắc

### Phương án 1 — Tự động suy diễn (Implicit / Auto-binding)
Container tự động quét toàn bộ các provider đã đăng ký, nếu thấy kiểu nào triển khai interface cần tìm thì tự động inject.

- **Ưu điểm:** Tiện lợi, không cần đăng ký thêm.
- **Nhược điểm:**
  - Vi phạm nguyên tắc thiết kế 3 của Injel ("Không giấu luồng chạy").
  - Gây nhập nhằng và không xác định (non-deterministic) khi có nhiều hơn một concrete type triển khai cùng một interface (ví dụ `MemoryStore` và `SQLStore`).
  - Lỗi chỉ xuất hiện khi phát sinh thêm struct thứ hai triển khai interface.

### Phương án 2 — Đăng ký tường minh (Explicit Binding via ProvideAs / Bind)
Cung cấp API đăng ký tường minh để liên kết một interface mục tiêu với một implementation concrete.

- **Ưu điểm:**
  - Hoàn toàn xác định, minh bạch và dễ đọc.
  - Kiểm tra hợp lệ ngay khi đăng ký: target phải là interface, source phải implement target (`AssignableTo`).
  - Đảm bảo singleton instance được dùng chung giữa interface và concrete type.
  - Báo lỗi trùng lặp (`DuplicateProviderError`) nếu interface đã có provider hoặc binding khác.
- **Nhược điểm:**
  - Lập trình viên phải gọi thêm một dòng đăng ký hoặc dùng `ProvideAs`.

### Phương án 3 — Chỉ cho phép provider trả về interface
Bắt buộc người dùng phải viết provider trả về interface nếu muốn inject interface.

- **Ưu điểm:** Không cần thêm cơ chế binding trong container.
- **Nhược điểm:** Không thể lấy lại concrete type từ container nếu cần thiết lập mở rộng, và làm giảm tính linh hoạt khi tái sử dụng các thư viện bên ngoài có sẵn constructor trả về struct pointer.

## Quyết định

Chọn **Phương án 2** kết hợp với **Phương án 3**.

1. Injel tiếp tục hỗ trợ provider trả về interface trực tiếp.
2. Cung cấp API đăng ký tường minh:
   - `c.ProvideAs(fn, (*Interface)(nil))` và generic `di.ProvideAs[Interface](c, fn)`: Đăng ký provider và liên kết kết quả vào interface.
   - `c.Bind((*Interface)(nil), (*Concrete)(nil))` và generic `di.Bind[Interface, Concrete](c)`: Liên kết một interface tới một concrete type đã/sẽ được đăng ký provider.
   - Các facade tương ứng trên `injel.App`: `app.ProvideAs(...)`, `injel.ProvideAs[Interface](app, fn)`, `injel.Bind[Interface, Concrete](app)`.

### Quy tắc kiểm tra tính hợp lệ
- Target bắt buộc phải là `reflect.Interface`. Nếu không, trả về `InvalidBindingError`.
- Source bắt buộc phải gán được cho Target (`source.AssignableTo(target)`). Nếu không, trả về `InvalidBindingError`.
- Target không được trùng với provider hoặc binding đã tồn tại. Nếu trùng, trả về `DuplicateProviderError`.
- Khi resolve interface, container resolve concrete type tương ứng, lưu cùng một `reflect.Value` vào cache singleton của cả interface lẫn concrete type, đảm bảo cùng một đối tượng duy nhất.
- Đồ thị phụ thuộc (`Validate`) kiểm tra binding xuyên suốt và hiển thị đúng đường dẫn phụ thuộc nếu xảy ra lỗi.
