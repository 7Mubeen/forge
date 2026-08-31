package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"forge/internal/object"
)

var (
	ErrManifestNotFound = errors.New("manifest not found")
	ErrManifestCorrupt  = errors.New("corrupt manifest")
)

// Store persists manifests using the Forge content-addressed object store.
type Store struct {
	objects *object.Store
}

// NewStore creates a manifest store backed by an object store.
func NewStore(objects *object.Store) (*Store, error) {
	if objects == nil {
		return nil, errors.New("object store is nil")
	}

	return &Store{
		objects: objects,
	}, nil
}

// Put stores a manifest and returns its content-derived object ID.
func (s *Store) Put(m *Manifest) (string, error) {
	if s == nil || s.objects == nil {
		return "", errors.New("manifest store is nil")
	}

	data, err := m.Marshal()
	if err != nil {
		return "", fmt.Errorf("marshal manifest: %w", err)
	}

	id, err := s.objects.Put(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("store manifest: %w", err)
	}

	return id, nil
}

// Get retrieves and validates a manifest by its object ID.
//
// The underlying object is verified against its content hash before
// the manifest is decoded.
func (s *Store) Get(id string) (*Manifest, error) {
	if s == nil || s.objects == nil {
		return nil, errors.New("manifest store is nil")
	}

	if err := s.Verify(id); err != nil {
		return nil, err
	}

	reader, err := s.objects.Get(id)
	if err != nil {
		if errors.Is(err, object.ErrObjectNotFound) {
			return nil, ErrManifestNotFound
		}

		return nil, fmt.Errorf(
			"%w: get object: %v",
			ErrManifestCorrupt,
			err,
		)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: read object: %v",
			ErrManifestCorrupt,
			err,
		)
	}

	m, err := Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: invalid manifest data: %v",
			ErrManifestCorrupt,
			err,
		)
	}

	return m, nil
}

// Exists reports whether a manifest object exists.
//
// It only checks object existence. It does not validate the manifest.
func (s *Store) Exists(id string) (bool, error) {
	if s == nil || s.objects == nil {
		return false, errors.New("manifest store is nil")
	}

	exists, err := s.objects.Exists(id)
	if err != nil {
		return false, fmt.Errorf("check manifest: %w", err)
	}

	return exists, nil
}

// Verify verifies the integrity of a manifest object.
//
// This checks the content hash and then validates the serialized
// manifest itself.
func (s *Store) Verify(id string) error {
	if s == nil || s.objects == nil {
		return errors.New("manifest store is nil")
	}

	if err := s.objects.Verify(id); err != nil {
		if errors.Is(err, object.ErrObjectNotFound) {
			return ErrManifestNotFound
		}

		if errors.Is(err, object.ErrInvalidObject) {
			return fmt.Errorf(
				"%w: object hash verification failed",
				ErrManifestCorrupt,
			)
		}

		return fmt.Errorf(
			"%w: object verification failed: %v",
			ErrManifestCorrupt,
			err,
		)
	}

	reader, err := s.objects.Get(id)
	if err != nil {
		if errors.Is(err, object.ErrObjectNotFound) {
			return ErrManifestNotFound
		}

		return fmt.Errorf(
			"%w: get object for validation: %v",
			ErrManifestCorrupt,
			err,
		)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf(
			"%w: read object for validation: %v",
			ErrManifestCorrupt,
			err,
		)
	}

	if _, err := Unmarshal(data); err != nil {
		return fmt.Errorf(
			"%w: invalid manifest data: %v",
			ErrManifestCorrupt,
			err,
		)
	}

	return nil
}
