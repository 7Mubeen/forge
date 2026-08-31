package commit

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"forge/internal/object"
)

var (
	ErrCommitNotFound = errors.New("commit not found")
	ErrCommitCorrupt  = errors.New("commit corrupt")
)

// Store persists commits using the Forge content-addressed object store.
type Store struct {
	objects *object.Store
}

// NewStore creates a commit store backed by an object store.
func NewStore(objects *object.Store) (*Store, error) {
	if objects == nil {
		return nil, errors.New("object store is nil")
	}

	return &Store{
		objects: objects,
	}, nil
}

// Put validates and stores a commit.
//
// The returned ID is the content ID of the serialized commit.
func (s *Store) Put(c *Commit) (string, error) {
	if s == nil || s.objects == nil {
		return "", errors.New("commit store is not initialized")
	}

	data, err := c.Marshal()
	if err != nil {
		return "", fmt.Errorf("marshal commit: %w", err)
	}

	id, err := s.objects.Put(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("store commit: %w", err)
	}

	return id, nil
}

// Get retrieves and validates a commit by object ID.
func (s *Store) Get(id string) (*Commit, error) {
	if s == nil || s.objects == nil {
		return nil, errors.New("commit store is not initialized")
	}

	if err := s.Verify(id); err != nil {
		return nil, err
	}

	reader, err := s.objects.Get(id)
	if err != nil {
		if errors.Is(err, object.ErrObjectNotFound) {
			return nil, ErrCommitNotFound
		}

		return nil, fmt.Errorf(
			"%w: get object: %v",
			ErrCommitCorrupt,
			err,
		)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: read object: %v",
			ErrCommitCorrupt,
			err,
		)
	}

	c, err := Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: invalid commit data: %v",
			ErrCommitCorrupt,
			err,
		)
	}

	return c, nil
}

// Exists reports whether a commit object exists.
//
// It only checks object existence. It does not validate the commit.
func (s *Store) Exists(id string) (bool, error) {
	if s == nil || s.objects == nil {
		return false, errors.New("commit store is not initialized")
	}

	exists, err := s.objects.Exists(id)
	if err != nil {
		return false, fmt.Errorf("check commit: %w", err)
	}

	return exists, nil
}

// Verify verifies the integrity of a commit object.
//
// This checks both the content hash and the serialized commit structure.
func (s *Store) Verify(id string) error {
	if s == nil || s.objects == nil {
		return errors.New("commit store is not initialized")
	}

	if err := s.objects.Verify(id); err != nil {
		if errors.Is(err, object.ErrObjectNotFound) {
			return ErrCommitNotFound
		}

		if errors.Is(err, object.ErrInvalidObject) {
			return fmt.Errorf(
				"%w: object hash verification failed",
				ErrCommitCorrupt,
			)
		}

		return fmt.Errorf(
			"%w: object verification failed: %v",
			ErrCommitCorrupt,
			err,
		)
	}

	reader, err := s.objects.Get(id)
	if err != nil {
		if errors.Is(err, object.ErrObjectNotFound) {
			return ErrCommitNotFound
		}

		return fmt.Errorf(
			"%w: get object for validation: %v",
			ErrCommitCorrupt,
			err,
		)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf(
			"%w: read object for validation: %v",
			ErrCommitCorrupt,
			err,
		)
	}

	if _, err := Unmarshal(data); err != nil {
		return fmt.Errorf(
			"%w: invalid commit data: %v",
			ErrCommitCorrupt,
			err,
		)
	}

	return nil
}
