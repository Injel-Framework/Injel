package config

import (
	"encoding"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
	durationType        = reflect.TypeFor[time.Duration]()
)

// LookupFunc defines the function signature used to query environment variables.
type LookupFunc func(key string) (string, bool)

// Option configures the behavior of the configuration loader.
type Option func(*options)

type options struct {
	lookup LookupFunc
}

// WithLookup configures a custom environment variable lookup function.
func WithLookup(fn LookupFunc) Option {
	return func(o *options) {
		if fn != nil {
			o.lookup = fn
		}
	}
}

// WithMap configures an in-memory map as the environment variable source.
func WithMap(m map[string]string) Option {
	return func(o *options) {
		o.lookup = func(key string) (string, bool) {
			if m == nil {
				return "", false
			}
			val, ok := m[key]
			return val, ok
		}
	}
}

// Load loads environment variables into a new value of type T.
// T must be a struct type or a pointer to a struct type.
func Load[T any](opts ...Option) (T, error) {
	var zero T
	typ := reflect.TypeFor[T]()

	if typ == nil {
		return zero, &InvalidTargetError{Reason: "target type cannot be nil"}
	}

	if typ.Kind() == reflect.Pointer {
		if typ.Elem().Kind() != reflect.Struct {
			return zero, &InvalidTargetError{
				Target: zero,
				Reason: fmt.Sprintf("target pointer must point to a struct, got pointer to %s", typ.Elem().Kind()),
			}
		}
		instance := reflect.New(typ.Elem())
		if err := Bind(instance.Interface(), opts...); err != nil {
			return zero, err
		}
		typed, _ := instance.Interface().(T)
		return typed, nil
	}

	if typ.Kind() != reflect.Struct {
		return zero, &InvalidTargetError{
			Target: zero,
			Reason: fmt.Sprintf("target must be a struct or pointer to struct, got %s", typ.Kind()),
		}
	}

	val := reflect.New(typ).Elem()
	if err := bindValue(val, newOptions(opts)); err != nil {
		return zero, err
	}
	typed, _ := val.Interface().(T)
	return typed, nil
}

// Bind loads environment variables into an existing struct pointed to by target.
// target must be a non-nil pointer to a struct.
func Bind(target any, opts ...Option) error {
	if target == nil {
		return &InvalidTargetError{Reason: "target must not be nil"}
	}

	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Pointer || val.IsNil() {
		return &InvalidTargetError{
			Target: target,
			Reason: "target must be a non-nil pointer to a struct",
		}
	}

	elem := val.Elem()
	if elem.Kind() != reflect.Struct {
		return &InvalidTargetError{
			Target: target,
			Reason: fmt.Sprintf("target must point to a struct, got %s", elem.Kind()),
		}
	}

	return bindValue(elem, newOptions(opts))
}

func newOptions(opts []Option) *options {
	o := &options{
		lookup: os.LookupEnv,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	return o
}

func bindValue(val reflect.Value, opt *options) error {
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !field.IsExported() {
			continue
		}

		// Check for nested struct (not time.Duration and not TextUnmarshaler)
		if fieldVal.Kind() == reflect.Struct && fieldVal.Type() != durationType && !implementsTextUnmarshaler(fieldVal) {
			if err := bindValue(fieldVal, opt); err != nil {
				return err
			}
			continue
		}

		if fieldVal.Kind() == reflect.Pointer && fieldVal.Type().Elem().Kind() == reflect.Struct && fieldVal.Type().Elem() != durationType {
			if fieldVal.IsNil() {
				fieldVal.Set(reflect.New(fieldVal.Type().Elem()))
			}
			if err := bindValue(fieldVal.Elem(), opt); err != nil {
				return err
			}
			continue
		}

		tagStr, hasTag := field.Tag.Lookup("env")
		if !hasTag {
			continue
		}

		ft, err := parseTag(field.Name, tagStr)
		if err != nil {
			return err
		}

		rawVal, exists := opt.lookup(ft.key)
		var effectiveVal string

		if !exists {
			if ft.required {
				return &MissingRequiredError{Key: ft.key, Field: field.Name}
			}
			if ft.hasDefault {
				effectiveVal = ft.defaultVal
			} else {
				continue
			}
		} else {
			if rawVal == "" && ft.required {
				return &MissingRequiredError{Key: ft.key, Field: field.Name}
			}
			effectiveVal = rawVal
		}

		if err := setFieldValue(fieldVal, effectiveVal, ft.key, field.Name); err != nil {
			return err
		}
	}

	return nil
}

type fieldTag struct {
	key        string
	required   bool
	hasDefault bool
	defaultVal string
}

