package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
)

const CurrentVersion = 1

var (
	ErrInvalidManifest = errors.New("invalid manifest")
	ErrDuplicatePath   = errors.New("duplicate manifest path")
)

// Manifest describes a complete filesystem state.
//
// A manifest contains metadata about files and the ordered chunk IDs
// required to reconstruct each file.
type Manifest struct {
	Version int    `json:"version"`
	Files   []File `json:"files"`
}

// File describes one file in a Manifest.
type File struct {
	Path   string   `json:"path"`
	Size   int64    `json:"size"`
	Chunks []string `json:"chunks"`
}

// New creates an empty V1 manifest.
func New() *Manifest {
	return &Manifest{
		Version: CurrentVersion,
		Files:   make([]File, 0),
	}
}

// AddFile adds a file to the manifest.
//
// Paths are normalized and must be repository-relative.
func (m *Manifest) AddFile(file File) error {
	if m == nil {
		return ErrInvalidManifest
	}

	if err := validateFile(file); err != nil {
		return err
	}

	normalized, err := normalizePath(file.Path)
	if err != nil {
		return err
	}

	file.Path = normalized

	for _, existing := range m.Files {
		if existing.Path == file.Path {
			return fmt.Errorf("%w: %s", ErrDuplicatePath, file.Path)
		}
	}

	// Prevent the caller from modifying the manifest by changing
	// the original chunk slice after AddFile returns.
	file.Chunks = append([]string(nil), file.Chunks...)

	m.Files = append(m.Files, file)

	return nil
}

// Marshal returns the canonical serialized representation.
//
// Files are sorted by path before serialization, making equivalent
// manifests produce identical bytes regardless of insertion order.
func (m *Manifest) Marshal() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	canonical := m.clone()

	sort.Slice(canonical.Files, func(i, j int) bool {
		return canonical.Files[i].Path < canonical.Files[j].Path
	})

	data, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}

	return data, nil
}

// Unmarshal parses and validates a serialized manifest.
func Unmarshal(data []byte) (*Manifest, error) {
	if len(data) == 0 {
		return nil, ErrInvalidManifest
	}

	var m Manifest

	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}

	if err := m.Validate(); err != nil {
		return nil, err
	}

	return &m, nil
}

// Validate checks that the manifest is structurally valid.
func (m *Manifest) Validate() error {
	if m == nil {
		return ErrInvalidManifest
	}

	if m.Version != CurrentVersion {
		return fmt.Errorf(
			"%w: unsupported version %d",
			ErrInvalidManifest,
			m.Version,
		)
	}

	seen := make(map[string]struct{}, len(m.Files))

	for _, file := range m.Files {
		if err := validateFile(file); err != nil {
			return err
		}

		normalized, err := normalizePath(file.Path)
		if err != nil {
			return err
		}

		if normalized != file.Path {
			return fmt.Errorf(
				"%w: path is not normalized: %q",
				ErrInvalidManifest,
				file.Path,
			)
		}

		if _, exists := seen[file.Path]; exists {
			return fmt.Errorf(
				"%w: %s",
				ErrDuplicatePath,
				file.Path,
			)
		}

		seen[file.Path] = struct{}{}
	}

	return nil
}

// Clone returns an independent copy of the manifest.
func (m *Manifest) Clone() *Manifest {
	if m == nil {
		return nil
	}

	return m.clone()
}

func (m *Manifest) clone() *Manifest {
	result := &Manifest{
		Version: m.Version,
		Files:   make([]File, len(m.Files)),
	}

	for i, file := range m.Files {
		result.Files[i] = File{
			Path:   file.Path,
			Size:   file.Size,
			Chunks: append([]string(nil), file.Chunks...),
		}
	}

	return result
}

func validateFile(file File) error {
	if file.Path == "" {
		return fmt.Errorf(
			"%w: file path is empty",
			ErrInvalidManifest,
		)
	}

	if file.Size < 0 {
		return fmt.Errorf(
			"%w: negative file size for %q",
			ErrInvalidManifest,
			file.Path,
		)
	}

	for _, id := range file.Chunks {
		if !validObjectID(id) {
			return fmt.Errorf(
				"%w: invalid chunk ID %q",
				ErrInvalidManifest,
				id,
			)
		}
	}

	return nil
}

func normalizePath(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf(
			"%w: empty path",
			ErrInvalidManifest,
		)
	}

	// Forge manifests always use forward slashes.
	value = strings.ReplaceAll(value, `\`, "/")

	// Manifest paths must be repository-relative.
	if strings.HasPrefix(value, "/") {
		return "", fmt.Errorf(
			"%w: absolute path %q",
			ErrInvalidManifest,
			value,
		)
	}

	cleaned := path.Clean(value)

	if cleaned == "." ||
		cleaned == ".." ||
		strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf(
			"%w: path escapes repository: %q",
			ErrInvalidManifest,
			value,
		)
	}

	return cleaned, nil
}

func validObjectID(id string) bool {
	const prefix = "blake3:"

	if !strings.HasPrefix(id, prefix) {
		return false
	}

	digest := strings.TrimPrefix(id, prefix)

	if len(digest) != 64 {
		return false
	}

	for _, c := range digest {
		if !((c >= '0' && c <= '9') ||
			(c >= 'a' && c <= 'f')) {
			return false
		}
	}

	return true
}

// Equal reports whether two manifests have the same canonical
// filesystem description.
func Equal(a, b *Manifest) bool {
	if a == nil || b == nil {
		return a == b
	}

	first, err := a.Marshal()
	if err != nil {
		return false
	}

	second, err := b.Marshal()
	if err != nil {
		return false
	}

	return bytes.Equal(first, second)
}
