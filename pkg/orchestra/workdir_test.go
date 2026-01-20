package orchestra

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkdirManager_Creation(t *testing.T) {
	tmpDir := t.TempDir()

	mgr, err := NewWorkdirManager(tmpDir)
	require.NoError(t, err)

	// Should create .claude/workdir
	workdirBase := filepath.Join(tmpDir, ".claude", "workdir")
	assert.DirExists(t, workdirBase)
	assert.NotNil(t, mgr)
}

func TestWorkdirManager_Create(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewWorkdirManager(tmpDir)
	require.NoError(t, err)

	path, err := mgr.Create("test-task")
	require.NoError(t, err)
	assert.DirExists(t, path)
	assert.Contains(t, path, "test-task")
}

func TestWorkdirManager_WriteRequirements(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewWorkdirManager(tmpDir)
	require.NoError(t, err)

	_, err = mgr.Create("test")
	require.NoError(t, err)

	content := "# Requirements\n\n1. First requirement"
	err = mgr.WriteRequirements(content)
	require.NoError(t, err)

	// Read back
	read, err := mgr.ReadRequirements()
	require.NoError(t, err)
	assert.Equal(t, content, read)
}

func TestWorkdirManager_WriteStep(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewWorkdirManager(tmpDir)
	require.NoError(t, err)

	_, err = mgr.Create("test")
	require.NoError(t, err)

	content := "# Step 1\n\nImplement feature X"
	err = mgr.WriteStep(1, content)
	require.NoError(t, err)

	// Read back
	read, err := mgr.ReadStep(1)
	require.NoError(t, err)
	assert.Equal(t, content, read)
}

func TestWorkdirManager_WriteStepImpl(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewWorkdirManager(tmpDir)
	require.NoError(t, err)

	_, err = mgr.Create("test")
	require.NoError(t, err)

	content := "# Step 1 Implementation\n\nChanges made..."
	err = mgr.WriteStepImpl(1, content)
	require.NoError(t, err)

	read, err := mgr.ReadStepImpl(1)
	require.NoError(t, err)
	assert.Equal(t, content, read)
}

func TestWorkdirManager_WriteSummary(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewWorkdirManager(tmpDir)
	require.NoError(t, err)

	_, err = mgr.Create("test")
	require.NoError(t, err)

	content := "# Summary\n\nTask completed successfully"
	err = mgr.WriteSummary(content)
	require.NoError(t, err)

	read, err := mgr.ReadSummary()
	require.NoError(t, err)
	assert.Equal(t, content, read)
}

func TestWorkdirManager_HasSummary(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewWorkdirManager(tmpDir)
	require.NoError(t, err)

	_, err = mgr.Create("test")
	require.NoError(t, err)

	// Initially should not exist
	assert.False(t, mgr.HasSummary())

	// Write summary
	err = mgr.WriteSummary("# Summary")
	require.NoError(t, err)

	// Now should exist
	assert.True(t, mgr.HasSummary())
}

func TestWorkdirManager_WriteLog(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewWorkdirManager(tmpDir)
	require.NoError(t, err)

	path, err := mgr.Create("test")
	require.NoError(t, err)

	content := []byte("build output here...")
	err = mgr.WriteLog("build.log", content)
	require.NoError(t, err)

	logPath := filepath.Join(path, "logs", "build.log")
	assert.FileExists(t, logPath)
}

func TestWorkdirManager_ListFiles(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewWorkdirManager(tmpDir)
	require.NoError(t, err)

	_, err = mgr.Create("test")
	require.NoError(t, err)

	mgr.WriteRequirements("reqs")
	mgr.WriteStep(1, "step 1")
	mgr.WriteSummary("summary")

	files, err := mgr.ListFiles()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(files), 3)
}

func TestWorkdirManager_Path(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewWorkdirManager(tmpDir)
	require.NoError(t, err)

	// Before Create, Path should be empty
	assert.Empty(t, mgr.Path())

	// After Create, should have path
	created, _ := mgr.Create("test")
	assert.Equal(t, created, mgr.Path())
}

func TestWorkdirManager_NoWorkdir(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewWorkdirManager(tmpDir)
	require.NoError(t, err)

	// Without calling Create, write operations should fail
	err = mgr.WriteRequirements("test")
	assert.Error(t, err)
}

func TestGetLatestWorkdir(t *testing.T) {
	tmpDir := t.TempDir()
	mgr, err := NewWorkdirManager(tmpDir)
	require.NoError(t, err)

	// Create a workdir
	_, err = mgr.Create("first-task")
	require.NoError(t, err)

	// Get latest should find it
	latest, err := GetLatestWorkdir(tmpDir)
	require.NoError(t, err)
	assert.Contains(t, latest, "first-task")
}

func TestGetLatestWorkdir_NoWorkdirs(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create empty workdir base
	os.MkdirAll(filepath.Join(tmpDir, ".claude", "workdir"), 0755)

	_, err := GetLatestWorkdir(tmpDir)
	assert.Error(t, err)
}
