package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"forge/internal/commit"
	"forge/internal/manifest"
	"forge/internal/object"
)

const (
	// ForgeDir is the name of the hidden directory used by Forge.
	ForgeDir = ".forge"

	// ObjectsDir is the subdirectory for content-addressed objects.
	ObjectsDir = "objects"

	// RefsDir is the subdirectory for references (branches/tags).
	RefsDir = "refs"

	// ConfigFile is the name of the repository configuration file.
	ConfigFile = "config"
)

// Config represents the repository configuration.
type Config struct {
	// Version is the format version of the config file.
	Version int `json:"version"`

	// DefaultChunkSize is the default chunk size in bytes for new operations.
	DefaultChunkSize int64 `json:"default_chunk_size"`
}

// Repository represents a local Forge repository.
type Repository struct {
	Root string

	ObjectStore   *object.Store
	ManifestStore *manifest.Store
	CommitStore   *commit.Store

	Config Config
}

// Init creates a new Forge repository at the given root path.
func Init(root string) (*Repository, error) {
	forgePath := filepath.Join(root, ForgeDir)

	// Check if already exists
	if _, err := os.Stat(forgePath); err == nil {
		return nil, fmt.Errorf("repository already exists at %s", root)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("error checking repository path: %w", err)
	}

	// Create directory structure
	dirs := []string{
		filepath.Join(forgePath, ObjectsDir),
		filepath.Join(forgePath, RefsDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create default config
	config := Config{
		Version:          1,
		DefaultChunkSize: 4 * 1024 * 1024, // 4 MiB
	}

	configPath := filepath.Join(forgePath, ConfigFile)
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config file: %w", err)
	}

	// Initialize stores
	objectStore, err := object.NewStore(filepath.Join(forgePath, ObjectsDir))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize object store: %w", err)
	}

	manifestStore, err := manifest.NewStore(objectStore)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize manifest store: %w", err)
	}

	commitStore, err := commit.NewStore(objectStore)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize commit store: %w", err)
	}

	return &Repository{
		Root:          root,
		ObjectStore:   objectStore,
		ManifestStore: manifestStore,
		CommitStore:   commitStore,
		Config:        config,
	}, nil
}

// Open opens an existing Forge repository at the given root path.
func Open(root string) (*Repository, error) {
	forgePath := filepath.Join(root, ForgeDir)

	// Check if .forge directory exists
	info, err := os.Stat(forgePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no forge repository found at %s", root)
		}
		return nil, fmt.Errorf("error accessing repository path: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", forgePath)
	}

	// Load config
	configPath := filepath.Join(forgePath, ConfigFile)
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.Version != 1 {
		return nil, fmt.Errorf("unsupported repository config version: %d", config.Version)
	}

	// Initialize stores
	objectStore, err := object.NewStore(filepath.Join(forgePath, ObjectsDir))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize object store: %w", err)
	}

	manifestStore, err := manifest.NewStore(objectStore)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize manifest store: %w", err)
	}

	commitStore, err := commit.NewStore(objectStore)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize commit store: %w", err)
	}

	return &Repository{
		Root:          root,
		ObjectStore:   objectStore,
		ManifestStore: manifestStore,
		CommitStore:   commitStore,
		Config:        config,
	}, nil
}

// Exists checks if a Forge repository exists at the given root path.
func Exists(root string) bool {
	forgePath := filepath.Join(root, ForgeDir)
	info, err := os.Stat(forgePath)
	return err == nil && info.IsDir()
}

// GetHead returns the current HEAD commit ID, or an empty string if the repository is new.
func (r *Repository) GetHead() (string, error) {
	headPath := filepath.Join(r.Root, ForgeDir, "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// SetHead updates the HEAD reference to point to the given commit ID.
func (r *Repository) SetHead(commitID string) error {
	headPath := filepath.Join(r.Root, ForgeDir, "HEAD")
	return os.WriteFile(headPath, []byte(commitID+"\n"), 0644)
}
