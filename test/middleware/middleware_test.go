package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"session-service/internal/middleware"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestLoggingMiddleware(t *testing.T) {
	// Create a buffer to capture logs
	var buf bytes.Buffer
	encoderConfig := zap.NewProductionEncoderConfig()
	encoder := zapcore.NewJSONEncoder(encoderConfig)
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buf), zapcore.InfoLevel)
	logger := zap.New(core)

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create the middleware
	mw := middleware.LoggingMiddleware(logger)
	handler := mw(testHandler)

	// Create a request
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(rr, req)

	// Check response
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if body := rr.Body.String(); body != "OK" {
		t.Errorf("handler returned wrong body: got %v want %v", body, "OK")
	}

	// Check logs
	logOutput := buf.String()
	if logOutput == "" {
		t.Error("expected logs, got empty string")
	}

	// Simple check if log contains expected fields
	expectedFields := []string{
		`"msg":"HTTP request"`,
		`"method":"GET"`,
		`"path":"/test"`,
		`"status":200`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(logOutput, field) {
			t.Errorf("log output missing field: %s", field)
		}
	}
}
