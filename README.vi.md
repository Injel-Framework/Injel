# Injel — Go (Golang) Backend Framework

[English](README.md) | **Tiếng Việt**

**Spring-inspired developer experience. Native Go simplicity.**

Injel (Injection + Element) là framework backend viết bằng Go, lấy cảm hứng từ kiến trúc và trải nghiệm phát triển của Java Spring Boot nhưng tuân thủ triệt để tính tự nhiên, minh bạch và hiệu năng của Go. Mục tiêu của Injel không phải sao chép Spring Boot vào Go, mà là giảm thiểu boilerplate khi kết nối các thành phần của một ứng dụng backend (Dependency Injection, Configuration, Lifecycle, HTTP Server), đồng thời giữ luồng chạy rõ ràng, dễ đọc và dễ debug.

> Tài liệu đầu vào của dự án từng mang tên **GoVa** và **Tertex** trong các tài liệu khảo sát ban đầu. Tên chính thức hiện tại của sản phẩm là **Injel**; các tài liệu lịch sử vẫn có thể lưu giữ tên cũ.

---

## Vấn đề cần giải quyết

Một ứng dụng Go nhiều thành phần thường phải tự lựa chọn và kết nối router, cấu hình, logging, database, validation và vòng đời ứng dụng. Khi số lượng thành phần tăng, code khởi tạo trở nên dài, lặp lại và khó kiểm tra:

- Không có quy ước thống nhất để đăng ký và lấy dependency.
- Lỗi thiếu dependency hoặc chu trình dependency thường chỉ xuất hiện khi chạy.
- Việc dựng và dừng tài nguyên theo đúng thứ tự dễ bị bỏ sót.
- Mỗi dự án giải quyết cùng một nhóm nhu cầu theo một cách khác nhau.

Injel hướng tới một mô hình thống nhất cho các vấn đề trên, bắt đầu từ Dependency Injection (DI) rõ ràng, kiểm tra sớm ở startup và tích hợp mượt mà với thư viện chuẩn Go.

---

## Nguyên tắc thiết kế

1. **Provider function là API chính:** Dependency được khai báo qua tham số và kết quả của hàm Go thông thường (`func NewService(repo Repository) (*Service, error)`).
2. **Lỗi xuất hiện sớm (Fail-fast):** Provider trùng, dependency thiếu, chu trình và xung đột route được phát hiện trước khi ứng dụng phục vụ request.
3. **Không giấu luồng chạy:** Không package scanning ngầm, không global container, không annotation giả lập.
4. **Tự động hóa phần lặp lại, không che giấu phần quan trọng:** Abstraction chỉ được thêm khi giải quyết một bài toán thực tế cụ thể.
5. **Core nhỏ, module rời, zero external dependencies:** Core và HTTP module không phụ thuộc vào framework ngoài (không Gin, Echo, Chi, Viper), hoàn toàn dựa trên thư viện chuẩn Go.
6. **Không tuyên bố hiệu năng khi chưa đo:** Mọi tuyên bố kỹ thuật đều phải có kiểm chứng và test thực tế.

---

## Trạng thái phát triển

| Thành phần | Trạng thái | Ghi chú |
| --- | --- | --- |
| Tài liệu sản phẩm, roadmap, ADR | Implemented | Xem `docs/` |
| DI container: provider function, singleton, resolve theo kiểu | Implemented | Reflection tại thời điểm resolve startup; không chạy trên mỗi request |
| Phát hiện provider trùng, dependency thiếu, chu trình | Implemented | Có lỗi có ngữ cảnh chi tiết và test đầy đủ |
| Constructor trả `error` | Implemented | Lỗi được bọc với tên provider và kiểu kết quả |
| Interface binding (`ProvideAs`, `Bind`) | Implemented | Singleton dùng chung giữa concrete type và interface |
| Application lifecycle (start/stop, rollback, graceful shutdown) | Implemented | Thứ tự topo theo đồ thị dependency; rollback khi start lỗi |
| Configuration từ environment | Implemented | Struct tag `env`, default, required, type conversion, isolated test |
| HTTP module trên `net/http` | Implemented | Router sandbox pre-validation, middleware chain, OnStart/OnStop lifecycle, health check |
| CLI và Project Scaffolding | Implemented | `injel new`, `injel run`, `injel doctor`, zero external deps |
| Database integration (`database/sql`) | Implemented | DI singleton, startup ping, graceful shutdown, connection pool, zero external deps |
| Tích hợp logging, Redis, broker | Planned | Ngoài phạm vi MVP hiện tại |

