// Package monitor provides live monitoring capabilities for Iter agents.
package monitor

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Monitor provides live visibility into agent execution.
type Monitor interface {
	// Start begins the monitoring server.
	Start(ctx context.Context) error

	// Stop shuts down the monitor.
	Stop() error

	// Emit sends an event to subscribers.
	Emit(event Event)

	// Subscribe returns a channel for receiving events.
	Subscribe() <-chan Event

	// Unsubscribe removes a subscription.
	Unsubscribe(ch <-chan Event)
}

// EventType categorizes monitor events.
type EventType string

const (
	EventIterationStarted    EventType = "iteration_started"
	EventIterationCompleted  EventType = "iteration_completed"
	EventStepStarted         EventType = "step_started"
	EventStepCompleted       EventType = "step_completed"
	EventValidationStarted   EventType = "validation_started"
	EventValidationCompleted EventType = "validation_completed"
	EventBuildOutput         EventType = "build_output"
	EventTestOutput          EventType = "test_output"
	EventError               EventType = "error"
	EventRateLimitHit        EventType = "rate_limit_hit"
	EventCircuitOpened       EventType = "circuit_opened"
	EventExitDetected        EventType = "exit_detected"
)

// Event represents a monitoring event.
type Event struct {
	Type      EventType      `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

// NewEvent creates a new event.
func NewEvent(eventType EventType) Event {
	return Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      make(map[string]any),
	}
}

// WithData adds data to the event.
func (e Event) WithData(key string, value any) Event {
	if e.Data == nil {
		e.Data = make(map[string]any)
	}
	e.Data[key] = value
	return e
}

// HTTPMonitor implements Monitor with an HTTP/WebSocket server.
type HTTPMonitor struct {
	mu sync.RWMutex

	addr   string
	server *http.Server

	subscribers map[chan Event]bool
	history     []Event
	maxHistory  int

	running bool
}

// NewHTTPMonitor creates a new HTTP monitor.
func NewHTTPMonitor(addr string) *HTTPMonitor {
	return &HTTPMonitor{
		addr:        addr,
		subscribers: make(map[chan Event]bool),
		history:     make([]Event, 0),
		maxHistory:  1000,
	}
}

// Start begins the monitoring server.
func (m *HTTPMonitor) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/status", m.handleStatus)
	mux.HandleFunc("/metrics", m.handleMetrics)
	mux.HandleFunc("/events", m.handleEvents)
	mux.HandleFunc("/history", m.handleHistory)

	m.server = &http.Server{
		Addr:    m.addr,
		Handler: mux,
	}

	go func() {
		_ = m.server.ListenAndServe()
	}()

	return nil
}

// Stop shuts down the monitor.
func (m *HTTPMonitor) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.running = false

	// Close all subscriber channels
	for ch := range m.subscribers {
		close(ch)
		delete(m.subscribers, ch)
	}

	if m.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return m.server.Shutdown(ctx)
	}

	return nil
}

// Emit sends an event to subscribers.
func (m *HTTPMonitor) Emit(event Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Add to history
	m.history = append(m.history, event)
	if len(m.history) > m.maxHistory {
		m.history = m.history[1:]
	}

	// Send to subscribers (non-blocking)
	for ch := range m.subscribers {
		select {
		case ch <- event:
		default:
			// Skip if channel is full
		}
	}
}

// Subscribe returns a channel for receiving events.
func (m *HTTPMonitor) Subscribe() <-chan Event {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan Event, 100)
	m.subscribers[ch] = true
	return ch
}

// Unsubscribe removes a subscription.
func (m *HTTPMonitor) Unsubscribe(ch <-chan Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the matching send channel
	for sendCh := range m.subscribers {
		// This is a hack - in real code we'd need to track both ends
		if sendCh == ch {
			close(sendCh)
			delete(m.subscribers, sendCh)
			break
		}
	}
}

// handleStatus returns current agent state.
func (m *HTTPMonitor) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]any{
		"running":     m.running,
		"subscribers": len(m.subscribers),
		"history":     len(m.history),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleMetrics returns execution metrics.
func (m *HTTPMonitor) handleMetrics(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Count events by type
	eventCounts := make(map[string]int)
	for _, event := range m.history {
		eventCounts[string(event.Type)]++
	}

	metrics := map[string]any{
		"event_counts": eventCounts,
		"total_events": len(m.history),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// handleEvents implements Server-Sent Events stream.
func (m *HTTPMonitor) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := m.Subscribe()
	defer m.Unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			w.Write([]byte("data: "))
			w.Write(data)
			w.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
}

// handleHistory returns recent events.
func (m *HTTPMonitor) handleHistory(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	history := make([]Event, len(m.history))
	copy(history, m.history)
	m.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// NoopMonitor is a no-op monitor implementation.
type NoopMonitor struct{}

// NewNoopMonitor creates a no-op monitor.
func NewNoopMonitor() *NoopMonitor {
	return &NoopMonitor{}
}

// Start is a no-op.
func (m *NoopMonitor) Start(ctx context.Context) error { return nil }

// Stop is a no-op.
func (m *NoopMonitor) Stop() error { return nil }

// Emit is a no-op.
func (m *NoopMonitor) Emit(event Event) {}

// Subscribe returns a closed channel.
func (m *NoopMonitor) Subscribe() <-chan Event {
	ch := make(chan Event)
	close(ch)
	return ch
}

// Unsubscribe is a no-op.
func (m *NoopMonitor) Unsubscribe(ch <-chan Event) {}
