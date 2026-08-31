package object

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"forge/internal/hash"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	root := filepath.Join(t.TempDir(), "objects")

	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	return store
}

func TestPutAndGet(t *testing.T) {
	store := newTestStore(t)

	data := []byte("hello forge")

	id, err := store.Put(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	expected := hash.Sum(data)

	if id != expected {
		t.Fatalf("Put() ID = %q, want %q", id, expected)
	}

	reader, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if !bytes.Equal(got, data) {
		t.Fatalf("Get() = %q, want %q", got, data)
	}
}

func TestPutDeduplicates(t *testing.T) {
	store := newTestStore(t)

	data := []byte("duplicate content")

	firstID, err := store.Put(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("first Put() error = %v", err)
	}

	secondID, err := store.Put(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("second Put() error = %v", err)
	}

	if firstID != secondID {
		t.Fatalf(
			"duplicate content produced different IDs: %q != %q",
			firstID,
			secondID,
		)
	}

	path, err := store.path(firstID)
	if err != nil {
		t.Fatalf("path() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	if info.IsDir() {
		t.Fatal("object path is a directory")
	}
}

func TestExists(t *testing.T) {
	store := newTestStore(t)

	data := []byte("exists")

	id, err := store.Put(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	exists, err := store.Exists(id)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}

	if !exists {
		t.Fatal("Exists() = false, want true")
	}

	missing := hash.Sum([]byte("missing"))

	exists, err = store.Exists(missing)
	if err != nil {
		t.Fatalf("Exists(missing) error = %v", err)
	}

	if exists {
		t.Fatal("Exists(missing) = true, want false")
	}
}

func TestGetMissing(t *testing.T) {
	store := newTestStore(t)

	id := hash.Sum([]byte("does not exist"))

	_, err := store.Get(id)
	if !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf(
			"Get() error = %v, want ErrObjectNotFound",
			err,
		)
	}
}

func TestVerify(t *testing.T) {
	store := newTestStore(t)

	data := []byte("integrity matters")

	id, err := store.Put(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if err := store.Verify(id); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestVerifyDetectsCorruption(t *testing.T) {
	store := newTestStore(t)

	data := []byte("original content")

	id, err := store.Put(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	path, err := store.path(id)
	if err != nil {
		t.Fatalf("path() error = %v", err)
	}

	// Deliberately corrupt the object.
	if err := os.WriteFile(path, []byte("corrupted content"), 0o644); err != nil {
		t.Fatalf("corrupt object: %v", err)
	}

	err = store.Verify(id)
	if !errors.Is(err, ErrInvalidObject) {
		t.Fatalf(
			"Verify() error = %v, want ErrInvalidObject",
			err,
		)
	}
}

func TestEmptyObject(t *testing.T) {
	store := newTestStore(t)

	id, err := store.Put(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	reader, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if len(data) != 0 {
		t.Fatalf("empty object contains %d bytes", len(data))
	}

	if err := store.Verify(id); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestLargeStream(t *testing.T) {
	store := newTestStore(t)

	const size = 8 * 1024 * 1024

	data := make([]byte, size)

	for i := range data {
		data[i] = byte(i % 251)
	}

	id, err := store.Put(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	reader, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if !bytes.Equal(got, data) {
		t.Fatal("large object contents differ")
	}

	if err := store.Verify(id); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestInvalidObjectID(t *testing.T) {
	store := newTestStore(t)

	tests := []string{
		"",
		"invalid",
		"sha256:abcdef",
		"blake3:",
		"blake3:abc",
		"blake3:ABCDEF",
	}

	for _, id := range tests {
		t.Run(id, func(t *testing.T) {
			_, err := store.Get(id)

			if !errors.Is(err, ErrInvalidObject) {
				t.Fatalf(
					"Get(%q) error = %v, want ErrInvalidObject",
					id,
					err,
				)
			}
		})
	}
}

func TestConcurrentDuplicatePut(t *testing.T) {
	store := newTestStore(t)

	data := bytes.Repeat([]byte("forge"), 1024)

	const workers = 16

	ids := make([]string, workers)
	errs := make([]error, workers)

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func(index int) {
			defer wg.Done()

			ids[index], errs[index] =
				store.Put(bytes.NewReader(data))
		}(i)
	}

	wg.Wait()

	var expected string

	for i := 0; i < workers; i++ {
		if errs[i] != nil {
			t.Fatalf(
				"worker %d Put() error = %v",
				i,
				errs[i],
			)
		}

		if i == 0 {
			expected = ids[i]
			continue
		}

		if ids[i] != expected {
			t.Fatalf(
				"worker %d returned ID %q, want %q",
				i,
				ids[i],
				expected,
			)
		}
	}

	if err := store.Verify(expected); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}
