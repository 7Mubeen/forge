package repository

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")

	// Initialize repository
	repo, err := Init(repoPath)
	if err != nil {
		t.Fatalf("Failed to init repository: %v", err)
	}

	if repo.Root != repoPath {
		t.Errorf("Expected root %s, got %s", repoPath, repo.Root)
	}

	if repo.Config.Version != 1 {
		t.Errorf("Expected config version 1, got %d", repo.Config.Version)
	}

	if repo.Config.DefaultChunkSize != 4*1024*1024 {
		t.Errorf("Expected default chunk size 4MiB, got %d", repo.Config.DefaultChunkSize)
	}

	// Check directory structure
	forgePath := filepath.Join(repoPath, ForgeDir)
	if _, err := os.Stat(forgePath); err != nil {
		t.Fatalf(".forge directory not created: %v", err)
	}

	objectsPath := filepath.Join(forgePath, ObjectsDir)
	if _, err := os.Stat(objectsPath); err != nil {
		t.Fatalf("objects directory not created: %v", err)
	}

	refsPath := filepath.Join(forgePath, RefsDir)
	if _, err := os.Stat(refsPath); err != nil {
		t.Fatalf("refs directory not created: %v", err)
	}

	configPath := filepath.Join(forgePath, ConfigFile)
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
}

func TestInitAlreadyExists(t *testing.T) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")

	// Initialize repository
	_, err := Init(repoPath)
	if err != nil {
		t.Fatalf("Failed to init repository: %v", err)
	}

	// Try to initialize again
	_, err = Init(repoPath)
	if err == nil {
		t.Fatal("Expected error when initializing existing repository")
	}
}

func TestOpen(t *testing.T) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")

	// Initialize repository
	_, err := Init(repoPath)
	if err != nil {
		t.Fatalf("Failed to init repository: %v", err)
	}

	// Open repository
	repo, err := Open(repoPath)
	if err != nil {
		t.Fatalf("Failed to open repository: %v", err)
	}

	if repo.Root != repoPath {
		t.Errorf("Expected root %s, got %s", repoPath, repo.Root)
	}

	if repo.Config.Version != 1 {
		t.Errorf("Expected config version 1, got %d", repo.Config.Version)
	}
}

func TestOpenNonExistent(t *testing.T) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "non-existent-repo")

	_, err := Open(repoPath)
	if err == nil {
		t.Fatal("Expected error when opening non-existent repository")
	}
}

func TestExists(t *testing.T) {
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "test-repo")

	// Should not exist initially
	if Exists(repoPath) {
		t.Fatal("Repository should not exist initially")
	}

	// Initialize repository
	_, err := Init(repoPath)
	if err != nil {
		t.Fatalf("Failed to init repository: %v", err)
	}

	// Should exist now
	if !Exists(repoPath) {
		t.Fatal("Repository should exist after initialization")
	}
}
