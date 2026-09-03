package index

import (
	"path/filepath"
	"testing"
)

func TestLoadSave(t *testing.T) {
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "index")

	// Load non-existent should return empty index
	idx, err := Load(indexPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(idx.Entries) != 0 {
		t.Errorf("Expected empty index, got %d entries", len(idx.Entries))
	}

	// Add entry and save
	idx.AddOrUpdate(Entry{Path: "a.txt", Size: 10, Chunks: []string{"id1"}})
	if err := idx.Save(indexPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load again
	idx2, err := Load(indexPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(idx2.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(idx2.Entries))
	}
	if idx2.Entries[0].Path != "a.txt" {
		t.Errorf("Expected path a.txt, got %s", idx2.Entries[0].Path)
	}
}

func TestAddOrUpdate(t *testing.T) {
	idx := &Index{}
	idx.AddOrUpdate(Entry{Path: "a.txt", Size: 10, Chunks: []string{"id1"}})
	idx.AddOrUpdate(Entry{Path: "b.txt", Size: 20, Chunks: []string{"id2"}})

	if len(idx.Entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(idx.Entries))
	}

	// Update existing
	idx.AddOrUpdate(Entry{Path: "a.txt", Size: 15, Chunks: []string{"id3"}})
	if len(idx.Entries) != 2 {
		t.Fatalf("Expected 2 entries after update, got %d", len(idx.Entries))
	}
	if idx.Entries[0].Size != 15 {
		t.Errorf("Expected updated size 15, got %d", idx.Entries[0].Size)
	}
}
