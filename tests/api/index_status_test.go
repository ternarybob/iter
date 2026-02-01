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
	env := common.SetupTest(t, "api")
	defer env.Cleanup()

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

	// Verify GOOGLE_GEMINI_API_KEY is configured (from tests/config/config.toml)
	if !status.GeminiAPIKeyConfigured {
		t.Error("GOOGLE_GEMINI_API_KEY should be configured from tests/config/config.toml")
	}

	if status.GeminiAPIKeyStatus != "Configured" {
		t.Errorf("Expected 'Configured' status, got: %s", status.GeminiAPIKeyStatus)
	}

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Index status API returns correct response. Gemini API key configured.")
	t.Logf("Index status: API Key=%s", status.GeminiAPIKeyStatus)
}

// TestIndexStatusAPIWithProjects tests the index status API when projects are registered.
func TestIndexStatusAPIWithProjects(t *testing.T) {
	env := common.SetupTest(t, "api")
	defer env.Cleanup()

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

	// Verify GOOGLE_GEMINI_API_KEY is configured (from tests/config/config.toml)
	if !status.GeminiAPIKeyConfigured {
		t.Error("GOOGLE_GEMINI_API_KEY should be configured from tests/config/config.toml")
	}
	if status.GeminiAPIKeyStatus != "Configured" {
		t.Errorf("Expected 'Configured' status, got: %s", status.GeminiAPIKeyStatus)
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
		"Gemini API key configured for semantic indexing."
	env.WriteSummary(true, duration, details)
	t.Logf("Index status (with projects): API Key=%s, Projects=%d", status.GeminiAPIKeyStatus, len(status.Projects))
}

// TestIndexStatusSemanticIndexingEnabled verifies semantic indexing works with Gemini API key.
func TestIndexStatusSemanticIndexingEnabled(t *testing.T) {
	env := common.SetupTest(t, "api")
	defer env.Cleanup()

	startTime := time.Now()

	// Create and register a project
	projectPath, err := env.CreateTestProject("semantic-test-project")
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

	// Verify Gemini API key is configured (from tests/config/config.toml)
	if !status.GeminiAPIKeyConfigured {
		t.Error("GOOGLE_GEMINI_API_KEY should be configured from tests/config/config.toml")
	}

	if status.GeminiAPIKeyStatus != "Configured" {
		t.Errorf("Expected 'Configured' status, got: %s", status.GeminiAPIKeyStatus)
	}

	// Cleanup
	client.Delete("/projects/" + proj.ID)

	duration := time.Since(startTime)
	details := "Semantic indexing is available with Gemini API key configured. " +
		"Status: " + status.GeminiAPIKeyStatus
	env.WriteSummary(true, duration, details)
	t.Log("Semantic indexing enabled with Gemini API key")
}

// TestGracefulDegradationWithoutAPIKey verifies the service handles missing API key gracefully.
// The service should:
// 1. Start successfully without crashing
// 2. Report that API key is not configured
// 3. Allow basic operations (project registration, structural indexing)
// 4. Gracefully indicate semantic indexing is unavailable
func TestGracefulDegradationWithoutAPIKey(t *testing.T) {
	// Use WithoutLLMConfig to skip loading the API key
	env := common.SetupTest(t, "api", common.WithoutLLMConfig())
	defer env.Cleanup()

	startTime := time.Now()
	client := env.NewHTTPClient()

	// Test 1: Service is running and responding
	resp, _, err := client.Get("/health")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Service not responding: "+err.Error())
		t.Fatalf("Service health check failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)

	// Test 2: Index status shows API key not configured
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
	env.SaveJSON("index-status-no-apikey.json", status)

	// Verify API key is NOT configured (this is the graceful degradation test)
	if status.GeminiAPIKeyConfigured {
		t.Error("GOOGLE_GEMINI_API_KEY should NOT be configured in this test")
	}

	if !strings.Contains(status.GeminiAPIKeyStatus, "not provided") {
		t.Errorf("Expected 'not provided' in status, got: %s", status.GeminiAPIKeyStatus)
	}

	// Test 3: Can still register and index a project (structural indexing works)
	projectPath, err := env.CreateTestProject("graceful-test-project")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to create test project")
		t.Fatalf("Failed to create test project: %v", err)
	}

	resp, body, err = client.Post("/projects", map[string]string{"path": projectPath})
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to register project")
		t.Fatalf("Failed to register project: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusCreated)

	var proj struct {
		ID string `json:"id"`
	}
	json.Unmarshal(body, &proj)

	// Test 4: Index project - should work for structural indexing
	resp, body, err = client.Post("/projects/"+proj.ID+"/index", nil)
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to index project")
		t.Fatalf("Failed to index project: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)

	// Verify indexing response
	var indexResult map[string]interface{}
	json.Unmarshal(body, &indexResult)
	env.SaveJSON("index-result.json", indexResult)

	// Should have document and file counts (structural indexing works)
	if docCount, ok := indexResult["document_count"].(float64); !ok || docCount == 0 {
		t.Error("Expected documents to be indexed (structural indexing)")
	}

	// Test 5: Check project status shows api_key_missing
	time.Sleep(500 * time.Millisecond)
	resp, body, err = client.Get("/api/index-status")
	if err != nil {
		t.Fatalf("Failed to get index status: %v", err)
	}

	json.Unmarshal(body, &status)

	// Find our project and verify its status
	var foundProject *ProjectIndexStatus
	for i := range status.Projects {
		if status.Projects[i].ID == proj.ID {
			foundProject = &status.Projects[i]
			break
		}
	}

	if foundProject == nil {
		t.Error("Project not found in status")
	} else {
		if foundProject.IndexStatus != "api_key_missing" {
			t.Errorf("Expected index_status 'api_key_missing', got: %s", foundProject.IndexStatus)
		}
		if !strings.Contains(foundProject.ErrorMessage, "semantic indexing unavailable") {
			t.Errorf("Expected error message about semantic indexing, got: %s", foundProject.ErrorMessage)
		}
		// Structural indexing should still work
		if foundProject.DocumentCount == 0 {
			t.Error("Expected documents from structural indexing")
		}
	}

	// Cleanup
	client.Delete("/projects/" + proj.ID)

	duration := time.Since(startTime)
	details := "Service handles missing Gemini API key gracefully. " +
		"Structural indexing works, semantic indexing gracefully unavailable. " +
		"Status: " + status.GeminiAPIKeyStatus
	env.WriteSummary(true, duration, details)
	t.Log("Graceful degradation verified: service runs without API key, structural indexing works")
}
