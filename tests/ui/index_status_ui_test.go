// Package ui provides UI tests for iter-service web interface.
// This file tests the index status UI page.
// All UI tests MUST capture before/after screenshots using chromedp.
//
// Run with: go test -v ./tests/ui/... -run TestIndexStatusUI
package ui

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/iter/tests/common"
)

// TestIndexStatusUIWithoutProjects tests the index status page when no projects are registered.
func TestIndexStatusUIWithoutProjects(t *testing.T) {
	env := common.NewTestEnv(t, "ui", "index-status-no-projects")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Create browser for screenshots
	browser, err := env.NewBrowser()
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}
	defer browser.Close()

	// Before screenshot - index status page initial load
	if err := browser.NavigateAndScreenshot("/web/index-status", "01-before"); err != nil {
		t.Fatalf("Failed to capture before screenshot: %v", err)
	}

	// Verify page content via HTTP client
	client := env.NewHTTPClient()
	html, err := client.GetHTML("/web/index-status")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to get index status page")
		t.Fatalf("Failed to get index status page: %v", err)
	}

	htmlStr := string(html)
	env.SaveResult("index-status-page.html", html)

	// Verify page title
	if !strings.Contains(htmlStr, "Index Status") {
		t.Error("Expected 'Index Status' in page title")
	}

	// Verify GEMINI_API_KEY status is shown as NOT configured
	if !strings.Contains(htmlStr, "GEMINI_API_KEY") {
		t.Error("Expected GEMINI_API_KEY mentioned on page")
	}

	if !strings.Contains(htmlStr, "not provided") && !strings.Contains(htmlStr, "Not configured") {
		t.Error("Expected API key not configured message")
	}

	// Verify empty state message for no projects
	if !strings.Contains(htmlStr, "No projects") {
		t.Error("Expected 'No projects' message")
	}

	// After screenshot - page fully loaded
	if err := browser.FullPageScreenshot("02-after"); err != nil {
		t.Fatalf("Failed to capture after screenshot: %v", err)
	}

	// Verify required screenshots exist
	env.RequireScreenshots([]string{"01-before", "02-after"})

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Index status UI shows GEMINI_API_KEY not provided with no projects registered")
}

// TestIndexStatusUIWithProjects tests the index status page when projects are registered.
func TestIndexStatusUIWithProjects(t *testing.T) {
	env := common.NewTestEnv(t, "ui", "index-status-with-projects")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Create and register test projects
	projectPath1, err := env.CreateTestProject("ui-test-project-1")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to create test project 1")
		t.Fatalf("Failed to create test project 1: %v", err)
	}

	projectPath2, err := env.CreateTestProject("ui-test-project-2")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to create test project 2")
		t.Fatalf("Failed to create test project 2: %v", err)
	}

	client := env.NewHTTPClient()

	// Register projects
	resp1, _, err := client.Post("/projects", map[string]string{"path": projectPath1})
	if err != nil {
		t.Fatalf("Failed to register project 1: %v", err)
	}
	common.AssertStatusCode(t, resp1, http.StatusCreated)

	resp2, _, err := client.Post("/projects", map[string]string{"path": projectPath2})
	if err != nil {
		t.Fatalf("Failed to register project 2: %v", err)
	}
	common.AssertStatusCode(t, resp2, http.StatusCreated)

	// Allow service to process
	time.Sleep(500 * time.Millisecond)

	// Create browser for screenshots
	browser, err := env.NewBrowser()
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}
	defer browser.Close()

	// Before screenshot - index status page with projects
	if err := browser.NavigateAndScreenshot("/web/index-status", "01-before"); err != nil {
		t.Fatalf("Failed to capture before screenshot: %v", err)
	}

	// Get page content
	html, err := client.GetHTML("/web/index-status")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to get index status page")
		t.Fatalf("Failed to get index status page: %v", err)
	}

	htmlStr := string(html)
	env.SaveResult("index-status-page.html", html)

	// Verify page shows API key not configured
	if !strings.Contains(htmlStr, "GEMINI_API_KEY") {
		t.Error("Page should mention GEMINI_API_KEY")
	}

	if !strings.Contains(htmlStr, "not provided") && !strings.Contains(htmlStr, "error") {
		t.Error("Page should indicate API key error status")
	}

	// Verify projects are listed
	if !strings.Contains(htmlStr, "ui-test-project-1") {
		t.Error("Page should list ui-test-project-1")
	}

	if !strings.Contains(htmlStr, "ui-test-project-2") {
		t.Error("Page should list ui-test-project-2")
	}

	// After screenshot - shows projects with status
	if err := browser.FullPageScreenshot("02-after"); err != nil {
		t.Fatalf("Failed to capture after screenshot: %v", err)
	}

	// Verify required screenshots exist
	env.RequireScreenshots([]string{"01-before", "02-after"})

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Index status UI shows 2 projects with GEMINI_API_KEY not provided warning")
}

