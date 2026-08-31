package chunk

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"forge/internal/object"
)

// DefaultSize is the default V1 chunk size.
const DefaultSize int64 = 4 * 1024 * 1024

var ErrInvalidChunkSize = errors.New("invalid chunk size")

// Chunker splits a stream into fixed-size chunks and stores each chunk
// in a content-addressed object store.
type Chunker struct {
	store *object.Store
	size  int
}

// New creates a Chunker using the default chunk size.
func New(store *object.Store) (*Chunker, error) {
	return NewWithSize(store, DefaultSize)
}

// NewWithSize creates a Chunker using the specified chunk size.
func NewWithSize(store *object.Store, size int64) (*Chunker, error) {
	if store == nil {
		return nil, errors.New("object store is nil")
	}

	if size <= 0 {
		return nil, ErrInvalidChunkSize
	}

	// Forge V1 expects a chunk size that fits in an int because it
	// determines the size of the in-memory chunk buffer.
	if int64(int(size)) != size {
		return nil, ErrInvalidChunkSize
	}

	return &Chunker{
		store: store,
		size:  int(size),
	}, nil
}

// Chunk reads from r, splits the data into fixed-size chunks, and stores
// each chunk in the object store.
//
// The returned IDs preserve the exact order of the chunks.
//
// Empty input produces an empty chunk list.
func (c *Chunker) Chunk(r io.Reader) ([]string, error) {
	if r == nil {
		return nil, errors.New("reader is nil")
	}

	buffer := make([]byte, c.size)
	chunks := make([]string, 0)

	for {
		n, err := io.ReadFull(r, buffer)

		if n > 0 {
			id, storeErr := c.store.Put(bytes.NewReader(buffer[:n]))
			if storeErr != nil {
				return nil, fmt.Errorf("store chunk: %w", storeErr)
			}

			chunks = append(chunks, id)
		}

		if err == nil {
			continue
		}

		if errors.Is(err, io.EOF) ||
			errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}

		return nil, fmt.Errorf("read chunk: %w", err)
	}

	return chunks, nil
}

// ChunkFile opens path and chunks the file through the object store.
//
// The file is streamed and is never loaded entirely into memory.
func (c *Chunker) ChunkFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	return c.Chunk(file)
}
