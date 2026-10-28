# Định hướng sản phẩm Injel

**Phiên bản:** 1.0<br>
**Ngày:** 27/10/2026 (Cập nhật: 28/10/2026)<br>
**Trạng thái:** Tài liệu nền tảng cho giai đoạn MVP

> Tài liệu này tổng hợp từ hai tài liệu đầu vào `GoVa_Tom_Tat_Danh_Gia_Y_Tuong.docx` và `GoVa_Plan_Trien_Khai_Cac_Buoc_Dau_Tien.docx`. Dự án ban đầu có tên là **GoVa**, sau đó từng mang tên **Tertex** trong giai đoạn đầu, và tên chính thức hiện tại là **Injel** (Injel = Injection + Element). Nội dung ở đây là bản tổng hợp kỹ thuật, không phải bản sao nguyên văn của tài liệu gốc.

## 1. Tuyên bố sản phẩm

Injel là framework backend viết bằng Go, mang đến trải nghiệm phát triển có cấu trúc tương tự Spring Boot: dependency injection rõ ràng, cấu hình thống nhất, khởi tạo và dừng ứng dụng có kiểm soát — nhưng vẫn giữ sự đơn giản, minh bạch và tính tự nhiên của Go.

**Định vị ngắn:** *Spring-inspired developer experience. Native Go simplicity.*

Injel không phải bản sao Spring Boot viết lại bằng Go. Framework chỉ sử dụng abstraction khi nó giải quyết một vấn đề cụ thể và có thể giải thích được.

## 2. Vấn đề cần giải quyết

### Vấn đề 1 — Phân mảnh công cụ

Mỗi dự án Go phải tự chọn và kết nối router, cấu hình, DI, logging, database, validation và lifecycle. Không có quy ước chung khiến chi phí onboarding và bảo trì tăng khi đội ngũ hoặc dự án thay đổi.

### Vấn đề 2 — Boilerplate khởi tạo

Khi số thành phần tăng, code nối `config → DB → repository → service → handler` trở nên dài, lặp lại và dễ sai thứ tự. Việc thiếu dependency hoặc tạo chu trình thường chỉ lộ ra khi ứng dụng đã chạy.

### Vấn đề 3 — Thiếu quy ước xuyên suốt

Cùng một nhóm nhu cầu nhưng mỗi dự án giải quyết theo cách khác nhau. Điều này làm giảm khả năng tái sử dụng kiến thức, tăng chi phí đọc hiểu và khó chuẩn hóa vận hành.

## 3. Đối tượng sử dụng

- Đội phát triển Go xây dựng REST API hoặc backend service có nhiều thành phần.
- Lập trình viên quen với Spring Boot muốn mô hình tổ chức quen thuộc nhưng không muốn mang theo độ phức tạp của JVM/Java.
- Dự án cần core nhỏ, ít dependency, có thể biên dịch native và kiểm thử nhanh.

## 4. Nguyên tắc thiết kế

1. **Provider function là API chính.** Dependency được khai báo qua chữ ký hàm:
   ```go
   func NewUserService(repo *UserRepository) (*UserService, error)
   ```
2. **Kiểm tra sớm.** Provider trùng, dependency thiếu và chu trình phải được phát hiện ở startup, trước khi nhận request.
3. **Không giấu luồng chạy.** Không package scanning ngầm, không global state không cần thiết, không annotation giả lập.
4. **Lỗi có ngữ cảnh.** Thông báo lỗi cần nêu kiểu, provider và đường đi dependency để người dùng sửa được.
5. **Singleton mặc định trong application context.** Không resolve lại dependency trên mỗi request.
6. **Core nhỏ, module rời.** Core không kéo theo ORM, Redis, message broker hoặc framework HTTP bên ngoài.
7. **Đo trước khi tuyên bố.** Không công bố "nhanh hơn" hoặc "nhẹ nhất" khi chưa có benchmark tái lập.

## 5. Kiến trúc định hướng

```text
Application Context
├── DI / IoC container
│   ├── Provider registry
│   ├── Dependency graph validation
│   └── Singleton cache
├── Lifecycle (start/stop, graceful shutdown)
├── Configuration (environment trước)
├── HTTP (net/http trước, abstraction sau nếu cần)
└── Optional integrations (database, logging, validation, ...)
```

Ở giai đoạn hiện tại, **DI container** (bao gồm interface binding tường minh), **Application Lifecycle** (Start/Stop, graceful shutdown, rollback) và **Configuration module** (nạp từ environment variables, struct tag `env`, default, required, isolated testing) cùng facade `App` đã được triển khai hoàn chỉnh với unit test và race detector. HTTP module là giai đoạn tiếp theo.

## 6. Phạm vi MVP

