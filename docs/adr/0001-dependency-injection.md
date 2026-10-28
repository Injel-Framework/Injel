# ADR 0001 — Dependency Injection cho Injel MVP

- **Trạng thái:** Accepted
- **Ngày:** 27/10/2026
- **Người quyết định:** Phạm Trương Thông
- **Phạm vi:** DI core của MVP

## Bối cảnh

Injel cần một cách thống nhất để đăng ký và kết nối các thành phần trong ứng dụng Go:

```go
func NewDatabase(cfg *Config) (*sql.DB, error)
func NewUserRepository(db *sql.DB) *UserRepository
func NewUserService(repo *UserRepository) *UserService
```

Mục tiêu không phải tạo một IoC container giống Spring, mà là giảm boilerplate khởi tạo, phát hiện lỗi sớm và giữ dependency hiển thị rõ qua chữ ký hàm. Các yêu cầu:

- API tự nhiên với Go, không cần annotation.
- Phát hiện provider trùng, dependency thiếu và chu trình trước khi phục vụ request.
- Không resolve dependency trên mỗi request.
- Lỗi có đủ ngữ cảnh để debug.
- Core không phụ thuộc thư viện ngoài.

## Các phương án đã cân nhắc

### Phương án A — Runtime reflection

Đăng ký hàm provider; container dùng reflection để đọc kiểu tham số và kết quả, sau đó resolve đệ quy tại startup.

**Ưu điểm:**

- API Go tự nhiên, không cần sinh code.
- Thêm provider chỉ là thêm một dòng đăng ký.
- Có thể kiểm tra toàn bộ graph trước khi dựng đối tượng.
- Triển khai và bảo trì đơn giản ở giai đoạn đầu.
- Lỗi có thể được bọc với tên provider và đường đi dependency.

**Nhược điểm:**

- Lỗi chữ ký provider chỉ phát hiện khi chạy (dù có thể phát hiện ngay lúc đăng ký).
- Reflection có chi phí khởi động; cần đo trước khi kết luận.
- Khó hỗ trợ một số trường hợp tinh vi như generic method, variadic hoặc binding interface nếu không thiết kế thêm.
- Debug phụ thuộc vào chất lượng thông báo lỗi do framework tạo ra.

### Phương án B — Code generation

Sinh code nối dependency tại build time (theo hướng Wire).

**Ưu điểm:**

- Kiểm tra kiểu tại compile time.
- Không reflection lúc chạy.
- Dependency graph hiển thị trong code sinh ra.
- Hiệu năng và khả năng dự đoán tốt.

**Nhược điểm:**

- Cần CLI/build step và quy trình generate trong CI.
- Người dùng phải hiểu thêm một lớp công cụ.
- Thay đổi provider có thể cần generate lại; lỗi build khó đọc nếu codegen chưa tốt.
- Google Wire upstream đã archive; cần đánh giá fork và chi phí bảo trì.
- Phải duy trì thêm một cơ chế trong khi API DI chưa ổn định.

### Phương án C — DI tường minh, không container

Tự gọi constructor trong `main`:

```go
cfg := NewConfig()
db := NewDatabase(cfg)
repo := NewUserRepository(db)
svc := NewUserService(repo)
```

**Ưu điểm:**

- Đơn giản nhất, minh bạch nhất, không magic.
- Không thêm dependency hay abstraction.
- Dễ debug và phù hợp với ứng dụng nhỏ.

**Nhược điểm:**

- Boilerplate tăng theo số thành phần.
- Không có cơ chế kiểm tra graph thống nhất.
- Khó chuẩn hóa lifecycle và cấu hình khi dự án lớn dần.
- Không giải quyết mục tiêu trải nghiệm phát triển của Injel.

### Phương án D — Kết hợp

Dùng DI tường minh cho ứng dụng nhỏ, reflection ở startup cho MVP, và chỉ thêm code generation sau khi có bằng chứng reflection gây vấn đề.

**Ưu điểm:**

- MVP có thể triển khai nhanh, API ổn định trước.
- Không khóa người dùng vào codegen.
- Có lộ trình nâng cấp dựa trên dữ liệu thực tế.

**Nhược điểm:**

- Nếu sau này thêm codegen, có nguy cơ phải duy trì hai cơ chế.
- Cần kỷ luật để không mở rộng reflection thành một "ngôn ngữ" cấu hình phức tạp.

## Quyết định

Chọn **phương án D, triển khai phần reflection ở startup cho MVP**:

1. **Provider function là API chính.** Đăng ký bằng `app.Provide(fn)`.
2. **Contract MVP:**
   - `fn` là hàm không variadic.
   - Trả về đúng một giá trị `T`, hoặc hai giá trị `(T, error)`.
   - `T` không phải kiểu `error`.
   - Các tham số được resolve từ registry theo **kiểu chính xác**.
3. **Singleton theo kiểu.** Mỗi kiểu có tối đa một provider; instance được cache sau lần resolve đầu tiên.
4. **Validate trước khi dựng đối tượng.** `App.Validate()` kiểm tra dependency thiếu và chu trình mà không gọi constructor.
5. **Kiểm tra ngay khi đăng ký.** Provider sai chữ ký hoặc trùng kiểu kết quả bị từ chối ngay.
6. **Lỗi có ngữ cảnh.** Lỗi nêu kiểu, provider và đường đi dependency; constructor error được bọc, không panic mơ hồ.
7. **Không annotation, không package scanning, không global container.**
8. **Không code generation trong MVP.** Chỉ đánh giá sau khi API ổn định và có benchmark/phản hồi thực tế.

## Hệ quả

### Tích cực

- API nhỏ, dễ đọc và không cần công cụ ngoài.
- Lỗi cấu hình dependency xuất hiện trước khi ứng dụng nhận request.
- Core không có dependency bên ngoài.
- Có nền tảng để thêm lifecycle và HTTP mà không phải thay đổi contract provider.

### Tiêu cực và giới hạn

- Reflection chỉ chạy khi resolve, nhưng vẫn có chi phí khởi động chưa được đo.
- Chưa hỗ trợ binding từ concrete type sang interface. Nếu cần, sẽ bổ sung API tường minh thay vì tự động suy đoán.
- Chưa hỗ trợ named provider, scope, optional dependency hoặc lazy provider.
- Constructor không nên gọi ngược container trong lúc resolve (có thể gây deadlock hoặc hành vi khó hiểu); điều này cần được tài liệu hóa và kiểm tra.
- Provider trả về cùng một kiểu từ hai nguồn là lỗi, kể cả khi người dùng muốn nhiều instance. Named provider sẽ giải quyết khi có nhu cầu.

## Điều chưa được kiểm chứng

- Chi phí reflection thực tế trên ứng dụng có hàng trăm provider.
- Việc thiếu interface binding có gây khó dùng trong dự án thực tế hay không.
- Code generation có mang lại lợi ích đủ lớn để đánh đổi quy trình build hay không.
- Nhu cầu thực tế về scope (request scope, transient) và named provider.
- Mô hình lifecycle phù hợp nhất với DI (hook theo instance, hook theo provider, hay cả hai).

## Điều kiện xem lại quyết định

Xem lại ADR này khi:

- API provider đã ổn định qua ít nhất một vòng phản hồi người dùng.
- Có benchmark cho thấy reflection là điểm nghẽn đáng kể.
- Có nhu cầu interface binding hoặc named provider lặp lại ở nhiều dự án.
- Có quyết định về module path và phạm vi phát hành công khai.
