# Roadmap Injel

**Cập nhật:** 28/10/2026

Roadmap này mô tả thứ tự ưu tiên, không cam kết ngày hoàn thành. Mỗi giai đoạn chỉ bắt đầu khi giai đoạn trước có test và tiêu chí hoàn thành rõ ràng.

## Trạng thái

| Giai đoạn | Nội dung | Trạng thái |
| --- | --- | --- |
| 0 | Tài liệu nền tảng, module Go, cấu trúc repository | Implemented |
| 1 | DI core: provider, singleton, validation, lỗi, interface binding | Implemented |
| 2 | Application lifecycle: start/stop, graceful shutdown, rollback | Implemented |
| 3 | Configuration từ environment | Planned |
| 4 | HTTP module trên `net/http` | Planned |
| 5 | CLI và code generation | Planned / cần kiểm chứng |
| 6 | Tích hợp tùy chọn: database, logging, validation | Planned |
| 7 | So sánh, benchmark và ổn định API | Planned |

## Giai đoạn 0 — Nền tảng

**Mục tiêu:** có tài liệu định vị, quyết định kiến trúc và bộ khung Go chạy được.

- README mô tả đúng vấn đề, triết lý, trạng thái và non-goals.
- Product brief, roadmap và ADR 0001.
- Module Go tối thiểu, không dependency ngoài.
- `go test ./...` và `go vet ./...` chạy thành công.

**Trạng thái:** hoàn thành.

## Giai đoạn 1 — DI core & Interface Binding

**Mục tiêu:** chứng minh provider function + reflection ở startup là đủ cho MVP và hỗ trợ interface binding tường minh.

Đã có:

- `di.Container` và facade `injel.App`.
- Provider `func(...) T` hoặc `func(...) (T, error)`.
- Resolve theo kiểu chính xác, cache singleton.
- Phát hiện provider trùng ngay khi đăng ký.
- `Validate()` kiểm tra dependency thiếu và chu trình mà không gọi constructor.
- Lỗi có tên provider, kiểu và đường đi dependency.
- Interface binding tường minh: `ProvideAs` và `Bind` (cả reflection-based và generic API `di.ProvideAs[T]`, `di.Bind[T, S]`).
- Chia sẻ cùng một singleton instance giữa interface và concrete type.
- Test cho luồng thành công, singleton, lỗi thiếu/trùng/chu trình, provider sai chữ ký, constructor trả lỗi, unwrap lỗi (`errors.Is`, `errors.As`), và concurrent resolution.

Còn mở:

- Named provider và scope (chỉ làm khi có use case).
- Cơ chế resolve dependency của chính container (ví dụ logger, config) mà không dùng global.
- Đánh giá khả năng hiển thị dependency graph cho mục đích debug.
- Đo chi phí reflection khi khởi động trước khi cân nhắc code generation.

## Giai đoạn 2 — Application lifecycle

**Mục tiêu:** quản lý tài nguyên và dừng ứng dụng an toàn.

Đã có:

- Interface hook chuẩn: `StartHook` (`OnStart(context.Context) error`) và `StopHook` (`OnStop(context.Context) error`).
- Facade `app.Start(ctx)` và `app.Stop(ctx)` cùng kiểm tra trạng thái an toàn đa luồng (`StateNew`, `StateStarting`, `StateRunning`, `StateStopping`, `StateStopped`, `StateFailed`).
- Khởi động theo thứ tự dependency graph (topological order): dependency khởi tạo và start trước consumer.
- Dừng theo thứ tự ngược lại (LIFO): consumer dừng trước dependency.
- Cơ chế rollback tự động khi một thành phần khởi động thất bại: dọn dẹp các thành phần đã khởi động thành công trước đó theo thứ tự ngược.
- Bảo toàn thông tin lỗi khởi động và tất cả lỗi dọn dẹp bằng `errors.Join`.
- Hỗ trợ timeout và context cancellation ở cả start và stop.
- Đăng ký hook thủ công linh hoạt qua `app.AppendHook(Hook)`.
- Unit test bao phủ thứ tự start/stop, lỗi start, rollback, lỗi cleanup, context timeout, gọi lặp lại, gọi đồng thời và kiểm tra race condition.

**Trạng thái:** hoàn thành.

## Giai đoạn 3 — Configuration

**Mục tiêu:** cấu hình thống nhất, bắt đầu từ environment.

- Đọc biến môi trường với tên rõ ràng.
- Giá trị mặc định và lỗi thiếu cấu hình bắt buộc.
- Không tự động đọc file, secret manager hoặc remote config trong bản đầu.
- Không che giấu precedence; tài liệu hóa thứ tự ưu tiên.
- Test cho giá trị mặc định, thiếu biến, sai kiểu và override.

**Chưa làm:** profile phức tạp, auto-refresh, hệ thống starter.

## Giai đoạn 4 — HTTP

**Mục tiêu:** REST API nhỏ dùng `net/http` trước.

- Adapter mỏng để đăng ký route và middleware.
- Health endpoint và error handling cơ bản.
- Tích hợp DI mà không resolve dependency trên mỗi request.
- Test bằng `httptest`, không cần dịch vụ ngoài.
- Chỉ tạo abstraction router riêng khi có bằng chứng `net/http` không đủ.

**Chưa làm:** router thay thế `net/http`, binding/validation framework lớn, OpenAPI generator.

## Giai đoạn 5 — CLI và code generation

**Mục tiêu:** cải thiện trải nghiệm sau khi DI API ổn định.

- CLI tối thiểu để tạo hoặc chạy ứng dụng mẫu.
- Đánh giá code generation dựa trên phản hồi và benchmark thực tế.
- Không duy trì song song reflection và codegen nếu chưa có lý do rõ ràng.
- Không mô phỏng annotation Java nếu không có công cụ phân tích/generate đi kèm.

**Trạng thái:** chưa bắt đầu; cần dữ liệu từ người dùng thực tế.

## Giai đoạn 6 — Tích hợp tùy chọn

Chỉ thêm module khi có use case và test:

- Database (`database/sql` trước, ORM sau nếu thực sự cần).
- Logging (`log/slog` trước).
- Validation.
- Các adapter cho dịch vụ ngoài.

Mỗi tích hợp phải là module riêng, không làm nặng core.

## Giai đoạn 7 — Đo lường và ổn định

- Benchmark khởi động, bộ nhớ và throughput trên cùng máy/cấu hình.
- So sánh với Go thuần, Go-Spring và Fx bằng cùng một bài toán.
- Ghi lại khác biệt đã kiểm chứng, không chỉ khác biệt trên marketing.
- Versioning API và chính sách tương thích.
- Rà soát module path, tên và khả năng phát hành công khai.

## Thứ tự ưu tiên ngay sau phiên này

1. Chốt contract DI và bổ sung test cho interface/binding nếu cần.
2. Thiết kế và triển khai lifecycle start/stop có test.
3. Viết ví dụ Hello API với `net/http`, health endpoint và graceful shutdown.
4. Thiết kế configuration từ environment.
5. Chỉ sau đó cân nhắc CLI, codegen và tích hợp ngoài.

## Non-goals dài hạn (trừ khi có quyết định mới)

- Trở thành bản sao đầy đủ của Spring Boot.
- Annotation processor giả lập Java.
- Microservices platform hoặc service mesh.
- ORM riêng.
- Tuyên bố hiệu năng chưa được đo.
