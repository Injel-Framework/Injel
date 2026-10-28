# Injel

**Spring-inspired developer experience. Native Go simplicity.**

Injel (Injection + Element) là framework backend viết bằng Go, lấy cảm hứng từ mô hình phát triển của Java Spring Boot nhưng giữ cách thiết kế và đặc tính của Go. Mục tiêu của Injel không phải mô phỏng Java trong Go, mà là giảm boilerplate khi kết nối các thành phần của một ứng dụng backend, đồng thời giữ luồng chạy rõ ràng, dễ đọc và dễ debug.

> Tài liệu đầu vào của dự án từng mang tên **GoVa** và **Tertex** trong các tài liệu trước đây. Tên chính thức hiện tại của sản phẩm là **Injel**; các tài liệu lịch sử vẫn có thể lưu giữ tên cũ.

## Vấn đề cần giải quyết

Một ứng dụng Go nhiều thành phần thường phải tự lựa chọn và kết nối router, cấu hình, logging, database, validation và vòng đời ứng dụng. Khi số lượng thành phần tăng, code khởi tạo trở nên dài, lặp lại và khó kiểm tra:

- Không có quy ước thống nhất để đăng ký và lấy dependency.
- Lỗi thiếu dependency hoặc chu trình dependency thường chỉ xuất hiện khi chạy.
- Việc dựng và dừng tài nguyên theo đúng thứ tự dễ bị bỏ sót.
- Mỗi dự án giải quyết cùng một nhóm nhu cầu theo một cách khác nhau.

Injel hướng tới một mô hình thống nhất cho các vấn đề trên, bắt đầu từ Dependency Injection (DI) rõ ràng và có kiểm tra sớm.

## Nguyên tắc thiết kế

1. **Provider function là API chính.** Dependency được khai báo qua tham số và kết quả của hàm Go thông thường.
2. **Lỗi xuất hiện sớm.** Provider trùng, dependency thiếu và chu trình được phát hiện trước khi ứng dụng phục vụ request.
3. **Không giấu luồng chạy.** Không package scanning ngầm, không global container, không annotation giả lập.
4. **Tự động hóa phần lặp lại, không che giấu phần quan trọng.** Abstraction chỉ được thêm khi có nhu cầu thực tế.
5. **Core nhỏ, module rời.** Core không phụ thuộc ORM, message broker hay dịch vụ ngoài.
6. **Không tuyên bố hiệu năng khi chưa đo.** Benchmark chỉ có ý nghĩa khi implementation ổn định.

## Trạng thái phát triển

Dự án đang ở giai đoạn đầu. Không có tính năng nào dưới đây được quảng cáo ngoài những gì đã thực sự có test.

| Thành phần | Trạng thái | Ghi chú |
| --- | --- | --- |
| Tài liệu sản phẩm, roadmap, ADR | Implemented | Xem `docs/` |
| DI container: provider function, singleton, resolve theo kiểu | Implemented | Reflection tại thời điểm resolve; không chạy trên mỗi request |
| Phát hiện provider trùng, dependency thiếu, chu trình | Implemented | Có lỗi có ngữ cảnh và test |
| Constructor trả `error` | Implemented | Lỗi được bọc với tên provider và kiểu kết quả |
| Interface binding (`ProvideAs`, `Bind`) | Implemented | Singleton dùng chung giữa concrete type và interface |
| Application lifecycle (start/stop, rollback, graceful shutdown) | Implemented | Thứ tự topo theo đồ thị dependency; rollback khi start lỗi |
| Configuration từ environment | Planned | Chưa triển khai |
| HTTP module trên `net/http` | Planned | Chưa triển khai |
| CLI / code generation | Planned | Chưa triển khai |
| Tích hợp database, Redis, broker | Planned | Ngoài phạm vi MVP hiện tại |

## Yêu cầu môi trường

- Go 1.24 trở lên (phiên bản đã kiểm tra: Go 1.27.1).

## Cài đặt và chạy test

```bash
go test ./...
go vet ./...
go test -race ./...
```

Chạy ví dụ:

```bash
go run ./examples/basic
```

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
	DSN string
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

func NewConfig() *Config {
	return &Config{DSN: "file:injel.db"}
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

	for _, provider := range []any{
		NewConfig,
		NewUserRepository,
		NewUserService,
	} {
		if err := app.Provide(provider); err != nil {
			log.Fatalf("register provider: %v", err)
		}
	}

	ctx := context.Background()

	// Start kiểm tra graph, resolve dependency theo thứ tự topo và chạy OnStart hook.
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

## Cấu trúc repository

```text
injel/
├── README.md
├── go.mod
├── injel.go               # Facade công khai: App, New, Get, Start, Stop, Lifecycle
├── injel_test.go
├── di/                    # DI engine công khai
│   ├── container.go
│   ├── provider.go
│   ├── errors.go
│   └── container_test.go
├── docs/
│   ├── product-brief.md
│   ├── roadmap.md
│   └── adr/
│       ├── 0001-dependency-injection.md
│       ├── 0002-interface-binding.md
│       └── 0003-application-lifecycle.md
└── examples/
    └── basic/
        └── main.go
```

## Giới hạn đã biết của DI & Lifecycle hiện tại

- Mỗi kiểu chỉ có một provider (singleton theo kiểu); chưa hỗ trợ named provider hoặc scope động.
- Provider có thể có nhiều tham số và trả về `T` hoặc `(T, error)`.
- Reflection chỉ diễn ra khi resolve ở startup, không diễn ra trên mỗi request.
- Interface binding yêu cầu khai báo tường minh qua `ProvideAs` hoặc `Bind`.
- Constructor không nên gọi ngược lại container trong quá trình resolve.

## Module path

Module chính thức của dự án:

```go
module github.com/Injel-Framework/Injel
```

## Tài liệu

- [`docs/product-brief.md`](docs/product-brief.md) — định hướng sản phẩm, phạm vi MVP, tiêu chí thành công.
- [`docs/roadmap.md`](docs/roadmap.md) — lộ trình từ core tới HTTP, configuration và CLI.
- [`docs/adr/0001-dependency-injection.md`](docs/adr/0001-dependency-injection.md) — quyết định kiến trúc DI.
- [`docs/adr/0002-interface-binding.md`](docs/adr/0002-interface-binding.md) — quyết định kiến trúc Interface Binding.
- [`docs/adr/0003-application-lifecycle.md`](docs/adr/0003-application-lifecycle.md) — quyết định kiến trúc Application Lifecycle.
