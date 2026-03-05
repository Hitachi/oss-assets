package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestServer_GetHealth(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	server := NewServer()

	// Execute
	err := server.GetHealth(c)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestServer_GetHello(t *testing.T) {
	// Setup
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/hello", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	server := NewServer()

	// Execute
	err := server.GetHello(c)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	expectedBody := `{"message":"Hello, World!"}` + "\n"
	assert.Equal(t, expectedBody, rec.Body.String())

	// Check Content-Type header
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}
