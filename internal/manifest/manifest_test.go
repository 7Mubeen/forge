package manifest

import (
	"strings"
	"testing"

	"forge/internal/hash"
)

func chunkID(data string) string {
	return hash.Sum([]byte(data))
}

func TestNew(t *testing.T) {
	m := New()

	if m.Version != CurrentVersion {
		t.Fatalf(
			"version = %d, want %d",
			m.Version,
			CurrentVersion,
		)
	}

	if m.Files == nil {
		t.Fatal("Files is nil")
	}

	if len(m.Files) != 0 {
		t.Fatalf("got %d files, want 0", len(m.Files))
	}
}

func TestAddFile(t *testing.T) {
	m := New()

	err := m.AddFile(File{
		Path: "images/cat.jpg",
		Size: 1234,
		Chunks: []string{
			chunkID("chunk one"),
			chunkID("chunk two"),
		},
	})
	if err != nil {
		t.Fatalf("AddFile() error = %v", err)
	}

	if len(m.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(m.Files))
	}

	file := m.Files[0]

	if file.Path != "images/cat.jpg" {
		t.Fatalf("path = %q, want %q", file.Path, "images/cat.jpg")
	}

	if file.Size != 1234 {
		t.Fatalf("size = %d, want 1234", file.Size)
	}

	if len(file.Chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(file.Chunks))
	}
}

func TestAddFileNormalizesPath(t *testing.T) {
	m := New()

	err := m.AddFile(File{
		Path:   "images/./cat.jpg",
		Size:   10,
		Chunks: []string{chunkID("cat")},
	})
	if err != nil {
		t.Fatalf("AddFile() error = %v", err)
	}

	if m.Files[0].Path != "images/cat.jpg" {
		t.Fatalf(
			"path = %q, want %q",
			m.Files[0].Path,
			"images/cat.jpg",
		)
	}
}

func TestAddFileDuplicate(t *testing.T) {
	m := New()

	file := File{
		Path:   "file.txt",
		Size:   4,
		Chunks: []string{chunkID("data")},
	}

	if err := m.AddFile(file); err != nil {
		t.Fatalf("first AddFile() error = %v", err)
	}

	err := m.AddFile(file)

	if err == nil {
		t.Fatal("second AddFile() succeeded")
	}

	if !strings.Contains(err.Error(), ErrDuplicatePath.Error()) {
		t.Fatalf(
			"error = %v, want duplicate path error",
			err,
		)
	}
}

func TestManifestDeterministic(t *testing.T) {
	first := New()

	if err := first.AddFile(File{
		Path:   "z.txt",
		Size:   1,
		Chunks: []string{chunkID("z")},
	}); err != nil {
		t.Fatal(err)
	}

	if err := first.AddFile(File{
		Path:   "a.txt",
		Size:   1,
		Chunks: []string{chunkID("a")},
	}); err != nil {
		t.Fatal(err)
	}

	second := New()

	if err := second.AddFile(File{
		Path:   "a.txt",
		Size:   1,
		Chunks: []string{chunkID("a")},
	}); err != nil {
		t.Fatal(err)
	}

	if err := second.AddFile(File{
		Path:   "z.txt",
		Size:   1,
		Chunks: []string{chunkID("z")},
	}); err != nil {
		t.Fatal(err)
	}

	firstBytes, err := first.Marshal()
	if err != nil {
		t.Fatalf("first Marshal() error = %v", err)
	}

	secondBytes, err := second.Marshal()
	if err != nil {
		t.Fatalf("second Marshal() error = %v", err)
	}

	if string(firstBytes) != string(secondBytes) {
		t.Fatalf(
			"equivalent manifests produced different bytes:\n%s\n%s",
			firstBytes,
			secondBytes,
		)
	}
}

func TestMarshalUnmarshal(t *testing.T) {
	original := New()

	if err := original.AddFile(File{
		Path: "dataset/images/image.bin",
		Size: 8,
		Chunks: []string{
			chunkID("abcd"),
			chunkID("efgh"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	decoded, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if !Equal(original, decoded) {
		t.Fatal("decoded manifest differs from original")
	}
}

func TestInvalidAbsolutePath(t *testing.T) {
	m := New()

	err := m.AddFile(File{
		Path:   "/etc/passwd",
		Size:   1,
		Chunks: []string{chunkID("data")},
	})

	if err == nil {
		t.Fatal("absolute path was accepted")
	}
}

func TestInvalidParentPath(t *testing.T) {
	m := New()

	err := m.AddFile(File{
		Path:   "../secret.txt",
		Size:   1,
		Chunks: []string{chunkID("data")},
	})

	if err == nil {
		t.Fatal("parent path was accepted")
	}
}

func TestInvalidObjectID(t *testing.T) {
	m := New()

	err := m.AddFile(File{
		Path:   "file.txt",
		Size:   1,
		Chunks: []string{"not-a-valid-object-id"},
	})

	if err == nil {
		t.Fatal("invalid chunk ID was accepted")
	}
}

func TestNegativeFileSize(t *testing.T) {
	m := New()

	err := m.AddFile(File{
		Path:   "file.txt",
		Size:   -1,
		Chunks: []string{chunkID("data")},
	})

	if err == nil {
		t.Fatal("negative file size was accepted")
	}
}

func TestUnsupportedVersion(t *testing.T) {
	data := []byte(`{"version":999,"files":[]}`)

	_, err := Unmarshal(data)
	if err == nil {
		t.Fatal("unsupported version was accepted")
	}
}

func TestEmptyManifest(t *testing.T) {
	m := New()

	data, err := m.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	decoded, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if !Equal(m, decoded) {
		t.Fatal("empty manifest changed after round trip")
	}
}

func TestChunkOrderPreserved(t *testing.T) {
	m := New()

	first := chunkID("first")
	second := chunkID("second")
	third := chunkID("third")

	if err := m.AddFile(File{
		Path: "file.bin",
		Size: 15,
		Chunks: []string{
			first,
			second,
			third,
		},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := m.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	decoded, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	chunks := decoded.Files[0].Chunks

	if chunks[0] != first ||
		chunks[1] != second ||
		chunks[2] != third {
		t.Fatalf("chunk order changed: %v", chunks)
	}
}

func TestAddFileCopiesChunks(t *testing.T) {
	m := New()

	chunks := []string{
		chunkID("one"),
		chunkID("two"),
	}

	if err := m.AddFile(File{
		Path:   "file.txt",
		Size:   6,
		Chunks: chunks,
	}); err != nil {
		t.Fatal(err)
	}

	chunks[0] = "changed"

	if m.Files[0].Chunks[0] == "changed" {
		t.Fatal("manifest shares caller's chunk slice")
	}
}

func TestEqual(t *testing.T) {
	first := New()
	second := New()

	id := chunkID("data")

	if err := first.AddFile(File{
		Path:   "file.txt",
		Size:   4,
		Chunks: []string{id},
	}); err != nil {
		t.Fatal(err)
	}

	if err := second.AddFile(File{
		Path:   "file.txt",
		Size:   4,
		Chunks: []string{id},
	}); err != nil {
		t.Fatal(err)
	}

	if !Equal(first, second) {
		t.Fatal("equivalent manifests are not equal")
	}
}
