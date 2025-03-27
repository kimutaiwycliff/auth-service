package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kimutaiwycliff/auth-service/config"
	"github.com/kimutaiwycliff/auth-service/internal/api"
	"github.com/kimutaiwycliff/auth-service/internal/repositories"
	"github.com/kimutaiwycliff/auth-service/internal/services"
	"github.com/kimutaiwycliff/auth-service/pkg/database"
	"github.com/kimutaiwycliff/auth-service/pkg/redis"
	"gorm.io/gorm"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()
	log.Printf("Loaded configuration: %+v", cfg)

	// Initialize database with retry logic
	var db *gorm.DB
	var err error
	maxDBRetries := 5
	dbRetryDelay := 5 * time.Second

	for i := 0; i < maxDBRetries; i++ {
		db, err = database.NewPostgresDB(cfg.DB.DSN, cfg.DB.MaxOpenConns)
		if err == nil {
			break
		}
		log.Printf("Failed to connect to database (attempt %d/%d): %v", i+1, maxDBRetries, err)
		if i < maxDBRetries-1 {
			time.Sleep(dbRetryDelay)
		}
	}
	if err != nil {
		log.Fatalf("Failed to connect to database after %d attempts: %v", maxDBRetries, err)
	}

	// Initialize Redis with retry logic
	var redisClient services.RedisService
	maxRedisRetries := 5
	redisRetryDelay := 2 * time.Second

	for i := 0; i < maxRedisRetries; i++ {
		client := redis.NewRedisClient(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
		if err := client.Ping(context.Background()); err == nil {
			redisClient = client
			break
		}
		log.Printf("Failed to connect to Redis (attempt %d/%d): %v", i+1, maxRedisRetries, err)
		if i < maxRedisRetries-1 {
			time.Sleep(redisRetryDelay)
		}
	}
	if redisClient == nil {
		log.Fatal("Could not connect to Redis after maximum retries")
	}

	// Run migrations
	if err := database.Migrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Rest of your main function remains the same...
	// Initialize layers
	userRepo := repositories.NewUserRepository(db)
	jwtService := services.NewJWTService(cfg.JWT.Secret, cfg.JWT.AccessExpiry, cfg.JWT.RefreshExpiry)
	authService := services.NewAuthService(userRepo, jwtService, redisClient)
	authHandler := api.NewAuthHandler(authService)
	middleware := api.NewMiddleware(jwtService, redisClient)

	// Create Fiber app
	app := api.NewFiberApp(cfg)
	api.SetupRoutes(app, authHandler, middleware)

	// Graceful shutdown
	go func() {
		if err := app.Listen(":" + cfg.Server.Port); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	if err := app.Shutdown(); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	if err := database.Close(db); err != nil {
		log.Printf("Database connection close error: %v", err)
	}

	if closer, ok := redisClient.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			log.Printf("Redis connection close error: %v", err)
		}
	}
	log.Println("Server exited properly")
}
