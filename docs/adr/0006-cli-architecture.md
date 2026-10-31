# ADR 0006 — Injel CLI Architecture & Project Scaffolding

- **Trạng thái:** Accepted
- **Ngày:** 31/10/2026
- **Tác giả & Project Maintainer:** Phạm Trương Thông (@TogPam)
- **Người quyết định:** Phạm Trương Thông
- **Phạm vi:** Developer Tooling, Project Scaffolding, CLI Binary

---

## 1. Bối cảnh

Sau khi hoàn thành DI core (Phase 1), Application Lifecycle (Phase 2), Configuration (Phase 3) và HTTP Server Module (Phase 4), framework Injel đã có đầy đủ các khối xây dựng nền tảng cho một backend service hoàn chỉnh.

Tuy nhiên, việc thiết lập một dự án mới đòi hỏi lập trình viên phải tự tạo cấu trúc thư mục, khởi tạo `go.mod`, khai báo cấu hình môi trường, đăng ký provider, và thiết lập HTTP router cùng graceful shutdown. Điều này làm tăng thời gian khởi đầu (time-to-first-request) và dễ dẫn đến sự thiếu đồng nhất về cấu trúc project giữa các nhóm phát triển.

Injel cần một công cụ dòng lệnh chính thức (`injel` CLI) nhằm:
1. Tạo nhanh một project backend mới với kiến trúc chuẩn, đầy đủ DI, Config, HTTP router và Graceful Shutdown.
2. Kiểm tra môi trường phát triển (Go version, module status).
3. Hỗ trợ chạy ứng dụng thuận tiện.
4. Chuẩn bị nền tảng cho các công cụ code generation trong tương lai mà không làm phức tạp hóa core.

---

## 2. Các phương án kiến trúc CLI đã cân nhắc

### Phương án A — Tách CLI thành Repository riêng biệt
- **Ưu điểm:** Tách biệt mã nguồn công cụ và framework.
- **Nhược điểm:**
  - Tăng độ phức tạp quản lý đa repo (multi-repo overhead), phiên bản CLI dễ lệch pha với phiên bản framework.
  - Khó kiểm thử tích hợp (integration tests) giữa CLI và các thay đổi mới nhất của framework.
  - Phức tạp hóa CI/CD.

### Phương án B — Sử dụng thư viện CLI ngoài (Cobra, Urfave CLI)
- **Ưu điểm:** Có sẵn hệ thống cờ lệnh phong phú, tự động sinh completion shell.
- **Nhược điểm:**
  - Vi phạm nguyên tắc thiết kế số 5 của Injel (*"Core nhỏ, module rời, zero external dependencies"*).
  - Tăng kích thước binary và kéo theo cây dependency của bên thứ ba.
  - Giảm tính độc lập và khả năng kiểm soát toàn diện đối với mã nguồn.

### Phương án C — Xây dựng CLI mỏng bằng Thư viện chuẩn Go trong chính Repository (Được chọn)
- **Vị trí mã nguồn:** `cmd/injel/` (entrypoint `main.go`) và `internal/cli/`, `internal/scaffold/`.
- **Cơ chế parse:** Sử dụng `flag.FlagSet` của thư viện chuẩn kết hợp với bộ phân phối lệnh (command dispatcher) đơn giản, tường minh.
- **Scaffolding:** Sử dụng `embed.FS` và `text/template` đóng gói toàn bộ mẫu project vào bên trong binary duy nhất.
- **Kiểm thử:** Tách biệt logic thực thi thành `Run(args []string, stdout, stderr io.Writer) int` giúp kiểm thử 100% không cần mock `os.Exit`.
- **Zero dependencies:** Không thêm bất kỳ package bên thứ ba nào vào `go.mod`.

---

## 3. Thiết kế chi tiết

### 3.1. Cấu trúc thư mục

```text
Injel/
├── cmd/
│   └── injel/
│       └── main.go          # Entrypoint mỏng gọi internal/cli.Run
├── internal/
│   ├── cli/
│   │   ├── cli.go           # Bộ điều phối lệnh, parse arguments, exit codes
│   │   ├── help.go          # Render trợ giúp cú pháp
│   │   ├── version.go       # Thông tin phiên bản, git commit, build date, Go runtime
│   │   ├── doctor.go        # Kiểm tra môi trường Go, module status
│   │   ├── run.go           # Wrapper chạy go run với signal forwarding
│   │   └── cli_test.go      # Kiểm thử toàn diện các lệnh CLI
│   └── scaffold/
│       ├── scaffold.go      # Engine sinh mã nguồn, validation, rollback
│       ├── templates/       # Mẫu project nhúng bằng embed.FS
│       │   ├── go.mod.tmpl
│       │   ├── gitignore.tmpl
│       │   ├── env.example.tmpl
│       │   ├── README.md.tmpl
│       │   ├── main.go.tmpl
│       │   ├── config.go.tmpl
│       │   └── handler.go.tmpl
│       └── scaffold_test.go # Unit test & integration test biên dịch project mẫu
```

### 3.2. Danh sách các lệnh chính thức

1. **`injel help` / `injel -h` / `injel --help`:**
   - Hiển thị tài liệu sử dụng, danh sách command, cờ lệnh và ví dụ cú pháp.
2. **`injel version` / `injel --version`:**
   - Hiển thị Injel version (`dev` hoặc tag release), git commit, build date (hỗ trợ `-ldflags`) và Go runtime version.
