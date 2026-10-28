# ADR 0003 — Application Lifecycle trong Injel

- **Trạng thái:** Accepted
- **Ngày:** 28/10/2026
- **Người quyết định:** Nhóm phát triển Injel
- **Phạm vi:** Application facade & Lifecycle management

## Bối cảnh

Một ứng dụng backend điển hình cần khởi động các tài nguyên (kết nối database, pool kết nối, HTTP listener, consumer background) và giải phóng chúng một cách an toàn (graceful shutdown) khi ứng dụng dừng hoặc nhận tín hiệu kết thúc từ hệ điều hành.

Khi ứng dụng gồm nhiều tầng phụ thuộc (ví dụ: `Config` → `Database` → `Repository` → `Service` → `Worker`), thứ tự khởi động và dừng phải tuân thủ nghiêm ngặt:
- **Khởi động:** Tài nguyên nền tảng (Database, Pool) phải sẵn sàng trước khi Service hoặc Worker sử dụng chúng.
- **Dừng (Graceful Shutdown):** Worker/Handler phải ngừng nhận và xử lý công việc mới trước khi Database Connection bị ngắt.

Nếu thiếu cơ chế quản lý vòng đời có cấu trúc:
1. Thứ tự dừng thường bị đảo lộn dẫn đến panic hoặc mất dữ liệu (data loss).
2. Khi một thành phần khởi động thất bại giữa chừng, các thành phần đã mở trước đó dễ bị rò rỉ (resource leak).
3. Không có sự thống nhất về timeout và context cancellation.

## Các phương án đã cân nhắc

### Phương án 1 — Callback / Event-driven Bus phức tạp
Xây dựng hệ thống Event Bus (`ApplicationReadyEvent`, `ContextClosedEvent`, ...) tương tự Java Spring ApplicationEvent.

- **Nhược điểm:** Quá cồng kềnh, tạo ra luồng chạy ngầm khó debug, vi phạm nguyên tắc thiết kế 3 ("Không giấu luồng chạy") và 4 ("Tự động hóa phần lặp lại, không che giấu phần quan trọng").

### Phương án 2 — Tự xử lý thủ công trong hàm main
Không cung cấp lifecycle trong framework; lập trình viên tự gọi `Close()` hoặc `Shutdown()` thủ công trong `main.go`.

- **Nhược điểm:** Không giải quyết được bài toán boilerplate và dễ sai sót thứ tự khi đồ thị dependency lớn dần.

### Phương án 3 — Native Interface Hooks kết hợp Đồ thị Phụ thuộc (Được chọn)
Sử dụng các interface Go tự nhiên (`StartHook`, `StopHook`) kết hợp với thứ tự sắp xếp topo (topological dependency order) từ DI container:
- Khởi động theo thứ tự dependency: dependency khởi động trước consumer.
- Dừng theo thứ tự ngược lại: consumer dừng trước dependency.
- Rollback tự động khi khởi động thất bại.

## Thiết kế chi tiết

### 1. Hợp đồng Lifecycle Hook (Contracts)

```go
// StartHook cho phép component thực thi logic khi ứng dụng khởi động.
type StartHook interface {
    OnStart(context.Context) error
}

// StopHook cho phép component giải phóng tài nguyên khi ứng dụng dừng.
type StopHook interface {
    OnStop(context.Context) error
}

// Hook cho phép đăng ký trực tiếp hàm khởi động/dừng thủ công.
type Hook struct {
    OnStart func(context.Context) error
    OnStop  func(context.Context) error
}
```

Mọi component được resolve bởi DI container nếu triển khai `StartHook` hoặc `StopHook` sẽ tự động được nhận diện và đưa vào danh sách quản lý vòng đời.

### 2. State Machine của Ứng dụng

Ứng dụng (`App`) có các trạng thái rõ ràng:
- `StateNew`: Ứng dụng vừa khởi tạo, chưa chạy `Start`.
- `StateStarting`: Đang chạy quá trình khởi động.
- `StateRunning`: Tất cả provider đã resolve và hook `OnStart` đã chạy thành công.
- `StateStopping`: Đang trong quá trình giải phóng tài nguyên.
- `StateStopped`: Đã dừng hoàn toàn, tài nguyên đã được dọn dẹp sạch sẽ.
- `StateFailed`: Gặp lỗi khi khởi động hoặc gặp lỗi trong quá trình dừng.

**Quy tắc chuyển trạng thái & Concurrency:**
- Mọi thao tác chuyển trạng thái được bảo vệ bởi `sync.Mutex`.
- `Start()` chỉ hợp lệ từ trạng thái `StateNew`. Gọi `Start()` khi đang chạy/đang dừng sẽ trả về lỗi `injel: application already started` hoặc trạng thái không hợp lệ.
- `Stop()` chỉ hợp lệ khi ứng dụng đang ở `StateRunning`. Gọi `Stop()` khi ứng dụng đã `StateStopped` là thao tác an toàn (idempotent, trả về `nil`).
- Gọi `Stop()` khi chưa từng `Start()` sẽ trả về lỗi `injel: application has not been started`.

### 3. Quy trình Khởi động (Startup Flow)

1. Gọi `Validate()` trên container. Nếu đồ thị phụ thuộc có lỗi (thiếu dependency hoặc chu trình), trả về lỗi ngay lập tức, chuyển trạng thái sang `StateFailed`.
2. Duyệt khởi tạo (resolve) tất cả các component theo thứ tự phụ thuộc (topological order).
3. Với mỗi component đã khởi tạo thành công, nếu có `OnStart(ctx)`:
   - Kiểm tra `ctx.Err()`. Nếu context bị hủy/timeout, ngắt tiến trình.
   - Thực thi `OnStart(ctx)`.
   - Nếu `OnStart` thành công, lưu hook vào danh sách `startedHooks`.
   - Nếu `OnStart` thất bại:
     - Kích hoạt cơ chế **Rollback**: gọi `OnStop` cho tất cả hook đã `OnStart` thành công trước đó theo thứ tự đảo ngược.
     - Dùng `errors.Join` để gộp lỗi khởi động ban đầu và các lỗi cleanup (nếu có).
     - Chuyển sang `StateFailed` và trả về lỗi tổng hợp.
4. Chuyển trạng thái sang `StateRunning`.

### 4. Quy trình Dừng (Shutdown Flow)

1. Duyệt danh sách `startedHooks` theo **thứ tự đảo ngược** (LIFO: Last-In-First-Out).
2. Với mỗi hook có `OnStop(ctx)`:
   - Gọi `OnStop(ctx)` và ghi nhận lỗi nếu có.
   - Tiếp tục dọn dẹp các component còn lại, không ngắt quãng giữa chừng để tránh rò rỉ các tài nguyên khác.
3. Nếu context bị cancel hoặc timeout, lỗi context được thu thập vào danh sách lỗi.
4. Dùng `errors.Join` kết hợp toàn bộ lỗi dọn dẹp.
5. Nếu không có lỗi, chuyển trạng thái sang `StateStopped`. Nếu có lỗi, chuyển sang `StateFailed` và trả về lỗi.
