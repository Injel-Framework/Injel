package injel

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/Injel-Framework/Injel/di"
)

// State represents the lifecycle state of an Injel application.
type State int

const (
	StateNew State = iota
	StateStarting
	StateRunning
	StateStopping
	StateStopped
	StateFailed
)

func (s State) String() string {
	switch s {
	case StateNew:
		return "NEW"
	case StateStarting:
		return "STARTING"
	case StateRunning:
		return "RUNNING"
	case StateStopping:
		return "STOPPING"
	case StateStopped:
		return "STOPPED"
	case StateFailed:
		return "FAILED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(s))
	}
}

// StartHook allows a component to execute startup logic when the application starts.
type StartHook interface {
	OnStart(context.Context) error
}

// StopHook allows a component to release resources when the application stops.
type StopHook interface {
	OnStop(context.Context) error
}

// Starter is an alias for StartHook.
type Starter = StartHook

// Stopper is an alias for StopHook.
type Stopper = StopHook

// Hook represents manual lifecycle callbacks.
type Hook struct {
	OnStart func(context.Context) error
	OnStop  func(context.Context) error
}

type funcStopper func(context.Context) error

func (f funcStopper) OnStop(ctx context.Context) error {
	return f(ctx)
}

// App is the public application facade for Injel. It provides a cohesive API
// over the dependency injection container and manages the application lifecycle.
type App struct {
	mu           sync.Mutex
	container    *di.Container
	state        State
	manualHooks  []Hook
	startedHooks []StopHook
}

// New constructs an empty Injel application in the StateNew state.
func New() *App {
	return &App{
		container: di.New(),
		state:     StateNew,
	}
}

// Container returns the underlying DI container.
func (a *App) Container() *di.Container {
	if a == nil {
		return nil
	}
	return a.container
}

// State returns the current lifecycle state of the application.
func (a *App) State() State {
	if a == nil {
		return StateFailed
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state
}

// Provide registers a provider function into the application container.
// A provider must be a non-variadic function returning T or (T, error).
func (a *App) Provide(fn any) error {
	if a == nil {
		return fmt.Errorf("injel: cannot provide to a nil app")
	}
	return a.container.Provide(fn)
}

// ProvideAs registers a provider function and binds its output to an interface target.
// target may be a reflect.Type of kind Interface or a nil pointer to the interface (e.g. (*MyInterface)(nil)).
func (a *App) ProvideAs(fn any, target any) error {
	if a == nil {
		return fmt.Errorf("injel: cannot provide to a nil app")
	}
	return a.container.ProvideAs(fn, target)
}

// Bind binds an interface target to a concrete source type.
func (a *App) Bind(target any, source any) error {
	if a == nil {
		return fmt.Errorf("injel: cannot bind in a nil app")
	}
	return a.container.Bind(target, source)
}

// AppendHook registers manual lifecycle callbacks into the application.
func (a *App) AppendHook(hook Hook) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.manualHooks = append(a.manualHooks, hook)
}

// Validate verifies that every registered provider and binding can be resolved
// without missing dependencies or cycles, without running any provider function.
func (a *App) Validate() error {
	if a == nil {
		return fmt.Errorf("injel: cannot validate a nil app")
	}
	return a.container.Validate()
}

// Resolve returns the singleton value for the given reflect.Type.
func (a *App) Resolve(typ reflect.Type) (any, error) {
	if a == nil {
		return nil, fmt.Errorf("injel: cannot resolve from a nil app")
	}
	return a.container.Resolve(typ)
}

