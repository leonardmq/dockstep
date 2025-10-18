package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dockstep.dev/types"
)

func TestStore(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)

	// Test Init
	if err := store.Init(); err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	// Check that directories were created
	dirs := []string{
		filepath.Join(tmpDir, ".dockstep", StateDir),
		filepath.Join(tmpDir, ".dockstep", LogsDir),
		filepath.Join(tmpDir, ".dockstep", ImagesDir),
		filepath.Join(tmpDir, ".dockstep", ArtifactsDir),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", dir)
		}
	}
}

func TestBlockState(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	store.Init()

	// Test SaveBlockState
	state := &types.BlockState{
		ID:        "test-block",
		Status:    types.StatusSuccess,
		Digest:    "sha256:abc123",
		Hash:      "def456",
		Timestamp: time.Now(),
		ExitCode:  0,
		Duration:  time.Second,
	}

	if err := store.SaveBlockState("test-block", state); err != nil {
		t.Fatalf("Failed to save block state: %v", err)
	}

	// Test LoadBlockState
	loaded, err := store.LoadBlockState("test-block")
	if err != nil {
		t.Fatalf("Failed to load block state: %v", err)
	}

	if loaded.ID != state.ID {
		t.Errorf("Expected ID %s, got %s", state.ID, loaded.ID)
	}

	if loaded.Status != state.Status {
		t.Errorf("Expected status %s, got %s", state.Status, loaded.Status)
	}
}

func TestLogs(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	store.Init()

	// Test SaveLogs
	logs := []byte("Hello, world!\nThis is a test log.")
	if err := store.SaveLogs("test-block", logs); err != nil {
		t.Fatalf("Failed to save logs: %v", err)
	}

	// Test LoadLogs
	loaded, err := store.LoadLogs("test-block")
	if err != nil {
		t.Fatalf("Failed to load logs: %v", err)
	}

	if string(loaded) != string(logs) {
		t.Errorf("Expected logs %s, got %s", string(logs), string(loaded))
	}
}

func TestDiff(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	store.Init()

	// Diff functionality removed in new schema - test image digest instead
	if err := store.SaveImageDigest("test-block", "sha256:test123"); err != nil {
		t.Fatalf("Failed to save image digest: %v", err)
	}

	// Test LoadImageDigest
	loaded, err := store.LoadImageDigest("test-block")
	if err != nil {
		t.Fatalf("Failed to load image digest: %v", err)
	}

	if loaded != "sha256:test123" {
		t.Errorf("Expected digest sha256:test123, got %s", loaded)
	}
}

func TestComputeBlockHash(t *testing.T) {
	block := types.Block{
		ID:           "test-block",
		From:         "alpine:latest",
		Instructions: []string{"RUN echo hello", "WORKDIR /app", "ENV KEY=value"},
		Context:      ".",
	}

	parentDigest := "sha256:parent123"
	hash1 := ComputeBlockHash(block, parentDigest)
	hash2 := ComputeBlockHash(block, parentDigest)

	// Hash should be deterministic
	if hash1 != hash2 {
		t.Error("Hash should be deterministic")
	}

	// Hash should be different for different inputs
	block2 := block
	block2.Instructions = []string{"RUN echo world", "WORKDIR /app", "ENV KEY=value"}
	hash3 := ComputeBlockHash(block2, parentDigest)

	if hash1 == hash3 {
		t.Error("Hash should be different for different commands")
	}
}
