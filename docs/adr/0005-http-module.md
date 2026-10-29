# ADR 0005 — HTTP Module & REST API trong Injel

- **Trạng thái:** Accepted
- **Ngày:** 29/10/2026
- **Người quyết định:** Nhóm phát triển Injel
- **Phạm vi:** Package `httpserver` & REST API capabilities

## Bối cảnh

Sau khi hoàn thành DI Core (Phase 1), Application Lifecycle (Phase 2) và Configuration Module (Phase 3), Injel cần một tầng HTTP server thực dụng để người dùng xây dựng các REST API service.

Một ứng dụng HTTP trong Go cần:
1. Đăng ký các route xử lý request (GET, POST, PUT, DELETE,...) và path variables.
2. Chuỗi middleware thực thi theo thứ tự rõ ràng (logging, recovery, request ID,...).
3. Đưa dependency (service, repository, config) vào handler mà không dùng biến toàn cục và không chạy reflection trên mỗi request.
4. Tích hợp chặt chẽ vào Application Lifecycle: khởi động server có kiểm soát cổng mạng, graceful shutdown khi ứng dụng nhận tín hiệu dừng hoặc context timeout.

## Các phương án đã cân nhắc

### Phương án 1 — Sử dụng Router ngoài (Chi, Gin, Echo, Fiber)
- **Ưu điểm:** Có sẵn hệ sinh thái middleware phong phú.
- **Nhược điểm:**
  - Vi phạm nguyên tắc thiết kế 5 (*"Core nhỏ, module rời. Core không phụ thuộc thư viện ngoài"*).
  - Từ Go 1.22+, thư viện chuẩn `net/http.ServeMux` đã hỗ trợ đầy đủ method matching (ví dụ `GET /health`) và path variables (`GET /users/{id}`), đáp ứng hầu hết nhu cầu của một REST API mà không cần router bên thứ ba.
  - Các framework như Gin/Fiber thường mang theo abstractions lớn hoặc khác biệt so với chuẩn `net/http` (ví dụ `gin.Context` hoặc `fiber.Ctx` dùng fasthttp).

### Phương án 2 — Tự viết Matching Engine Router riêng từ đầu
- **Ưu điểm:** Tự do kiểm soát cú pháp matching.
- **Nhược điểm:** Tốn kém chi phí bảo trì, dễ sinh lỗi và lặp lại công việc mà thư viện chuẩn `net/http` của Go đã tối ưu hóa rất tốt.

### Phương án 3 — Xây dựng Module `httpserver` trên nền `net/http` và `ServeMux` (Được chọn)
- Sử dụng `net/http.ServeMux` của Go làm router engine cốt lõi.
- Xây dựng một lớp `Server` mỏng bao gói:
  - Route registry kiểm tra lỗi cú pháp và xung đột pattern sớm (fail-fast).
  - Chuỗi middleware theo mô hình chuẩn Go `func(http.Handler) http.Handler`.
  - Tích hợp trực tiếp vào DI container và thực thi `StartHook` (`OnStart`) và `StopHook` (`OnStop`) của Injel.

## Thiết kế chi tiết

### 1. Package Layout

```text
httpserver/
├── server.go        # Cấu trúc Server, StartHook/StopHook lifecycle, listener management
├── router.go        # Đăng ký route, pattern validation, ServeMux wrapper
├── middleware.go    # Định nghĩa Middleware, chain wrapper và built-in middlewares
├── errors.go        # Các kiểu lỗi có ngữ cảnh (RouteError, ServerError)
└── server_test.go   # Test bao phủ router, middleware, lifecycle và concurrency
```

### 2. Định nghĩa Route & Router

Router xây dựng trên `net/http.ServeMux`:
- Hỗ trợ cú pháp chuẩn Go: `"METHOD /path"` và `"METHOD /path/{var}"`.
- Lấy path variable qua thư viện chuẩn: `r.PathValue("id")`.
- API đăng ký:
  ```go
  server.Handle(pattern string, handler http.Handler) error
  server.HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) error
  ```
- **Kiểm soát lỗi và an toàn (Fail-fast):**
  - Mọi route được xác thực tính hợp lệ cú pháp và phát hiện xung đột pattern ngay khi đăng ký bằng cách kiểm tra trước trong một sandbox `ServeMux` tách biệt. Nếu pattern không hợp lệ hoặc trùng lặp, trả về `RouteError` cụ thể thay vì panic không kiểm soát.
  - Sau khi server đã khởi động (`OnStart`), việc cố tình đăng ký thêm route sẽ bị từ chối với lỗi `RouteError`, tránh thay đổi cấu hình ngầm lúc ứng dụng đang chạy.

