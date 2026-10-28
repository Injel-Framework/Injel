package config

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

type serverConfig struct {
	Host     string        `env:"HOST,default=127.0.0.1"`
	Port     int           `env:"PORT,default=8080"`
	Debug    bool          `env:"DEBUG,default=false"`
	Timeout  time.Duration `env:"TIMEOUT,default=5s"`
	Ratio    float64       `env:"RATIO,default=0.75"`
	Tags     []string      `env:"TAGS,default=web,api"`
	Ports    []int         `env:"PORTS,default=80,443"`
	Secret   string        `env:"SECRET_KEY,required"`
	Optional string        `env:"OPTIONAL"`
}

func TestConfigDefaultsAndOverrides(t *testing.T) {
	// 1. All defaults except required SECRET_KEY
	env := map[string]string{
		"SECRET_KEY": "super-secret",
	}

	cfg, err := Load[serverConfig](WithMap(env))
	if err != nil {
		t.Fatalf("Load with defaults failed: %v", err)
	}

	if cfg.Host != "127.0.0.1" {
		t.Fatalf("Host = %q, want 127.0.0.1", cfg.Host)
	}
	if cfg.Port != 8080 {
		t.Fatalf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.Debug != false {
		t.Fatalf("Debug = %v, want false", cfg.Debug)
	}
	if cfg.Timeout != 5*time.Second {
		t.Fatalf("Timeout = %v, want 5s", cfg.Timeout)
	}
	if cfg.Ratio != 0.75 {
		t.Fatalf("Ratio = %f, want 0.75", cfg.Ratio)
	}
	if len(cfg.Tags) != 2 || cfg.Tags[0] != "web" || cfg.Tags[1] != "api" {
		t.Fatalf("Tags = %v, want [web, api]", cfg.Tags)
	}
	if len(cfg.Ports) != 2 || cfg.Ports[0] != 80 || cfg.Ports[1] != 443 {
		t.Fatalf("Ports = %v, want [80, 443]", cfg.Ports)
	}
	if cfg.Secret != "super-secret" {
		t.Fatalf("Secret = %q, want super-secret", cfg.Secret)
	}
	if cfg.Optional != "" {
		t.Fatalf("Optional = %q, want empty", cfg.Optional)
	}

	// 2. Override all defaults
	overrideEnv := map[string]string{
		"HOST":       "0.0.0.0",
		"PORT":       "9090",
		"DEBUG":      "true",
		"TIMEOUT":    "30s",
		"RATIO":      "1.5",
		"TAGS":       "grpc,internal",
		"PORTS":      "8000,8080,8443",
		"SECRET_KEY": "new-secret",
		"OPTIONAL":   "present",
	}

	cfgOverride, err := Load[*serverConfig](WithMap(overrideEnv))
	if err != nil {
		t.Fatalf("Load with overrides failed: %v", err)
	}

	if cfgOverride.Host != "0.0.0.0" {
		t.Fatalf("Host = %q, want 0.0.0.0", cfgOverride.Host)
	}
	if cfgOverride.Port != 9090 {
		t.Fatalf("Port = %d, want 9090", cfgOverride.Port)
	}
	if cfgOverride.Debug != true {
		t.Fatalf("Debug = %v, want true", cfgOverride.Debug)
	}
	if cfgOverride.Timeout != 30*time.Second {
		t.Fatalf("Timeout = %v, want 30s", cfgOverride.Timeout)
	}
	if cfgOverride.Ratio != 1.5 {
		t.Fatalf("Ratio = %f, want 1.5", cfgOverride.Ratio)
	}
	if len(cfgOverride.Tags) != 2 || cfgOverride.Tags[0] != "grpc" || cfgOverride.Tags[1] != "internal" {
		t.Fatalf("Tags = %v, want [grpc, internal]", cfgOverride.Tags)
	}
	if len(cfgOverride.Ports) != 3 || cfgOverride.Ports[0] != 8000 || cfgOverride.Ports[1] != 8080 || cfgOverride.Ports[2] != 8443 {
		t.Fatalf("Ports = %v, want [8000, 8080, 8443]", cfgOverride.Ports)
	}
	if cfgOverride.Secret != "new-secret" {
		t.Fatalf("Secret = %q, want new-secret", cfgOverride.Secret)
	}
	if cfgOverride.Optional != "present" {
		t.Fatalf("Optional = %q, want present", cfgOverride.Optional)
	}
}

func TestMissingRequiredVariable(t *testing.T) {
	// Missing SECRET_KEY entirely
	_, err := Load[serverConfig](WithMap(map[string]string{}))
	if err == nil {
		t.Fatal("expected error for missing required variable")
	}

	var reqErr *MissingRequiredError
	if !errors.As(err, &reqErr) {
		t.Fatalf("expected MissingRequiredError, got: %T (%v)", err, err)
	}
	if reqErr.Key != "SECRET_KEY" || reqErr.Field != "Secret" {
		t.Fatalf("unexpected fields in MissingRequiredError: %+v", reqErr)
	}
	if !strings.Contains(err.Error(), "SECRET_KEY") {
		t.Fatalf("error message missing key name: %v", err)
	}

	// Variable exists but is empty
	_, err = Load[serverConfig](WithMap(map[string]string{"SECRET_KEY": ""}))
	if err == nil {
		t.Fatal("expected error for empty required variable")
	}
	if !errors.As(err, &reqErr) {
		t.Fatalf("expected MissingRequiredError for empty value, got: %T", err)
	}
}

func TestExistingEmptyVariableNonRequired(t *testing.T) {
	type emptyTestConfig struct {
		Name string `env:"APP_NAME,default=MyApp"`
	}

	// When APP_NAME is explicitly set to empty string, it overrides the default!
	env := map[string]string{"APP_NAME": ""}
	cfg, err := Load[emptyTestConfig](WithMap(env))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "" {
		t.Fatalf("APP_NAME = %q, want empty string override", cfg.Name)
	}
}

func TestConversionErrors(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"invalid int", map[string]string{"SECRET_KEY": "s", "PORT": "not-an-int"}},
		{"invalid bool", map[string]string{"SECRET_KEY": "s", "DEBUG": "not-a-bool"}},
		{"invalid duration", map[string]string{"SECRET_KEY": "s", "TIMEOUT": "5x"}},
		{"invalid float", map[string]string{"SECRET_KEY": "s", "RATIO": "xyz"}},
		{"invalid slice element", map[string]string{"SECRET_KEY": "s", "PORTS": "80,abc,443"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load[serverConfig](WithMap(tc.env))
			if err == nil {
				t.Fatalf("expected conversion error for %s", tc.name)
			}
			var convErr *ConversionError
			if !errors.As(err, &convErr) {
				t.Fatalf("expected ConversionError, got: %T (%v)", err, err)
			}
			if convErr.Unwrap() == nil {
				t.Fatal("expected ConversionError.Unwrap() to return underlying error")
			}
		})
	}
}

