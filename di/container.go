package di

import (
	"fmt"
	"reflect"
	"sync"
)

// Container stores provider functions and resolves singleton values by their
// exact result type or bound interface.
//
// A Container is safe for concurrent use. Resolution is serialized: providers
// must not call back into the container while they are being constructed.
// The zero value is an empty container; New is provided for discoverability.
type Container struct {
	mu            sync.Mutex
	providers     map[reflect.Type]*provider
	instances     map[reflect.Type]reflect.Value
	bindings      map[reflect.Type]reflect.Type // target (interface) -> source (concrete)
	order         []reflect.Type
	resolvedOrder []reflect.Type
}

// New creates an empty container.
func New() *Container {
	return &Container{
		providers: make(map[reflect.Type]*provider),
		instances: make(map[reflect.Type]reflect.Value),
		bindings:  make(map[reflect.Type]reflect.Type),
	}
}

// Provide registers a provider function.
//
// A provider must be a non-variadic function returning either T or (T, error),
// where T is not error. Its parameters are resolved from previously or later
// registered providers by exact type.
func (c *Container) Provide(fn any) error {
	if c == nil {
		return fmt.Errorf("injel: cannot provide to a nil container")
	}

	p, err := newProvider(fn)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.providers == nil {
		c.providers = make(map[reflect.Type]*provider)
	}
	if c.instances == nil {
		c.instances = make(map[reflect.Type]reflect.Value)
	}
	if c.bindings == nil {
		c.bindings = make(map[reflect.Type]reflect.Type)
	}

	if existing, ok := c.providers[p.output]; ok {
		return &DuplicateProviderError{
			Type:     p.output,
			Existing: existing.name,
			Incoming: p.name,
		}
	}
	if existing, ok := c.bindings[p.output]; ok {
		return &DuplicateProviderError{
			Type:     p.output,
			Existing: existing.String(),
			Incoming: p.name,
		}
	}
	if _, ok := c.instances[p.output]; ok {
		return &DuplicateProviderError{
			Type:     p.output,
			Existing: "<already resolved>",
			Incoming: p.name,
		}
	}

	c.providers[p.output] = p
	c.order = append(c.order, p.output)
	return nil
}

