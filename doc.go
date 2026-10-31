// Package injel provides application construction, dependency injection,
// configuration management, and application lifecycle control for Go backend services.
//
// Injel is inspired by the developer experience of Spring Boot while strictly
// embracing native Go idioms, explicit dependency declaration, and zero external dependencies.
//
// # Core Concepts
//
// An Injel application revolves around the [App] facade, which integrates:
//   - Dependency Injection: constructor-based provider functions with cycle detection,
//     missing dependency detection, duplicate detection, and explicit interface binding.
//   - Configuration: automatic unmarshaling of environment variables into typed Go structs
//     using the struct tag `env:"KEY,opt..."` via [ProvideConfig].
//   - Application Lifecycle: ordered startup (topological dependency order) and graceful
//     shutdown (reverse LIFO order) via [StartHook] and [StopHook].
//
// # Quick Example
//
//	package main
//
//	import (
//		"context"
//		"log"
//
//		"github.com/Injel-Framework/Injel"
//	)
//
//	type AppConfig struct {
//		Port int `env:"PORT,default=8080"`
//	}
//
//	type Database struct{}
//
//	func NewDatabase(cfg *AppConfig) *Database {
//		return &Database{}
//	}
//
//	func main() {
//		app := injel.New()
//
//		// Bind environment configuration
//		if err := injel.ProvideConfig[AppConfig](app); err != nil {
//			log.Fatal(err)
//		}
//
//		// Register providers
//		if err := app.Provide(NewDatabase); err != nil {
//			log.Fatal(err)
//		}
//
//		ctx := context.Background()
//		if err := app.Start(ctx); err != nil {
//			log.Fatal(err)
//		}
//		defer app.Stop(ctx)
//	}
package injel
