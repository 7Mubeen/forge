package manifest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"forge/internal/hash"
	"forge/internal/object"
)

func newTestManifestStore(t *testing.T) (*Store, string) {
	t.Helper()

	root := filepath.Join(t.TempDir(), "objects")

	objects, err := object.NewStore(root)
	if err != nil {
		t.Fatalf("object.NewStore() error = %v", err)
	}

	store, err := NewStore(objects)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	return store, root
}

func testManifest() *Manifest {
	m := New()

	if err := m.AddFile(File{
		Path: "dataset/images/a.bin",
		Size: 8,
		Chunks: []string{
			hash.Sum([]byte("chunk-a")),
			hash.Sum([]byte("chunk-b")),
		},
	}); err != nil {
		panic(err)
	}

	if err := m.AddFile(File{
		Path: "dataset/labels.txt",
		Size: 5,
		Chunks: []string{
			hash.Sum([]byte("labels")),
		},
	}); err != nil {
		panic(err)
	}

	return m
}

func objectPath(root, id string) string {
	digest := id[len("blake3:"):]

	return filepath.Join(
		root,
		digest[:2],
		digest[2:],
	)
}

func TestStorePutAndGet(t *testing.T) {
	store, _ := newTestManifestStore(t)

	original := testManifest()

	id, err := store.Put(original)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if id == "" {
		t.Fatal("Put() returned an empty ID")
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if !Equal(original, got) {
		t.Fatal("retrieved manifest differs from original")
	}
}

func TestStoreDeduplicatesIdenticalManifests(t *testing.T) {
	store, _ := newTestManifestStore(t)

	firstID, err := store.Put(testManifest())
	if err != nil {
		t.Fatalf("first Put() error = %v", err)
	}

	secondID, err := store.Put(testManifest())
	if err != nil {
		t.Fatalf("second Put() error = %v", err)
	}

	if firstID != secondID {
		t.Fatalf(
			"identical manifests produced different IDs: %q != %q",
			firstID,
			secondID,
		)
	}
}

func TestStoreEquivalentManifestsHaveSameID(t *testing.T) {
	store, _ := newTestManifestStore(t)

	first := New()

	if err := first.AddFile(File{
		Path:   "z.txt",
		Size:   3,
		Chunks: []string{hash.Sum([]byte("z"))},
	}); err != nil {
		t.Fatal(err)
	}

	if err := first.AddFile(File{
		Path:   "a.txt",
		Size:   3,
		Chunks: []string{hash.Sum([]byte("a"))},
	}); err != nil {
		t.Fatal(err)
	}

	second := New()

	if err := second.AddFile(File{
		Path:   "a.txt",
		Size:   3,
		Chunks: []string{hash.Sum([]byte("a"))},
	}); err != nil {
		t.Fatal(err)
	}

	if err := second.AddFile(File{
		Path:   "z.txt",
		Size:   3,
		Chunks: []string{hash.Sum([]byte("z"))},
	}); err != nil {
		t.Fatal(err)
	}

	firstID, err := store.Put(first)
	if err != nil {
		t.Fatalf("first Put() error = %v", err)
	}

	secondID, err := store.Put(second)
	if err != nil {
		t.Fatalf("second Put() error = %v", err)
	}

	if firstID != secondID {
		t.Fatalf(
			"equivalent manifests produced different IDs: %q != %q",
			firstID,
			secondID,
		)
	}
}

func TestStoreMissingManifest(t *testing.T) {
	store, _ := newTestManifestStore(t)

	id := hash.Sum([]byte("missing manifest"))

	_, err := store.Get(id)

	if !errors.Is(err, ErrManifestNotFound) {
		t.Fatalf(
			"Get() error = %v, want ErrManifestNotFound",
			err,
		)
	}
}

func TestStoreDetectsCorruptedManifest(t *testing.T) {
	store, root := newTestManifestStore(t)

	id, err := store.Put(testManifest())
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	path := objectPath(root, id)

	if err := os.WriteFile(
		path,
		[]byte("corrupted manifest"),
		0o644,
	); err != nil {
		t.Fatalf("corrupt object: %v", err)
	}

	_, err = store.Get(id)

	if !errors.Is(err, ErrManifestCorrupt) {
		t.Fatalf(
			"Get() error = %v, want ErrManifestCorrupt",
			err,
		)
	}
}

func TestStoreRejectsInvalidManifest(t *testing.T) {
	store, _ := newTestManifestStore(t)

	invalid := New()

	invalid.Files = append(invalid.Files, File{
		Path:   "/etc/passwd",
		Size:   10,
		Chunks: []string{hash.Sum([]byte("data"))},
	})

	_, err := store.Put(invalid)

	if err == nil {
		t.Fatal("Put() accepted invalid manifest")
	}
}

func TestNewStoreRejectsNilObjectStore(t *testing.T) {
	_, err := NewStore(nil)

	if err == nil {
		t.Fatal("NewStore(nil) succeeded")
	}
}