func parseTag(fieldName, tagStr string) (*fieldTag, error) {
	tagStr = strings.TrimSpace(tagStr)
	if tagStr == "" {
		return nil, &InvalidTagError{
			Field:  fieldName,
			Tag:    tagStr,
			Reason: "tag content cannot be empty",
		}
	}

	parts := strings.Split(tagStr, ",")
	key := strings.TrimSpace(parts[0])
	if key == "" {
		return nil, &InvalidTagError{
			Field:  fieldName,
			Tag:    tagStr,
			Reason: "environment variable key cannot be empty",
		}
	}

	ft := &fieldTag{key: key}

	i := 1
	for i < len(parts) {
		part := strings.TrimSpace(parts[i])
		if part == "required" {
			ft.required = true
			i++
		} else if strings.HasPrefix(part, "default=") {
			ft.hasDefault = true
			defaultParts := []string{strings.TrimPrefix(part, "default=")}
			i++
			for i < len(parts) {
				trimmed := strings.TrimSpace(parts[i])
				if trimmed == "required" {
					ft.required = true
					i++
					break
				}
				defaultParts = append(defaultParts, parts[i])
				i++
			}
			ft.defaultVal = strings.Join(defaultParts, ",")
		} else {
			return nil, &InvalidTagError{
				Field:  fieldName,
				Tag:    tagStr,
				Reason: fmt.Sprintf("unknown option %q", part),
			}
		}
	}

	if ft.required && ft.hasDefault {
		return nil, &InvalidTagError{
			Field:  fieldName,
			Tag:    tagStr,
			Reason: "field cannot be both required and have a default value",
		}
	}

	return ft, nil
}

func implementsTextUnmarshaler(val reflect.Value) bool {
	if val.Type().Implements(textUnmarshalerType) {
		return true
	}
	if val.CanAddr() && val.Addr().Type().Implements(textUnmarshalerType) {
		return true
	}
	return false
}

func setFieldValue(val reflect.Value, raw, key, fieldName string) error {
	// 1. TextUnmarshaler support
	if val.CanAddr() {
		if unmarshaler, ok := val.Addr().Interface().(encoding.TextUnmarshaler); ok {
			if err := unmarshaler.UnmarshalText([]byte(raw)); err != nil {
				return &ConversionError{
					Key:        key,
					Value:      raw,
					TargetType: val.Type(),
					Field:      fieldName,
					Err:        err,
				}
			}
			return nil
		}
	}

	// 2. time.Duration
	if val.Type() == durationType {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return &ConversionError{
				Key:        key,
				Value:      raw,
				TargetType: val.Type(),
				Field:      fieldName,
				Err:        err,
			}
		}
		val.SetInt(int64(d))
		return nil
	}

	// 3. Built-in primitive types & slices
	switch val.Kind() {
	case reflect.String:
		val.SetString(raw)
		return nil

	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return &ConversionError{
				Key:        key,
				Value:      raw,
				TargetType: val.Type(),
				Field:      fieldName,
				Err:        err,
			}
		}
		val.SetBool(b)
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, val.Type().Bits())
		if err != nil {
			return &ConversionError{
				Key:        key,
				Value:      raw,
				TargetType: val.Type(),
				Field:      fieldName,
				Err:        err,
			}
		}
		val.SetInt(n)
		return nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, val.Type().Bits())
		if err != nil {
			return &ConversionError{
				Key:        key,
				Value:      raw,
				TargetType: val.Type(),
				Field:      fieldName,
				Err:        err,
			}
		}
		val.SetUint(n)
		return nil

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, val.Type().Bits())
		if err != nil {
			return &ConversionError{
				Key:        key,
				Value:      raw,
				TargetType: val.Type(),
				Field:      fieldName,
				Err:        err,
			}
		}
		val.SetFloat(f)
		return nil

	case reflect.Slice:
		return setSliceValue(val, raw, key, fieldName)

	default:
		return &UnsupportedTypeError{
			Field: fieldName,
			Type:  val.Type(),
		}
	}
}

func setSliceValue(val reflect.Value, raw, key, fieldName string) error {
	elemType := val.Type().Elem()

	if raw == "" {
		val.Set(reflect.MakeSlice(val.Type(), 0, 0))
		return nil
	}

	parts := strings.Split(raw, ",")
	slice := reflect.MakeSlice(val.Type(), len(parts), len(parts))

	for i, part := range parts {
		trimmed := strings.TrimSpace(part)
		elemVal := slice.Index(i)

		if elemType.Kind() == reflect.String {
			elemVal.SetString(trimmed)
			continue
		}

		if err := setFieldValue(elemVal, trimmed, key, fieldName); err != nil {
			return err
		}
	}

	val.Set(slice)
	return nil
}
