package chunk

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"forge/internal/hash"
	"forge/internal/object"
)

func newTestChunker(t *testing.T, size int64) *Chunker {
	t.Helper()

	root := filepath.Join(t.TempDir(), "objects")

	store, err := object.NewStore(root)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	chunker, err := NewWithSize(store, size)
	if err != nil {
		t.Fatalf("NewWithSize() error = %v", err)
	}

	return chunker
}

func TestChunk(t *testing.T) {
	chunker := newTestChunker(t, 4)

	data := []byte("abcdefghij")

	ids, err := chunker.Chunk(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(ids) != 3 {
		t.Fatalf("got %d chunks, want 3", len(ids))
	}

	expected := []string{
		hash.Sum([]byte("abcd")),
		hash.Sum([]byte("efgh")),
		hash.Sum([]byte("ij")),
	}

	for i := range expected {
		if ids[i] != expected[i] {
			t.Fatalf(
				"chunk %d = %q, want %q",
				i,
				ids[i],
				expected[i],
			)
		}
	}
}

func TestChunkExactBoundary(t *testing.T) {
	chunker := newTestChunker(t, 4)

	data := []byte("abcdefgh")

	ids, err := chunker.Chunk(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(ids) != 2 {
		t.Fatalf("got %d chunks, want 2", len(ids))
	}

	if ids[0] != hash.Sum([]byte("abcd")) {
		t.Fatalf("first chunk has wrong ID")
	}

	if ids[1] != hash.Sum([]byte("efgh")) {
		t.Fatalf("second chunk has wrong ID")
	}
}

func TestChunkEmpty(t *testing.T) {
	chunker := newTestChunker(t, 4)

	ids, err := chunker.Chunk(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(ids) != 0 {
		t.Fatalf("got %d chunks, want 0", len(ids))
	}
}

func TestChunkSingleSmallFile(t *testing.T) {
	chunker := newTestChunker(t, 4)

	data := []byte("abc")

	ids, err := chunker.Chunk(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(ids) != 1 {
		t.Fatalf("got %d chunks, want 1", len(ids))
	}

	if ids[0] != hash.Sum(data) {
		t.Fatalf("chunk ID = %q, want %q", ids[0], hash.Sum(data))
	}
}

func TestChunkFile(t *testing.T) {
	chunker := newTestChunker(t, 4)

	path := filepath.Join(t.TempDir(), "example.bin")

	data := []byte("abcdefghij")

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	ids, err := chunker.ChunkFile(path)
	if err != nil {
		t.Fatalf("ChunkFile() error = %v", err)
	}

	if len(ids) != 3 {
		t.Fatalf("got %d chunks, want 3", len(ids))
	}
}

func TestChunkFileMissing(t *testing.T) {
	chunker := newTestChunker(t, 4)

	_, err := chunker.ChunkFile("/does/not/exist")

	if err == nil {
		t.Fatal("ChunkFile() succeeded for missing file")
	}
}

func TestInvalidChunkSize(t *testing.T) {
	root := filepath.Join(t.TempDir(), "objects")

	store, err := object.NewStore(root)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	tests := []int64{0, -1}

	for _, size := range tests {
		t.Run(
			fmt.Sprintf("%d", size),
			func(t *testing.T) {
				_, err := NewWithSize(store, size)

				if !errors.Is(err, ErrInvalidChunkSize) {
					t.Fatalf(
						"NewWithSize(%d) error = %v, want ErrInvalidChunkSize",
						size,
						err,
					)
				}
			},
		)
	}
}

func TestNilStore(t *testing.T) {
	_, err := NewWithSize(nil, 4)

	if err == nil {
		t.Fatal("NewWithSize(nil, 4) succeeded")
	}
}

func TestNilReader(t *testing.T) {
	chunker := newTestChunker(t, 4)

	_, err := chunker.Chunk(nil)

	if err == nil {
		t.Fatal("Chunk(nil) succeeded")
	}
}

func TestChunkDeduplicates(t *testing.T) {
	chunker := newTestChunker(t, 4)

	// "abcd" appears twice.
	data := []byte("abcdxxxxabcd")

	ids, err := chunker.Chunk(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}

	if len(ids) != 3 {
		t.Fatalf("got %d chunks, want 3", len(ids))
	}

	if ids[0] != ids[2] {
		t.Fatalf(
			"identical chunks produced different IDs: %q != %q",
			ids[0],
			ids[2],
		)
	}
}
