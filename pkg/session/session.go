// Package session provides conversation history and state management.
package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ternarybob/iter/pkg/sdk"
)

// Session provides conversation history and state.
type Session interface {
	// ID returns the session identifier.
	ID() string

	// History returns conversation messages.
	History() []sdk.Message

	// AddMessage appends a message.
	AddMessage(msg sdk.Message)

	// GetState retrieves stored state.
	GetState(key string) (any, bool)

	// SetState stores state.
	SetState(key string, value any)

	// Clear removes all history.
	Clear()

	// Save persists the session.
	Save() error

	// Load restores the session.
	Load() error
}

// MemorySession implements Session with in-memory storage.
type MemorySession struct {
	mu       sync.RWMutex
	id       string
	messages []sdk.Message
	state    map[string]any
}

// NewMemorySession creates a new in-memory session.
func NewMemorySession(id string) *MemorySession {
	return &MemorySession{
		id:       id,
		messages: make([]sdk.Message, 0),
		state:    make(map[string]any),
	}
}

// ID returns the session identifier.
func (s *MemorySession) ID() string {
	return s.id
}

// History returns conversation messages.
func (s *MemorySession) History() []sdk.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]sdk.Message, len(s.messages))
	copy(result, s.messages)
	return result
}

// AddMessage appends a message.
func (s *MemorySession) AddMessage(msg sdk.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}

// GetState retrieves stored state.
func (s *MemorySession) GetState(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.state[key]
	return v, ok
}

// SetState stores state.
func (s *MemorySession) SetState(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state[key] = value
}

// Clear removes all history.
func (s *MemorySession) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = make([]sdk.Message, 0)
}

// Save is a no-op for memory session.
func (s *MemorySession) Save() error {
	return nil
}

// Load is a no-op for memory session.
func (s *MemorySession) Load() error {
	return nil
}

// FileSession implements Session with file-based persistence.
type FileSession struct {
	MemorySession
	path string
}

// NewFileSession creates a file-backed session.
func NewFileSession(id, dir string) (*FileSession, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	s := &FileSession{
		MemorySession: *NewMemorySession(id),
		path:          filepath.Join(dir, id+".json"),
	}

	// Try to load existing session
	_ = s.Load()

	return s, nil
}

// sessionData is the persisted session format.
type sessionData struct {
	ID        string            `json:"id"`
	Messages  []sdk.Message     `json:"messages"`
	State     map[string]any    `json:"state"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Save persists the session to disk.
func (s *FileSession) Save() error {
	s.mu.RLock()
	data := sessionData{
		ID:        s.id,
		Messages:  s.messages,
		State:     s.state,
		UpdatedAt: time.Now(),
	}
	s.mu.RUnlock()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, jsonData, 0644)
}

// Load restores the session from disk.
func (s *FileSession) Load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	var sd sessionData
	if err := json.Unmarshal(data, &sd); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.id = sd.ID
	s.messages = sd.Messages
	s.state = sd.State

	return nil
}

// Store manages multiple sessions.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]Session
	dir      string
}

// NewStore creates a new session store.
func NewStore(dir string) (*Store, error) {
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	return &Store{
		sessions: make(map[string]Session),
		dir:      dir,
	}, nil
}

// Get retrieves or creates a session.
func (st *Store) Get(id string) (Session, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if s, ok := st.sessions[id]; ok {
		return s, nil
	}

	var s Session
	var err error

	if st.dir != "" {
		s, err = NewFileSession(id, st.dir)
	} else {
		s = NewMemorySession(id)
	}

	if err != nil {
		return nil, err
	}

	st.sessions[id] = s
	return s, nil
}

// Delete removes a session.
func (st *Store) Delete(id string) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	delete(st.sessions, id)

	if st.dir != "" {
		path := filepath.Join(st.dir, id+".json")
		return os.Remove(path)
	}

	return nil
}

// List returns all session IDs.
func (st *Store) List() []string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	ids := make([]string, 0, len(st.sessions))
	for id := range st.sessions {
		ids = append(ids, id)
	}
	return ids
}