// Start validates the application, eagerly constructs all dependencies in
// topological order, and executes OnStart hooks for each component.
//
// If any hook fails, already started hooks are stopped in reverse order (rollback).
func (a *App) Start(ctx context.Context) error {
	if a == nil {
		return fmt.Errorf("injel: cannot start a nil app")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.state == StateRunning || a.state == StateStarting {
		return fmt.Errorf("injel: application already started (state: %s)", a.state)
	}
	if a.state != StateNew {
		return fmt.Errorf("injel: cannot start application in state %s", a.state)
	}

	a.state = StateStarting

	if ctx.Err() != nil {
		a.state = StateFailed
		return fmt.Errorf("injel: startup context cancelled: %w", ctx.Err())
	}

	// 1. Resolve all dependencies eagerly in dependency order.
	if err := a.container.ResolveAll(); err != nil {
		a.state = StateFailed
		return fmt.Errorf("injel: resolution failed: %w", err)
	}

	// 2. Discover and invoke StartHooks in dependency order.
	instances := a.container.ResolvedInstances()
	a.startedHooks = nil

	for _, inst := range instances {
		if ctx.Err() != nil {
			return a.rollback(fmt.Errorf("injel: startup context cancelled: %w", ctx.Err()))
		}
		if s, ok := inst.(StartHook); ok {
			if err := s.OnStart(ctx); err != nil {
				return a.rollback(err)
			}
		}
		if stopper, ok := inst.(StopHook); ok {
			a.startedHooks = append(a.startedHooks, stopper)
		}
	}

	// 3. Process manual hooks.
	for _, hook := range a.manualHooks {
		if ctx.Err() != nil {
			return a.rollback(fmt.Errorf("injel: startup context cancelled: %w", ctx.Err()))
		}
		if hook.OnStart != nil {
			if err := hook.OnStart(ctx); err != nil {
				return a.rollback(err)
			}
		}
		if hook.OnStop != nil {
			a.startedHooks = append(a.startedHooks, funcStopper(hook.OnStop))
		}
	}

	a.state = StateRunning
	return nil
}

func (a *App) rollback(initialErr error) error {
	a.state = StateFailed
	rollbackCtx := context.Background()

	var errs []error
	errs = append(errs, initialErr)

	for i := len(a.startedHooks) - 1; i >= 0; i-- {
		hook := a.startedHooks[i]
		if stopErr := hook.OnStop(rollbackCtx); stopErr != nil {
			errs = append(errs, fmt.Errorf("cleanup error: %w", stopErr))
		}
	}

	return errors.Join(errs...)
}

// Stop stops the application, executing OnStop hooks in reverse order of startup.
func (a *App) Stop(ctx context.Context) error {
	if a == nil {
		return fmt.Errorf("injel: cannot stop a nil app")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.state == StateStopped {
		return nil
	}
	if a.state == StateNew {
		return fmt.Errorf("injel: application has not been started")
	}
	if a.state != StateRunning {
		return fmt.Errorf("injel: cannot stop application in state %s", a.state)
	}

	a.state = StateStopping

	var errs []error
	for i := len(a.startedHooks) - 1; i >= 0; i-- {
		if ctx.Err() != nil {
			errs = append(errs, fmt.Errorf("injel: shutdown context cancelled: %w", ctx.Err()))
		}
		hook := a.startedHooks[i]
		if err := hook.OnStop(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		a.state = StateFailed
		return errors.Join(errs...)
	}

	a.state = StateStopped
	return nil
}

// Get resolves a value of type T from the application container.
func Get[T any](a *App) (T, error) {
	var zero T
	if a == nil {
		return zero, fmt.Errorf("injel: cannot resolve from a nil app")
	}
	return di.Get[T](a.container)
}

// ProvideAs binds target interface Target to the result of provider function fn.
func ProvideAs[Target any](a *App, fn any) error {
	if a == nil {
		return fmt.Errorf("injel: cannot provide to a nil app")
	}
	return di.ProvideAs[Target](a.container, fn)
}

// Bind binds target interface to source concrete type.
func Bind[Target any, Source any](a *App) error {
	if a == nil {
		return fmt.Errorf("injel: cannot bind in a nil app")
	}
	return di.Bind[Target, Source](a.container)
}