3. **`injel doctor`:**
   - Kiểm tra `go` executable trong `$PATH`, kiểm tra Go version >= 1.24 (yêu cầu của Injel), nhận diện thư mục hiện tại có thuộc Go module hay không.
4. **`injel run [args...]`:**
   - Kiểm tra `go.mod` tại thư mục hiện tại, chuyển tiếp an toàn các tham số sang `go run . [args...]`, kết nối trực tiếp `os.Stdin`, `os.Stdout`, `os.Stderr`, và bắt tín hiệu ngắt (`os.Interrupt`, `syscall.SIGTERM`).
5. **`injel new` (Project Scaffolding):**
   - Cú pháp:
     ```bash
     injel new --name <project-name> --module <module-path> [flags]
     ```
   - Các flag:
     - `--name`: Tên project và thư mục (bắt buộc, phải là định danh hợp lệ, chống path traversal).
     - `--module`: Go module path (bắt buộc, kiểm tra format module path hợp lệ).
     - `--dir`: Thư mục cha (mặc định là thư mục hiện tại `.`).
     - `--injel-version`: Phiên bản Injel phát hành (ví dụ: `v0.1.0`).
     - `--local-injel`: Đường dẫn local tới source Injel (dành cho phát triển framework nội bộ).

### 3.3. Chiến lược quản lý Injel Dependency (Version Policy)

Do Injel đang ở giai đoạn phát triển và chưa phát hành tag release chính thức lên proxy Go toàn cầu:
1. **Chế độ Phát hành (`--injel-version`):**
   - Ghi trực tiếp `require github.com/Injel-Framework/Injel <version>` vào `go.mod`.
2. **Chế độ Phát triển Cục bộ (`--local-injel`):**
   - **Tuyệt đối không** đưa đường dẫn tuyệt đối cục bộ vào `go.mod` (ví dụ `replace ... => /home/...`) vì sẽ làm hỏng tính di động và gây lỗi khi commit code lên Git.
   - **Giải pháp tối ưu:** Sinh file `go.work` tại thư mục gốc của project mới và tự động đưa `go.work` vào `.gitignore` của project đó:
     ```text
     go 1.24.0

     use (
         .
         <absolute-path-to-local-injel>
     )
     ```
   - Giúp các công cụ Go (`go build`, `go test`, `go run`, gopls) nhận diện và biên dịch ngay lập tức mà không phụ thuộc Internet và không làm bẩn `go.mod`.
3. Hai cờ `--injel-version` và `--local-injel` loại trừ lẫn nhau. Nếu cả hai đều không được truyền và CLI đang ở bản `dev`, CLI sẽ tự động phát hiện nếu đang chạy trong chính repo Injel để gợi ý hoặc yêu cầu người dùng chỉ định rõ ràng.

### 3.4. An toàn khi sinh file (Safety & Rollback)
- Kiểm tra trước nếu thư mục đích đã tồn tại: từ chối tạo mới để tránh ghi đè dữ liệu người dùng.
- Kiểm tra tính hợp lệ của tên project (chỉ cho phép ký tự chữ cái, số, gạch ngang, gạch dưới; cấm `..`, `/`, `\`).
- Nếu xảy ra lỗi giữa chừng trong quá trình ghi file: tự động thu hồi và xóa sạch các file/thư mục dở dang đã tạo trước đó.

---

## 4. Quyết định về Code Generation (So với Project Scaffolding)

Phân biệt rõ:
- **Project Scaffolding:** Sinh cấu trúc khởi đầu cho dự án backend. Đây là tính năng cốt lõi của Phase 5.
- **Compile-time DI Code Generation:** Thay thế reflection startup bằng sinh code nối dependency tại compile-time (tương tự Google Wire).
  - **Kết luận:** Được ghi nhận chi tiết tại `docs/research/code-generation.md`. Ở thời điểm hiện tại, DI core dùng reflection ở startup rất nhanh (< 1ms đối với hàng chục components), không chạy trên request path, và có khả năng validate chu trình, thiếu dependency cực kỳ chính xác. Việc áp dụng codegen ở giai đoạn này sẽ tạo thêm gánh nặng công cụ (build steps, generated files) cho người dùng mà chưa đem lại lợi ích đo lường được. Do đó, hoãn việc triển khai compile-time DI codegen cho đến khi có benchmark thực tế chứng minh nhu cầu.

---

## 5. Hậu quả & Đánh giá

- **Tích cực:**
  - Lập trình viên có thể khởi tạo một ứng dụng Injel hoàn chỉnh chỉ với 1 câu lệnh.
  - Dự án mẫu sinh ra tuân thủ 100% kiến trúc Injel chuẩn (DI, Config, HTTP router, Graceful shutdown).
  - Binary CLI độc lập, nhẹ (< 10MB), biên dịch nhanh, zero external dependencies.
  - Kiểm thử tích hợp tự động đảm bảo project sinh ra luôn luôn biên dịch và chạy được.
- **Tiêu cực / Giới hạn:**
  - Chưa hỗ trợ tương tác đồ họa phức tạp hoặc plugin bên ngoài.
  - Lệnh `injel run` chỉ là wrapper an toàn cho `go run .`, chưa có hot reload (chủ đích thiết kế nhằm giữ công cụ đơn giản).
