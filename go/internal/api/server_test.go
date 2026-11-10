package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.glkj.example.com/glkj/harness/go/internal/api"
	"git.glkj.example.com/glkj/harness/go/internal/config"
)

func TestHealthReportsTheProfile(t *testing.T) {
	server := &api.Server{Config: config.Config{Profile: "web"}}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["profile"] != "web" {
		t.Fatalf("payload = %v", payload)
	}
}

func TestConfigDumpServesComposedText(t *testing.T) {
	server := &api.Server{
		Config:  config.Config{Profile: "web"},
		Compose: func() (string, error) { return "- id: timer\n", nil },
	}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/config", nil))

	if recorder.Code != http.StatusOK || recorder.Body.String() != "- id: timer\n" {
		t.Fatalf("status = %d body = %q", recorder.Code, recorder.Body.String())
	}
}

func TestConfigDumpReportsMissingProfile(t *testing.T) {
	server := &api.Server{Config: config.Config{Profile: "web"}}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/config", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
}
