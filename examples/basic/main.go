// Command basic demonstrates provider functions, singleton resolution, and application lifecycle.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Injel-Framework/Injel"
)

// Config is an application-level configuration value.
type Config struct {
	DSN string
}

// UserRepository is a data-access component with lifecycle management.
type UserRepository struct {
	dsn string
}

func (r *UserRepository) OnStart(ctx context.Context) error {
	fmt.Println("UserRepository: connected to database", r.dsn)
	return nil
}

func (r *UserRepository) OnStop(ctx context.Context) error {
	fmt.Println("UserRepository: closed database connection")
	return nil
}

// UserService depends on the repository.
type UserService struct {
	repo *UserRepository
}

// NewConfig constructs the application configuration.
func NewConfig() *Config {
	return &Config{DSN: "file:injel.db"}
}

// NewUserRepository constructs the repository from configuration.
func NewUserRepository(cfg *Config) *UserRepository {
	return &UserRepository{dsn: cfg.DSN}
}

// NewUserService constructs the service from the repository.
func NewUserService(repo *UserRepository) (*UserService, error) {
	if repo == nil {
		return nil, fmt.Errorf("user repository is nil")
	}
	return &UserService{repo: repo}, nil
}

func main() {
	app := injel.New()

	for _, provider := range []any{
		NewConfig,
		NewUserRepository,
		NewUserService,
	} {
		if err := app.Provide(provider); err != nil {
			log.Fatalf("register provider: %v", err)
		}
	}

	ctx := context.Background()

	// Start validates the graph, resolves instances, and invokes OnStart hooks.
	if err := app.Start(ctx); err != nil {
		log.Fatalf("start application: %v", err)
	}
	defer func() {
		// Stop executes OnStop hooks in reverse dependency order.
		if err := app.Stop(ctx); err != nil {
			log.Fatalf("stop application: %v", err)
		}
	}()

	service, err := injel.Get[*UserService](app)
	if err != nil {
		log.Fatalf("resolve user service: %v", err)
	}

	fmt.Println("user service resolved with repository DSN:", service.repo.dsn)
}
