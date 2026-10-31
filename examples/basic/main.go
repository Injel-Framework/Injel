// Command basic demonstrates configuration, provider functions, singleton resolution, and application lifecycle.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Injel-Framework/Injel"
)

// Config is an application-level configuration value loaded from environment variables.
type Config struct {
	DSN string `env:"DATABASE_URL,default=file:injel.db"`
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

	// Load Config from environment variables (defaults to file:injel.db)
	if err := injel.ProvideConfig[Config](app); err != nil {
		log.Fatalf("register config provider: %v", err)
	}

	for _, provider := range []any{
		NewUserRepository,
		NewUserService,
	} {
		if err := app.Provide(provider); err != nil {
			log.Fatalf("register provider: %v", err)
		}
	}

	ctx := context.Background()

	// Start validates the graph, loads configuration, resolves instances, and invokes OnStart hooks.
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
