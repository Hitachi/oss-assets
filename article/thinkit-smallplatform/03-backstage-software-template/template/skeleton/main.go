package main

import (
	"log"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/${{values.destination.owner + "/" + values.destination.repo}}/internal/api"
)

func main() {
	// Create Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Create API server instance
	server := api.NewServer()

	// Register handlers
	api.RegisterHandlers(e, server)

	// Start server on port 8080
	log.Println("Starting server on :8080")
	log.Fatal(e.Start(":8080"))
}
