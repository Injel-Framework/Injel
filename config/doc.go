// Package config provides strongly typed configuration loading from environment
// variables into Go structs using struct tags.
//
// # Struct Tags
//
// Fields in the target struct use the `env` tag:
//
//	type DatabaseConfig struct {
//		Host     string        `env:"DB_HOST,default=127.0.0.1"`
//		Port     int           `env:"DB_PORT,default=5432"`
//		Password string        `env:"DB_PASSWORD,required"`
//		Timeout  time.Duration `env:"DB_TIMEOUT,default=5s"`
//		Tags     []string      `env:"DB_TAGS,default=master,replica"`
//	}
//
// # Supported Types
//
//   - Strings, booleans, signed/unsigned integers (int8..int64, uint8..uint64), floats (float32, float64).
//   - [time.Duration] parsed via [time.ParseDuration].
//   - Slices of supported scalar types (comma-separated).
//   - Nested structs and pointers to nested structs.
//   - Custom types implementing [encoding.TextUnmarshaler].
//
// # Isolated Testing
//
// Use [WithMap] or [WithLookup] to load configurations from mock data sources without
// modifying host environment variables.
package config