func TestInvalidTags(t *testing.T) {
	type badTagRequiredAndDefault struct {
		Val string `env:"VAL,required,default=hello"`
	}
	_, err := Load[badTagRequiredAndDefault](WithMap(nil))
	if err == nil {
		t.Fatal("expected error when both required and default are specified")
	}
	var tagErr *InvalidTagError
	if !errors.As(err, &tagErr) {
		t.Fatalf("expected InvalidTagError, got: %T", err)
	}

	type badTagUnknownOption struct {
		Val string `env:"VAL,unknown_opt"`
	}
	_, err = Load[badTagUnknownOption](WithMap(nil))
	if err == nil {
		t.Fatal("expected error for unknown option")
	}
	if !errors.As(err, &tagErr) {
		t.Fatalf("expected InvalidTagError, got: %T", err)
	}

	type badTagEmptyKey struct {
		Val string `env:",required"`
	}
	_, err = Load[badTagEmptyKey](WithMap(nil))
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if !errors.As(err, &tagErr) {
		t.Fatalf("expected InvalidTagError, got: %T", err)
	}

	type badTagEmptyContent struct {
		Val string `env:""`
	}
	_, err = Load[badTagEmptyContent](WithMap(nil))
	if err == nil {
		t.Fatal("expected error for empty tag")
	}
	if !errors.As(err, &tagErr) {
		t.Fatalf("expected InvalidTagError, got: %T", err)
	}
}

