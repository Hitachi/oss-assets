package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Server implements the ServerInterface
type Server struct{}

// NewServer creates a new API server instance
func NewServer() *Server {
	return &Server{}
}

// GetHealth handles the health check endpoint
// (GET /health)
func (s *Server) GetHealth(ctx echo.Context) error {
	// Return 204 No Content for healthy status
	return ctx.NoContent(http.StatusNoContent)
}

// GetHello handles the hello world endpoint
// (GET /hello)
func (s *Server) GetHello(ctx echo.Context) error {
	response := HelloResponse{
		Message: "Hello, World!",
	}
	return ctx.JSON(http.StatusOK, response)
}