---

## Tích hợp Database

Package `database` cung cấp tích hợp `database/sql` với DI container và lifecycle của Injel.

### Sử dụng nhanh

```go
import (
    _ "github.com/lib/pq"  // import driver trong application — không phải trong Injel

    injel "github.com/Injel-Framework/Injel"
    "github.com/Injel-Framework/Injel/database"
)

func main() {
    app := injel.New()

    // Đăng ký *sql.DB vào DI với lifecycle hooks
    if err := database.Provide(app, database.Config{
        DriverName: "postgres",
        DSN:        os.Getenv("DATABASE_URL"),
        // Pool settings tùy chọn (giá trị mặc định):
        MaxOpenConns:    25,
        MaxIdleConns:    5,
        ConnMaxLifetime: 30 * time.Minute,
        ConnMaxIdleTime: 5 * time.Minute,
        PingTimeout:     5 * time.Second,
    }); err != nil {
        log.Fatal(err)
    }

    // Inject *sql.DB vào component qua constructor
    app.Provide(NewUserRepository)

    // app.Start → ping database; fail nhanh nếu không kết nối được
    // app.Stop  → đóng connection pool
}

// Repository nhận *sql.DB qua Injel DI
func NewUserRepository(db *sql.DB) *UserRepository {
    return &UserRepository{db: db}
}
```

### Nguyên tắc thiết kế

- Không ORM, không query builder — dùng SQL thuần hoặc thư viện tùy chọn (sqlx, bun, squirrel...).
- Driver do application quản lý, không phải framework.
- DSN/password không bao giờ xuất hiện trong error message hay log.
- Lỗi có thể unwrap qua `errors.Is`/`errors.As`.

---

## Yêu cầu môi trường

- Go 1.24 trở lên (phiên bản đã kiểm tra: Go 1.27.1).

---

## Cài đặt và chạy test

```bash
go test ./...
go vet ./...
go test -race ./...
```

Chạy ví dụ thực tế:

```bash
# Ví dụ cơ bản về DI, Configuration và Lifecycle
go run ./examples/basic

# Ví dụ REST API hoàn chỉnh với HTTP server, middleware và DI
go run ./examples/http-api
```

---

## Ví dụ sử dụng

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Injel-Framework/Injel"
)

type Config struct {
	DSN string `env:"DATABASE_URL,default=file:injel.db"`
}

type UserRepository struct {
	dsn string
}

func (r *UserRepository) OnStart(ctx context.Context) error {
	fmt.Println("UserRepository: connected to database", r.dsn)
	return nil
}

func (r *UserRepository) OnStop(ctx context.Context) error {
	fmt.Println("UserRepository: closed database connection")
	return nil
}

type UserService struct {
	repo *UserRepository
}

func NewUserRepository(cfg *Config) *UserRepository {
	return &UserRepository{dsn: cfg.DSN}
}

func NewUserService(repo *UserRepository) (*UserService, error) {
	if repo == nil {
		return nil, fmt.Errorf("user repository is nil")
	}
	return &UserService{repo: repo}, nil
}

