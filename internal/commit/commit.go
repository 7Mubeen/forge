package commit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const CurrentVersion = 1

var (
	ErrInvalidCommit = errors.New("invalid commit")
	ErrInvalidID     = errors.New("invalid object ID")
)

// Commit represents an immutable Forge version.
//
// A commit points to a manifest and optionally to its parent commit.
// The commit itself will later be stored as a content-addressed object.
type Commit struct {
	Version   int       `json:"version"`
	Parent    string    `json:"parent,omitempty"`
	Manifest  string    `json:"manifest"`
	Timestamp time.Time `json:"timestamp"`
	Author    string    `json:"author"`
	Message   string    `json:"message"`
}

// New creates a new commit.
func New(manifestID, author, message string) (*Commit, error) {
	c := &Commit{
		Version:   CurrentVersion,
		Manifest:  manifestID,
		Timestamp: time.Now().UTC(),
		Author:    author,
		Message:   message,
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}

	return c, nil
}

// NewWithParent creates a new commit with a parent commit.
func NewWithParent(
	parentID,
	manifestID,
	author,
	message string,
) (*Commit, error) {
	c := &Commit{
		Version:   CurrentVersion,
		Parent:    parentID,
		Manifest:  manifestID,
		Timestamp: time.Now().UTC(),
		Author:    author,
		Message:   message,
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}

	return c, nil
}

// Validate verifies that the commit is structurally valid.
func (c *Commit) Validate() error {
	if c == nil {
		return fmt.Errorf("%w: commit is nil", ErrInvalidCommit)
	}

	if c.Version != CurrentVersion {
		return fmt.Errorf(
			"%w: unsupported version %d",
			ErrInvalidCommit,
			c.Version,
		)
	}

	if err := validateObjectID(c.Manifest); err != nil {
		return fmt.Errorf(
			"%w: invalid manifest: %v",
			ErrInvalidCommit,
			err,
		)
	}

	if c.Parent != "" {
		if err := validateObjectID(c.Parent); err != nil {
			return fmt.Errorf(
				"%w: invalid parent: %v",
				ErrInvalidCommit,
				err,
			)
		}
	}

	if c.Timestamp.IsZero() {
		return fmt.Errorf(
			"%w: timestamp is required",
			ErrInvalidCommit,
		)
	}

	if strings.TrimSpace(c.Author) == "" {
		return fmt.Errorf(
			"%w: author is required",
			ErrInvalidCommit,
		)
	}

	if strings.TrimSpace(c.Message) == "" {
		return fmt.Errorf(
			"%w: message is required",
			ErrInvalidCommit,
		)
	}

	return nil
}

// Marshal returns the canonical serialized representation of the commit.
//
// JSON keys are emitted in struct order, and the timestamp is encoded
// using time.Time's deterministic RFC3339 representation.
func (c *Commit) Marshal() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshal commit: %w", err)
	}

	return data, nil
}

// Unmarshal decodes and validates a commit.
func Unmarshal(data []byte) (*Commit, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf(
			"%w: empty data",
			ErrInvalidCommit,
		)
	}

	var c Commit

	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf(
			"%w: decode JSON: %v",
			ErrInvalidCommit,
			err,
		)
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}

	return &c, nil
}

// Equal reports whether two commits contain the same data.
func Equal(a, b *Commit) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.Version == b.Version &&
		a.Parent == b.Parent &&
		a.Manifest == b.Manifest &&
		a.Timestamp.Equal(b.Timestamp) &&
		a.Author == b.Author &&
		a.Message == b.Message
}

func validateObjectID(id string) error {
	const prefix = "blake3:"

	if !strings.HasPrefix(id, prefix) {
		return fmt.Errorf("%w: %q", ErrInvalidID, id)
	}

	digest := strings.TrimPrefix(id, prefix)

	if len(digest) != 64 {
		return fmt.Errorf("%w: %q", ErrInvalidID, id)
	}

	for _, c := range digest {
		if !isLowerHex(c) {
			return fmt.Errorf("%w: %q", ErrInvalidID, id)
		}
	}

	return nil
}

func isLowerHex(c rune) bool {
	return (c >= '0' && c <= '9') ||
		(c >= 'a' && c <= 'f')
}
