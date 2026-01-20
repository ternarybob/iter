package session

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ternarybob/iter/pkg/sdk"
)

func TestMemorySession_Creation(t *testing.T) {
	session := NewMemorySession("test-session")

	assert.Equal(t, "test-session", session.ID())
	assert.Empty(t, session.History())
}

func TestMemorySession_AddMessage(t *testing.T) {
	session := NewMemorySession("test")

	msg := sdk.Message{Role: "user", Content: "Hello"}
	session.AddMessage(msg)

	history := session.History()
	require.Len(t, history, 1)
	assert.Equal(t, "Hello", history[0].Content)
}

func TestMemorySession_History(t *testing.T) {
	session := NewMemorySession("test")

	session.AddMessage(sdk.Message{Role: "user", Content: "msg1"})
	session.AddMessage(sdk.Message{Role: "assistant", Content: "msg2"})
	session.AddMessage(sdk.Message{Role: "user", Content: "msg3"})

	history := session.History()
	assert.Len(t, history, 3)
}

func TestMemorySession_State(t *testing.T) {
	session := NewMemorySession("test")

	session.SetState("key1", "value1")
	session.SetState("key2", 42)

	val1, ok1 := session.GetState("key1")
	assert.True(t, ok1)
	assert.Equal(t, "value1", val1)

	val2, ok2 := session.GetState("key2")
	assert.True(t, ok2)
	assert.Equal(t, 42, val2)

	_, ok3 := session.GetState("nonexistent")
	assert.False(t, ok3)
}

func TestMemorySession_Clear(t *testing.T) {
	session := NewMemorySession("test")

	session.AddMessage(sdk.Message{Role: "user", Content: "test"})
	require.NotEmpty(t, session.History())

	session.Clear()

	assert.Empty(t, session.History())
}

func TestFileSession_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Create and populate session
	session, err := NewFileSession("test-file-session", tmpDir)
	require.NoError(t, err)

	session.AddMessage(sdk.Message{Role: "user", Content: "Hello"})
	session.SetState("version", "1.0")

	// Save
	err = session.Save()
	require.NoError(t, err)

	// Verify file exists
	sessionPath := filepath.Join(tmpDir, "test-file-session.json")
	assert.FileExists(t, sessionPath)

	// Load into new session
	loaded, err := NewFileSession("test-file-session", tmpDir)
	require.NoError(t, err)

	// Verify data
	assert.Len(t, loaded.History(), 1)
	val, _ := loaded.GetState("version")
	assert.Equal(t, "1.0", val)
}

func TestFileSession_LoadNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Creating a new session for nonexistent file should work (creates new)
	session, err := NewFileSession("new-session", tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, session)
}

func TestStore_GetAndList(t *testing.T) {
	tmpDir := t.TempDir()
	
	store, err := NewStore(tmpDir)
	require.NoError(t, err)

	// Get creates session if not exists
	s1, err := store.Get("session1")
	require.NoError(t, err)
	assert.NotNil(t, s1)

	s2, err := store.Get("session2")
	require.NoError(t, err)
	assert.NotNil(t, s2)

	// List should show both
	ids := store.List()
	assert.Len(t, ids, 2)
}


func TestMemoryStore(t *testing.T) {
	// Store with empty dir uses memory sessions
	store, err := NewStore("")
	require.NoError(t, err)

	s, err := store.Get("memory-session")
	require.NoError(t, err)
	assert.NotNil(t, s)
}

func TestSession_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		messages int
		wantLen  int
	}{
		{
			name:     "empty session",
			messages: 0,
			wantLen:  0,
		},
		{
			name:     "single message",
			messages: 1,
			wantLen:  1,
		},
		{
			name:     "multiple messages",
			messages: 5,
			wantLen:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := NewMemorySession(tt.name)

			for i := 0; i < tt.messages; i++ {
				session.AddMessage(sdk.Message{Role: "user", Content: "msg"})
			}

			assert.Len(t, session.History(), tt.wantLen)
		})
	}
}
