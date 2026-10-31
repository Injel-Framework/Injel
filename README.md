# Injel — A Go (Golang) Backend Framework

**English** | [Tiếng Việt](README.vi.md)

[![CI](https://github.com/Injel-Framework/Injel/actions/workflows/ci.yml/badge.svg)](https://github.com/Injel-Framework/Injel/actions)
[![Go Version](https://img.shields.io/badge/go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![GitHub Repository](https://img.shields.io/badge/github-Injel--Framework%2FInjel-blue?logo=github)](https://github.com/Injel-Framework/Injel)

> **Spring-inspired developer experience. Native Go simplicity.**

Injel is a Go backend framework inspired by Spring Boot, designed to simplify dependency injection, configuration, HTTP development, and application lifecycle management while preserving native Go simplicity.

---

## What is Injel?

**Injel** (*Injection + Element*) is a lightweight, modern Go backend framework that provides a cohesive foundation for building robust REST APIs and microservices.

In standard Go development, assembling an application requires stitching together disparate libraries for routing, dependency injection, configuration, and lifecycle hooks. As projects grow:
- Initialization boilerplate multiplies across `main.go` and constructor chains.
- Missing dependencies and circular dependency cycles are discovered only at runtime.
- Proper startup sequencing and graceful resource shutdown are difficult to coordinate.
- Every project reinvents its own wiring conventions.

Injel solves these problems by providing:
1. **Predictable IoC Container:** Clean, constructor-based dependency injection with early dependency graph validation.
2. **Type-Safe Environment Configuration:** Automatic mapping of environment variables into Go structs using struct tags.
3. **Structured Application Lifecycle:** Deterministic startup ordering, automatic rollback on initialization failure, and graceful shutdown.
4. **Standard-Library HTTP Engine:** High-performance HTTP routing, sandbox route validation, and idiomatic middleware built directly on `net/http` without third-party router dependencies.

---

## Core Features

- **Constructor-Based Dependency Injection (DI):** Register regular Go constructors (`func NewService(repo Repository) (*Service, error)`). Injel analyzes parameters and return types at startup and resolves singletons on demand.
- **Explicit Interface Binding:** Bind concrete implementations to interface abstractions (`ProvideAs`, `Bind`) while sharing a single underlying instance.
- **Fail-Fast Graph Validation:** Validates the entire dependency graph—detecting cycles, missing dependencies, and duplicate providers—without side effects before the application starts serving requests.
- **Structured Application Lifecycle:** Components implement `StartHook` and `StopHook`. Startup proceeds in topological dependency order; shutdown executes in reverse (LIFO) order with graceful context timeouts.
- **Startup Failure Rollback:** If any component fails to start, previously initialized components are automatically rolled back and cleaned up in reverse order.
- **Environment Configuration Module:** Read environment variables into strongly typed structs using `env:"KEY,opts..."` tags with support for `default`, `required`, scalar types, slices, durations, and `encoding.TextUnmarshaler`.
- **HTTP Server & REST Routing:** Built entirely on Go standard library `net/http` (Go 1.22+ `net/http.ServeMux`) with path variables (`r.PathValue("id")`) and zero external dependencies.
- **Router Sandbox Pre-validation:** Catches route syntax errors and pattern conflicts early in an isolated sandbox mux, returning structured errors instead of panicking.
- **Idiomatic Middleware Chain:** Standard Go middleware signature (`func(http.Handler) http.Handler`) featuring built-in `RequestID` (with injection protection), structured `Logging`, and `Recovery` (capturing panics and returning HTTP 500 JSON).
- **Database Integration (`database/sql`):** Connect any SQL database through the `database` package. Registers `*sql.DB` as a DI singleton, pings on startup, and closes on graceful shutdown. Zero external dependencies — drivers are managed by the application.

---

## Database Integration

The `database` package provides first-class `database/sql` integration with Injel's DI container and application lifecycle.

### Quick Start

```go
import (
    _ "github.com/lib/pq"  // import your driver — managed by your app, not Injel

    injel "github.com/Injel-Framework/Injel"
    "github.com/Injel-Framework/Injel/database"
)

func main() {
    app := injel.New()

    // Register *sql.DB with DI and lifecycle hooks
    if err := database.Provide(app, database.Config{
        DriverName: "postgres",
        DSN:        os.Getenv("DATABASE_URL"),
        // Optional pool settings (defaults shown):
        MaxOpenConns:    25,
        MaxIdleConns:    5,
        ConnMaxLifetime: 30 * time.Minute,
        ConnMaxIdleTime: 5 * time.Minute,
        PingTimeout:     5 * time.Second,
    }); err != nil {
        log.Fatal(err)
    }

    // Inject *sql.DB into any component via constructor
    app.Provide(NewUserRepository)

    // app.Start → pings database; fails fast if unreachable
    // app.Stop  → closes connection pool gracefully
}

// Repository receives *sql.DB through Injel DI
func NewUserRepository(db *sql.DB) *UserRepository {
    return &UserRepository{db: db}
}
```

### What Injel provides

| Feature | Details |
|---------|---------|
| DI singleton | All components injecting `*sql.DB` share the same connection pool |
| Startup ping | `PingContext` with configurable timeout; startup fails if unreachable |
| Graceful shutdown | `db.Close()` on `app.Stop()`, idempotent |
| Config validation | Fails fast on missing driver/DSN, negative pool values, constraint violations |
| Error wrapping | `*ConfigError`, `*OpenError`, `*PingError`, `*CloseError` — all support `errors.Is`/`errors.As` |
| No credentials in errors | DSN/password never appears in error messages or logs |

### What Injel does NOT provide

- ORM, query builder, or schema migration — use sqlx, bun, squirrel, or any SQL library you prefer.
- Built-in database driver — import the driver you need in your application.
- Multiple named databases — currently only one `*sql.DB` per application context.

### Supported drivers (examples)

The following are application-level dependencies, not framework dependencies:

```go
import _ "github.com/lib/pq"                   // PostgreSQL
import _ "github.com/go-sql-driver/mysql"       // MySQL
import _ "github.com/mattn/go-sqlite3"          // SQLite (CGO)
import _ "modernc.org/sqlite"                   // SQLite (pure Go)
import _ "github.com/jackc/pgx/v5/stdlib"      // pgx PostgreSQL
```

---

## Architecture

```text
Injel Application (injel.App)
├── Inversion of Control & DI Engine (di.Container)
│   ├── Provider Registry (func(...) T / func(...) (T, error))
│   ├── Graph Validation (Cycle, Missing, Duplicate Detection)
│   ├── Singleton Cache (Reflection solely at startup)
│   └── Explicit Interface Binding (ProvideAs / Bind)
├── Environment Configuration (config)
│   ├── Struct Tag Parsing (`env:"KEY,default=...,required"`)
│   ├── Type Conversions (Scalars, Slices, Durations, Unmarshalers)
│   └── Isolated Lookup for Testing (WithLookup / WithMap)
├── Application Lifecycle Management
│   ├── StartHook (Topological Dependency Ordering)
│   ├── StopHook (Reverse LIFO Graceful Shutdown)
│   └── Automatic Rollback on Startup Failure
└── HTTP Server & REST API Toolkit (httpserver)
    ├── Router with Sandbox Pre-validation (net/http.ServeMux)
    ├── Idiomatic Middleware Chain (RequestID -> Logging -> Recovery)
    └── Lifecycle Hooks (Non-blocking Serve & Graceful Drain)
```

---

## Quick Start

### 1. Minimal Application with DI and Lifecycle

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Injel-Framework/Injel"
)

type DatabaseConfig struct {
	DSN string `env:"DATABASE_URL,default=file:app.db"`
}

type Database struct {
	dsn string
}

func (db *Database) OnStart(ctx context.Context) error {
	fmt.Println("Database: connected to", db.dsn)
	return nil
}

func (db *Database) OnStop(ctx context.Context) error {
	fmt.Println("Database: connection closed")
	return nil
}

func NewDatabase(cfg *DatabaseConfig) *Database {
	return &Database{dsn: cfg.DSN}
}

type UserService struct {
	db *Database
}

func NewUserService(db *Database) *UserService {
	return &UserService{db: db}
}

func main() {
	app := injel.New()

	// 1. Bind environment configuration
	if err := injel.ProvideConfig[DatabaseConfig](app); err != nil {
		log.Fatalf("provide config: %v", err)
	}

	// 2. Register constructor providers
	if err := app.Provide(NewDatabase); err != nil {
		log.Fatalf("provide database: %v", err)
	}
	if err := app.Provide(NewUserService); err != nil {
		log.Fatalf("provide user service: %v", err)
	}

	// 3. Start application (validates graph, loads config, starts lifecycle)
	ctx := context.Background()
	if err := app.Start(ctx); err != nil {
		log.Fatalf("start application: %v", err)
	}
	defer func() {
		if err := app.Stop(ctx); err != nil {
			log.Fatalf("stop application: %v", err)
		}
	}()

	svc, err := injel.Get[*UserService](app)
	if err != nil {
		log.Fatalf("get user service: %v", err)
	}
	fmt.Println("Application ready! Service using DB:", svc.db.dsn)
}
```

### 2. Building a REST API Server

```go
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Injel-Framework/Injel"
	"github.com/Injel-Framework/Injel/httpserver"
)

type ServerConfig struct {
	Port int `env:"PORT,default=8080"`
}

type UserHandler struct{}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": id, "name": "Alice"})
}

func NewHTTPServer(cfg *ServerConfig, h *UserHandler) (*httpserver.Server, error) {
	server := httpserver.New(
		httpserver.WithConfig(httpserver.Config{
			Addr: fmt.Sprintf(":%d", cfg.Port),
		}),
		httpserver.WithMiddleware(
			httpserver.RequestID(),
			httpserver.Logging(log.Printf),
			httpserver.Recovery(log.Printf),
		),
	)

	_ = server.RegisterHealth() // GET /health
	if err := server.HandleFunc("GET /users/{id}", h.GetUser); err != nil {
		return nil, err
	}
	return server, nil
}
```

---

## Project Structure

```text
injel/
├── README.md              # Primary documentation (English)
├── README.vi.md           # Vietnamese documentation
├── AUTHORS.md             # Authors and project maintainers
├── CITATION.cff           # Software citation metadata
├── doc.go                 # Root package documentation
├── go.mod                 # Go module definition (zero dependencies)
├── injel.go               # Public application facade
├── injel_test.go          # Application integration tests
├── cmd/
│   └── injel/             # Injel CLI entrypoint
│       └── main.go
├── internal/
│   ├── cli/               # CLI commands: help, version, doctor, run, new
│   │   ├── cli.go
│   │   ├── help.go
│   │   ├── version.go
│   │   ├── doctor.go
│   │   ├── run.go
│   │   ├── new.go
│   │   └── cli_test.go
│   └── scaffold/          # Project scaffolding engine with embedded templates
│       ├── scaffold.go
│       ├── scaffold_test.go
│       └── templates/
├── config/                # Environment configuration module
│   ├── doc.go
│   ├── config.go          # Struct tag reflection & type conversion
│   ├── errors.go
│   └── config_test.go
├── di/                    # Dependency injection engine
│   ├── doc.go
│   ├── container.go       # Provider registry, singleton cache, interface binding
│   ├── provider.go
│   ├── errors.go
│   └── container_test.go
├── httpserver/            # HTTP server & routing module
│   ├── doc.go
│   ├── errors.go
│   ├── middleware.go      # RequestID, Logging, Recovery middlewares
│   ├── router.go          # ServeMux wrapper with sandbox pre-validation
│   ├── server.go          # HTTP server with StartHook/StopHook lifecycle
│   └── server_test.go
├── docs/
│   ├── product-brief.md   # Product brief and design philosophy
│   ├── roadmap.md         # Framework development roadmap
│   ├── branding/
│   │   └── github-metadata.md
│   ├── research/          # Research and architectural evaluations
│   │   └── code-generation.md
│   └── adr/               # Architecture Decision Records
│       ├── 0001-dependency-injection.md
│       ├── 0002-interface-binding.md
│       ├── 0003-application-lifecycle.md
│       ├── 0004-configuration.md
│       ├── 0005-http-module.md
│       └── 0006-cli-architecture.md
└── examples/
    ├── basic/             # DI, configuration, and lifecycle example
    │   └── main.go
    └── http-api/          # Full REST API server example
        └── main.go
```

---

## Injel CLI

Injel provides an official CLI tool (`injel`) built entirely with standard Go libraries and zero external dependencies.

### Installation & Build

Build the CLI binary locally:

```bash
go build -o bin/injel ./cmd/injel
```

Or install it into your `$GOPATH/bin`:

```bash
go install ./cmd/injel
```

### CLI Commands

- **`injel version`**: Displays version, commit hash, build date, and Go runtime information.
- **`injel doctor`**: Verifies that the Go toolchain is installed (Go >= 1.24) and detects the current module.
- **`injel run [args...]`**: Thin, safe runner executing `go run .` in the current Go module with signal handling.
- **`injel new --name <name> --module <module>`**: Scaffolds a new, idiomatic Injel application using embedded templates.

### Creating a New Project

During development (linking to a local Injel clone via an auto-ignored `go.work`):

```bash
injel new \
  --name my-api \
  --module github.com/username/my-api \
  --local-injel /path/to/Injel
```

Using a published Injel release version:

```bash
injel new \
  --name my-api \
  --module github.com/username/my-api \
  --injel-version v0.1.0
```

The generated project is fully configured and ready to run:

```text
my-api/
├── .env.example           # Pre-configured environment variables
├── .gitignore             # Standard Go gitignore (ignores go.work)
├── README.md              # Project quickstart instructions
├── go.mod                 # Go module definition
├── main.go                # Application entrypoint with DI and lifecycle
└── internal/
    ├── config/            # Strongly typed AppConfig struct with env tags
    │   └── config.go
    └── handler/           # Sample REST API handler
        └── hello.go
```

---

## Why Injel?

| Principle | Injel Approach |
| --- | --- |
| **Native Go Conventions** | Uses ordinary Go constructors, standard interfaces, and idiomatic error handling. No code generation required. |
| **Explicit Wiring** | Dependencies are clearly visible in function signatures. No hidden package scanning or global variables. |
| **Fail-Fast Safety** | Graph cycles, missing components, duplicate providers, and conflicting routes are caught immediately at startup. |
| **Zero External Dependencies** | Core, DI, configuration, and HTTP modules rely exclusively on the Go standard library. |
| **Startup-Only Reflection** | Reflection occurs once during application startup. No reflection on the HTTP request execution path. |
| **Graceful Lifecycle** | Deterministic topological startup, LIFO shutdown, and automatic cleanup rollback on failure. |

---

## Requirements & Testing

Injel requires **Go 1.24** or higher.

To run the full test suite with race detection:

```bash
go test ./...
go test -race ./...
go vet ./...
```

To run the included runnable examples:

```bash
# Basic DI, configuration, and lifecycle demo
go run ./examples/basic

# Full REST API server demo with HTTP routing and middleware
go run ./examples/http-api
```

---

## Creator & Maintainer

**Phạm Trương Thông (TogPam)**

Creator and maintainer of Injel Framework.

GitHub: [@TogPam](https://github.com/TogPam)

Injel is an independently developed Go backend framework focused on developer experience, dependency injection, application lifecycle and native Go simplicity.

---

## Documentation & References

- [`README.vi.md`](README.vi.md) — Vietnamese documentation.
- [`AUTHORS.md`](AUTHORS.md) — Authors and maintainers.
- [`CITATION.cff`](CITATION.cff) — Software citation metadata.
- [`docs/product-brief.md`](docs/product-brief.md) — Product brief and design philosophy.
- [`docs/roadmap.md`](docs/roadmap.md) — Framework roadmap and milestones.
- [`docs/branding/github-metadata.md`](docs/branding/github-metadata.md) — GitHub metadata, topics, and description.
- [`docs/research/code-generation.md`](docs/research/code-generation.md) — Research: Compile-time DI Codegen vs Startup Reflection.
- [`docs/adr/0001-dependency-injection.md`](docs/adr/0001-dependency-injection.md) — ADR: Dependency Injection.
- [`docs/adr/0002-interface-binding.md`](docs/adr/0002-interface-binding.md) — ADR: Interface Binding.
- [`docs/adr/0003-application-lifecycle.md`](docs/adr/0003-application-lifecycle.md) — ADR: Application Lifecycle.
- [`docs/adr/0004-configuration.md`](docs/adr/0004-configuration.md) — ADR: Configuration Module.
- [`docs/adr/0005-http-module.md`](docs/adr/0005-http-module.md) — ADR: HTTP Server & REST API.
- [`docs/adr/0006-cli-architecture.md`](docs/adr/0006-cli-architecture.md) — ADR: Injel CLI Architecture & Scaffolding.

---

## Contributing & License

Injel is currently under active foundational development. Contributions, bug reports, and suggestions are welcome via [GitHub Issues](https://github.com/Injel-Framework/Injel/issues) and Pull Requests.
