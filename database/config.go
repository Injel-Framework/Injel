// Package database provides database/sql integration for the Injel Framework.
// It manages connection pooling, lifecycle hooks and graceful shutdown
// without imposing any ORM or specific database driver.
//
// Drivers must be registered by the application, not by this package:
//
//	import _ "github.com/mattn/go-sqlite3" // register driver in your app
package database

import (
	"fmt"
	"time"
)

// Default connection pool and timeout values.
const (
	// DefaultPingTimeout is the timeout used for the startup health check.
	DefaultPingTimeout = 5 * time.Second

	// DefaultMaxOpenConns is the maximum number of open connections to the database.
	DefaultMaxOpenConns = 25

	// DefaultMaxIdleConns is the maximum number of idle connections in the pool.
	DefaultMaxIdleConns = 5

	// DefaultConnMaxLifetime is the maximum amount of time a connection may be reused.
	DefaultConnMaxLifetime = 30 * time.Minute

	// DefaultConnMaxIdleTime is the maximum amount of time a connection may be idle.
	DefaultConnMaxIdleTime = 5 * time.Minute
)

// Config holds the configuration for a database connection.
//
// DriverName and DSN are required. All other fields have sensible defaults
// applied automatically via applyDefaults.
//
// Important: DSN often contains sensitive credentials. Injel never logs or
// includes DSN in error messages.
type Config struct {
	// DriverName is the name of the registered database driver (e.g. "sqlite3", "mysql", "postgres").
	// Required.
	DriverName string

	// DSN is the data source name passed to sql.Open.
	// Required. Never logged or included in error output.
	DSN string

	// MaxOpenConns sets the maximum number of open connections to the database.
	// 0 means unlimited. Must not be negative.
	MaxOpenConns int

	// MaxIdleConns sets the maximum number of idle connections in the connection pool.
	// Must not be negative. Must not exceed MaxOpenConns when MaxOpenConns > 0.
	MaxIdleConns int

	// ConnMaxLifetime is the maximum amount of time a connection may be reused.
	// 0 means no limit. Must not be negative.
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is the maximum amount of time a connection may be idle before
	// being closed. 0 means no limit. Must not be negative.
	ConnMaxIdleTime time.Duration

	// PingTimeout is the timeout for the startup database ping.
	// 0 is replaced by DefaultPingTimeout.
	PingTimeout time.Duration
}

// applyDefaults returns a copy of c with zero-value fields replaced by defaults.
func (c Config) applyDefaults() Config {
	if c.PingTimeout == 0 {
		c.PingTimeout = DefaultPingTimeout
	}
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = DefaultMaxOpenConns
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = DefaultMaxIdleConns
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = DefaultConnMaxLifetime
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = DefaultConnMaxIdleTime
	}
	return c
}

// validate checks that the configuration is valid.
// It returns a *ConfigError on the first violation found.
func (c Config) validate() error {
	if c.DriverName == "" {
		return &ConfigError{Field: "DriverName", Message: "driver name is required"}
	}
	if c.DSN == "" {
		return &ConfigError{Field: "DSN", Message: "DSN is required"}
	}
	if c.MaxOpenConns < 0 {
		return &ConfigError{
			Field:   "MaxOpenConns",
			Message: fmt.Sprintf("must not be negative, got %d", c.MaxOpenConns),
		}
	}
	if c.MaxIdleConns < 0 {
		return &ConfigError{
			Field:   "MaxIdleConns",
			Message: fmt.Sprintf("must not be negative, got %d", c.MaxIdleConns),
		}
	}
	if c.ConnMaxLifetime < 0 {
		return &ConfigError{
			Field:   "ConnMaxLifetime",
			Message: fmt.Sprintf("must not be negative, got %v", c.ConnMaxLifetime),
		}
	}
	if c.ConnMaxIdleTime < 0 {
		return &ConfigError{
			Field:   "ConnMaxIdleTime",
			Message: fmt.Sprintf("must not be negative, got %v", c.ConnMaxIdleTime),
		}
	}
	if c.MaxOpenConns > 0 && c.MaxIdleConns > c.MaxOpenConns {
		return &ConfigError{
			Field: "MaxIdleConns",
			Message: fmt.Sprintf(
				"must not exceed MaxOpenConns (%d), got %d",
				c.MaxOpenConns, c.MaxIdleConns,
			),
		}
	}
	if c.PingTimeout < 0 {
		return &ConfigError{
			Field:   "PingTimeout",
			Message: fmt.Sprintf("must not be negative, got %v", c.PingTimeout),
		}
	}
	return nil
}
