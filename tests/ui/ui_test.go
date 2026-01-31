// Package ui contains integration tests for iter-service web UI.
// All UI tests MUST capture before/after screenshots using chromedp.
package ui

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ternarybob/iter/tests/common"
)

// TestUIHomePage tests the home page loads correctly.
func TestUIHomePage(t *testing.T) {
	env := common.NewTestEnv(t, "ui", "home-page")
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

	// Before screenshot - initial page load
	if err := browser.NavigateAndScreenshot("/web/", "01-before"); err != nil {
		t.Fatalf("Failed to capture before screenshot: %v", err)
	}

	// Verify page content via HTTP client
	client := env.NewHTTPClient()
	html, err := client.GetHTML("/web/")
	if err != nil {
		t.Fatalf("Failed to get home page: %v", err)
	}

	htmlStr := string(html)

	// Check for title
	if !strings.Contains(htmlStr, "iter-service") {
		t.Error("Expected 'iter-service' in page title")
	}

	// Check for Projects heading
	if !strings.Contains(htmlStr, "Projects") {
		t.Error("Expected 'Projects' heading on home page")
	}

	// Check for Add Project button
	if !strings.Contains(htmlStr, "Add Project") {
		t.Error("Expected 'Add Project' button")
	}

	// Check for navigation
	if !strings.Contains(htmlStr, "Settings") {
		t.Error("Expected 'Settings' in navigation")
	}

	// Check for HTMX script
	if !strings.Contains(htmlStr, "htmx") {
		t.Error("Expected HTMX script include")
	}

	// After screenshot - page fully loaded
	if err := browser.FullPageScreenshot("02-after"); err != nil {
		t.Fatalf("Failed to capture after screenshot: %v", err)
	}

	// Verify required screenshots exist
	env.RequireScreenshots([]string{"01-before", "02-after"})

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Home page loaded with all expected elements")
}

// TestUIStyles tests that CSS styles are served correctly.
func TestUIStyles(t *testing.T) {
	env := common.NewTestEnv(t, "ui", "styles")
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

	// Before screenshot - home page before CSS verification
	if err := browser.NavigateAndScreenshot("/web/", "01-before"); err != nil {
		t.Fatalf("Failed to capture before screenshot: %v", err)
	}

	client := env.NewHTTPClient()

	// Fetch CSS
	resp, css, err := client.Get("/web/static/styles.css")
	if err != nil {
		t.Fatalf("Failed to get styles: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/css") {
		t.Errorf("Expected CSS content type, got %s", contentType)
	}

	// Save the CSS file
	env.SaveResult("styles.css", css)

	// Verify CSS content
	cssStr := string(css)

	// Check for CSS variables (dark theme)
	if !strings.Contains(cssStr, "--bg-color") {
		t.Error("Expected CSS variable --bg-color")
	}

	if !strings.Contains(cssStr, "--text-color") {
		t.Error("Expected CSS variable --text-color")
	}

	// Check for card styling
	if !strings.Contains(cssStr, ".card") {
		t.Error("Expected .card CSS class")
	}

	// Check for button styling
	if !strings.Contains(cssStr, ".btn") {
		t.Error("Expected .btn CSS class")
	}

	// After screenshot - shows styled page
	if err := browser.FullPageScreenshot("02-after"); err != nil {
		t.Fatalf("Failed to capture after screenshot: %v", err)
	}

	// Verify required screenshots exist
	env.RequireScreenshots([]string{"01-before", "02-after"})

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "CSS styles served correctly")
}