### Có trong MVP

- Application context và DI container hoàn chỉnh.
- Provider function với kết quả `T` hoặc `(T, error)`.
- Singleton theo kiểu trong phạm vi ứng dụng.
- Interface binding tường minh (`ProvideAs`, `Bind`) dùng chung singleton instance.
- Application Lifecycle: quản lý `Start`/`Stop`, hook `OnStart`/`OnStop`, rollback khi start lỗi và bảo toàn lỗi bằng `errors.Join`.
- Configuration module: struct tag `env`, giá trị mặc định, kiểm tra bắt buộc, chuyển đổi kiểu dữ liệu, TextUnmarshaler, và tích hợp `ProvideConfig` vào DI container.
- Phát hiện provider trùng, dependency thiếu, chu trình dependency và lỗi cấu hình trước khi chạy.
- Lỗi khởi tạo có tên provider, kiểu và đường đi dependency; hỗ trợ `errors.Is`/`errors.As`.
- Unit test cho luồng thành công, lỗi cốt lõi, concurrency và thread-safety.
- Ví dụ chạy được bằng `go run`.
- Tài liệu quickstart, roadmap và các ADR (0001, 0002, 0003, 0004).

### Chưa làm trong MVP

- ORM hoặc transaction framework riêng.
- Router HTTP hoàn chỉnh (trước mắt dùng `net/http`).
- Annotation giống Java (`@Bean`, `@Service`, `@Autowired`).
- CLI sinh toàn bộ ứng dụng.
- Auto-configuration phức tạp, profile, Starter lớn.
- Hệ thống plugin nhiều tầng.
- Tích hợp Redis, RabbitMQ, Kafka hoặc dịch vụ ngoài.
- Microservices platform.
- Benchmark quy mô lớn khi core chưa ổn định.

## 7. Tiêu chí thành công

### Developer experience

- Người mới có thể tạo một ứng dụng nhỏ mà không phải tự nối toàn bộ constructor.
- Ví dụ trong README chạy được đúng như mô tả.

### Độ tin cậy

- Có test cho provider hợp lệ, dependency thiếu, provider trùng, chu trình và constructor trả lỗi.
- Lỗi khởi tạo đủ thông tin để xác định thành phần gây lỗi.
- `go test ./...`, `go vet ./...` và `go test -race ./...` chạy rõ ràng.

### Hiệu quả kỹ thuật

- Core không phụ thuộc database, message broker hay dịch vụ ngoài.
- Reflection chỉ dùng ở startup/resolve, không nằm trên đường xử lý request.
- Có benchmark so với Go thuần và ít nhất một framework DI khác trước khi đưa ra tuyên bố hiệu năng.

### Khả năng bảo trì

- API nhỏ, có tài liệu và versioning rõ ràng.
- Không phải duy trì đồng thời reflection và code generation trước khi có nhu cầu được kiểm chứng.

## 8. Rủi ro và cách giảm thiểu

| Rủi ro | Cách giảm thiểu |
| --- | --- |
| Trở thành "Spring phiên bản Go" quá phức tạp | Giới hạn abstraction; không annotation, không package scanning; mỗi tính năng phải có use case |
| DI khó dự đoán | Contract provider rõ ràng; validate graph; lỗi nêu đường đi dependency |
| Không đủ khác biệt so với Go-Spring/Fx | So sánh cùng một bài toán bằng checklist, benchmark và trải nghiệm thực tế trước khi mở rộng |
| Xung đột tên/thương hiệu | Chưa public module path; kiểm tra tên repository, namespace và khả năng tìm kiếm trước khi phát hành |
| Reflection gây lỗi muộn hoặc khó debug | Resolve một lần, validate startup, thông báo lỗi có ngữ cảnh; đánh giá codegen sau |
| Overhead không cần thiết | Giữ core nhỏ; không thêm module mặc định; đo trước khi tối ưu |

## 9. Điều kiện để coi là có tiến triển thực tế

1. Một ví dụ REST API nhỏ chạy được, có test.
2. DI phát hiện đúng các lỗi cốt lõi và có test tương ứng.
3. Lifecycle start/stop có thứ tự, có test và graceful shutdown.
4. Có so sánh trải nghiệm với ít nhất một framework tương đương.
5. README và tài liệu đủ để người khác tự chạy mà không cần hỏi tác giả.

## 10. Quyết định cần xác nhận sau

- Module path chính thức và nơi phát hành.
- Có cần binding interface (`ProvideAs`) trong giai đoạn kế tiếp hay không.
- Lifecycle hook nên là interface hay function registration.
- Có cần named provider/scope trước khi có nhu cầu thực tế hay không.
- Có cần code generation sau khi DI API ổn định hay không.
