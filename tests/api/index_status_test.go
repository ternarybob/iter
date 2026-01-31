// Package api provides API tests for iter-service.
// This file tests the index status API endpoint.
//
// Run with: go test -v ./tests/api/... -run TestIndexStatus
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
	env := common.NewTestEnv(t, "api", "index-status-no-projects")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

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

	// Verify GEMINI_API_KEY status (should NOT be configured in test environment)
	if status.GeminiAPIKeyConfigured {
		t.Error("GEMINI_API_KEY should not be configured in test environment")
	}

	if !strings.Contains(status.GeminiAPIKeyStatus, "not provided") {
		t.Errorf("Expected 'not provided' in status, got: %s", status.GeminiAPIKeyStatus)
	}

	// Verify no projects
	if len(status.Projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(status.Projects))
	}

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Index status API returns correct response with no projects. GEMINI_API_KEY not provided.")
	t.Logf("Index status (no projects): API Key=%s, Projects=%d", status.GeminiAPIKeyStatus, len(status.Projects))
}

// TestIndexStatusAPIWithProjects tests the index status API when projects are registered.
func TestIndexStatusAPIWithProjects(t *testing.T) {
	env := common.NewTestEnv(t, "api", "index-status-with-projects")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Create test projects
	projectPath1, err := env.CreateTestProject("test-project-1")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to create test project 1")
		t.Fatalf("Failed to create test project 1: %v", err)
	}

	projectPath2, err := env.CreateTestProject("test-project-2")
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
	_, _, err = client.Post("/projects/"+proj1.ID+"/index", nil)
	if err != nil {
		t.Fatalf("Failed to index project 1: %v", err)
	}

	_, _, err = client.Post("/projects/"+proj2.ID+"/index", nil)
	if err != nil {
		t.Fatalf("Failed to index project 2: %v", err)
	}

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

	// Verify GEMINI_API_KEY status
	if status.GeminiAPIKeyConfigured {
		t.Error("GEMINI_API_KEY should not be configured")
	}
	if !strings.Contains(status.GeminiAPIKeyStatus, "not provided") {
		t.Errorf("Status should indicate API key not provided, got: %s", status.GeminiAPIKeyStatus)
	}

	// Verify projects are listed
	if len(status.Projects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(status.Projects))
	}

	// Verify each project shows api_key_missing status
	for _, proj := range status.Projects {
		if proj.IndexStatus != "api_key_missing" {
			t.Errorf("Project %s should have api_key_missing status, got: %s", proj.Name, proj.IndexStatus)
		}
		if !strings.Contains(proj.ErrorMessage, "GEMINI_API_KEY") {
			t.Errorf("Error message should mention GEMINI_API_KEY, got: %s", proj.ErrorMessage)
		}
	}

	duration := time.Since(startTime)
	details := "Index status API returns correct response with 2 projects. " +
		"All projects show api_key_missing status because GEMINI_API_KEY is not provided."
	env.WriteSummary(true, duration, details)
	t.Logf("Index status (with projects): API Key=%s, Projects=%d", status.GeminiAPIKeyStatus, len(status.Projects))
}

// TestIndexStatusRequiresGeminiAPIKey verifies that semantic indexing requires GEMINI_API_KEY.
func TestIndexStatusRequiresGeminiAPIKey(t *testing.T) {
	env := common.NewTestEnv(t, "api", "index-status-api-key-required")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

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
	_, _, err = client.Post("/projects/"+proj.ID+"/index", nil)
	if err != nil {
		t.Fatalf("Failed to index project: %v", err)
	}

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
		t.Fatal("Test environment should NOT have GEMINI_API_KEY configured")
	}

	// Verify the project shows the correct status
	if len(status.Projects) != 1 {
		t.Fatalf("Expected 1 project, got %d", len(status.Projects))
	}

	proj1 := status.Projects[0]
	if proj1.IndexStatus != "api_key_missing" {
		t.Errorf("Index status should be 'api_key_missing', got: %s", proj1.IndexStatus)
	}
	if !strings.Contains(proj1.ErrorMessage, "GEMINI_API_KEY not provided") {
		t.Errorf("Error message should indicate GEMINI_API_KEY is not provided, got: %s", proj1.ErrorMessage)
	}

	duration := time.Since(startTime)
	details := "EXPECTED: Semantic indexing is unavailable without GEMINI_API_KEY. " +
		"Project status: " + proj1.IndexStatus + " - " + proj1.ErrorMessage
	env.WriteSummary(true, duration, details)
	t.Log("EXPECTED: Semantic indexing is unavailable without GEMINI_API_KEY")
	t.Logf("Project index status: %s - %s", proj1.IndexStatus, proj1.ErrorMessage)
}