// ProvideAs registers a provider function and binds its constructed output type
// to the target interface.
//
// target can be a reflect.Type of kind reflect.Interface, or a nil pointer to the interface
// type (e.g. (*MyInterface)(nil)).
func (c *Container) ProvideAs(fn any, target any) error {
	if c == nil {
		return fmt.Errorf("injel: cannot provide to a nil container")
	}

	p, err := newProvider(fn)
	if err != nil {
		return err
	}

	targetType, err := extractInterfaceType(target)
	if err != nil {
		return err
	}

	if !p.output.AssignableTo(targetType) {
		return &InvalidBindingError{
			Target: targetType,
			Source: p.output,
			Reason: fmt.Sprintf("%s does not implement %s", p.output, targetType),
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.providers == nil {
		c.providers = make(map[reflect.Type]*provider)
	}
	if c.instances == nil {
		c.instances = make(map[reflect.Type]reflect.Value)
	}
	if c.bindings == nil {
		c.bindings = make(map[reflect.Type]reflect.Type)
	}

	if existing, ok := c.providers[p.output]; ok {
		return &DuplicateProviderError{
			Type:     p.output,
			Existing: existing.name,
			Incoming: p.name,
		}
	}
	if existing, ok := c.providers[targetType]; ok {
		return &DuplicateProviderError{
			Type:     targetType,
			Existing: existing.name,
			Incoming: p.name,
		}
	}
	if existing, ok := c.bindings[targetType]; ok {
		return &DuplicateProviderError{
			Type:     targetType,
			Existing: existing.String(),
			Incoming: p.output.String(),
		}
	}

	c.providers[p.output] = p
	c.order = append(c.order, p.output)

	c.bindings[targetType] = p.output
	c.order = append(c.order, targetType)
	return nil
}

// Bind binds an interface target to an existing or later registered concrete source type.
// Both target and source can be reflect.Type or nil pointers to their types.
func (c *Container) Bind(target any, source any) error {
	if c == nil {
		return fmt.Errorf("injel: cannot bind in a nil container")
	}

	targetType, err := extractInterfaceType(target)
	if err != nil {
		return err
	}

	sourceType, err := extractConcreteType(source)
	if err != nil {
		return err
	}

	if !sourceType.AssignableTo(targetType) {
		return &InvalidBindingError{
			Target: targetType,
			Source: sourceType,
			Reason: fmt.Sprintf("%s does not implement %s", sourceType, targetType),
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.providers == nil {
		c.providers = make(map[reflect.Type]*provider)
	}
	if c.instances == nil {
		c.instances = make(map[reflect.Type]reflect.Value)
	}
	if c.bindings == nil {
		c.bindings = make(map[reflect.Type]reflect.Type)
	}

	if existing, ok := c.providers[targetType]; ok {
		return &DuplicateProviderError{
			Type:     targetType,
			Existing: existing.name,
			Incoming: sourceType.String(),
		}
	}
	if existing, ok := c.bindings[targetType]; ok {
		return &DuplicateProviderError{
			Type:     targetType,
			Existing: existing.String(),
			Incoming: sourceType.String(),
		}
	}
	if _, ok := c.instances[targetType]; ok {
		return &DuplicateProviderError{
			Type:     targetType,
			Existing: "<already resolved>",
			Incoming: sourceType.String(),
		}
	}

	c.bindings[targetType] = sourceType
	c.order = append(c.order, targetType)
	return nil
}

// ProvideAs binds target interface T to the result of provider function fn.
func ProvideAs[Target any](c *Container, fn any) error {
	return c.ProvideAs(fn, reflect.TypeFor[Target]())
}

// Bind binds target interface to source concrete type.
func Bind[Target any, Source any](c *Container) error {
	return c.Bind(reflect.TypeFor[Target](), reflect.TypeFor[Source]())
}

// Validate checks every registered provider and binding for missing dependencies
// and dependency cycles. It does not call any provider function.
func (c *Container) Validate() error {
	if c == nil {
		return fmt.Errorf("injel: cannot validate a nil container")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, typ := range c.order {
		if err := c.validate(typ, nil, make(map[reflect.Type]bool)); err != nil {
			return err
		}
	}
	return nil
}

func (c *Container) validate(typ reflect.Type, stack []reflect.Type, visiting map[reflect.Type]bool) error {
	if visiting[typ] {
		return &CycleError{Path: cyclePath(stack, typ)}
	}

	if source, ok := c.bindings[typ]; ok {
		visiting[typ] = true
		defer delete(visiting, typ)
		return c.validate(source, extendPath(stack, typ), visiting)
	}

	p, ok := c.providers[typ]
	if !ok {
		return missingDependencyError(c, typ, stack)
	}

	visiting[typ] = true
	defer delete(visiting, typ)

	for _, input := range p.inputs {
		if err := c.validate(input, extendPath(stack, typ), visiting); err != nil {
			return err
		}
	}
	return nil
}

// Resolve returns the singleton value for typ, constructing it and its
// dependencies if necessary.
func (c *Container) Resolve(typ reflect.Type) (any, error) {
	if c == nil {
		return nil, fmt.Errorf("injel: cannot resolve from a nil container")
	}
	if typ == nil {
		return nil, fmt.Errorf("injel: cannot resolve a nil type")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	value, err := c.resolve(typ, nil, make(map[reflect.Type]bool))
	if err != nil {
		return nil, err
	}
	return value.Interface(), nil
}

func (c *Container) resolve(typ reflect.Type, stack []reflect.Type, visiting map[reflect.Type]bool) (reflect.Value, error) {
	if value, ok := c.instances[typ]; ok {
		return value, nil
	}
	if visiting[typ] {
		return reflect.Value{}, &CycleError{Path: cyclePath(stack, typ)}
	}

	if source, ok := c.bindings[typ]; ok {
		visiting[typ] = true
		defer delete(visiting, typ)

		val, err := c.resolve(source, extendPath(stack, typ), visiting)
		if err != nil {
			return reflect.Value{}, err
		}
		c.instances[typ] = val
		return val, nil
	}

	p, ok := c.providers[typ]
	if !ok {
		return reflect.Value{}, missingDependencyError(c, typ, stack)
	}

	visiting[typ] = true
	defer delete(visiting, typ)

	args := make([]reflect.Value, len(p.inputs))
	for i, input := range p.inputs {
		value, err := c.resolve(input, extendPath(stack, typ), visiting)
		if err != nil {
			return reflect.Value{}, err
		}
		args[i] = value
	}

	results := p.fn.Call(args)
	if len(results) == 2 && !results[1].IsNil() {
		providerErr, _ := results[1].Interface().(error)
		return reflect.Value{}, &ProviderError{
			Provider: p.name,
			Type:     typ,
			Path:     extendPath(stack, typ),
			Err:      providerErr,
		}
	}

	value := results[0]
	if (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) && value.IsNil() {
		return reflect.Value{}, &ProviderError{
			Provider: p.name,
			Type:     typ,
			Path:     extendPath(stack, typ),
			Err:      fmt.Errorf("injel: provider returned a nil value"),
		}
	}

	c.instances[typ] = value
	c.resolvedOrder = append(c.resolvedOrder, typ)
	return value, nil
}

// ResolveAll validates the container and eagerly constructs all registered
// providers and bindings in topological dependency order.
func (c *Container) ResolveAll() error {
	if c == nil {
		return fmt.Errorf("injel: cannot resolve from a nil container")
	}

	if err := c.Validate(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, typ := range c.order {
		if _, ok := c.instances[typ]; !ok {
			if _, err := c.resolve(typ, nil, make(map[reflect.Type]bool)); err != nil {
				return err
			}
		}
	}
	return nil
}

// ResolvedInstances returns all constructed singleton instances in the exact order
// of their construction (dependencies before consumers).
func (c *Container) ResolvedInstances() []any {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]any, 0, len(c.resolvedOrder))
	for _, typ := range c.resolvedOrder {
		if val, ok := c.instances[typ]; ok && val.IsValid() {
			out = append(out, val.Interface())
		}
	}
	return out
}

// Get resolves a value of type T from the container.
func Get[T any](c *Container) (T, error) {
	var zero T

	if c == nil {
		return zero, fmt.Errorf("injel: cannot resolve from a nil container")
	}

	typ := reflect.TypeFor[T]()
	value, err := c.Resolve(typ)
	if err != nil {
		return zero, err
	}
	if value == nil {
		return zero, fmt.Errorf("injel: provider for %s returned a nil value", typ)
	}

	resolved := reflect.ValueOf(value)
	if !resolved.Type().AssignableTo(typ) {
		return zero, fmt.Errorf(
			"injel: resolved value of type %s is not assignable to %s",
			resolved.Type(), typ,
		)
	}
	typed, ok := resolved.Interface().(T)
	if !ok {
		return zero, fmt.Errorf(
			"injel: resolved value of type %s cannot be converted to %s",
			resolved.Type(), typ,
		)
	}
	return typed, nil
}

func extractInterfaceType(target any) (reflect.Type, error) {
	if target == nil {
		return nil, &InvalidBindingError{
			Target: nil,
			Source: nil,
			Reason: "target interface must not be nil",
		}
	}
	if t, ok := target.(reflect.Type); ok {
		if t.Kind() != reflect.Interface {
			return nil, &InvalidBindingError{
				Target: t,
				Source: nil,
				Reason: fmt.Sprintf("target must be an interface, got %s", t.Kind()),
			}
		}
		return t, nil
	}
	t := reflect.TypeOf(target)
	if t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Interface {
		return t.Elem(), nil
	}
	if t.Kind() == reflect.Interface {
		return t, nil
	}
	return nil, &InvalidBindingError{
		Target: t,
		Source: nil,
		Reason: fmt.Sprintf("target must be an interface or pointer to interface, got %s", t),
	}
}

func extractConcreteType(source any) (reflect.Type, error) {
	if source == nil {
		return nil, &InvalidBindingError{
			Target: nil,
			Source: nil,
			Reason: "source implementation type must not be nil",
		}
	}
	if t, ok := source.(reflect.Type); ok {
		return t, nil
	}
	return reflect.TypeOf(source), nil
}

func missingDependencyError(c *Container, typ reflect.Type, stack []reflect.Type) error {
	requiredBy := ""
	if len(stack) > 0 {
		parentType := stack[len(stack)-1]
		if parent, ok := c.providers[parentType]; ok {
			requiredBy = parent.name
		} else if _, ok := c.bindings[parentType]; ok {
			requiredBy = parentType.String()
		}
	}
	return &MissingDependencyError{
		Type:       typ,
		RequiredBy: requiredBy,
		Path:       extendPath(stack, typ),
	}
}

func cyclePath(stack []reflect.Type, typ reflect.Type) []reflect.Type {
	for i, candidate := range stack {
		if candidate == typ {
			path := make([]reflect.Type, 0, len(stack)-i+1)
			path = append(path, stack[i:]...)
			path = append(path, typ)
			return path
		}
	}
	return extendPath(stack, typ)
}

func extendPath(path []reflect.Type, typ reflect.Type) []reflect.Type {
	out := make([]reflect.Type, len(path)+1)
	copy(out, path)
	out[len(path)] = typ
	return out
}
