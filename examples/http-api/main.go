// Command http-api demonstrates an Injel REST API with DI, Configuration, Middleware, and Graceful Shutdown.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Injel-Framework/Injel"
	"github.com/Injel-Framework/Injel/httpserver"
)

// AppConfig defines the application and HTTP server configurations.
type AppConfig struct {
	AppName    string `env:"APP_NAME,default=injel-http-demo"`
	HTTPServer httpserver.Config
}

// User represents a user model.
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// UserRepository manages in-memory user data.
type UserRepository struct {
	mu    sync.RWMutex
	users map[string]User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		users: map[string]User{
			"1": {ID: "1", Name: "Alice", Role: "Admin"},
			"2": {ID: "2", Name: "Bob", Role: "Engineer"},
		},
	}
}

func (r *UserRepository) Find(id string) (User, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	return u, ok
}

func (r *UserRepository) List() []User {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, u)
	}
	return out
}

func (r *UserRepository) Create(u User) User {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[u.ID] = u
	return u
}

// UserService handles user business logic.
type UserService struct {
	repo *UserRepository
}

func NewUserService(repo *UserRepository) *UserService {
	return &UserService{repo: repo}
}

// UserHandler handles HTTP requests for user endpoints.
type UserHandler struct {
	service *UserService
}

func NewUserHandler(svc *UserService) *UserHandler {
	return &UserHandler{service: svc}
}

func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users := h.service.repo.List()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(users)
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	user, ok := h.service.repo.Find(id)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
		return
	}
	_ = json.NewEncoder(w).Encode(user)
}

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var user User
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil || user.ID == "" || user.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid user payload, id and name required"})
		return
	}
	created := h.service.repo.Create(user)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

// NewHTTPServer constructs and wires the HTTP server with middlewares and routes.
func NewHTTPServer(cfg *AppConfig, userHandler *UserHandler) (*httpserver.Server, error) {
	server := httpserver.New(
		httpserver.WithConfig(cfg.HTTPServer),
		httpserver.WithMiddleware(
			httpserver.RequestID(),
			httpserver.Logging(log.Printf),
			httpserver.Recovery(log.Printf),
		),
	)

	// Health check endpoint
	if err := server.RegisterHealth(); err != nil {
		return nil, err
	}

	// Business routes
	if err := server.HandleFunc("GET /users", userHandler.ListUsers); err != nil {
		return nil, err
	}
	if err := server.HandleFunc("GET /users/{id}", userHandler.GetUser); err != nil {
		return nil, err
	}
	if err := server.HandleFunc("POST /users", userHandler.CreateUser); err != nil {
		return nil, err
	}

	// Demo panic route to verify recovery middleware without crashing the process
	if err := server.HandleFunc("GET /panic", func(w http.ResponseWriter, r *http.Request) {
		panic("simulated critical database error for recovery demo")
	}); err != nil {
		return nil, err
	}

	return server, nil
}

func main() {
	app := injel.New()

	// 1. Provide configuration from environment variables
	if err := injel.ProvideConfig[AppConfig](app); err != nil {
		log.Fatalf("provide config: %v", err)
	}

	// 2. Register dependencies into Injel DI container
	for _, provider := range []any{
		NewUserRepository,
		NewUserService,
		NewUserHandler,
		NewHTTPServer,
	} {
		if err := app.Provide(provider); err != nil {
			log.Fatalf("provide component: %v", err)
		}
	}

	// 3. Graceful shutdown on OS signal (SIGINT, SIGTERM)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Start(ctx); err != nil {
		log.Fatalf("start application: %v", err)
	}

	server, _ := injel.Get[*httpserver.Server](app)
	log.Printf("Injel HTTP Server listening on %s", server.Addr())

	// Wait for OS shutdown signal
	<-ctx.Done()
	log.Println("Shutting down HTTP server...")

	// 4. Graceful stop with timeout
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.Stop(stopCtx); err != nil {
		log.Fatalf("stop application: %v", err)
	}

	log.Println("Injel HTTP Server stopped gracefully.")
}
