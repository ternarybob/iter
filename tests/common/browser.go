// Package common provides shared test utilities for iter-service tests.
package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
)

// Browser provides chromedp-based browser automation for UI tests.
type Browser struct {
	env         *TestEnv
	ctx         context.Context
	cancel      context.CancelFunc
	allocCtx    context.Context
	allocCancel context.CancelFunc
}

// NewBrowser creates a new headless Chrome browser for UI testing.
func (e *TestEnv) NewBrowser() (*Browser, error) {
	// Create allocator context with headless options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(1280, 800),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)

	// Set timeout for browser operations
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)

	return &Browser{
		env:         e,
		ctx:         ctx,
		cancel:      cancel,
		allocCtx:    allocCtx,
		allocCancel: allocCancel,
	}, nil
}

// Close closes the browser.
func (b *Browser) Close() {
	if b.cancel != nil {
		b.cancel()
	}
	if b.allocCancel != nil {
		b.allocCancel()
	}
}

// Screenshot captures a screenshot of the current page state.
// name should be like "01-before" or "02-after" (without extension).
func (b *Browser) Screenshot(name string) error {
	var buf []byte
	if err := chromedp.Run(b.ctx,
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		return fmt.Errorf("capture screenshot: %w", err)
	}

	path := filepath.Join(b.env.ResultsDir, name+".png")
	if err := os.WriteFile(path, buf, 0644); err != nil {
		return fmt.Errorf("write screenshot: %w", err)
	}

	b.env.Log("Screenshot saved: %s", name+".png")
	return nil
}

// FullPageScreenshot captures a full-page screenshot.
func (b *Browser) FullPageScreenshot(name string) error {
	var buf []byte
	if err := chromedp.Run(b.ctx,
		chromedp.FullScreenshot(&buf, 100),
	); err != nil {
		return fmt.Errorf("capture full screenshot: %w", err)
	}

	path := filepath.Join(b.env.ResultsDir, name+".png")
	if err := os.WriteFile(path, buf, 0644); err != nil {
		return fmt.Errorf("write screenshot: %w", err)
	}

	b.env.Log("Full screenshot saved: %s", name+".png")
	return nil
}

// Navigate navigates to a URL path (relative to the test environment base URL).
func (b *Browser) Navigate(path string) error {
	url := b.env.BaseURL + path
	b.env.Log("Navigating to: %s", url)

	if err := chromedp.Run(b.ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
	); err != nil {
		return fmt.Errorf("navigate to %s: %w", path, err)
	}

	return nil
}

// NavigateAbsolute navigates to an absolute URL (including file:// URLs).
func (b *Browser) NavigateAbsolute(url string) error {
	b.env.Log("Navigating to: %s", url)

	if err := chromedp.Run(b.ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
	); err != nil {
		return fmt.Errorf("navigate to %s: %w", url, err)
	}

	return nil
}

// NavigateAndScreenshot navigates to a path and captures a screenshot.
func (b *Browser) NavigateAndScreenshot(path, screenshotName string) error {
	if err := b.Navigate(path); err != nil {
		return err
	}
	return b.FullPageScreenshot(screenshotName)
}

// WaitVisible waits for an element to be visible.
func (b *Browser) WaitVisible(selector string) error {
	return chromedp.Run(b.ctx,
		chromedp.WaitVisible(selector, chromedp.ByQuery),
	)
}

// WaitReady waits for an element to be ready in the DOM.
func (b *Browser) WaitReady(selector string) error {
	return chromedp.Run(b.ctx,
		chromedp.WaitReady(selector, chromedp.ByQuery),
	)
}

// Click clicks an element.
func (b *Browser) Click(selector string) error {
	return chromedp.Run(b.ctx,
		chromedp.Click(selector, chromedp.ByQuery),
	)
}

// Fill fills an input field.
func (b *Browser) Fill(selector, value string) error {
	return chromedp.Run(b.ctx,
		chromedp.Clear(selector, chromedp.ByQuery),
		chromedp.SendKeys(selector, value, chromedp.ByQuery),
	)
}

// GetText gets the text content of an element.
func (b *Browser) GetText(selector string) (string, error) {
	var text string
	if err := chromedp.Run(b.ctx,
		chromedp.Text(selector, &text, chromedp.ByQuery),
	); err != nil {
		return "", err
	}
	return text, nil
}

// GetHTML gets the outer HTML of an element.
func (b *Browser) GetHTML(selector string) (string, error) {
	var html string
	if err := chromedp.Run(b.ctx,
		chromedp.OuterHTML(selector, &html, chromedp.ByQuery),
	); err != nil {
		return "", err
	}
	return html, nil
}

// Sleep pauses execution for the specified duration.
func (b *Browser) Sleep(d time.Duration) error {
	return chromedp.Run(b.ctx, chromedp.Sleep(d))
}

// EvaluateJS evaluates JavaScript and returns the result.
func (b *Browser) EvaluateJS(expr string, result interface{}) error {
	return chromedp.Run(b.ctx,
		chromedp.Evaluate(expr, result),
	)
}

// HasScreenshots checks if the test results directory contains the required screenshots.
// Returns a list of missing screenshot names.
func (e *TestEnv) HasScreenshots(required []string) []string {
	var missing []string
	for _, name := range required {
		path := filepath.Join(e.ResultsDir, name+".png")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, name)
		}
	}
	return missing
}

// RequireScreenshots fails the test if required screenshots are missing.
func (e *TestEnv) RequireScreenshots(required []string) {
	missing := e.HasScreenshots(required)
	if len(missing) > 0 {
		e.T.Errorf("Missing required screenshots: %v", missing)
	}
}