func main() {
	app := injel.New()

	// Nạp cấu hình từ biến môi trường (hoặc giá trị mặc định file:injel.db)
	if err := injel.ProvideConfig[Config](app); err != nil {
		log.Fatalf("register config provider: %v", err)
	}

	for _, provider := range []any{
		NewUserRepository,
		NewUserService,
	} {
		if err := app.Provide(provider); err != nil {
			log.Fatalf("register provider: %v", err)
		}
	}

	ctx := context.Background()

	// Start kiểm tra graph, nạp config, resolve dependency theo thứ tự topo và chạy OnStart hook.
	if err := app.Start(ctx); err != nil {
		log.Fatalf("start application: %v", err)
	}
	defer func() {
		// Stop chạy các OnStop hook theo thứ tự ngược lại (graceful shutdown).
		if err := app.Stop(ctx); err != nil {
			log.Fatalf("stop application: %v", err)
		}
	}()

	service, err := injel.Get[*UserService](app)
	if err != nil {
		log.Fatalf("resolve user service: %v", err)
	}

	fmt.Println("user service resolved with repository DSN:", service.repo.dsn)
}
```

---

## Kiến trúc tổng thể

```text
Injel Application
├── Application Context (injel.App)
│   ├── Dependency Injection (di.Container)
│   │   ├── Provider Registry
│   │   ├── Dependency Graph Validation (Cycle / Missing / Duplicate)
│   │   ├── Singleton Cache
│   │   └── Explicit Interface Binding (ProvideAs / Bind)
│   ├── Configuration (config)
│   │   ├── Struct Tag Parser (`env:"KEY,opts..."`)
│   │   ├── Scalar / Slice / Struct / TextUnmarshaler Converters
│   │   └── Isolated Lookup Testing
│   └── Application Lifecycle
│       ├── StartHook (Topological Order Startup)
│       ├── StopHook (Reverse LIFO Graceful Shutdown)
│       └── Rollback on Startup Failure
└── HTTP Server (httpserver)
    ├── Router with Sandbox Pre-validation (net/http.ServeMux)
    ├── Idiomatic Middleware Chain (RequestID, Logging, Recovery)
    └── HTTP Lifecycle Integration (Non-blocking Serve & Shutdown Timeout)
```

---

## Cấu trúc repository

```text
injel/
├── README.md              # Tài liệu chính (English)
├── README.vi.md           # Tài liệu tiếng Việt
├── AUTHORS.md             # Tác giả và người duy trì
├── CITATION.cff           # Citation File Format metadata
├── doc.go                 # Package GoDoc cho root package
├── go.mod
├── injel.go               # Facade công khai: App, New, Get, Start, Stop, ProvideConfig
├── injel_test.go
├── cmd/
│   └── injel/             # Injel CLI entrypoint
│       └── main.go
├── internal/
│   ├── cli/               # Bộ điều phối lệnh CLI (new, run, doctor, version, help)
│   │   ├── cli.go
│   │   ├── help.go
│   │   ├── version.go
│   │   ├── doctor.go
│   │   ├── run.go
│   │   ├── new.go
│   │   └── cli_test.go
│   └── scaffold/          # Engine sinh mã nguồn dự án với embedded templates
│       ├── scaffold.go
│       ├── scaffold_test.go
│       └── templates/
├── config/                # Module cấu hình từ biến môi trường
│   ├── doc.go
│   ├── config.go
│   ├── errors.go
│   └── config_test.go
├── di/                    # DI engine công khai
│   ├── doc.go
│   ├── container.go
│   ├── provider.go
│   ├── errors.go
│   └── container_test.go
├── httpserver/            # HTTP server & routing module dựa trên net/http
│   ├── doc.go
│   ├── errors.go
│   ├── middleware.go
│   ├── router.go
│   ├── server.go
│   └── server_test.go
├── docs/
│   ├── product-brief.md
│   ├── roadmap.md
│   ├── branding/
│   │   └── github-metadata.md
│   ├── research/
│   │   └── code-generation.md
│   └── adr/
│       ├── 0001-dependency-injection.md
│       ├── 0002-interface-binding.md
│       ├── 0003-application-lifecycle.md
│       ├── 0004-configuration.md
│       ├── 0005-http-module.md
│       └── 0006-cli-architecture.md
└── examples/
    ├── basic/
    │   └── main.go
    └── http-api/
        └── main.go
```

---

## Injel CLI

Injel cung cấp công cụ dòng lệnh chính thức (`injel`) được xây dựng 100% bằng thư viện chuẩn Go, không kéo theo bất kỳ dependency bên thứ ba nào.

### Cài đặt và biên dịch

Biên dịch binary CLI cục bộ:

```bash
go build -o bin/injel ./cmd/injel
```

Hoặc cài đặt trực tiếp vào `$GOPATH/bin`:

```bash
go install ./cmd/injel
```

### Các lệnh CLI

- **`injel version`**: Hiển thị phiên bản, commit hash, ngày build và thông tin Go runtime.
- **`injel doctor`**: Kiểm tra môi trường Go (yêu cầu >= 1.24) và module hiện tại.
- **`injel run [args...]`**: Wrapper chạy `go run .` trong thư mục module hiện tại, hỗ trợ chuyển tiếp cờ lệnh và bắt signal dừng an toàn.
- **`injel new --name <name> --module <module>`**: Khởi tạo cấu trúc project Injel hoàn chỉnh với DI, config, HTTP routes và graceful shutdown.

### Khởi tạo project mới

Trong quá trình phát triển (kết nối với thư mục local Injel qua `go.work` tự động):

```bash
injel new \
  --name my-api \
  --module github.com/username/my-api \
  --local-injel /path/to/Injel
