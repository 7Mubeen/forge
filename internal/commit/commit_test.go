package commit

import (
	"strings"
	"testing"
	"time"

	"forge/internal/hash"
)

func testManifestID() string {
	return hash.Sum([]byte("test manifest"))
}

func testParentID() string {
	return hash.Sum([]byte("parent commit"))
}

func TestNewCommit(t *testing.T) {
	manifestID := testManifestID()

	c, err := New(
		manifestID,
		"mub",
		"initial dataset",
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if c.Version != CurrentVersion {
		t.Fatalf(
			"Version = %d, want %d",
			c.Version,
			CurrentVersion,
		)
	}

	if c.Manifest != manifestID {
		t.Fatalf(
			"Manifest = %q, want %q",
			c.Manifest,
			manifestID,
		)
	}

	if c.Parent != "" {
		t.Fatalf(
			"Parent = %q, want empty",
			c.Parent,
		)
	}

	if c.Author != "mub" {
		t.Fatalf(
			"Author = %q, want %q",
			c.Author,
			"mub",
		)
	}

	if c.Message != "initial dataset" {
		t.Fatalf(
			"Message = %q, want %q",
			c.Message,
			"initial dataset",
		)
	}

	if c.Timestamp.IsZero() {
		t.Fatal("Timestamp is zero")
	}
}

func TestNewCommitWithParent(t *testing.T) {
	parentID := testParentID()
	manifestID := testManifestID()

	c, err := NewWithParent(
		parentID,
		manifestID,
		"mub",
		"second version",
	)
	if err != nil {
		t.Fatalf("NewWithParent() error = %v", err)
	}

	if c.Parent != parentID {
		t.Fatalf(
			"Parent = %q, want %q",
			c.Parent,
			parentID,
		)
	}

	if c.Manifest != manifestID {
		t.Fatalf(
			"Manifest = %q, want %q",
			c.Manifest,
			manifestID,
		)
	}
}

func TestCommitValidate(t *testing.T) {
	c := &Commit{
		Version:   CurrentVersion,
		Manifest:  testManifestID(),
		Timestamp: time.Now().UTC(),
		Author:    "mub",
		Message:   "valid commit",
	}

	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestCommitMarshalUnmarshal(t *testing.T) {
	timestamp := time.Date(
		2026,
		8,
		31,
		12,
		30,
		45,
		0,
		time.UTC,
	)

	original := &Commit{
		Version:   CurrentVersion,
		Parent:    testParentID(),
		Manifest:  testManifestID(),
		Timestamp: timestamp,
		Author:    "mub",
		Message:   "dataset update",
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
		t.Fatal("decoded commit differs from original")
	}
}

func TestCommitMarshalDeterministic(t *testing.T) {
	timestamp := time.Date(
		2026,
		8,
		31,
		12,
		30,
		45,
		0,
		time.UTC,
	)

	first := &Commit{
		Version:   CurrentVersion,
		Manifest:  testManifestID(),
		Timestamp: timestamp,
		Author:    "mub",
		Message:   "same commit",
	}

	second := &Commit{
		Version:   CurrentVersion,
		Manifest:  testManifestID(),
		Timestamp: timestamp,
		Author:    "mub",
		Message:   "same commit",
	}

	firstData, err := first.Marshal()
	if err != nil {
		t.Fatalf("first Marshal() error = %v", err)
	}

	secondData, err := second.Marshal()
	if err != nil {
		t.Fatalf("second Marshal() error = %v", err)
	}

	if string(firstData) != string(secondData) {
		t.Fatalf(
			"identical commits produced different encodings:\n%s\n%s",
			firstData,
			secondData,
		)
	}
}

func TestCommitInvalidVersion(t *testing.T) {
	c := &Commit{
		Version:   999,
		Manifest:  testManifestID(),
		Timestamp: time.Now().UTC(),
		Author:    "mub",
		Message:   "invalid version",
	}

	if err := c.Validate(); err == nil {
		t.Fatal("Validate() accepted unsupported version")
	}
}

func TestCommitInvalidManifest(t *testing.T) {
	c := &Commit{
		Version:   CurrentVersion,
		Manifest:  "invalid",
		Timestamp: time.Now().UTC(),
		Author:    "mub",
		Message:   "invalid manifest",
	}

	if err := c.Validate(); err == nil {
		t.Fatal("Validate() accepted invalid manifest ID")
	}
}

func TestCommitInvalidParent(t *testing.T) {
	c := &Commit{
		Version:   CurrentVersion,
		Parent:    "invalid",
		Manifest:  testManifestID(),
		Timestamp: time.Now().UTC(),
		Author:    "mub",
		Message:   "invalid parent",
	}

	if err := c.Validate(); err == nil {
		t.Fatal("Validate() accepted invalid parent ID")
	}
}

func TestCommitMissingTimestamp(t *testing.T) {
	c := &Commit{
		Version:  CurrentVersion,
		Manifest: testManifestID(),
		Author:   "mub",
		Message:  "missing timestamp",
	}

	if err := c.Validate(); err == nil {
		t.Fatal("Validate() accepted zero timestamp")
	}
}

func TestCommitMissingAuthor(t *testing.T) {
	c := &Commit{
		Version:   CurrentVersion,
		Manifest:  testManifestID(),
		Timestamp: time.Now().UTC(),
		Message:   "missing author",
	}

	if err := c.Validate(); err == nil {
		t.Fatal("Validate() accepted empty author")
	}
}

func TestCommitWhitespaceAuthor(t *testing.T) {
	c := &Commit{
		Version:   CurrentVersion,
		Manifest:  testManifestID(),
		Timestamp: time.Now().UTC(),
		Author:    "   ",
		Message:   "invalid author",
	}

	if err := c.Validate(); err == nil {
		t.Fatal("Validate() accepted whitespace-only author")
	}
}

func TestCommitMissingMessage(t *testing.T) {
	c := &Commit{
		Version:   CurrentVersion,
		Manifest:  testManifestID(),
		Timestamp: time.Now().UTC(),
		Author:    "mub",
	}

	if err := c.Validate(); err == nil {
		t.Fatal("Validate() accepted empty message")
	}
}

func TestCommitWhitespaceMessage(t *testing.T) {
	c := &Commit{
		Version:   CurrentVersion,
		Manifest:  testManifestID(),
		Timestamp: time.Now().UTC(),
		Author:    "mub",
		Message:   "   ",
	}

	if err := c.Validate(); err == nil {
		t.Fatal("Validate() accepted whitespace-only message")
	}
}

func TestCommitNil(t *testing.T) {
	var c *Commit

	if err := c.Validate(); err == nil {
		t.Fatal("Validate() accepted nil commit")
	}
}

func TestUnmarshalEmpty(t *testing.T) {
	_, err := Unmarshal(nil)

	if err == nil {
		t.Fatal("Unmarshal() accepted empty data")
	}
}

func TestUnmarshalInvalidJSON(t *testing.T) {
	_, err := Unmarshal([]byte("{invalid json"))

	if err == nil {
		t.Fatal("Unmarshal() accepted invalid JSON")
	}
}

func TestCommitEqual(t *testing.T) {
	timestamp := time.Date(
		2026,
		8,
		31,
		12,
		30,
		45,
		0,
		time.UTC,
	)

	first := &Commit{
		Version:   CurrentVersion,
		Manifest:  testManifestID(),
		Timestamp: timestamp,
		Author:    "mub",
		Message:   "same",
	}

	second := &Commit{
		Version:   CurrentVersion,
		Manifest:  testManifestID(),
		Timestamp: timestamp,
		Author:    "mub",
		Message:   "same",
	}

	if !Equal(first, second) {
		t.Fatal("Equal() = false, want true")
	}

	second.Message = "different"

	if Equal(first, second) {
		t.Fatal("Equal() = true for different commits")
	}
}

func TestCommitObjectIDsAreValid(t *testing.T) {
	ids := []string{
		testManifestID(),
		testParentID(),
	}

	for _, id := range ids {
		if !strings.HasPrefix(id, "blake3:") {
			t.Fatalf("invalid test object ID: %q", id)
		}
	}
}