func TestUnsupportedType(t *testing.T) {
	type badTypeConfig struct {
		Ch chan int `env:"CHAN"`
	}
	_, err := Load[badTypeConfig](WithMap(map[string]string{"CHAN": "foo"}))
	if err == nil {
		t.Fatal("expected error for unsupported chan type")
	}
	var unsuppErr *UnsupportedTypeError
	if !errors.As(err, &unsuppErr) {
		t.Fatalf("expected UnsupportedTypeError, got: %T", err)
	}
}

func TestInvalidTarget(t *testing.T) {
	// Not a pointer in Bind
	cfg := serverConfig{}
	err := Bind(cfg)
	if err == nil {
		t.Fatal("expected error when binding to non-pointer")
	}
	var targetErr *InvalidTargetError
	if !errors.As(err, &targetErr) {
		t.Fatalf("expected InvalidTargetError, got: %T", err)
	}

	// Nil target
	err = Bind(nil)
	if err == nil {
		t.Fatal("expected error binding to nil target")
	}

	// Non-struct pointer
	var str string
	err = Bind(&str)
	if err == nil {
		t.Fatal("expected error binding to string pointer")
	}

	// Load with non-struct type
	_, err = Load[int]()
	if err == nil {
		t.Fatal("expected error loading into int")
	}
}

type nestedDBConfig struct {
	DSN         string `env:"DB_DSN,required"`
	MaxOpenConn int    `env:"DB_MAX_OPEN,default=10"`
}

type parentConfig struct {
	AppName  string `env:"APP_NAME,default=my-app"`
	Database nestedDBConfig
	DBPtr    *nestedDBConfig
}

func TestNestedStructs(t *testing.T) {
	env := map[string]string{
		"DB_DSN": "postgres://user:pass@localhost:5432/db",
	}

	cfg, err := Load[parentConfig](WithMap(env))
	if err != nil {
		t.Fatalf("Load parentConfig failed: %v", err)
	}

	if cfg.AppName != "my-app" {
		t.Fatalf("AppName = %q, want my-app", cfg.AppName)
	}
	if cfg.Database.DSN != "postgres://user:pass@localhost:5432/db" {
		t.Fatalf("Database.DSN = %q", cfg.Database.DSN)
	}
	if cfg.Database.MaxOpenConn != 10 {
		t.Fatalf("Database.MaxOpenConn = %d, want 10", cfg.Database.MaxOpenConn)
	}
	if cfg.DBPtr == nil || cfg.DBPtr.DSN != "postgres://user:pass@localhost:5432/db" {
		t.Fatalf("DBPtr = %+v", cfg.DBPtr)
	}
}

type customIPConfig struct {
	IP net.IP `env:"LISTEN_IP,default=192.168.1.1"`
}

func TestTextUnmarshalerSupport(t *testing.T) {
	cfg, err := Load[customIPConfig](WithMap(nil))
	if err != nil {
		t.Fatalf("Load customIPConfig failed: %v", err)
	}
	if cfg.IP.String() != "192.168.1.1" {
		t.Fatalf("IP = %v, want 192.168.1.1", cfg.IP)
	}

	// Override
	cfg, err = Load[customIPConfig](WithMap(map[string]string{"LISTEN_IP": "10.0.0.1"}))
	if err != nil {
		t.Fatalf("Load customIPConfig override failed: %v", err)
	}
	if cfg.IP.String() != "10.0.0.1" {
		t.Fatalf("IP = %v, want 10.0.0.1", cfg.IP)
	}
}
