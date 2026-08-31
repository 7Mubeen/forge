package object

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"forge/internal/hash"
)

var (
	ErrObjectNotFound = errors.New("object not found")
	ErrInvalidObject  = errors.New("invalid object")
)

// Store is a local content-addressed object store.
//
// Objects are immutable and identified by their content hash.
//
// The directory passed to NewStore is the objects directory itself:
//
//	.forge/objects/
type Store struct {
	root string
}

// NewStore creates a new object store rooted at root.
//
// If root does not exist, it is created.
func NewStore(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("object store root is empty")
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create object store: %w", err)
	}

	return &Store{
		root: root,
	}, nil
}

// Put streams content into the object store.
//
// The content is written to a temporary file while its BLAKE3 hash is
// calculated. Once hashing is complete, the temporary file is installed
// at the path determined by the resulting object ID.
//
// If the object already exists, the existing object is left untouched.
func (s *Store) Put(r io.Reader) (string, error) {
	if r == nil {
		return "", errors.New("reader is nil")
	}

	tmp, err := os.CreateTemp(s.root, ".tmp-*")
	if err != nil {
		return "", fmt.Errorf("create temporary object: %w", err)
	}

	tmpName := tmp.Name()
	cleanup := true

	defer func() {
		_ = tmp.Close()

		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	// Write the input to disk while hashing it.
	//
	// This keeps memory usage bounded regardless of object size.
	tee := io.TeeReader(r, tmp)

	id, err := hash.SumReader(tee)
	if err != nil {
		return "", fmt.Errorf("hash object: %w", err)
	}

	// Ensure all data has reached the filesystem before installation.
	if err := tmp.Sync(); err != nil {
		return "", fmt.Errorf("sync temporary object: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temporary object: %w", err)
	}

	objectPath, err := s.path(id)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(objectPath), 0o755); err != nil {
		return "", fmt.Errorf("create object directory: %w", err)
	}

	// os.Link provides create-if-absent semantics.
	//
	// This is important because objects are immutable. We must never
	// overwrite an object that another process may already have created.
	if err := os.Link(tmpName, objectPath); err != nil {
		if errors.Is(err, os.ErrExist) {
			// Another writer already stored the same content.
			return id, nil
		}

		return "", fmt.Errorf("install object: %w", err)
	}

	// The permanent object now exists at objectPath.
	cleanup = false

	if err := os.Remove(tmpName); err != nil {
		return "", fmt.Errorf("remove temporary object: %w", err)
	}

	return id, nil
}

// Get opens an object for reading.
//
// The caller is responsible for closing the returned ReadCloser.
func (s *Store) Get(id string) (io.ReadCloser, error) {
	objectPath, err := s.path(id)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(objectPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrObjectNotFound
		}

		return nil, fmt.Errorf("open object: %w", err)
	}

	return file, nil
}

// Exists reports whether an object exists.
//
// Exists only checks whether the object is present. It does not verify
// that its contents match its object ID.
func (s *Store) Exists(id string) (bool, error) {
	objectPath, err := s.path(id)
	if err != nil {
		return false, err
	}

	info, err := os.Stat(objectPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return false, fmt.Errorf("stat object: %w", err)
	}

	if info.IsDir() {
		return false, ErrInvalidObject
	}

	return true, nil
}

// Verify reads an object and verifies that its contents produce the
// requested object ID.
//
// Verification is streaming and does not load the entire object into memory.
func (s *Store) Verify(id string) error {
	file, err := s.Get(id)
	if err != nil {
		return err
	}
	defer file.Close()

	actual, err := hash.SumReader(file)
	if err != nil {
		return fmt.Errorf("hash object during verification: %w", err)
	}

	if actual != id {
		return fmt.Errorf(
			"%w: expected %s, got %s",
			ErrInvalidObject,
			id,
			actual,
		)
	}

	return nil
}

// path returns the filesystem path for an object.
//
// Example:
//
//	blake3:abcdef123456...
//
// becomes:
//
//	<root>/ab/cdef123456...
//
// The "blake3:" prefix remains part of the logical object ID but is not
// stored in the filesystem path.
func (s *Store) path(id string) (string, error) {
	const prefix = "blake3:"

	if !strings.HasPrefix(id, prefix) {
		return "", fmt.Errorf(
			"%w: invalid object ID %q",
			ErrInvalidObject,
			id,
		)
	}

	digest := strings.TrimPrefix(id, prefix)

	if len(digest) != 64 {
		return "", fmt.Errorf(
			"%w: invalid object ID %q",
			ErrInvalidObject,
			id,
		)
	}

	for _, c := range digest {
		if !isLowerHex(c) {
			return "", fmt.Errorf(
				"%w: invalid object ID %q",
				ErrInvalidObject,
				id,
			)
		}
	}

	return filepath.Join(
		s.root,
		digest[:2],
		digest[2:],
	), nil
}

func isLowerHex(c rune) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'a' && c <= 'f')
}
