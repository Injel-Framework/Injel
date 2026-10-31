// Package di implements a constructor-based dependency injection container
// and inversion of control (IoC) engine for Go.
//
// The container resolves components by exact type and manages singletons across
// an application. Reflection is utilized solely at startup during graph resolution,
// ensuring zero reflection overhead during request handling.
//
// # Features
//
//   - Constructor Provider Functions: Supports `func(...) T` and `func(...) (T, error)`.
//   - Early Graph Validation: [Container.Validate] inspects missing dependencies, duplicate
//     providers, and circular dependency cycles without invoking constructors.
//   - Explicit Interface Binding: Supports [Container.ProvideAs] and [Container.Bind]
//     to map concrete implementations to interface abstractions sharing the same singleton instance.
//   - Thread-safe Resolution: Concurrent resolution calls are safely synchronized.
//   - Rich Contextual Errors: Clear error paths detailing provider names and missing types.
package di
