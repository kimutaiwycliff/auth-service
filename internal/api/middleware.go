package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/kimutaiwycliff/auth-service/config"
	"github.com/kimutaiwycliff/auth-service/internal/services"
)

type Middleware struct {
	jwtService  services.JWTService
	redisService services.RedisService // Changed to use interface
}

func NewMiddleware(jwtService services.JWTService, redisService services.RedisService) *Middleware {
	return &Middleware{
		jwtService:  jwtService,
		redisService: redisService, // Now using the interface
	}
}

func (m *Middleware) AuthRequired(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authorization header missing",
		})
	}

	token := authHeader[7:] // Remove "Bearer " prefix

	// Check if token is blacklisted using RedisService
	blacklisted, err := m.redisService.IsTokenBlacklisted(c.Context(), token)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}
	if blacklisted {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Token has been invalidated",
		})
	}

	// Verify token and get claims
	claims, err := m.jwtService.ValidateAccessToken(token)
	if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Invalid token",
			})
	}

	// Properly extract userID from claims
	userID, ok := claims["sub"].(string)
	if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Invalid token claims",
			})
	}

	// Store userID and token in context
	c.Locals("userID", userID)
	c.Locals("accessToken", token)

	return c.Next()
}

func (m *Middleware) RateLimiter(limit int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.IP()
		key := "rate_limit:" + ip

		count, err := m.redisService.IncrementRequestCount(c.Context(), key, window)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Internal server error",
			})
		}

		if count > limit {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests",
			})
		}

		return c.Next()
	}
}

func NewFiberApp(cfg *config.Config) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "Auth Service",
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	})

	// Middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))
	app.Use(logger.New())
	app.Use(recover.New())

	return app
}