```

Sử dụng phiên bản Injel đã phát hành:

```bash
injel new \
  --name my-api \
  --module github.com/username/my-api \
  --injel-version v0.1.0
```

Cấu trúc dự án được tạo ra:

```text
my-api/
├── .env.example           # Cấu hình biến môi trường mẫu
├── .gitignore             # Git ignore chuẩn (tự động bỏ qua go.work)
├── README.md              # Hướng dẫn chạy và phát triển
├── go.mod                 # Khai báo Go module
├── main.go                # Khởi động ứng dụng với DI và lifecycle
└── internal/
    ├── config/            # Cấu trúc AppConfig gắn thẻ struct tag env
    │   └── config.go
    └── handler/           # Handler REST API mẫu
        └── hello.go
```

---

## Giới hạn đã biết của các module hiện tại

- Mỗi kiểu chỉ có một provider (singleton theo kiểu); chưa hỗ trợ named provider hoặc scope động.
- Provider có thể có nhiều tham số và trả về `T` hoặc `(T, error)`.
- Reflection chỉ diễn ra khi resolve ở startup, không diễn ra trên mỗi request.
- Interface binding yêu cầu khai báo tường minh qua `ProvideAs` hoặc `Bind`.
- Configuration tập trung vào Environment variables và struct tag `env`; chưa tự động đọc file YAML/JSON hay remote secrets.
- Constructor không nên gọi ngược lại container trong quá trình resolve.
- HTTP server tận dụng `net/http.ServeMux` tiêu chuẩn của Go 1.22+ (method matching, path parameters); chưa tích hợp request body auto-validation hoặc OpenAPI schema generator.

---

## Module path

Module chính thức của dự án:

```go
module github.com/Injel-Framework/Injel
```

---

## Creator & Maintainer

**Phạm Trương Thông (TogPam)**

Creator and maintainer of Injel Framework.

GitHub: [@TogPam](https://github.com/TogPam)

Injel is an independently developed Go backend framework focused on developer experience, dependency injection, application lifecycle and native Go simplicity.

---

## Tài liệu tham khảo

- [`AUTHORS.md`](AUTHORS.md) — danh sách tác giả và người duy trì.
- [`CITATION.cff`](CITATION.cff) — định dạng trích dẫn phần mềm chuẩn CFF.
- [`docs/product-brief.md`](docs/product-brief.md) — định hướng sản phẩm, phạm vi MVP, tiêu chí thành công.
- [`docs/roadmap.md`](docs/roadmap.md) — lộ trình từ core tới HTTP, configuration và CLI.
- [`docs/branding/github-metadata.md`](docs/branding/github-metadata.md) — metadata, description và topics trên GitHub.
- [`docs/research/code-generation.md`](docs/research/code-generation.md) — nghiên cứu: Compile-time DI Codegen vs Startup Reflection.
- [`docs/adr/0001-dependency-injection.md`](docs/adr/0001-dependency-injection.md) — quyết định kiến trúc DI.
- [`docs/adr/0002-interface-binding.md`](docs/adr/0002-interface-binding.md) — quyết định kiến trúc Interface Binding.
- [`docs/adr/0003-application-lifecycle.md`](docs/adr/0003-application-lifecycle.md) — quyết định kiến trúc Application Lifecycle.
- [`docs/adr/0004-configuration.md`](docs/adr/0004-configuration.md) — quyết định kiến trúc Configuration Module.
- [`docs/adr/0005-http-module.md`](docs/adr/0005-http-module.md) — quyết định kiến trúc HTTP Server & REST API Module.
- [`docs/adr/0006-cli-architecture.md`](docs/adr/0006-cli-architecture.md) — quyết định kiến trúc Injel CLI & Project Scaffolding.
