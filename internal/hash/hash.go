package hash

import (
	"fmt"
	"io"

	"github.com/zeebo/blake3"
)

// Sum returns the Forge content ID for data.
func Sum(data []byte) string {
	digest := blake3.Sum256(data)
	return formatDigest(digest[:])
}

// SumReader returns the Forge content ID for data read from r.
func SumReader(r io.Reader) (string, error) {
	hasher := blake3.New()

	if _, err := io.Copy(hasher, r); err != nil {
		return "", err
	}

	digest := hasher.Sum(nil)
	return formatDigest(digest), nil
}

func formatDigest(digest []byte) string {
	return fmt.Sprintf("blake3:%x", digest)
}
