package api

import (
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App, authHandler *AuthHandler, middleware *Middleware) {
	// Public routes
	api := app.Group("/api/v1")
	{
		auth := api.Group("/auth")
		{
			auth.Post("/register", authHandler.Register)
			auth.Post("/login", authHandler.Login)
			auth.Post("/refresh", authHandler.Refresh)
		}
	}

	// Protected routes
	protected := api.Group("", middleware.AuthRequired)
	{
		protected.Post("/auth/logout", authHandler.Logout)
		protected.Get("/auth/me", authHandler.Me)
	}
}
