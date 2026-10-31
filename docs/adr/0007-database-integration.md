# ADR 0007 — Database Integration (`database/sql`)

- **Trạng thái:** Accepted
- **Ngày:** 01/11/2026
- **Tác giả & Project Maintainer:** Phạm Trương Thông (@TogPam)
- **Người quyết định:** Phạm Trương Thông
- **Phạm vi:** Database Integration, Connection Pooling, Lifecycle Management

---

## 1. Bối cảnh

Sau khi hoàn thành DI core (Phase 1), Application Lifecycle (Phase 2), Configuration (Phase 3), HTTP Server (Phase 4) và CLI Tooling (Phase 5), Injel cần tích hợp cơ sở dữ liệu.

Hầu hết ứng dụng backend cần kết nối với một database quan hệ. Vấn đề cần giải quyết:

- Cách kết nối database được khai báo và quản lý qua DI.
- Cách đảm bảo kết nối được kiểm tra trước khi ứng dụng bắt đầu nhận request.
- Cách đóng connection pool sạch sẽ khi ứng dụng dừng.
- Cách làm điều trên mà không ràng buộc người dùng với một ORM hoặc driver cụ thể.

---

## 2. Các phương án đã cân nhắc

### Phương án A — Tích hợp GORM hoặc Ent

**Ưu điểm:** Cung cấp ORM đầy đủ tính năng, code generation, migration.

**Nhược điểm:**
- Thêm external dependency nặng vào core framework.
- Vi phạm nguyên tắc "Core nhỏ, module rời" của Injel.
- Ràng buộc người dùng với model conventions của ORM.
- Khó tách ORM ra khỏi core sau khi đã tích hợp.

### Phương án B — Wrapper riêng bao quanh `database/sql`

**Ưu điểm:** Kiểm soát hoàn toàn API.

**Nhược điểm:**
- Người dùng phải học API mới thay vì `*sql.DB` chuẩn.
- Không tương thích với các thư viện SQL thuần (sqlx, squirrel, bun, v.v.).
- Tạo thêm abstraction không cần thiết.

### Phương án C — Tích hợp trực tiếp `database/sql` *(Được chọn)*

**Ưu điểm:**
- `database/sql` là thư viện chuẩn, không thêm dependency nào.
- `*sql.DB` là interface quen thuộc với mọi Go developer.
- Người dùng tự chọn driver (SQLite, PostgreSQL, MySQL, v.v.).
- Tương thích tự nhiên với toàn bộ hệ sinh thái SQL của Go.
- Core framework không biết về driver cụ thể nào.

**Nhược điểm:**
- Không cung cấp query building, migration, schema management — người dùng tự lo.
- Người dùng phải import driver trong application code.

---

## 3. Quyết định

Tích hợp `database/sql` thông qua package `database` với các đặc điểm:

### 3.1. Không có ORM, không có query builder

Injel không cung cấp ORM, không có annotation, không có struct tag mapping tự động. Người dùng viết SQL thuần hoặc dùng thư viện SQL bên ngoài (sqlx, squirrel, bun) theo lựa chọn của họ.

### 3.2. Driver do người dùng quản lý

Injel không import bất kỳ database driver nào vào `go.mod`. Người dùng tự import driver phù hợp trong application code:

```go
import _ "github.com/lib/pq"          // PostgreSQL
import _ "github.com/mattn/go-sqlite3" // SQLite
import _ "github.com/go-sql-driver/mysql" // MySQL
```

Điều này đảm bảo:
- `go.mod` của Injel không phình to vì driver.
- Không conflict license (một số driver có CGO hoặc license riêng).
- Người dùng hoàn toàn kiểm soát phiên bản driver.

### 3.3. DI integration

`database.Provide(app, cfg)` là API duy nhất để đăng ký database. Nó:

1. Validate cấu hình.
2. Gọi `sql.Open` để tạo `*sql.DB` handle (không ping tại bước này).
3. Cấu hình connection pool.
4. Đăng ký provider `func() *sql.DB` vào DI container.
5. Đăng ký lifecycle hook.

