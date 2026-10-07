package main

import (
	"context"
	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"example.com/otel-test/internal/api"
	"example.com/otel-test/internal/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

func main() {
	ctx := context.Background()

	// Init OpenTelemetry
	shutdown, err := telemetry.Setup(ctx)
	if err != nil {
		slog.Error("failed to set up telemetry", "error", err)
		return
	}
	defer func() { _ = shutdown(context.Background()) }()

	// Create Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(otelecho.Middleware("go-echo-template"))

	// Create API server instance
	server := api.NewServer()

	// Register handlers
	api.RegisterHandlers(e, server)

	// Start server on port 8080
	slog.Info("Starting server on :8080")
	if err := e.Start(":8080"); err != nil {
		slog.Error("server stopped", "error", err)
	}
}
