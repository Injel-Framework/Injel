package config

import (
	"fmt"
	"reflect"
)

// MissingRequiredError is returned when an environment variable marked as required
// is missing or empty.
type MissingRequiredError struct {
	Key   string
	Field string
}

func (e *MissingRequiredError) Error() string {
	return fmt.Sprintf("injel/config: required environment variable %q for field %q is missing or empty", e.Key, e.Field)
}

// ConversionError is returned when an environment variable value cannot be converted
// to the target struct field type.
type ConversionError struct {
	Key        string
	Value      string
	TargetType reflect.Type
	Field      string
	Err        error
}

func (e *ConversionError) Error() string {
	return fmt.Sprintf("injel/config: cannot convert %q=%q to type %s for field %q: %v",
		e.Key, e.Value, e.TargetType, e.Field, e.Err)
}

// Unwrap returns the underlying conversion error.
func (e *ConversionError) Unwrap() error {
	return e.Err
}

// InvalidTagError is returned when a struct field's `env` tag contains invalid syntax.
type InvalidTagError struct {
	Field  string
	Tag    string
	Reason string
}

func (e *InvalidTagError) Error() string {
	return fmt.Sprintf("injel/config: invalid tag %q on field %q: %s", e.Tag, e.Field, e.Reason)
}

// UnsupportedTypeError is returned when a struct field's type is not supported by the loader.
type UnsupportedTypeError struct {
	Field string
	Type  reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
	return fmt.Sprintf("injel/config: unsupported type %s for field %q", e.Type, e.Field)
}

// InvalidTargetError is returned when the target passed to Load is not a non-nil pointer to a struct.
type InvalidTargetError struct {
	Target any
	Reason string
}

func (e *InvalidTargetError) Error() string {
	return fmt.Sprintf("injel/config: invalid target: %s", e.Reason)
}