// TestIndexStatusUIShowsGeminiAPIKeyWarning verifies the UI prominently displays the API key warning.
func TestIndexStatusUIShowsGeminiAPIKeyWarning(t *testing.T) {
	env := common.NewTestEnv(t, "ui", "index-status-api-key-warning")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Create and register a test project
	projectPath, err := env.CreateTestProject("api-key-warning-test")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to create test project")
		t.Fatalf("Failed to create test project: %v", err)
	}

	client := env.NewHTTPClient()

	// Register and index project
	resp, body, err := client.Post("/projects", map[string]string{"path": projectPath})
	if err != nil {
		t.Fatalf("Failed to register project: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusCreated)

	proj := common.AssertJSON(t, body)
	projectID := proj["id"].(string)

	_, _, err = client.Post("/projects/"+projectID+"/index", nil)
	if err != nil {
		t.Fatalf("Failed to index project: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Create browser for screenshots
	browser, err := env.NewBrowser()
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}
	defer browser.Close()

	// Before screenshot - initial page load
	if err := browser.NavigateAndScreenshot("/web/index-status", "01-before"); err != nil {
		t.Fatalf("Failed to capture before screenshot: %v", err)
	}

	// Get page content
	html, err := client.GetHTML("/web/index-status")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to get index status page")
		t.Fatalf("Failed to get index status page: %v", err)
	}

	htmlStr := string(html)
	env.SaveResult("index-status-page.html", html)

	// Verify GEMINI_API_KEY warning is shown
	if !strings.Contains(htmlStr, "GEMINI_API_KEY") {
		t.Error("Page MUST mention GEMINI_API_KEY")
	}

	// Verify the error status indicator
	htmlLower := strings.ToLower(htmlStr)
	if !strings.Contains(htmlLower, "not provided") &&
		!strings.Contains(htmlLower, "not configured") &&
		!strings.Contains(htmlLower, "error") {
		t.Error("Page should indicate API key is not configured")
	}

	// Verify project shows API key missing status
	if !strings.Contains(htmlStr, "api_key_missing") &&
		!strings.Contains(htmlStr, "API Key Missing") {
		t.Error("Page should show API key missing status for indexed projects")
	}

	// After screenshot - shows warning prominently
	if err := browser.FullPageScreenshot("02-after"); err != nil {
		t.Fatalf("Failed to capture after screenshot: %v", err)
	}

	// Verify required screenshots exist
	env.RequireScreenshots([]string{"01-before", "02-after"})

	duration := time.Since(startTime)
	details := "EXPECTED: Index status UI shows GEMINI_API_KEY warning prominently. " +
		"Semantic indexing is unavailable without GEMINI_API_KEY."
	env.WriteSummary(true, duration, details)
	t.Log("EXPECTED: Index status UI shows GEMINI_API_KEY warning")
}

// TestIndexStatusUINavigation tests that the index status page is accessible from navigation.
func TestIndexStatusUINavigation(t *testing.T) {
	env := common.NewTestEnv(t, "ui", "index-status-navigation")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Create browser for screenshots
	browser, err := env.NewBrowser()
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}
	defer browser.Close()

	// Before screenshot - home page
	if err := browser.NavigateAndScreenshot("/web/", "01-home"); err != nil {
		t.Fatalf("Failed to capture home screenshot: %v", err)
	}

	// Navigate to index status page
	if err := browser.NavigateAndScreenshot("/web/index-status", "02-index-status"); err != nil {
		t.Fatalf("Failed to navigate to index status: %v", err)
	}

	// Get page content
	client := env.NewHTTPClient()
	html, err := client.GetHTML("/web/index-status")
	if err != nil {
		env.WriteSummary(false, time.Since(startTime), "Failed to get index status page")
		t.Fatalf("Failed to get index status page: %v", err)
	}

	htmlStr := string(html)
	env.SaveResult("index-status-page.html", html)

	// Verify we're on the index status page
	if !strings.Contains(htmlStr, "Index Status") {
		t.Error("Should be on index status page")
	}

	if !strings.Contains(htmlStr, "GEMINI_API_KEY") {
		t.Error("Page should show API key status")
	}

	// After screenshot - index status page
	if err := browser.FullPageScreenshot("03-after"); err != nil {
		t.Fatalf("Failed to capture after screenshot: %v", err)
	}

	// Verify required screenshots exist
	env.RequireScreenshots([]string{"01-home", "02-index-status", "03-after"})

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Index status page accessible via navigation")
}
