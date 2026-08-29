package hash

import (
	"bytes"
	"strings"
	"testing"
)

func TestSumDeterministic(t *testing.T) {
	data := []byte("Hello Forge!")

	first := Sum(data)
	second := Sum(data)

	if first != second {
		t.Fatalf("same data produced different hashes: %q != %q", first, second)
	}
}

func TestSumDifferentData(t *testing.T) {
	first := Sum([]byte("Hello Forge!"))
	second := Sum([]byte("Hello Git!"))

	if first == second {
		t.Fatalf("different data produced the same hash: %q", first)
	}
}

func TestSumFormat(t *testing.T) {
	result := Sum([]byte("Hello Forge!"))

	if !strings.HasPrefix(result, "blake3:") {
		t.Fatalf("hash has wrong format: %q", result)
	}

	digest := strings.TrimPrefix(result, "blake3:")

	if len(digest) != 64 {
		t.Fatalf("expected 64 hex characters, got %d", len(digest))
	}
}

func TestSumReaderMatchesSum(t *testing.T) {
	data := []byte("Hello Forge!")

	fromBytes := Sum(data)

	fromReader, err := SumReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("SumReader failed: %v", err)
	}

	if fromBytes != fromReader {
		t.Fatalf("reader and bytes produced different hashes: %q != %q", fromReader, fromBytes)
	}
}

func TestSumEmptyData(t *testing.T) {
	result := Sum([]byte{})

	if result == "" {
		t.Fatal("empty data produced an empty hash")
	}
}
