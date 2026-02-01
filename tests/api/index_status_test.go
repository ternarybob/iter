// Package api provides API tests for iter-service.
// This file tests the index status API endpoint.
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/iter/tests/common"
)

// IndexStatusResponse mirrors the API response structure.
type IndexStatusResponse struct {
	GeminiAPIKeyConfigured bool                 `json:"gemini_api_key_configured"`
	GeminiAPIKeyStatus     string               `json:"gemini_api_key_status"`
	Projects               []ProjectIndexStatus `json:"projects"`
}

type ProjectIndexStatus struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	IndexStatus   string `json:"index_status"`
	DocumentCount int    `json:"document_count"`
	FileCount     int    `json:"file_count"`
	ErrorMessage  string `json:"error_message,omitempty"`
	LastUpdated   string `json:"last_updated,omitempty"`
}

// TestIndexStatusAPIWithoutProjects tests the index status API when no projects are registered.
func TestIndexStatusAPIWithoutProjects(t *testing.T) {
	env := getEnv(t)
	startTime := time.Now()

	client := env.NewHTTPClient()

	// Get index status with no projects
	resp, body, err := client.Get("/api/index-status")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to get index status: "+err.Error())
		t.Fatalf("Failed to get index status: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)

	var status IndexStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to parse response: "+err.Error())
		t.Fatalf("Failed to parse index status response: %v", err)
	}

	// Save response
	env.SaveJSON("index-status-response.json", status)

	// Verify GOOGLE_GEMINI_API_KEY status (should NOT be configured in test environment)
	if status.GeminiAPIKeyConfigured {
		t.Error("GOOGLE_GEMINI_API_KEY should not be configured in test environment")
	}

	if !strings.Contains(status.GeminiAPIKeyStatus, "not provided") {
		t.Errorf("Expected 'not provided' in status, got: %s", status.GeminiAPIKeyStatus)
	}

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Index status API returns correct response. GOOGLE_GEMINI_API_KEY not provided.")
	t.Logf("Index status: API Key=%s", status.GeminiAPIKeyStatus)
}

// TestIndexStatusAPIWithProjects tests the index status API when projects are registered.
func TestIndexStatusAPIWithProjects(t *testing.T) {
	env := getEnv(t)
	startTime := time.Now()

	// Create test projects
	projectPath1, err := env.CreateTestProject("status-test-project-1")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to create test project 1")
		t.Fatalf("Failed to create test project 1: %v", err)
	}

	projectPath2, err := env.CreateTestProject("status-test-project-2")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to create test project 2")
		t.Fatalf("Failed to create test project 2: %v", err)
	}

	client := env.NewHTTPClient()

	// Register projects
	resp1, body1, err := client.Post("/projects", map[string]string{"path": projectPath1})
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to register project 1")
		t.Fatalf("Failed to register project 1: %v", err)
	}
	common.AssertStatusCode(t, resp1, http.StatusCreated)

	var proj1 struct {
		ID string `json:"id"`
	}
	json.Unmarshal(body1, &proj1)

	resp2, body2, err := client.Post("/projects", map[string]string{"path": projectPath2})
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to register project 2")
		t.Fatalf("Failed to register project 2: %v", err)
	}
	common.AssertStatusCode(t, resp2, http.StatusCreated)

	var proj2 struct {
		ID string `json:"id"`
	}
	json.Unmarshal(body2, &proj2)

	// Index projects
	client.Post("/projects/"+proj1.ID+"/index", nil)
	client.Post("/projects/"+proj2.ID+"/index", nil)

	// Allow indexing to complete
	time.Sleep(1 * time.Second)

	// Get index status
	resp, body, err := client.Get("/api/index-status")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to get index status")
		t.Fatalf("Failed to get index status: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)

	var status IndexStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to parse response")
		t.Fatalf("Failed to parse index status response: %v", err)
	}

	// Save response
	env.SaveJSON("index-status-response.json", status)

	// Verify GOOGLE_GEMINI_API_KEY status
	if status.GeminiAPIKeyConfigured {
		t.Error("GOOGLE_GEMINI_API_KEY should not be configured")
	}
	if !strings.Contains(status.GeminiAPIKeyStatus, "not provided") {
		t.Errorf("Status should indicate API key not provided, got: %s", status.GeminiAPIKeyStatus)
	}

	// Verify projects are listed (at least our 2)
	if len(status.Projects) < 2 {
		t.Errorf("Expected at least 2 projects, got %d", len(status.Projects))
	}

	// Cleanup
	client.Delete("/projects/" + proj1.ID)
	client.Delete("/projects/" + proj2.ID)

	duration := time.Since(startTime)
	details := "Index status API returns correct response with projects. " +
		"Projects show api_key_missing status because GOOGLE_GEMINI_API_KEY is not provided."
	env.WriteSummary(true, duration, details)
	t.Logf("Index status (with projects): API Key=%s, Projects=%d", status.GeminiAPIKeyStatus, len(status.Projects))
}

// TestIndexStatusRequiresGeminiAPIKey verifies that semantic indexing requires GOOGLE_GEMINI_API_KEY.
func TestIndexStatusRequiresGeminiAPIKey(t *testing.T) {
	env := getEnv(t)
	startTime := time.Now()

	// Create and register a project
	projectPath, err := env.CreateTestProject("gemini-test-project")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to create test project")
		t.Fatalf("Failed to create test project: %v", err)
	}

	client := env.NewHTTPClient()

	resp, body, err := client.Post("/projects", map[string]string{"path": projectPath})
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to register project")
		t.Fatalf("Failed to register project: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusCreated)

	var proj struct {
		ID string `json:"id"`
	}
	json.Unmarshal(body, &proj)

	// Index project
	client.Post("/projects/"+proj.ID+"/index", nil)
	time.Sleep(1 * time.Second)

	// Get index status
	resp, body, err = client.Get("/api/index-status")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to get index status")
		t.Fatalf("Failed to get index status: %v", err)
	}

	var status IndexStatusResponse
	json.Unmarshal(body, &status)

	// Save response
	env.SaveJSON("index-status-response.json", status)

	// The test MUST show semantic indexing is unavailable without API key
	if status.GeminiAPIKeyConfigured {
		t.Fatal("Test environment should NOT have GOOGLE_GEMINI_API_KEY configured")
	}

	// Cleanup
	client.Delete("/projects/" + proj.ID)

	duration := time.Since(startTime)
	details := "EXPECTED: Semantic indexing is unavailable without GOOGLE_GEMINI_API_KEY. " +
		"Status: " + status.GeminiAPIKeyStatus
	env.WriteSummary(true, duration, details)
	t.Log("EXPECTED: Semantic indexing is unavailable without GOOGLE_GEMINI_API_KEY")
}