Kết quả: mọi component inject `*sql.DB` nhận cùng một singleton instance.

```go
func NewUserRepository(db *sql.DB) *UserRepository {
    return &UserRepository{db: db}
}
```

### 3.4. Lifecycle: Startup Health Check

Khi `app.Start(ctx)` được gọi, `OnStart(ctx)` ping database với timeout:

```go
pingCtx, cancel := context.WithTimeout(ctx, cfg.PingTimeout)
defer cancel()
err := db.PingContext(pingCtx)
```

- **Nếu ping thành công:** Application tiếp tục khởi động.
- **Nếu ping thất bại:** Database handle được đóng, lỗi được wrap trong `*PingError` và trả về. Application rollback (gọi `OnStop` các component đã start trước đó).
- **DSN không bao giờ xuất hiện trong error message.**

### 3.5. Lifecycle: Graceful Shutdown

Khi `app.Stop(ctx)` được gọi (theo thứ tự LIFO), `OnStop(ctx)` đóng database:

```go
err := db.Close()
```

- Idempotent: gọi nhiều lần không gây double-close.
- Nếu `db.Close()` trả lỗi, được wrap trong `*CloseError`.
- Không cần goroutine thêm.

### 3.6. Connection Pool Management

Cấu hình pool qua `Config`:

| Trường | Mặc định | Ý nghĩa |
|--------|----------|---------|
| `MaxOpenConns` | 25 | Số connection tối đa |
| `MaxIdleConns` | 5 | Số idle connection tối đa |
| `ConnMaxLifetime` | 30 phút | Thời gian tối đa một connection được tái dùng |
| `ConnMaxIdleTime` | 5 phút | Thời gian tối đa một connection idle trước khi đóng |
| `PingTimeout` | 5 giây | Timeout cho startup ping |

---

## 4. Hệ quả

### Tích cực
- Không thêm dependency ngoài vào `go.mod`.
- API nhỏ gọn: một hàm `database.Provide(app, cfg)`.
- Tương thích với mọi driver và thư viện SQL của Go.
- Lifecycle được tích hợp tự nhiên với cơ chế hiện có.
- Errors có context, có thể unwrap qua `errors.Is`/`errors.As`.

### Tiêu cực / Giới hạn hiện tại
- Không hỗ trợ multiple databases (nhiều `*sql.DB` trong cùng container) — chỉ một provider cho mỗi type.
- Không có migration management.
- Không có query building.
- Không có health endpoint tự động cho database (dùng HTTP `/health` riêng nếu cần).
- Named providers (ví dụ: primary DB + replica DB) chưa hỗ trợ vì DI container hiện tại không có named bindings.

### Mở trong tương lai
- Named database providers khi DI hỗ trợ named bindings.
- Health check endpoint cho database status.
- Transaction helper (nếu có use case rõ ràng).

---

## 5. Ví dụ sử dụng

```go
// main.go của application (không phải framework)
import (
    _ "github.com/lib/pq"                        // driver do app quản lý
    injel "github.com/Injel-Framework/Injel"
    "github.com/Injel-Framework/Injel/database"
)

func main() {
    app := injel.New()

    if err := database.Provide(app, database.Config{
        DriverName: "postgres",
        DSN:        os.Getenv("DATABASE_URL"),
    }); err != nil {
        log.Fatal(err)
    }

    if err := app.Provide(NewUserRepository); err != nil {
        log.Fatal(err)
    }

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()

    if err := app.Start(ctx); err != nil {
        log.Fatal(err)
    }
    <-ctx.Done()

    stopCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
    defer cancel()
    app.Stop(stopCtx)
}

// Repository nhận *sql.DB qua DI
func NewUserRepository(db *sql.DB) *UserRepository {
    return &UserRepository{db: db}
}
```

---

## 6. Các quyết định liên quan

- ADR 0001: DI Core — Provider function là API chính.
- ADR 0003: Application Lifecycle — StartHook/StopHook pattern.
- ADR 0004: Configuration — Config struct từ environment.
