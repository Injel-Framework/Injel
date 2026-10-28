package di

import (
	"fmt"
	"reflect"
	"strings"
)

// InvalidProviderError is returned when a value registered with Provide is not
// a valid provider function.
type InvalidProviderError struct {
	Provider string
	Reason   string
}

func (e *InvalidProviderError) Error() string {
	return fmt.Sprintf("injel: invalid provider %s: %s", e.Provider, e.Reason)
}

// DuplicateProviderError is returned when two providers construct the same
// result type or when an interface binding conflicts with an existing provider.
type DuplicateProviderError struct {
	Type     reflect.Type
	Existing string
	Incoming string
}

func (e *DuplicateProviderError) Error() string {
	return fmt.Sprintf(
		"injel: duplicate provider for %s: already provided by %s, new provider %s",
		e.Type, e.Existing, e.Incoming,
	)
}

// MissingDependencyError is returned when no provider can construct a type
// required by another provider or by the application.
type MissingDependencyError struct {
	Type       reflect.Type
	RequiredBy string
	Path       []reflect.Type
}

func (e *MissingDependencyError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "injel: missing provider for %s", e.Type)
	if e.RequiredBy != "" {
		fmt.Fprintf(&b, " required by %s", e.RequiredBy)
	}
	if len(e.Path) > 0 {
		fmt.Fprintf(&b, "; dependency path: %s", formatTypes(e.Path))
	}
	return b.String()
}

// CycleError is returned when providers form a dependency cycle.
type CycleError struct {
	Path []reflect.Type
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("injel: dependency cycle detected: %s", formatTypes(e.Path))
}

// ProviderError wraps an error returned by a provider function during resolve.
type ProviderError struct {
	Provider string
	Type     reflect.Type
	Path     []reflect.Type
	Err      error
}

func (e *ProviderError) Error() string {
	message := fmt.Sprintf("injel: provider %s failed while constructing %s: %v", e.Provider, e.Type, e.Err)
	if len(e.Path) > 0 {
		return message + "; dependency path: " + formatTypes(e.Path)
	}
	return message
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// InvalidBindingError is returned when an interface binding is invalid,
// such as binding to a non-interface target or an unassignable concrete type.
type InvalidBindingError struct {
	Target reflect.Type
	Source reflect.Type
	Reason string
}

func (e *InvalidBindingError) Error() string {
	return fmt.Sprintf("injel: invalid binding from %s to %s: %s", e.Source, e.Target, e.Reason)
}

func formatTypes(types []reflect.Type) string {
	names := make([]string, len(types))
	for i, typ := range types {
		names[i] = typ.String()
	}
	return strings.Join(names, " -> ")
}
