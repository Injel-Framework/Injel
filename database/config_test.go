package database

import (
	"errors"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Config.validate() tests
// ---------------------------------------------------------------------------

func TestConfigValidate_Valid(t *testing.T) {
	cfg := Config{
		DriverName:      "fake",
		DSN:             "fake://localhost",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: 30 * time.Second,
		PingTimeout:     2 * time.Second,
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestConfigValidate_EmptyDriverName(t *testing.T) {
	cfg := Config{DSN: "fake://localhost"}
	var ce *ConfigError
	err := cfg.validate()
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "DriverName" {
		t.Errorf("expected Field=DriverName, got %q", ce.Field)
	}
}

func TestConfigValidate_EmptyDSN(t *testing.T) {
	cfg := Config{DriverName: "fake"}
	var ce *ConfigError
	err := cfg.validate()
	if !errors.As(err, &ce) {
		t.Fatalf("expected *ConfigError, got %T: %v", err, err)
	}
	if ce.Field != "DSN" {
		t.Errorf("expected Field=DSN, got %q", ce.Field)
	}
}

func TestConfigValidate_NegativeMaxOpenConns(t *testing.T) {
	cfg := Config{DriverName: "fake", DSN: "x", MaxOpenConns: -1}
	var ce *ConfigError
	if !errors.As(cfg.validate(), &ce) {
		t.Fatal("expected *ConfigError")
	}
	if ce.Field != "MaxOpenConns" {
		t.Errorf("unexpected field: %s", ce.Field)
	}
}

func TestConfigValidate_NegativeMaxIdleConns(t *testing.T) {
	cfg := Config{DriverName: "fake", DSN: "x", MaxIdleConns: -1}
	var ce *ConfigError
	if !errors.As(cfg.validate(), &ce) {
		t.Fatal("expected *ConfigError")
	}
	if ce.Field != "MaxIdleConns" {
		t.Errorf("unexpected field: %s", ce.Field)
	}
}

func TestConfigValidate_NegativeConnMaxLifetime(t *testing.T) {
	cfg := Config{DriverName: "fake", DSN: "x", ConnMaxLifetime: -time.Second}
	var ce *ConfigError
	if !errors.As(cfg.validate(), &ce) {
		t.Fatal("expected *ConfigError")
	}
	if ce.Field != "ConnMaxLifetime" {
		t.Errorf("unexpected field: %s", ce.Field)
	}
}

func TestConfigValidate_NegativeConnMaxIdleTime(t *testing.T) {
	cfg := Config{DriverName: "fake", DSN: "x", ConnMaxIdleTime: -time.Second}
	var ce *ConfigError
	if !errors.As(cfg.validate(), &ce) {
		t.Fatal("expected *ConfigError")
	}
	if ce.Field != "ConnMaxIdleTime" {
		t.Errorf("unexpected field: %s", ce.Field)
	}
}

func TestConfigValidate_MaxIdleExceedsMaxOpen(t *testing.T) {
	cfg := Config{DriverName: "fake", DSN: "x", MaxOpenConns: 5, MaxIdleConns: 10}
	var ce *ConfigError
	if !errors.As(cfg.validate(), &ce) {
		t.Fatal("expected *ConfigError")
	}
	if ce.Field != "MaxIdleConns" {
		t.Errorf("unexpected field: %s", ce.Field)
	}
}

func TestConfigValidate_MaxIdleEqualsMaxOpen(t *testing.T) {
	// MaxIdleConns == MaxOpenConns should be allowed
	cfg := Config{DriverName: "fake", DSN: "x", MaxOpenConns: 5, MaxIdleConns: 5}
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected no error for MaxIdleConns == MaxOpenConns, got: %v", err)
	}
}

func TestConfigValidate_MaxIdleExceedsUnlimitedOpen(t *testing.T) {
	// MaxOpenConns == 0 means unlimited, so MaxIdleConns can be any non-negative value.
	cfg := Config{DriverName: "fake", DSN: "x", MaxOpenConns: 0, MaxIdleConns: 100}
	if err := cfg.validate(); err != nil {
		t.Fatalf("expected no error when MaxOpenConns==0 (unlimited), got: %v", err)
	}
}

func TestConfigValidate_NegativePingTimeout(t *testing.T) {
	cfg := Config{DriverName: "fake", DSN: "x", PingTimeout: -time.Second}
	var ce *ConfigError
	if !errors.As(cfg.validate(), &ce) {
		t.Fatal("expected *ConfigError")
	}
	if ce.Field != "PingTimeout" {
		t.Errorf("unexpected field: %s", ce.Field)
	}
}

// ---------------------------------------------------------------------------
// Config.applyDefaults() tests
// ---------------------------------------------------------------------------

func TestConfigApplyDefaults_ZeroValues(t *testing.T) {
	cfg := Config{DriverName: "fake", DSN: "x"}
	d := cfg.applyDefaults()

	if d.PingTimeout != DefaultPingTimeout {
		t.Errorf("PingTimeout: want %v, got %v", DefaultPingTimeout, d.PingTimeout)
	}
	if d.MaxOpenConns != DefaultMaxOpenConns {
		t.Errorf("MaxOpenConns: want %d, got %d", DefaultMaxOpenConns, d.MaxOpenConns)
	}
	if d.MaxIdleConns != DefaultMaxIdleConns {
		t.Errorf("MaxIdleConns: want %d, got %d", DefaultMaxIdleConns, d.MaxIdleConns)
	}
	if d.ConnMaxLifetime != DefaultConnMaxLifetime {
		t.Errorf("ConnMaxLifetime: want %v, got %v", DefaultConnMaxLifetime, d.ConnMaxLifetime)
	}
	if d.ConnMaxIdleTime != DefaultConnMaxIdleTime {
		t.Errorf("ConnMaxIdleTime: want %v, got %v", DefaultConnMaxIdleTime, d.ConnMaxIdleTime)
	}
}

func TestConfigApplyDefaults_ExplicitValuesPreserved(t *testing.T) {
	cfg := Config{
		DriverName:      "fake",
		DSN:             "x",
		PingTimeout:     1 * time.Second,
		MaxOpenConns:    3,
		MaxIdleConns:    1,
		ConnMaxLifetime: 10 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
	}
	d := cfg.applyDefaults()
	if d.PingTimeout != cfg.PingTimeout {
		t.Errorf("PingTimeout should not be overwritten")
	}
	if d.MaxOpenConns != cfg.MaxOpenConns {
		t.Errorf("MaxOpenConns should not be overwritten")
	}
}

func TestConfigApplyDefaults_DoesNotMutateOriginal(t *testing.T) {
	orig := Config{DriverName: "fake", DSN: "x"}
	_ = orig.applyDefaults()
	// orig must still have zero values since applyDefaults returns a copy
	if orig.PingTimeout != 0 {
		t.Error("applyDefaults must not mutate the receiver")
	}
}

// ---------------------------------------------------------------------------
// Error type tests
// ---------------------------------------------------------------------------

func TestConfigError_Error(t *testing.T) {
	e := &ConfigError{Field: "DSN", Message: "is required"}
	msg := e.Error()
	if msg == "" {
		t.Error("expected non-empty error string")
	}
}

func TestOpenError_Unwrap(t *testing.T) {
	sentinel := errors.New("sentinel")
	e := &OpenError{Driver: "fake", Err: sentinel}
	if !errors.Is(e, sentinel) {
		t.Error("errors.Is should find sentinel through OpenError")
	}
}

func TestPingError_Unwrap(t *testing.T) {
	sentinel := errors.New("ping sentinel")
	e := &PingError{Err: sentinel}
	if !errors.Is(e, sentinel) {
		t.Error("errors.Is should find sentinel through PingError")
	}
}

func TestCloseError_Unwrap(t *testing.T) {
	sentinel := errors.New("close sentinel")
	e := &CloseError{Err: sentinel}
	if !errors.Is(e, sentinel) {
		t.Error("errors.Is should find sentinel through CloseError")
	}
}

func TestPingError_DoesNotContainDSN(t *testing.T) {
	e := &PingError{Err: errors.New("connection refused")}
	msg := e.Error()
	// Ensure DSN/password patterns aren't leaked
	if contains(msg, "password") || contains(msg, "secret") || contains(msg, "://") {
		t.Errorf("PingError message must not contain DSN/credentials: %s", msg)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