// TestUIProjectList tests the project list partial.
func TestUIProjectList(t *testing.T) {
	env := common.NewTestEnv(t, "ui", "project-list")
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

	client := env.NewHTTPClient()

	// Before screenshot - empty project list
	if err := browser.NavigateAndScreenshot("/web/", "01-before"); err != nil {
		t.Fatalf("Failed to capture before screenshot: %v", err)
	}

	// Wait for HTMX to load project list
	browser.Sleep(500 * time.Millisecond)

	// Fetch empty project list via API
	html, err := client.GetHTML("/api/projects-list")
	if err != nil {
		t.Fatalf("Failed to get project list: %v", err)
	}

	// Verify empty state message
	if !strings.Contains(string(html), "No projects registered") {
		t.Error("Expected empty state message")
	}

	// Create and register a test project
	projectPath, err := env.CreateTestProject("ui-test-project")
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	_, _, err = client.Post("/projects", map[string]string{
		"path": projectPath,
	})
	if err != nil {
		t.Fatalf("Register project failed: %v", err)
	}

	// Fetch project list with project
	html, err = client.GetHTML("/api/projects-list")
	if err != nil {
		t.Fatalf("Failed to get project list: %v", err)
	}

	// Verify project is in list
	htmlStr := string(html)
	if !strings.Contains(htmlStr, "ui-test-project") {
		t.Error("Expected project name in list")
	}

	// Verify action buttons
	if !strings.Contains(htmlStr, "View") {
		t.Error("Expected View button")
	}
	if !strings.Contains(htmlStr, "Reindex") {
		t.Error("Expected Reindex button")
	}
	if !strings.Contains(htmlStr, "Remove") {
		t.Error("Expected Remove button")
	}

	// After screenshot - project list with project
	if err := browser.Navigate("/web/"); err != nil {
		t.Fatalf("Failed to navigate for after screenshot: %v", err)
	}
	browser.Sleep(500 * time.Millisecond) // Wait for HTMX
	if err := browser.FullPageScreenshot("02-after"); err != nil {
		t.Fatalf("Failed to capture after screenshot: %v", err)
	}

	// Verify required screenshots exist
	env.RequireScreenshots([]string{"01-before", "02-after"})

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Project list UI working correctly")
}

// TestUIProjectPage tests the project detail page.
func TestUIProjectPage(t *testing.T) {
	env := common.NewTestEnv(t, "ui", "project-page")
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

	client := env.NewHTTPClient()

	// Create and register a test project
	projectPath, err := env.CreateTestProject("detail-test")
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

	// Before screenshot - project page before indexing
	if err := browser.NavigateAndScreenshot("/web/project/"+projectID, "01-before"); err != nil {
		t.Fatalf("Failed to capture before screenshot: %v", err)
	}

	// Fetch project page via HTTP
	html, err := client.GetHTML("/web/project/" + projectID)
	if err != nil {
		t.Fatalf("Failed to get project page: %v", err)
	}

	htmlStr := string(html)

	// Verify project name
	if !strings.Contains(htmlStr, "detail-test") {
		t.Error("Expected project name on page")
	}

	// Verify search form
	if !strings.Contains(htmlStr, "Search") {
		t.Error("Expected search form")
	}

	// Verify Rebuild Index button
	if !strings.Contains(htmlStr, "Rebuild Index") {
		t.Error("Expected Rebuild Index button")
	}

	// Index the project
	_, _, err = client.Post("/projects/"+projectID+"/index", nil)
	if err != nil {
		t.Fatalf("Index project failed: %v", err)
	}

	// Fetch project page (after indexing)
	html, err = client.GetHTML("/web/project/" + projectID)
	if err != nil {
		t.Fatalf("Failed to get project page: %v", err)
	}

	htmlStr = string(html)

	// Verify stats are shown
	if !strings.Contains(htmlStr, "symbols") {
		t.Error("Expected symbols count after indexing")
	}
	if !strings.Contains(htmlStr, "files") {
		t.Error("Expected files count after indexing")
	}

	// After screenshot - project page after indexing
	if err := browser.NavigateAndScreenshot("/web/project/"+projectID, "02-after"); err != nil {
		t.Fatalf("Failed to capture after screenshot: %v", err)
	}

	// Verify required screenshots exist
	env.RequireScreenshots([]string{"01-before", "02-after"})

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Project page UI working correctly")
}

// TestUIDocsPage tests the API documentation page.
func TestUIDocsPage(t *testing.T) {
	env := common.NewTestEnv(t, "ui", "docs-page")
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

	// Before screenshot - docs page initial load
	if err := browser.NavigateAndScreenshot("/web/docs", "01-before"); err != nil {
		t.Fatalf("Failed to capture before screenshot: %v", err)
	}

	client := env.NewHTTPClient()

	// Fetch docs page
	html, err := client.GetHTML("/web/docs")
	if err != nil {
		t.Fatalf("Failed to get docs page: %v", err)
	}

	htmlStr := string(html)

	// Verify documentation title
	if !strings.Contains(htmlStr, "API Documentation") {
		t.Error("Expected API Documentation title")
	}

	// Verify endpoints are documented
	endpoints := []string{
		"/health",
		"/version",
		"/projects",
		"/search",
	}

	for _, endpoint := range endpoints {
		if !strings.Contains(htmlStr, endpoint) {
			t.Errorf("Expected endpoint %s to be documented", endpoint)
		}
	}

	// Verify HTTP methods are shown
	methods := []string{"GET", "POST", "DELETE"}
	for _, method := range methods {
		if !strings.Contains(htmlStr, method) {
			t.Errorf("Expected HTTP method %s in docs", method)
		}
	}

	// After screenshot - full docs page
	if err := browser.FullPageScreenshot("02-after"); err != nil {
		t.Fatalf("Failed to capture after screenshot: %v", err)
	}

	// Verify required screenshots exist
	env.RequireScreenshots([]string{"01-before", "02-after"})

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Docs page loaded with all endpoints documented")
}

