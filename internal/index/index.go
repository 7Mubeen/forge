package index

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// Entry represents a single file in the index.
type Entry struct {
	Path   string   `json:"path"`
	Size   int64    `json:"size"`
	Hash   string   `json:"hash"`   // Overall BLAKE3 hash of the file content
	Chunks []string `json:"chunks"` // Ordered list of chunk object IDs
}

// Index represents the staging area for the next commit.
type Index struct {
	Entries []Entry `json:"entries"`
}

// Load reads the index from the given path.
// If the file does not exist, it returns an empty Index.
func Load(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Index{}, nil
		}
		return nil, fmt.Errorf("reading index: %w", err)
	}

	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parsing index: %w", err)
	}
	return &idx, nil
}

// Save writes the index to the given path.
// Entries are sorted by path before saving to ensure deterministic output.
func (idx *Index) Save(path string) error {
	sort.Slice(idx.Entries, func(i, j int) bool {
		return idx.Entries[i].Path < idx.Entries[j].Path
	})

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling index: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing index: %w", err)
	}
	return nil
}

// AddOrUpdate adds a new entry or updates an existing one.
func (idx *Index) AddOrUpdate(entry Entry) {
	for i, e := range idx.Entries {
		if e.Path == entry.Path {
			idx.Entries[i] = entry
			return
		}
	}
	idx.Entries = append(idx.Entries, entry)
}
