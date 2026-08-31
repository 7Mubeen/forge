package commit

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"forge/internal/hash"
	"forge/internal/object"
)

func newTestCommitStore(t *testing.T) (*Store, string) {
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

func testCommit() *Commit {
	return &Commit{
		Version:  CurrentVersion,
		Manifest: hash.Sum([]byte("test manifest")),
		Timestamp: time.Date(
			2026,
			8,
			31,
			12,
			0,
			0,
			0,
			time.UTC,
		),
		Author:  "mub",
		Message: "test commit",
	}
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
	store, _ := newTestCommitStore(t)

	original := testCommit()

	id, err := store.Put(original)
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if id == "" {
		t.Fatal("Put() returned empty ID")
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if !Equal(original, got) {
		t.Fatal("retrieved commit differs from original")
	}
}

func TestStoreDeduplicates(t *testing.T) {
	store, _ := newTestCommitStore(t)

	firstID, err := store.Put(testCommit())
	if err != nil {
		t.Fatalf("first Put() error = %v", err)
	}

	secondID, err := store.Put(testCommit())
	if err != nil {
		t.Fatalf("second Put() error = %v", err)
	}

	if firstID != secondID {
		t.Fatalf(
			"identical commits produced different IDs: %q != %q",
			firstID,
			secondID,
		)
	}
}

func TestStoreExists(t *testing.T) {
	store, _ := newTestCommitStore(t)

	id, err := store.Put(testCommit())
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

	missing := hash.Sum([]byte("missing commit"))

	exists, err = store.Exists(missing)
	if err != nil {
		t.Fatalf("Exists(missing) error = %v", err)
	}

	if exists {
		t.Fatal("Exists(missing) = true, want false")
	}
}

func TestStoreGetMissing(t *testing.T) {
	store, _ := newTestCommitStore(t)

	id := hash.Sum([]byte("missing commit"))

	_, err := store.Get(id)

	if !errors.Is(err, ErrCommitNotFound) {
		t.Fatalf(
			"Get() error = %v, want ErrCommitNotFound",
			err,
		)
	}
}

func TestStoreVerify(t *testing.T) {
	store, _ := newTestCommitStore(t)

	id, err := store.Put(testCommit())
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if err := store.Verify(id); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestStoreDetectsCorruption(t *testing.T) {
	store, root := newTestCommitStore(t)

	id, err := store.Put(testCommit())
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	path := objectPath(root, id)

	if err := os.WriteFile(
		path,
		[]byte("corrupted commit"),
		0o644,
	); err != nil {
		t.Fatalf("corrupt object: %v", err)
	}

	_, err = store.Get(id)

	if !errors.Is(err, ErrCommitCorrupt) {
		t.Fatalf(
			"Get() error = %v, want ErrCommitCorrupt",
			err,
		)
	}
}

func TestStoreVerifyMissing(t *testing.T) {
	store, _ := newTestCommitStore(t)

	id := hash.Sum([]byte("missing commit"))

	err := store.Verify(id)

	if !errors.Is(err, ErrCommitNotFound) {
		t.Fatalf(
			"Verify() error = %v, want ErrCommitNotFound",
			err,
		)
	}
}

func TestStoreRejectsInvalidCommit(t *testing.T) {
	store, _ := newTestCommitStore(t)

	invalid := &Commit{
		Version:   999,
		Manifest:  hash.Sum([]byte("manifest")),
		Timestamp: time.Now().UTC(),
		Author:    "mub",
		Message:   "invalid",
	}

	_, err := store.Put(invalid)

	if err == nil {
		t.Fatal("Put() accepted invalid commit")
	}
}

func TestStoreConcurrentDuplicatePut(t *testing.T) {
	store, _ := newTestCommitStore(t)

	commit := testCommit()

	const workers = 16

	ids := make([]string, workers)
	errs := make([]error, workers)

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func(index int) {
			defer wg.Done()

			ids[index], errs[index] = store.Put(commit)
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

func TestStoreParentCommit(t *testing.T) {
	store, _ := newTestCommitStore(t)

	parent := testCommit()

	parentID, err := store.Put(parent)
	if err != nil {
		t.Fatalf("store parent: %v", err)
	}

	child := &Commit{
		Version:   CurrentVersion,
		Parent:    parentID,
		Manifest:  hash.Sum([]byte("child manifest")),
		Timestamp: parent.Timestamp.Add(time.Minute),
		Author:    "mub",
		Message:   "second commit",
	}

	childID, err := store.Put(child)
	if err != nil {
		t.Fatalf("store child: %v", err)
	}

	got, err := store.Get(childID)
	if err != nil {
		t.Fatalf("Get(child) error = %v", err)
	}

	if got.Parent != parentID {
		t.Fatalf(
			"child Parent = %q, want %q",
			got.Parent,
			parentID,
		)
	}
}
