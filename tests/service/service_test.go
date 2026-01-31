// Package service contains integration tests for iter-service lifecycle.
package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/ternarybob/iter/tests/common"
)

// TestServiceStartStop tests the basic service lifecycle.
func TestServiceStartStop(t *testing.T) {
	env := common.NewTestEnv(t, "service", "start-stop")
	defer env.Cleanup()

	startTime := time.Now()

	// Start service
	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Verify service is running
	client := env.NewHTTPClient()
	resp, body, err := client.Get("/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	common.AssertStatusCode(t, resp, http.StatusOK)
	result := common.AssertJSON(t, body)

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", result["status"])
	}

	env.SaveJSON("health-response.json", result)

	// Stop service
	env.Stop()

	// Verify service is stopped
	_, _, err = client.Get("/health")
	if err == nil {
		t.Error("Expected error after service stop, but request succeeded")
	}

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Service started and stopped successfully")
}

// TestServiceVersion tests the version endpoint.
func TestServiceVersion(t *testing.T) {
	env := common.NewTestEnv(t, "service", "version")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	client := env.NewHTTPClient()
	resp, body, err := client.Get("/version")
	if err != nil {
		t.Fatalf("Version check failed: %v", err)
	}

	common.AssertStatusCode(t, resp, http.StatusOK)
	result := common.AssertJSON(t, body)

	if result["service"] != "iter-service" {
		t.Errorf("Expected service 'iter-service', got %v", result["service"])
	}

	if _, ok := result["version"]; !ok {
		t.Error("Expected version field in response")
	}

	env.SaveJSON("version-response.json", result)

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Version endpoint returned correct response")
}

// TestServiceHealthCheck tests the health endpoint under load.
func TestServiceHealthCheck(t *testing.T) {
	env := common.NewTestEnv(t, "service", "health-check")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	client := env.NewHTTPClient()

	// Make multiple health check requests
	successCount := 0
	totalRequests := 10

	for i := 0; i < totalRequests; i++ {
		resp, _, err := client.Get("/health")
		if err != nil {
			t.Logf("Request %d failed: %v", i, err)
			continue
		}
		if resp.StatusCode == http.StatusOK {
			successCount++
		}
	}

	if successCount != totalRequests {
		t.Errorf("Expected %d successful requests, got %d", totalRequests, successCount)
	}

	env.SaveJSON("health-check-results.json", map[string]interface{}{
		"total_requests": totalRequests,
		"success_count":  successCount,
		"success_rate":   float64(successCount) / float64(totalRequests),
	})

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "All health checks passed")
}

// TestServiceIsolation verifies that test environments are isolated.
func TestServiceIsolation(t *testing.T) {
	env1 := common.NewTestEnv(t, "service", "isolation-1")
	defer env1.Cleanup()

	env2 := common.NewTestEnv(t, "service", "isolation-2")
	defer env2.Cleanup()

	startTime := time.Now()

	// Start both services (with small delay to avoid race conditions)
	if err := env1.Start(); err != nil {
		t.Fatalf("Failed to start service 1: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	if err := env2.Start(); err != nil {
		t.Fatalf("Failed to start service 2: %v", err)
	}

	// Verify they have different ports
	if env1.Port == env2.Port {
		t.Errorf("Services should have different ports: %d == %d", env1.Port, env2.Port)
	}

	// Verify both are responding
	client1 := env1.NewHTTPClient()
	client2 := env2.NewHTTPClient()

	resp1, _, err1 := client1.Get("/health")
	resp2, _, err2 := client2.Get("/health")

	if err1 != nil {
		t.Fatalf("Service 1 health check failed: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("Service 2 health check failed: %v", err2)
	}

	common.AssertStatusCode(t, resp1, http.StatusOK)
	common.AssertStatusCode(t, resp2, http.StatusOK)

	env1.SaveJSON("isolation-results.json", map[string]interface{}{
		"port1": env1.Port,
		"port2": env2.Port,
		"isolated": env1.Port != env2.Port,
	})

	duration := time.Since(startTime)
	env1.WriteSummary(true, duration, "Services are properly isolated")
}

// TestServiceGracefulShutdown tests that the service shuts down gracefully.
func TestServiceGracefulShutdown(t *testing.T) {
	env := common.NewTestEnv(t, "service", "graceful-shutdown")
	defer env.Cleanup()

	startTime := time.Now()

	if err := env.Start(); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Verify service is running
	client := env.NewHTTPClient()
	resp, _, err := client.Get("/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	common.AssertStatusCode(t, resp, http.StatusOK)

	// Record stop time
	stopStart := time.Now()
	env.Stop()
	stopDuration := time.Since(stopStart)

	// Shutdown should be reasonably quick (under 10 seconds)
	if stopDuration > 10*time.Second {
		t.Errorf("Shutdown took too long: %v", stopDuration)
	}

	env.SaveJSON("shutdown-results.json", map[string]interface{}{
		"shutdown_duration_ms": stopDuration.Milliseconds(),
		"graceful":             stopDuration < 10*time.Second,
	})

	duration := time.Since(startTime)
	env.WriteSummary(true, duration, "Service shut down gracefully")
}
