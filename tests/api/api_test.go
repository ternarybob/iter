// Package api contains integration tests for iter-service REST API.
package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ternarybob/iter/tests/common"
)

// TestAPIProjectCRUD tests project create, read, update, delete operations.
func TestAPIProjectCRUD(t *testing.T) {
	env := common.NewTestEnv(t, "api", "project-crud")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	client := env.NewHTTPClient()

	// Create a test project directory
	projectPath, err := env.CreateTestProject("test-project")
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	// 1. List projects (should be empty)
	resp, body, err := client.Get("/projects")
	if err != nil {
		t.Fatalf("List projects failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)
	projects := common.AssertJSONArray(t, body)
	if len(projects) != 0 {
		t.Errorf("Expected 0 projects, got %d", len(projects))
	}
	env.SaveJSON("01-list-empty.json", projects)

	// 2. Register project
	resp, body, err = client.Post("/projects", map[string]string{
		"path": projectPath,
	})
	if err != nil {
		t.Fatalf("Register project failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusCreated)
	created := common.AssertJSON(t, body)

	projectID, ok := created["id"].(string)
	if !ok || projectID == "" {
		t.Fatal("Expected project ID in response")
	}
	env.SaveJSON("02-register-project.json", created)

	// 3. Get project details
	resp, body, err = client.Get("/projects/" + projectID)
	if err != nil {
		t.Fatalf("Get project failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)
	project := common.AssertJSON(t, body)

	if project["id"] != projectID {
		t.Errorf("Expected project ID %s, got %v", projectID, project["id"])
	}
	env.SaveJSON("03-get-project.json", project)

	// 4. List projects (should have one)
	resp, body, err = client.Get("/projects")
	if err != nil {
		t.Fatalf("List projects failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)
	projects = common.AssertJSONArray(t, body)
	if len(projects) != 1 {
		t.Errorf("Expected 1 project, got %d", len(projects))
	}
	env.SaveJSON("04-list-one-project.json", projects)

	// 5. Delete project
	resp, _, err = client.Delete("/projects/" + projectID)
	if err != nil {
		t.Fatalf("Delete project failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusNoContent)

	// 6. Verify deletion
	resp, body, err = client.Get("/projects")
	if err != nil {
		t.Fatalf("List projects failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)
	projects = common.AssertJSONArray(t, body)
	if len(projects) != 0 {
		t.Errorf("Expected 0 projects after deletion, got %d", len(projects))
	}
	env.SaveJSON("06-list-after-delete.json", projects)

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Project CRUD operations completed successfully")
}

// TestAPIProjectIndex tests project indexing operations.
func TestAPIProjectIndex(t *testing.T) {
	env := common.NewTestEnv(t, "api", "project-index")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	client := env.NewHTTPClient()

	// Create and register a test project
	projectPath, err := env.CreateTestProject("indexing-test")
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	resp, body, err := client.Post("/projects", map[string]string{
		"path": projectPath,
	})
	if err != nil {
		t.Fatalf("Register project failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusCreated)
	created := common.AssertJSON(t, body)
	projectID := created["id"].(string)
	env.SaveJSON("01-register-project.json", created)

	// Trigger index rebuild
	resp, body, err = client.Post("/projects/"+projectID+"/index", nil)
	if err != nil {
		t.Fatalf("Rebuild index failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)
	indexStats := common.AssertJSON(t, body)
	env.SaveJSON("02-rebuild-index.json", indexStats)

	// Verify index was built
	docCount, ok := indexStats["document_count"].(float64)
	if !ok || docCount == 0 {
		t.Error("Expected documents to be indexed")
	}

	fileCount, ok := indexStats["file_count"].(float64)
	if !ok || fileCount == 0 {
		t.Error("Expected files to be indexed")
	}

	env.Log("Indexed %v documents from %v files", docCount, fileCount)

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Project indexing completed successfully")
}

// TestAPISearch tests the search functionality.
func TestAPISearch(t *testing.T) {
	env := common.NewTestEnv(t, "api", "search")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	client := env.NewHTTPClient()

	// Create and register a test project
	projectPath, err := env.CreateTestProject("search-test")
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	resp, body, err := client.Post("/projects", map[string]string{
		"path": projectPath,
	})
	if err != nil {
		t.Fatalf("Register project failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusCreated)
	created := common.AssertJSON(t, body)
	projectID := created["id"].(string)

	// Rebuild index
	resp, _, err = client.Post("/projects/"+projectID+"/index", nil)
	if err != nil {
		t.Fatalf("Rebuild index failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)

	// Search for "HelloWorld" function
	resp, body, err = client.Post("/projects/"+projectID+"/search", map[string]interface{}{
		"query": "HelloWorld greeting",
		"limit": 5,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)
	searchResults := common.AssertJSON(t, body)
	env.SaveJSON("01-search-helloworld.json", searchResults)

	results, ok := searchResults["results"].([]interface{})
	if !ok {
		t.Fatal("Expected results array in search response")
	}

	if len(results) == 0 {
		t.Error("Expected at least one search result")
	}

	// Search for "Add" function
	resp, body, err = client.Post("/projects/"+projectID+"/search", map[string]interface{}{
		"query": "Add two numbers",
		"limit": 5,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)
	searchResults = common.AssertJSON(t, body)
	env.SaveJSON("02-search-add.json", searchResults)

	// Search with kind filter
	resp, body, err = client.Post("/projects/"+projectID+"/search", map[string]interface{}{
		"query": "function",
		"kind":  "function",
		"limit": 10,
	})
	if err != nil {
		t.Fatalf("Search with filter failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)
	searchResults = common.AssertJSON(t, body)
	env.SaveJSON("03-search-functions.json", searchResults)

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Search operations completed successfully")
}

// TestAPIErrorHandling tests API error responses.
func TestAPIErrorHandling(t *testing.T) {
	env := common.NewTestEnv(t, "api", "error-handling")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	client := env.NewHTTPClient()

	// 1. Get non-existent project
	resp, body, err := client.Get("/projects/nonexistent")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusNotFound)
	errorResp := common.AssertJSON(t, body)
	if _, ok := errorResp["error"]; !ok {
		t.Error("Expected error field in response")
	}
	env.SaveJSON("01-get-nonexistent.json", errorResp)

	// 2. Register project with invalid path
	resp, body, err = client.Post("/projects", map[string]string{
		"path": "/nonexistent/path/that/does/not/exist",
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusBadRequest)
	errorResp = common.AssertJSON(t, body)
	if _, ok := errorResp["error"]; !ok {
		t.Error("Expected error field in response")
	}
	env.SaveJSON("02-register-invalid-path.json", errorResp)

	// 3. Register project with empty path
	resp, body, err = client.Post("/projects", map[string]string{
		"path": "",
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusBadRequest)
	errorResp = common.AssertJSON(t, body)
	if _, ok := errorResp["error"]; !ok {
		t.Error("Expected error field in response")
	}
	env.SaveJSON("03-register-empty-path.json", errorResp)

	// 4. Delete non-existent project
	resp, body, err = client.Delete("/projects/nonexistent")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusNotFound)
	errorResp = common.AssertJSON(t, body)
	if _, ok := errorResp["error"]; !ok {
		t.Error("Expected error field in response")
	}
	env.SaveJSON("04-delete-nonexistent.json", errorResp)

	// 5. Search on non-existent project
	resp, body, err = client.Post("/projects/nonexistent/search", map[string]interface{}{
		"query": "test",
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusNotFound)
	errorResp = common.AssertJSON(t, body)
	if _, ok := errorResp["error"]; !ok {
		t.Error("Expected error field in response")
	}
	env.SaveJSON("05-search-nonexistent.json", errorResp)

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Error handling tests completed successfully")
}

// TestAPIMultipleProjects tests managing multiple projects.
func TestAPIMultipleProjects(t *testing.T) {
	env := common.NewTestEnv(t, "api", "multiple-projects")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	client := env.NewHTTPClient()

	// Create multiple test projects
	projectPaths := make([]string, 3)
	projectIDs := make([]string, 3)

	for i := 0; i < 3; i++ {
		projectPath, err := env.CreateTestProject(fmt.Sprintf("project-%d", i))
		if err != nil {
			t.Fatalf("Failed to create test project %d: %v", i, err)
		}
		projectPaths[i] = projectPath

		resp, body, err := client.Post("/projects", map[string]string{
			"path": projectPath,
		})
		if err != nil {
			t.Fatalf("Register project %d failed: %v", i, err)
		}
		common.AssertStatusCode(t, resp, http.StatusCreated)
		created := common.AssertJSON(t, body)
		projectIDs[i] = created["id"].(string)
	}

	// Verify all projects are listed
	resp, body, err := client.Get("/projects")
	if err != nil {
		t.Fatalf("List projects failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)
	projects := common.AssertJSONArray(t, body)
	if len(projects) != 3 {
		t.Errorf("Expected 3 projects, got %d", len(projects))
	}
	env.SaveJSON("01-list-all-projects.json", projects)

	// Index all projects
	for i, id := range projectIDs {
		resp, _, err := client.Post("/projects/"+id+"/index", nil)
		if err != nil {
			t.Fatalf("Index project %d failed: %v", i, err)
		}
		common.AssertStatusCode(t, resp, http.StatusOK)
	}

	// Delete one project
	resp, _, err = client.Delete("/projects/" + projectIDs[1])
	if err != nil {
		t.Fatalf("Delete project failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusNoContent)

	// Verify remaining projects
	resp, body, err = client.Get("/projects")
	if err != nil {
		t.Fatalf("List projects failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)
	projects = common.AssertJSONArray(t, body)
	if len(projects) != 2 {
		t.Errorf("Expected 2 projects after deletion, got %d", len(projects))
	}
	env.SaveJSON("02-list-after-delete.json", projects)

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Multiple projects management completed successfully")
}