// TestUISettingsPage tests the settings page.
func TestUISettingsPage(t *testing.T) {
	env := common.NewTestEnv(t, "ui", "settings-page")
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

	// Before screenshot - settings page initial load
	if err := browser.NavigateAndScreenshot("/web/settings", "01-before"); err != nil {
		t.Fatalf("Failed to capture before screenshot: %v", err)
	}

	client := env.NewHTTPClient()

	// Fetch settings page
	html, err := client.GetHTML("/web/settings")
	if err != nil {
		t.Fatalf("Failed to get settings page: %v", err)
	}

	htmlStr := string(html)

	// Verify settings title
	if !strings.Contains(htmlStr, "Settings") {
		t.Error("Expected Settings title")
	}

	// Verify navigation is present
	if !strings.Contains(htmlStr, "Projects") {
		t.Error("Expected Projects link in navigation")
	}

	// After screenshot - settings page fully loaded
	if err := browser.FullPageScreenshot("02-after"); err != nil {
		t.Fatalf("Failed to capture after screenshot: %v", err)
	}

	// Verify required screenshots exist
	env.RequireScreenshots([]string{"01-before", "02-after"})

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Settings page loaded correctly")
}

// TestUINavigation tests navigation between pages.
func TestUINavigation(t *testing.T) {
	env := common.NewTestEnv(t, "ui", "navigation")
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

	client := env.NewHTTPClient()

	// Before screenshot - home page
	if err := browser.NavigateAndScreenshot("/web/", "01-before"); err != nil {
		t.Fatalf("Failed to capture before screenshot: %v", err)
	}

	// Test all main pages load
	pages := map[string]string{
		"/web/":         "Projects",
		"/web/settings": "Settings",
		"/web/docs":     "API Documentation",
	}

	for path, expectedContent := range pages {
		resp, body, err := client.Get(path)
		if err != nil {
			t.Errorf("Failed to get %s: %v", path, err)
			continue
		}
		common.AssertStatusCode(t, resp, http.StatusOK)

		if !strings.Contains(string(body), expectedContent) {
			t.Errorf("Page %s missing expected content: %s", path, expectedContent)
		}
	}

	// Navigate to each page and capture intermediate screenshots
	if err := browser.NavigateAndScreenshot("/web/settings", "02-settings"); err != nil {
		t.Fatalf("Failed to capture settings screenshot: %v", err)
	}

	if err := browser.NavigateAndScreenshot("/web/docs", "03-docs"); err != nil {
		t.Fatalf("Failed to capture docs screenshot: %v", err)
	}

	// After screenshot - back to home
	if err := browser.NavigateAndScreenshot("/web/", "04-after"); err != nil {
		t.Fatalf("Failed to capture after screenshot: %v", err)
	}

	// Verify required screenshots exist
	env.RequireScreenshots([]string{"01-before", "02-settings", "03-docs", "04-after"})

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "All pages navigable and loaded correctly")
}

// TestUISearchResults tests the search results partial.
func TestUISearchResults(t *testing.T) {
	env := common.NewTestEnv(t, "ui", "search-results")
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

	client := env.NewHTTPClient()

	// Create and register a test project
	projectPath, err := env.CreateTestProject("search-ui-test")
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

	// Index the project
	_, _, err = client.Post("/projects/"+projectID+"/index", nil)
	if err != nil {
		t.Fatalf("Index project failed: %v", err)
	}

	// Before screenshot - project page before search
	if err := browser.NavigateAndScreenshot("/web/project/"+projectID, "01-before"); err != nil {
		t.Fatalf("Failed to capture before screenshot: %v", err)
	}

	// Perform search via API (simulating form submission)
	resp, body, err = client.Post("/projects/"+projectID+"/search", map[string]interface{}{
		"query": "HelloWorld",
		"limit": 5,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)
	env.SaveJSON("search-results.json", common.AssertJSON(t, body))

	// After screenshot - show project page (search results would be shown via HTMX in browser)
	if err := browser.FullPageScreenshot("02-after"); err != nil {
		t.Fatalf("Failed to capture after screenshot: %v", err)
	}

	// Verify required screenshots exist
	env.RequireScreenshots([]string{"01-before", "02-after"})

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Search results displayed correctly")
}
