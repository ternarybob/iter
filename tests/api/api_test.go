// Package api contains integration tests for iter-service REST API.
// Each test creates its own clean environment with an isolated service instance.
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
	env := common.SetupTest(t, "api")
	defer env.Cleanup()

	startTime := time.Now()
	client := env.NewHTTPClient()

	// Create a test project directory
	projectPath, err := env.CreateTestProject("test-project-crud")
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	// 1. List projects (should be empty initially or have prior test data)
	resp, body, err := client.Get("/projects")
	if err != nil {
		t.Fatalf("List projects failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)
	initialProjects := common.AssertJSONArray(t, body)
	env.SaveJSON("01-list-initial.json", initialProjects)

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

	// 4. List projects (should have the new one)
	resp, body, err = client.Get("/projects")
	if err != nil {
		t.Fatalf("List projects failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)
	projects := common.AssertJSONArray(t, body)
	found := false
	for _, p := range projects {
		if p["id"] == projectID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Created project not found in list")
	}
	env.SaveJSON("04-list-with-project.json", projects)

	// 5. Delete project
	resp, _, err = client.Delete("/projects/" + projectID)
	if err != nil {
		t.Fatalf("Delete project failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusNoContent)

	// 6. Verify deletion
	resp, _, err = client.Get("/projects/" + projectID)
	if err != nil {
		t.Fatalf("Get deleted project failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusNotFound)

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Project CRUD operations completed successfully")
}

// TestAPIProjectIndex tests project indexing operations.
func TestAPIProjectIndex(t *testing.T) {
	env := common.SetupTest(t, "api")
	defer env.Cleanup()

	startTime := time.Now()
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

	// Cleanup: delete the project
	client.Delete("/projects/" + projectID)

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Project indexing completed successfully")
}

// TestAPISearch tests the search functionality.
func TestAPISearch(t *testing.T) {
	env := common.SetupTest(t, "api")
	defer env.Cleanup()

	startTime := time.Now()
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

	// Cleanup
	client.Delete("/projects/" + projectID)

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Search operations completed successfully")
}

// TestAPIErrorHandling tests API error responses.
func TestAPIErrorHandling(t *testing.T) {
	env := common.SetupTest(t, "api")
	defer env.Cleanup()

	startTime := time.Now()
	client := env.NewHTTPClient()

	// 1. Get non-existent project
	resp, body, err := client.Get("/projects/nonexistent-id-12345")
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
	resp, body, err = client.Delete("/projects/nonexistent-id-12345")
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
	resp, body, err = client.Post("/projects/nonexistent-id-12345/search", map[string]interface{}{
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
	env := common.SetupTest(t, "api")
	defer env.Cleanup()

	startTime := time.Now()
	client := env.NewHTTPClient()

	// Create multiple test projects
	projectIDs := make([]string, 3)

	for i := 0; i < 3; i++ {
		projectPath, err := env.CreateTestProject(fmt.Sprintf("multi-project-%d", i))
		if err != nil {
			t.Fatalf("Failed to create test project %d: %v", i, err)
		}

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

	// Check all our projects exist
	for _, id := range projectIDs {
		found := false
		for _, p := range projects {
			if p["id"] == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Project %s not found in list", id)
		}
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

	// Verify it's gone
	resp, _, err = client.Get("/projects/" + projectIDs[1])
	if err != nil {
		t.Fatalf("Get deleted project failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusNotFound)

	// Cleanup remaining projects
	client.Delete("/projects/" + projectIDs[0])
	client.Delete("/projects/" + projectIDs[2])

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Multiple projects management completed successfully")
}