### 3. Middleware

- Kiểu chuẩn Go:
  ```go
  type Middleware func(http.Handler) http.Handler
  ```
- API đăng ký: `server.Use(middlewares ...Middleware)`
- Thứ tự thực thi: Mô hình củ hành (Onion model). Middleware đăng ký trước sẽ là tầng bọc ngoài cùng (chạy trước khi vào handler và chạy sau khi handler hoàn tất).
- Built-in middlewares tối thiểu:
  - `RequestID`: Xác thực và chuyển tiếp header `X-Request-ID` (giới hạn tối đa 128 ký tự ASCII an toàn để chống Log/Header Injection) hoặc tự động sinh ID ngẫu nhiên, đưa vào request `context` (`RequestIDFromContext(ctx)`).
  - `Recovery`: Bắt panic trong handler, bảo đảm trả về HTTP 500 nếu headers chưa gửi, log stack trace kèm request ID, và giữ an toàn không làm hỏng phản hồi nếu headers đã gửi.
  - `Logging`: Ghi log phương thức, đường dẫn, HTTP status, request ID và thời gian xử lý.
- Thứ tự chuẩn khuyến nghị: `RequestID` -> `Logging` -> `Recovery`. Khi có panic, `Recovery` xử lý trước, trả 500, cho phép `Logging` ghi nhận chính xác mã trạng thái 500.

### 4. Cấu hình Server (Configuration)

Tích hợp với package `config` của Injel:
```go
type Config struct {
    Addr              string        `env:"HTTP_ADDR,default=:8080"`
    ReadHeaderTimeout time.Duration `env:"HTTP_READ_HEADER_TIMEOUT,default=5s"`
    IdleTimeout       time.Duration `env:"HTTP_IDLE_TIMEOUT,default=60s"`
    ShutdownTimeout   time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT,default=10s"`
    MaxHeaderBytes    int           `env:"HTTP_MAX_HEADER_BYTES,default=1048576"`
}
```

### 5. Quản lý Vòng đời (Application Lifecycle)

`httpserver.Server` hiện thực trực tiếp `StartHook` và `StopHook`:

- **Khởi động (`OnStart`):**
  1. Kiểm tra cấu hình và tổng hợp chuỗi middleware cùng handler (tránh compile lại nếu đã gọi `server.Handler()`).
  2. Từ chối khởi động lại nếu server đã từng bị dừng (`server has been stopped`).
  3. Mở `net.Listener` bằng `net.ListenConfig` trên địa chỉ chỉ định.
  4. Nếu cổng mạng đã bị chiếm dụng (port collision) hoặc địa chỉ không hợp lệ, trả về lỗi ngay lập tức trong `OnStart`.
  5. Nếu dùng `:0` (thường gặp trong unit test), cung cấp phương thức `server.Addr()` để lấy địa chỉ và cổng thực tế đã được cấp phát.
  6. Chạy `http.Server.Serve(listener)` trong một goroutine riêng và giám sát lỗi phục vụ qua channel `server.ServeErrors()` (buffer 1, không gây deadlock nếu không đọc, tự động đóng khi goroutine phục vụ kết thúc) cùng phương thức `server.ServeError()`.
- **Dừng (`OnStop`):**
  1. Tắt HTTP keepalive (`SetKeepAlivesEnabled(false)`).
  2. Gọi `http.Server.Shutdown(shutdownCtx)` để cho phép các request đang xử lý hoàn thành trong giới hạn `ShutdownTimeout`.
  3. Nếu hết timeout mà request vẫn chưa xong, gọi `http.Server.Close()` để giải phóng tài nguyên.
  4. Xử lý an toàn khi gọi `OnStop` nhiều lần (idempotent).

### 6. Dependency Injection vào Handler

Injel không sử dụng reflection trên mỗi request HTTP. Thay vào đó:
- Handler được khai báo cùng các dependency thông qua constructor / provider function thông thường.
- Ví dụ:
  ```go
  type UserHandler struct {
      service *UserService
  }
  func NewUserHandler(svc *UserService) *UserHandler {
      return &UserHandler{service: svc}
  }
  func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
      id := r.PathValue("id")
      user, err := h.service.Find(id)
      ...
  }
  ```
- Server và Handler được nối kết bằng DI container của Injel một lần duy nhất lúc khởi tạo.

### 7. Phạm vi trong MVP vs Tương lai

- **Trong MVP:** HTTP server, routing trên `net/http.ServeMux`, middleware chain, recovery, request ID, graceful shutdown, health endpoint, ví dụ REST API.
- **Tương lai:** Content negotiation tự động, validation request body nâng cao, OpenAPI/Swagger generation.
