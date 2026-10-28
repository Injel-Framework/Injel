package di

import (
	"fmt"
	"reflect"
	"runtime"
)

var errorType = reflect.TypeFor[error]()

// provider describes a function that constructs one value for the container.
type provider struct {
	fn     reflect.Value
	name   string
	inputs []reflect.Type
	output reflect.Type
}

func newProvider(fn any) (*provider, error) {
	if fn == nil {
		return nil, &InvalidProviderError{Provider: "<nil>", Reason: "provider must not be nil"}
	}

	value := reflect.ValueOf(fn)
	typ := value.Type()

	if typ.Kind() != reflect.Func {
		return nil, &InvalidProviderError{
			Provider: typ.String(),
			Reason:   fmt.Sprintf("provider must be a function, got %s", typ.Kind()),
		}
	}
	if value.IsNil() {
		return nil, &InvalidProviderError{Provider: typ.String(), Reason: "provider function must not be nil"}
	}
	if typ.IsVariadic() {
		return nil, &InvalidProviderError{
			Provider: functionName(value),
			Reason:   "variadic providers are not supported",
		}
	}

	switch typ.NumOut() {
	case 1:
		if typ.Out(0) == errorType {
			return nil, &InvalidProviderError{
				Provider: functionName(value),
				Reason:   "provider must return a constructed value, not error",
			}
		}
	case 2:
		if typ.Out(1) != errorType {
			return nil, &InvalidProviderError{
				Provider: functionName(value),
				Reason:   fmt.Sprintf("second return value must be error, got %s", typ.Out(1)),
			}
		}
		if typ.Out(0) == errorType {
			return nil, &InvalidProviderError{
				Provider: functionName(value),
				Reason:   "provider must return a constructed value, not error",
			}
		}
	default:
		return nil, &InvalidProviderError{
			Provider: functionName(value),
			Reason:   fmt.Sprintf("provider must return one value or (value, error), got %d values", typ.NumOut()),
		}
	}

	inputs := make([]reflect.Type, typ.NumIn())
	for i := range inputs {
		input := typ.In(i)
		if input == errorType {
			return nil, &InvalidProviderError{
				Provider: functionName(value),
				Reason:   fmt.Sprintf("parameter %d must not be error", i),
			}
		}
		inputs[i] = input
	}

	return &provider{
		fn:     value,
		name:   functionName(value),
		inputs: inputs,
		output: typ.Out(0),
	}, nil
}

func functionName(fn reflect.Value) string {
	if fn.Kind() != reflect.Func || fn.IsNil() {
		return "<anonymous>"
	}
	f := runtime.FuncForPC(fn.Pointer())
	if f == nil {
		return "<anonymous>"
	}
	return f.Name()
}
