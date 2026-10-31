//go:build ignore

// This example demonstrates how to integrate database/sql with the Injel Framework
// using dependency injection and lifecycle management.
//
// To run this example, you need to register a database driver in your own application.
// The driver is a dependency of your application, NOT of the Injel framework.
//
// Example with SQLite (using github.com/mattn/go-sqlite3):
//
//	go get github.com/mattn/go-sqlite3
//	# then uncomment the import below and remove the //go:build ignore tag
//
// Example with PostgreSQL (using github.com/lib/pq):
//
//	go get github.com/lib/pq
//	# then uncomment the import below and remove the //go:build ignore tag
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	// Uncomment ONE of the following driver imports in your application:
	// _ "github.com/mattn/go-sqlite3" // SQLite
	// _ "github.com/lib/pq"           // PostgreSQL

	injel "github.com/Injel-Framework/Injel"
	"github.com/Injel-Framework/Injel/database"
)

// AppConfig holds the application configuration loaded from environment variables.
type AppConfig struct {
	DatabaseDriver string `env:"DB_DRIVER,default=sqlite3"`
	DatabaseDSN    string `env:"DB_DSN,default=file:app.db?cache=shared&mode=memory"`
}

// UserRepository demonstrates how to inject *sql.DB into a repository.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository is the provider function for DI.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// CreateTable creates the users table if it does not exist.
func (r *UserRepository) CreateTable(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS users (
		id   INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL
	)`)
	return err
}

// Insert adds a new user and returns the inserted ID.
func (r *UserRepository) Insert(ctx context.Context, name string) (int64, error) {
	res, err := r.db.ExecContext(ctx, "INSERT INTO users (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Count returns the number of users in the database.
func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func main() {
	// ------------------------------------------------------------------
	// 1. Create the Injel application
	// ------------------------------------------------------------------
	app := injel.New()

	// ------------------------------------------------------------------
	// 2. Load configuration from environment variables
	// ------------------------------------------------------------------
	cfg := AppConfig{
		DatabaseDriver: getenv("DB_DRIVER", "sqlite3"),
		DatabaseDSN:    getenv("DB_DSN", "file:app.db?cache=shared&mode=memory"),
	}

	// ------------------------------------------------------------------
	// 3. Register database with lifecycle and DI
	//    The driver must be imported by the application (see top of file).
	// ------------------------------------------------------------------
	if err := database.Provide(app, database.Config{
		DriverName:      cfg.DatabaseDriver,
		DSN:             cfg.DatabaseDSN,
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
		PingTimeout:     5 * time.Second,
	}); err != nil {
		log.Fatalf("database.Provide: %v", err)
	}

	// ------------------------------------------------------------------
	// 4. Register application components via DI
	// ------------------------------------------------------------------
	if err := app.Provide(NewUserRepository); err != nil {
		log.Fatalf("provide UserRepository: %v", err)
	}

	// ------------------------------------------------------------------
	// 5. Handle OS signals for graceful shutdown
	// ------------------------------------------------------------------
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ------------------------------------------------------------------
	// 6. Start the application (pings DB, runs OnStart hooks)
	// ------------------------------------------------------------------
	if err := app.Start(ctx); err != nil {
		log.Fatalf("app.Start: %v", err)
	}
	log.Println("Application started. Database connected.")

	// ------------------------------------------------------------------
	// 7. Resolve and use the repository
	// ------------------------------------------------------------------
	repo, err := injel.Get[*UserRepository](app)
	if err != nil {
		log.Fatalf("get UserRepository: %v", err)
	}

	if err := repo.CreateTable(ctx); err != nil {
		log.Fatalf("create table: %v", err)
	}

	id, err := repo.Insert(ctx, "Alice")
	if err != nil {
		log.Fatalf("insert user: %v", err)
	}
	fmt.Printf("Inserted user with ID: %d\n", id)

	count, err := repo.Count(ctx)
	if err != nil {
		log.Fatalf("count users: %v", err)
	}
	fmt.Printf("Total users: %d\n", count)

	// ------------------------------------------------------------------
	// 8. Wait for shutdown signal
	// ------------------------------------------------------------------
	<-ctx.Done()
	log.Println("Shutting down...")

	stopCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	if err := app.Stop(stopCtx); err != nil {
		log.Fatalf("app.Stop: %v", err)
	}
	log.Println("Application stopped gracefully.")
}

func getenv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
